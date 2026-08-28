package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/mulan-go/mulan/heartbeat/core"
)

// Conn WebSocket 连接包装
type Conn struct {
	ID       string          // 客户端 ID
	conn     *websocket.Conn // WebSocket 连接
	hub      *Hub            // 所属 Hub
	sendCh   chan []byte     // 发送通道
	closeCh  chan struct{}   // 关闭通道
	closed   bool
	closeMu  sync.Mutex
	lastPing time.Time
}

// NewConn 创建连接
func NewConn(id string, conn *websocket.Conn, hub *Hub) *Conn {
	return &Conn{
		ID:       id,
		conn:     conn,
		hub:      hub,
		sendCh:   make(chan []byte, 256),
		closeCh:  make(chan struct{}),
		lastPing: time.Now(),
	}
}

// Send 发送消息
func (c *Conn) Send(msg *core.WSMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return nil
	}
	c.closeMu.Unlock()

	select {
	case c.sendCh <- data:
		return nil
	default:
		return nil // 缓冲区满则丢弃
	}
}

// Close 关闭连接
func (c *Conn) Close() {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return
	}
	c.closed = true
	close(c.closeCh)
	c.closeMu.Unlock()
	if c.conn != nil {
		c.conn.Close()
	}
}

// writePump 写入协程
func (c *Conn) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.sendCh:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.closeCh:
			return
		}
	}
}

// readPump 读取协程
func (c *Conn) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.Close()
	}()

	c.conn.SetReadLimit(65536)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.lastPing = time.Now()
		return nil
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.log.Warn("WebSocket read error", zap.Error(err))
			}
			break
		}
		c.hub.handleMessage(c, msg)
	}
}

// Hub WebSocket 连接管理器
type Hub struct {
	clients    map[string]*Conn // clientID -> Conn
	clientsMu  sync.RWMutex
	register   chan *Conn           // 注册通道
	unregister chan *Conn           // 注销通道
	broadcast  chan *core.WSMessage // 广播通道
	store      core.Store
	serverInfo *core.ServerInfo
	log        *zap.Logger
	stopCh     chan struct{}
	wg         sync.WaitGroup

	// 回调
	onClientJoin  func(clientID string)
	onClientLeave func(clientID string)
}

// NewHub 创建 Hub
func NewHub(store core.Store, serverInfo *core.ServerInfo) *Hub {
	return &Hub{
		clients:    make(map[string]*Conn),
		register:   make(chan *Conn),
		unregister: make(chan *Conn),
		broadcast:  make(chan *core.WSMessage, 256),
		store:      store,
		serverInfo: serverInfo,
		log:        zap.L().Named("ws-hub"),
		stopCh:     make(chan struct{}),
	}
}

// SetLogger 设置日志
func (h *Hub) SetLogger(log *zap.Logger) *Hub {
	h.log = log.Named("ws-hub")
	return h
}

// OnClientJoin 设置客户端加入回调
func (h *Hub) OnClientJoin(fn func(clientID string)) *Hub {
	h.onClientJoin = fn
	return h
}

// OnClientLeave 设置客户端离开回调
func (h *Hub) OnClientLeave(fn func(clientID string)) *Hub {
	h.onClientLeave = fn
	return h
}

// Run 运行 Hub
func (h *Hub) Run() {
	h.wg.Add(1)
	go h.runLoop()
}

// runLoop 主循环
func (h *Hub) runLoop() {
	defer h.wg.Done()

	for {
		select {
		case conn := <-h.register:
			h.clientsMu.Lock()
			h.clients[conn.ID] = conn
			h.clientsMu.Unlock()

			h.log.Info("WebSocket client connected", zap.String("client_id", conn.ID))
			if h.onClientJoin != nil {
				h.onClientJoin(conn.ID)
			}

		case conn := <-h.unregister:
			h.clientsMu.Lock()
			if _, ok := h.clients[conn.ID]; ok {
				delete(h.clients, conn.ID)
				close(conn.sendCh)
			}
			h.clientsMu.Unlock()

			h.log.Info("WebSocket client disconnected", zap.String("client_id", conn.ID))
			if h.onClientLeave != nil {
				h.onClientLeave(conn.ID)
			}

		case msg := <-h.broadcast:
			h.broadcastToAll(msg)

		case <-h.stopCh:
			h.closeAllClients()
			return
		}
	}
}

// handleMessage 处理收到的消息
func (h *Hub) handleMessage(conn *Conn, data []byte) {
	var msg core.WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		h.log.Warn("Failed to parse message", zap.Error(err))
		return
	}

	switch msg.Type {
	case core.MsgTypeHeartbeat:
		h.handleHeartbeat(conn, msg.Payload)
	case core.MsgTypePing:
		h.handlePing(conn)
	default:
		h.log.Debug("Unknown message type", zap.String("type", msg.Type))
	}
}

// handleHeartbeat 处理心跳消息
func (h *Hub) handleHeartbeat(conn *Conn, payload interface{}) {
	var metadata map[string]string
	if m, ok := payload.(map[string]interface{}); ok {
		metadata = make(map[string]string)
		for k, v := range m {
			if s, ok := v.(string); ok {
				metadata[k] = s
			}
		}
	}

	if h.store != nil {
		_ = h.store.Heartbeat(context.Background(), conn.ID, metadata)
	}

	// 回复 pong
	rsp := &core.WSMessage{
		Type: core.MsgTypePong,
		Payload: &core.HeartbeatRsp{
			OK:        true,
			Server:    h.serverInfo,
			Timestamp: time.Now(),
		},
	}
	conn.Send(rsp)
}

// handlePing 处理 ping 消息
func (h *Hub) handlePing(conn *Conn) {
	rsp := &core.WSMessage{
		Type: core.MsgTypePong,
		Payload: map[string]interface{}{
			"timestamp": time.Now(),
		},
	}
	conn.Send(rsp)
}

// broadcastToAll 广播到所有客户端
func (h *Hub) broadcastToAll(msg *core.WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	for _, conn := range h.clients {
		select {
		case conn.sendCh <- data:
		default:
			// 缓冲区满，跳过
		}
	}
}

// Broadcast 广播消息
func (h *Hub) Broadcast(msg *core.WSMessage) {
	select {
	case h.broadcast <- msg:
	default:
	}
}

// BroadcastServerInfo 广播服务端信息
func (h *Hub) BroadcastServerInfo() {
	h.Broadcast(&core.WSMessage{
		Type: core.MsgTypeBroadcast,
		Payload: map[string]interface{}{
			"server":    h.serverInfo,
			"timestamp": time.Now(),
		},
	})
}

// BroadcastClients 广播客户端列表
func (h *Hub) BroadcastClients() {
	if h.store == nil {
		return
	}

	clients, err := h.store.ListOnline(context.Background())
	if err != nil {
		return
	}

	h.Broadcast(&core.WSMessage{
		Type: core.MsgTypeBroadcast,
		Payload: &core.ClientListRsp{
			Total:   len(clients),
			Clients: clients,
		},
	})
}

// SendTo 发送给指定客户端
func (h *Hub) SendTo(clientID string, msg *core.WSMessage) bool {
	h.clientsMu.RLock()
	conn, ok := h.clients[clientID]
	h.clientsMu.RUnlock()

	if !ok {
		return false
	}

	return conn.Send(msg) == nil
}

// ClientCount 获取连接数
func (h *Hub) ClientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

// ClientIDs 获取所有客户端 ID
func (h *Hub) ClientIDs() []string {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	ids := make([]string, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	return ids
}

// closeAllClients 关闭所有客户端
func (h *Hub) closeAllClients() {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	for _, conn := range h.clients {
		conn.Close()
	}
	h.clients = make(map[string]*Conn)
}

// Stop 停止 Hub
func (h *Hub) Stop() {
	close(h.stopCh)
	h.wg.Wait()
}
