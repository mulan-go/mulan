package ws

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/mulan-go/mulan/heartbeat/core"
)

// ServerConfig WebSocket 服务配置
type ServerConfig struct {
	Host              string        `json:"host" yaml:"host"`
	Port              int           `json:"port" yaml:"port"`
	Debug             bool          `json:"debug" yaml:"debug"`
	Path              string        `json:"path" yaml:"path"`
	BroadcastInterval time.Duration `json:"broadcast_interval" yaml:"broadcast_interval"`
}

// DefaultServerConfig 默认配置
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:              "0.0.0.0",
		Port:              8081,
		Debug:             false,
		Path:              "/ws",
		BroadcastInterval: 30 * time.Second,
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境需要限制
	},
}

// Server WebSocket 心跳服务端
type Server struct {
	cfg        *ServerConfig
	serverInfo *core.ServerInfo
	store      core.Store
	hub        *Hub
	engine     *gin.Engine
	httpServer *http.Server
	log        *zap.Logger
	stopCh     chan struct{}
}

// NewServer 创建 WebSocket 服务
func NewServer(cfg *ServerConfig, store core.Store, serverInfo *core.ServerInfo) *Server {
	if cfg == nil {
		cfg = DefaultServerConfig()
	}
	if serverInfo == nil {
		serverInfo = &core.ServerInfo{
			ID:   uuid.New().String(),
			Name: "heartbeat-ws-server",
		}
	}
	if serverInfo.ID == "" {
		serverInfo.ID = uuid.New().String()
	}

	hub := NewHub(store, serverInfo)

	return &Server{
		cfg:        cfg,
		store:      store,
		serverInfo: serverInfo,
		hub:        hub,
		log:        zap.L().Named("heartbeat-ws"),
		stopCh:     make(chan struct{}),
	}
}

// SetLogger 设置日志
func (s *Server) SetLogger(log *zap.Logger) *Server {
	s.log = log.Named("heartbeat-ws")
	s.hub.SetLogger(log)
	return s
}

// Hub 获取 Hub
func (s *Server) Hub() *Hub {
	return s.hub
}

// Build 构建 WebSocket 服务
func (s *Server) Build() *Server {
	if s.cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	s.engine = gin.New()
	s.engine.Use(gin.Recovery())

	// 注册路由
	s.registerRoutes()

	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// registerRoutes 注册路由
func (s *Server) registerRoutes() {
	s.engine.GET(s.cfg.Path, s.handleWebSocket)
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"clients":   s.hub.ClientCount(),
			"timestamp": time.Now(),
		})
	})
}

// RegisterRoutes 注册路由到外部 Gin 引擎
func (s *Server) RegisterRoutes(r gin.IRouter) {
	r.GET(s.cfg.Path, s.handleWebSocket)
}

// handleWebSocket 处理 WebSocket 连接
func (s *Server) handleWebSocket(c *gin.Context) {
	// 升级连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.log.Error("Failed to upgrade connection", zap.Error(err))
		return
	}

	// 获取或生成客户端 ID
	clientID := c.Query("client_id")
	if clientID == "" {
		clientID = uuid.New().String()
	}

	// 获取客户端名称
	clientName := c.Query("client_name")
	if clientName == "" {
		clientName = "ws-client"
	}

	// 注册到存储
	if s.store != nil {
		client := &core.ClientInfo{
			ID:       clientID,
			Name:     clientName,
			IP:       c.ClientIP(),
			Metadata: make(map[string]string),
		}
		if err := s.store.Register(c.Request.Context(), client); err != nil {
			s.log.Warn("Failed to register client to store", zap.Error(err))
		}
	}

	// 创建连接包装
	wsConn := NewConn(clientID, conn, s.hub)

	// 注册到 Hub
	s.hub.register <- wsConn

	// 发送欢迎消息
	welcomeMsg := &core.WSMessage{
		Type: core.MsgTypeRegister,
		Payload: &core.RegisterRsp{
			ClientID: clientID,
			Server:   s.serverInfo,
		},
	}
	wsConn.Send(welcomeMsg)

	// 启动读写协程
	go wsConn.writePump()
	wsConn.readPump()

	// 连接关闭后从存储中注销
	if s.store != nil {
		_ = s.store.Unregister(context.Background(), clientID)
	}
}

// Serve 启动服务
func (s *Server) Serve() error {
	if s.engine == nil {
		s.Build()
	}

	// 启动 Hub
	s.hub.Run()

	// 启动广播协程
	if s.cfg.BroadcastInterval > 0 {
		go s.broadcastLoop()
	}

	s.log.Info("WebSocket Heartbeat Server starting",
		zap.String("host", s.cfg.Host),
		zap.Int("port", s.cfg.Port),
		zap.String("path", s.cfg.Path),
	)

	errCh := make(chan error, 1)
	go func() {
		err := s.httpServer.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err
	case <-time.After(3 * time.Second):
		s.log.Info("WebSocket Heartbeat Server listening",
			zap.String("addr", s.httpServer.Addr),
		)
		return nil
	}
}

// broadcastLoop 定时广播
func (s *Server) broadcastLoop() {
	ticker := time.NewTicker(s.cfg.BroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.hub.BroadcastServerInfo()
		case <-s.stopCh:
			return
		}
	}
}

// Shutdown 优雅关闭
func (s *Server) Shutdown(ctx context.Context) error {
	close(s.stopCh)
	s.hub.Stop()
	return s.httpServer.Shutdown(ctx)
}

// Close 关闭服务
func (s *Server) Close() error {
	close(s.stopCh)
	s.hub.Stop()
	return s.httpServer.Close()
}

// Broadcast 广播消息
func (s *Server) Broadcast(msg *core.WSMessage) {
	s.hub.Broadcast(msg)
}

// SendTo 发送给指定客户端
func (s *Server) SendTo(clientID string, msg *core.WSMessage) bool {
	return s.hub.SendTo(clientID, msg)
}

// ClientCount 获取连接数
func (s *Server) ClientCount() int {
	return s.hub.ClientCount()
}
