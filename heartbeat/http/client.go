package http

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"resty.dev/v3"

	"github.com/mulan-go/mulan/heartbeat/core"
)

// ClientConfig HTTP 客户端配置
type ClientConfig struct {
	ServerURL       string            `json:"server_url" yaml:"server_url"`
	ClientID        string            `json:"client_id" yaml:"client_id"`
	ClientName      string            `json:"client_name" yaml:"client_name"`
	ClientIP        string            `json:"client_ip" yaml:"client_ip"`
	ClientPort      int               `json:"client_port" yaml:"client_port"`
	Metadata        map[string]string `json:"metadata" yaml:"metadata"`
	HeartbeatPeriod time.Duration     `json:"heartbeat_period" yaml:"heartbeat_period"`
	Timeout         time.Duration     `json:"timeout" yaml:"timeout"`
	RetryCount      int               `json:"retry_count" yaml:"retry_count"`
	AutoRegister    bool              `json:"auto_register" yaml:"auto_register"`
}

// DefaultClientConfig 默认客户端配置
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		ServerURL:       "http://localhost:8080",
		HeartbeatPeriod: 10 * time.Second,
		Timeout:         5 * time.Second,
		RetryCount:      3,
		AutoRegister:    true,
	}
}

// Client HTTP 心跳客户端
type Client struct {
	cfg        *ClientConfig
	clientID   string
	serverInfo *core.ServerInfo
	resty      *resty.Client
	log        *zap.Logger

	running   bool
	runningMu sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup

	// 回调函数
	onConnected    func(*core.ServerInfo)
	onDisconnected func(error)
	onHeartbeat    func(*core.HeartbeatRsp)
}

// NewClient 创建 HTTP 客户端
func NewClient(cfg *ClientConfig) *Client {
	if cfg == nil {
		cfg = DefaultClientConfig()
	}
	if cfg.ClientID == "" {
		cfg.ClientID = uuid.New().String()
	}
	client := &Client{
		cfg:      cfg,
		clientID: cfg.ClientID,
		log:      zap.L().Named("heartbeat-client"),
		stopCh:   make(chan struct{}),
		resty: resty.New().
			SetBaseURL(cfg.ServerURL).
			SetTimeout(cfg.Timeout).
			SetRetryCount(cfg.RetryCount),
	}
	return client
}

// SetLogger 设置日志
func (c *Client) SetLogger(log *zap.Logger) *Client {
	c.log = log.Named("heartbeat-client")
	return c
}

// OnConnected 设置连接成功回调
func (c *Client) OnConnected(fn func(*core.ServerInfo)) *Client { c.onConnected = fn; return c }

// OnDisconnected 设置断开连接回调
func (c *Client) OnDisconnected(fn func(error)) *Client { c.onDisconnected = fn; return c }

// OnHeartbeat 设置心跳响应回调
func (c *Client) OnHeartbeat(fn func(*core.HeartbeatRsp)) *Client { c.onHeartbeat = fn; return c }

// ClientID 获取客户端 ID
func (c *Client) ClientID() string { return c.clientID }

// ServerInfo 获取服务端信息
func (c *Client) ServerInfo() *core.ServerInfo { return c.serverInfo }

// Register 注册到服务端
func (c *Client) Register(ctx context.Context) error {
	req := &core.RegisterReq{
		ID:       c.cfg.ClientID,
		Name:     c.cfg.ClientName,
		IP:       c.cfg.ClientIP,
		Port:     c.cfg.ClientPort,
		Metadata: c.cfg.Metadata,
	}
	var rsp core.RegisterRsp
	resp, err := c.resty.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&rsp).
		Post("/heartbeat/register")
	if err != nil {
		c.log.Error("Failed to register",
			zap.Error(err),
		)
		return err
	}
	if resp.IsStatusFailure() {
		c.log.Error("Failed to register",
			zap.Int("status", resp.StatusCode()),
			zap.String("body", resp.String()),
		)
		return &ClientError{StatusCode: resp.StatusCode(), Message: resp.String()}
	}
	c.clientID = rsp.ClientID
	c.serverInfo = rsp.Server
	c.log.Info("Registered to server",
		zap.String("client_id", c.clientID),
		zap.String("server_id", c.serverInfo.ID),
	)
	if c.onConnected != nil {
		c.onConnected(c.serverInfo)
	}
	return nil
}

// Heartbeat 发送心跳
func (c *Client) Heartbeat(ctx context.Context, metadata map[string]string) (*core.HeartbeatRsp, error) {
	req := &core.HeartbeatReq{
		ID:       c.clientID,
		Metadata: metadata,
	}
	var rsp core.HeartbeatRsp
	resp, err := c.resty.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&rsp).
		Post("/heartbeat/heartbeat")
	if err != nil {
		c.log.Error("Failed to heartbeat",
			zap.Error(err),
		)
		return nil, err
	}
	if resp.IsStatusFailure() {
		c.log.Error("Failed to heartbeat",
			zap.Int("status", resp.StatusCode()),
			zap.String("body", resp.String()),
		)
		return nil, &ClientError{StatusCode: resp.StatusCode(), Message: resp.String()}
	}
	if c.onHeartbeat != nil {
		c.onHeartbeat(&rsp)
	}
	return &rsp, nil
}

// Unregister 注销
func (c *Client) Unregister(ctx context.Context) error {
	req := &core.UnregisterReq{ID: c.clientID}
	resp, err := c.resty.R().
		SetContext(ctx).
		SetBody(req).
		Post("/heartbeat/unregister")
	if err != nil {
		c.log.Error("Failed to unregister",
			zap.Error(err),
		)
		return err
	}
	if resp.IsStatusFailure() {
		c.log.Error("Failed to unregister",
			zap.Int("status", resp.StatusCode()), zap.String("body", resp.String()),
		)
		return &ClientError{StatusCode: resp.StatusCode(), Message: resp.String()}
	}
	c.log.Info("Unregistered from server", zap.String("client_id", c.clientID))
	return nil
}

// Start 启动心跳循环
func (c *Client) Start(ctx context.Context) error {
	c.runningMu.Lock()
	if c.running {
		c.runningMu.Unlock()
		return nil
	}
	c.running = true
	c.stopCh = make(chan struct{})
	c.runningMu.Unlock()
	// 自动注册
	if c.cfg.AutoRegister {
		if err := c.Register(ctx); err != nil {
			c.runningMu.Lock()
			c.running = false
			c.runningMu.Unlock()
			return err
		}
	}
	c.wg.Add(1)
	go c.heartbeatLoop()
	c.log.Info("Heartbeat client started",
		zap.String("client_id", c.clientID),
		zap.Duration("period", c.cfg.HeartbeatPeriod),
	)
	return nil
}

// heartbeatLoop 心跳循环
func (c *Client) heartbeatLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.HeartbeatPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
			_, err := c.Heartbeat(ctx, nil)
			cancel()
			if err != nil {
				c.log.Warn("Heartbeat failed", zap.Error(err))
				if c.onDisconnected != nil {
					c.onDisconnected(err)
				}
			}
		case <-c.stopCh:
			return
		}
	}
}

// Stop 停止心跳循环
func (c *Client) Stop(ctx context.Context) error {
	c.runningMu.Lock()
	if !c.running {
		c.runningMu.Unlock()
		return nil
	}
	c.running = false
	close(c.stopCh)
	c.runningMu.Unlock()
	c.wg.Wait()
	// 注销
	if err := c.Unregister(ctx); err != nil {
		c.log.Warn("Failed to unregister",
			zap.Error(err),
		)
	}
	c.log.Info("Heartbeat client stopped", zap.String("client_id", c.clientID))
	return nil
}

// IsRunning 检查是否运行中
func (c *Client) IsRunning() bool {
	c.runningMu.RLock()
	defer c.runningMu.RUnlock()
	return c.running
}

// Close 关闭客户端
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.Stop(ctx)
}

// ClientError 客户端错误
type ClientError struct {
	StatusCode int
	Message    string
}

func (e *ClientError) Error() string { return e.Message }
