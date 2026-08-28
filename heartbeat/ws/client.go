package ws

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/mulan-go/mulan/heartbeat/core"
)

// ClientConfig WebSocket 客户端配置
type ClientConfig struct {
	ServerURL       string            `json:"server_url" yaml:"server_url"`
	Path            string            `json:"path" yaml:"path"`
	ClientID        string            `json:"client_id" yaml:"client_id"`
	ClientName      string            `json:"client_name" yaml:"client_name"`
	Metadata        map[string]string `json:"metadata" yaml:"metadata"`
	HeartbeatPeriod time.Duration     `json:"heartbeat_period" yaml:"heartbeat_period"`
	ReconnectDelay  time.Duration     `json:"reconnect_delay" yaml:"reconnect_delay"`
	MaxReconnects   int               `json:"max_reconnects" yaml:"max_reconnects"`
}

// DefaultClientConfig 默认配置
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		ServerURL:       "ws://localhost:8081",
		Path:            "/ws",
		HeartbeatPeriod: 10 * time.Second,
		ReconnectDelay:  5 * time.Second,
		MaxReconnects:   -1, // -1 表示无限重连
	}
}

// Client WebSocket 客户端
type Client struct {
	cfg        *ClientConfig
	clientID   string
	serverInfo *core.ServerInfo
	conn       *websocket.Conn
	log        *zap.Logger

	running     bool
	runningMu   sync.RWMutex
	connected   bool
	connectedMu sync.RWMutex
	reconnects  int
	stopCh      chan struct{}
	sendCh      chan *core.WSMessage
	wg          sync.WaitGroup

	// 回调函数
	onConnected    func(*core.ServerInfo)
	onDisconnected func(error)
	onMessage      func(*core.WSMessage)
	onBroadcast    func(*core.WSMessage)
}

// NewClient 创建 WebSocket 客户端
func NewClient(cfg *ClientConfig) *Client {
	if cfg == nil {
		cfg = DefaultClientConfig()
	}
	if cfg.ClientID == "" {
		cfg.ClientID = uuid.New().String()
	}

	return &Client{
		cfg:      cfg,
		clientID: cfg.ClientID,
		log:      zap.L().Named("ws-client"),
		sendCh:   make(chan *core.WSMessage, 256),
	}
}

// SetLogger 设置日志
func (c *Client) SetLogger(log *zap.Logger) *Client {
	c.log = log.Named("ws-client")
	return c
}

// OnConnected 设置连接成功回调
func (c *Client) OnConnected(fn func(*core.ServerInfo)) *Client {
	c.onConnected = fn
	return c
}

// OnDisconnected 设置断开连接回调
func (c *Client) OnDisconnected(fn func(error)) *Client {
	c.onDisconnected = fn
	return c
}

// OnMessage 设置消息回调
func (c *Client) OnMessage(fn func(*core.WSMessage)) *Client {
	c.onMessage = fn
	return c
}

// OnBroadcast 设置广播消息回调
func (c *Client) OnBroadcast(fn func(*core.WSMessage)) *Client {
	c.onBroadcast = fn
	return c
}

// ClientID 获取客户端 ID
func (c *Client) ClientID() string {
	return c.clientID
}

// ServerInfo 获取服务端信息
func (c *Client) ServerInfo() *core.ServerInfo {
	return c.serverInfo
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.connectedMu.RLock()
	defer c.connectedMu.RUnlock()
	return c.connected
}

// Connect 连接到服务器
func (c *Client) Connect(ctx context.Context) error {
	u, err := url.Parse(c.cfg.ServerURL)
	if err != nil {
		return err
	}

	u.Path = c.cfg.Path
	q := u.Query()
	q.Set("client_id", c.clientID)
	if c.cfg.ClientName != "" {
		q.Set("client_name", c.cfg.ClientName)
	}
	u.RawQuery = q.Encode()

	c.log.Debug("Connecting to WebSocket server", zap.String("url", u.String()))

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return err
	}

	c.conn = conn
	c.setConnected(true)

	c.log.Info("Connected to WebSocket server",
		zap.String("client_id", c.clientID),
		zap.String("server", c.cfg.ServerURL),
	)

	return nil
}

// setConnected 设置连接状态
func (c *Client) setConnected(connected bool) {
	c.connectedMu.Lock()
	c.connected = connected
	c.connectedMu.Unlock()
}

// Start 启动客户端
func (c *Client) Start(ctx context.Context) error {
	c.runningMu.Lock()
	if c.running {
		c.runningMu.Unlock()
		return nil
	}
	c.running = true
	c.stopCh = make(chan struct{})
	c.runningMu.Unlock()

	// 首次连接
	if err := c.Connect(ctx); err != nil {
		c.runningMu.Lock()
		c.running = false
		c.runningMu.Unlock()
		return err
	}

	// 启动读写协程
	c.wg.Add(3)
	go c.readLoop()
	go c.writeLoop()
	go c.heartbeatLoop()

	return nil
}

// readLoop 读取循环
func (c *Client) readLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		if !c.IsConnected() {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.log.Warn("WebSocket read error", zap.Error(err))
			c.setConnected(false)

			if c.onDisconnected != nil {
				c.onDisconnected(err)
			}

			// 尝试重连
			if c.shouldReconnect() {
				c.reconnect()
			}
			continue
		}

		var msg core.WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.log.Warn("Failed to parse message", zap.Error(err))
			continue
		}

		c.handleMessage(&msg)
	}
}

// writeLoop 写入循环
func (c *Client) writeLoop() {
	defer c.wg.Done()

	for {
		select {
		case msg := <-c.sendCh:
			if !c.IsConnected() {
				continue
			}
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.log.Warn("WebSocket write error", zap.Error(err))
			}
		case <-c.stopCh:
			return
		}
	}
}

// heartbeatLoop 心跳循环
func (c *Client) heartbeatLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.cfg.HeartbeatPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.IsConnected() {
				c.SendHeartbeat(nil)
			}
		case <-c.stopCh:
			return
		}
	}
}

// handleMessage 处理消息
func (c *Client) handleMessage(msg *core.WSMessage) {
	switch msg.Type {
	case core.MsgTypeRegister:
		// 处理注册响应
		if payload, ok := msg.Payload.(map[string]interface{}); ok {
			if serverData, ok := payload["server"].(map[string]interface{}); ok {
				c.serverInfo = &core.ServerInfo{
					ID:       getString(serverData, "id"),
					Name:     getString(serverData, "name"),
					Hostname: getString(serverData, "hostname"),
				}
				if c.onConnected != nil {
					c.onConnected(c.serverInfo)
				}
			}
		}
	case core.MsgTypePong:
		// 心跳响应
		c.log.Debug("Received pong")
	case core.MsgTypeBroadcast:
		// 广播消息
		if c.onBroadcast != nil {
			c.onBroadcast(msg)
		}
	default:
		if c.onMessage != nil {
			c.onMessage(msg)
		}
	}
}

// shouldReconnect 是否应该重连
func (c *Client) shouldReconnect() bool {
	c.runningMu.RLock()
	running := c.running
	c.runningMu.RUnlock()

	if !running {
		return false
	}

	if c.cfg.MaxReconnects < 0 {
		return true // 无限重连
	}

	return c.reconnects < c.cfg.MaxReconnects
}

// reconnect 重连
func (c *Client) reconnect() {
	c.reconnects++
	c.log.Info("Attempting to reconnect",
		zap.Int("attempt", c.reconnects),
		zap.Duration("delay", c.cfg.ReconnectDelay),
	)

	time.Sleep(c.cfg.ReconnectDelay)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		c.log.Warn("Reconnect failed", zap.Error(err))
	} else {
		c.reconnects = 0 // 重置计数
	}
}

// Send 发送消息
func (c *Client) Send(msg *core.WSMessage) {
	select {
	case c.sendCh <- msg:
	default:
		c.log.Warn("Send buffer full, dropping message")
	}
}

// SendHeartbeat 发送心跳
func (c *Client) SendHeartbeat(metadata map[string]string) {
	c.Send(&core.WSMessage{
		Type: core.MsgTypeHeartbeat,
		Payload: &core.HeartbeatReq{
			ID:       c.clientID,
			Metadata: metadata,
		},
	})
}

// Stop 停止客户端
func (c *Client) Stop() error {
	c.runningMu.Lock()
	if !c.running {
		c.runningMu.Unlock()
		return nil
	}
	c.running = false
	close(c.stopCh)
	c.runningMu.Unlock()

	// 发送断开消息
	if c.IsConnected() {
		c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.conn.Close()
	}

	c.wg.Wait()
	c.log.Info("WebSocket client stopped", zap.String("client_id", c.clientID))
	return nil
}

// Close 关闭客户端
func (c *Client) Close() error {
	return c.Stop()
}

// IsRunning 检查是否运行中
func (c *Client) IsRunning() bool {
	c.runningMu.RLock()
	defer c.runningMu.RUnlock()
	return c.running
}

// getString 从 map 中获取字符串
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
