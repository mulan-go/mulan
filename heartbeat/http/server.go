package http

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/mulan-go/mulan/req"
	"github.com/mulan-go/mulan/rsp"

	"github.com/mulan-go/mulan/heartbeat/core"
)

// HandlerConfig Handler 配置
type HandlerConfig struct {
	// HeartbeatPeriod 建议的心跳间隔（返回给客户端）
	HeartbeatPeriod time.Duration `json:"heartbeat_period" yaml:"heartbeat_period"`
}

// DefaultHandlerConfig 默认配置
func DefaultHandlerConfig() *HandlerConfig {
	return &HandlerConfig{
		HeartbeatPeriod: 10 * time.Second,
	}
}

// Handler 心跳 HTTP Handler
type Handler struct {
	cfg        *HandlerConfig
	serverInfo *core.ServerInfo
	store      core.Store
	log        *zap.Logger
}

// NewHandler 创建 Handler
func NewHandler(store core.Store, serverInfo *core.ServerInfo, cfg *HandlerConfig) *Handler {
	if cfg == nil {
		cfg = DefaultHandlerConfig()
	}
	if serverInfo == nil {
		serverInfo = &core.ServerInfo{
			ID:   uuid.New().String(),
			Name: "heartbeat-server",
		}
	}
	if serverInfo.ID == "" {
		serverInfo.ID = uuid.New().String()
	}
	return &Handler{
		cfg:        cfg,
		store:      store,
		serverInfo: serverInfo,
		log:        zap.L().Named("heartbeat"),
	}
}

// SetLogger 设置日志
func (h *Handler) SetLogger(log *zap.Logger) *Handler {
	h.log = log.Named("heartbeat")
	return h
}

// ServerInfo 获取服务端信息
func (h *Handler) ServerInfo() *core.ServerInfo { return h.serverInfo }

// Store 获取存储实例
func (h *Handler) Store() core.Store { return h.store }

// RegisterRoutesWithPrefix 注册路由到指定前缀
// 所有接口统一使用 POST 方法
func (h *Handler) RegisterRoutesWithPrefix(r gin.IRouter, prefix string) {
	api := r.Group(prefix)
	{
		api.POST("/register", h.Register())
		api.POST("/heartbeat", h.Heartbeat())
		api.POST("/unregister", h.Unregister())
		api.POST("/clients", h.ListClients())
		api.POST("/server", h.ServerInfoHandler())
		api.POST("/health", h.Health())
	}
}

// Register 返回注册 Handler
func (h *Handler) Register() gin.HandlerFunc {
	return func(c *gin.Context) {
		var r core.RegisterReq
		if err := req.Bind(c, &r); err != nil {
			c.AbortWithStatusJSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
			return
		}
		// 自动生成 ID
		if r.ID == "" {
			r.ID = uuid.New().String()
		}
		// 自动获取 IP
		if r.IP == "" {
			r.IP = c.ClientIP()
		}
		client := &core.ClientInfo{
			ID:       r.ID,
			Name:     r.Name,
			IP:       r.IP,
			Port:     r.Port,
			Metadata: r.Metadata,
		}
		if err := h.store.Register(c.Request.Context(), client); err != nil {
			h.log.Error("Failed to register client",
				zap.String("client_id", r.ID),
				zap.Error(err),
			)
			c.AbortWithStatusJSON(500, rsp.C(err))
			return
		}
		h.log.Info("Client registered",
			zap.String("client_id", client.ID),
			zap.String("client_name", client.Name),
			zap.String("client_ip", client.IP),
		)
		c.JSON(200, rsp.S(&core.RegisterRsp{
			ClientID: client.ID,
			Server:   h.serverInfo,
		}))
	}
}

// Heartbeat 返回心跳 Handler
func (h *Handler) Heartbeat() gin.HandlerFunc {
	return func(c *gin.Context) {
		var r core.HeartbeatReq
		if err := req.Bind(c, &r); err != nil {
			c.AbortWithStatusJSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
			return
		}
		if r.ID == "" {
			c.AbortWithStatusJSON(400, rsp.E(rsp.ErrParamInvalid.WithMsg("client id is required")))
			return
		}
		if err := h.store.Heartbeat(c.Request.Context(), r.ID, r.Metadata); err != nil {
			if errors.Is(err, core.ErrClientNotFound) {
				c.AbortWithStatusJSON(404, rsp.E(rsp.ErrNotFound.WithMsg("client not found, please register first")))
				return
			}
			h.log.Error("Failed to update heartbeat",
				zap.String("client_id", r.ID),
				zap.Error(err),
			)
			c.AbortWithStatusJSON(500, rsp.C(err))
			return
		}
		count, _ := h.store.CountOnline(c.Request.Context())
		c.JSON(200, rsp.S(&core.HeartbeatRsp{
			OK:        true,
			Server:    h.serverInfo,
			Timestamp: time.Now(),
			Clients:   count,
			Interval:  int(h.cfg.HeartbeatPeriod.Seconds()),
		}))
	}
}

// Unregister 返回注销 Handler
func (h *Handler) Unregister() gin.HandlerFunc {
	return func(c *gin.Context) {
		var r core.UnregisterReq
		if err := req.Bind(c, &r); err != nil {
			c.AbortWithStatusJSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
			return
		}
		if r.ID == "" {
			c.AbortWithStatusJSON(400, rsp.E(rsp.ErrParamInvalid.WithMsg("client id is required")))
			return
		}
		if err := h.store.Unregister(c.Request.Context(), r.ID); err != nil {
			if errors.Is(err, core.ErrClientNotFound) {
				c.AbortWithStatusJSON(404, rsp.E(rsp.ErrNotFound.WithMsg("client not found")))
				return
			}
			h.log.Error("Failed to unregister client",
				zap.String("client_id", r.ID),
				zap.Error(err),
			)
			c.AbortWithStatusJSON(500, rsp.C(err))
			return
		}
		h.log.Info("Client unregistered", zap.String("client_id", r.ID))
		c.JSON(200, rsp.OK())
	}
}

// ListClientsReq 客户端列表请求
type ListClientsReq struct {
	OnlyOnline bool `json:"only_online"` // 是否只返回在线客户端
}

// ListClients 返回客户端列表 Handler
func (h *Handler) ListClients() gin.HandlerFunc {
	return func(c *gin.Context) {
		var r ListClientsReq
		if err := req.ShouldBind(c, &r); err != nil {
			c.AbortWithStatusJSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
			return
		}
		var clients []*core.ClientInfo
		var err error
		if r.OnlyOnline {
			clients, err = h.store.ListOnline(c.Request.Context())
		} else {
			clients, err = h.store.List(c.Request.Context())
		}
		if err != nil {
			h.log.Error("Failed to list clients", zap.Error(err))
			c.AbortWithStatusJSON(500, rsp.C(err))
			return
		}
		c.JSON(200, rsp.S(&core.ClientListRsp{
			Total:   len(clients),
			Clients: clients,
		}))
	}
}

// ServerInfoRsp 服务端信息响应
type ServerInfoRsp struct {
	Server        *core.ServerInfo `json:"server"`
	OnlineClients int              `json:"online_clients"`
	Timestamp     time.Time        `json:"timestamp"`
}

// ServerInfoHandler 返回服务端信息 Handler
func (h *Handler) ServerInfoHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		count, _ := h.store.CountOnline(c.Request.Context())
		c.JSON(200, rsp.S(&ServerInfoRsp{
			Server:        h.serverInfo,
			OnlineClients: count,
			Timestamp:     time.Now(),
		}))
	}
}

// HealthRsp 健康检查响应
type HealthRsp struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// Health 返回健康检查 Handler
func (h *Handler) Health() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, rsp.S(&HealthRsp{
			Status:    "ok",
			Timestamp: time.Now(),
		}))
	}
}
