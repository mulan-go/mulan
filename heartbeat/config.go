package heartbeat

import (
	"time"

	"github.com/mulan-go/mulan/heartbeat/core"
	httpx "github.com/mulan-go/mulan/heartbeat/http"
	"github.com/mulan-go/mulan/heartbeat/ws"
	"github.com/spf13/pflag"
)

// Config 心跳服务完整配置
type Config struct {
	// Server 服务端配置
	Server ServerConfig `json:"server" yaml:"server"`
	// Client 客户端配置
	Client ClientConfig `json:"client" yaml:"client"`
	// Store 存储配置
	Store StoreConfig `json:"store" yaml:"store"`
}

// ServerConfig 服务端配置
type ServerConfig struct {
	// ServerID 服务端 ID
	ServerID string `json:"server_id" yaml:"server_id"`
	// ServerName 服务端名称
	ServerName string `json:"server_name" yaml:"server_name"`
	// HTTP Handler 配置
	HTTP httpx.HandlerConfig `json:"http" yaml:"http"`
	// WebSocket 配置
	WS ws.ServerConfig `json:"ws" yaml:"ws"`
}

// ClientConfig 客户端配置
type ClientConfig struct {
	// Mode 模式: http, ws
	Mode string `json:"mode" yaml:"mode"`
	// HTTP 客户端配置
	HTTP httpx.ClientConfig `json:"http" yaml:"http"`
	// WebSocket 客户端配置
	WS ws.ClientConfig `json:"ws" yaml:"ws"`
}

// StoreConfig 存储配置
type StoreConfig struct {
	// Type 存储类型: memory, redis
	Type string `json:"type" yaml:"type"`
	// KeyPrefix Redis key 前缀
	KeyPrefix string `json:"key_prefix" yaml:"key_prefix"`
	// Timeout 超时时间
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	// CleanupInterval 清理间隔
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`
	// Redis 配置
	Redis RedisConfig `json:"redis" yaml:"redis"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Password string `json:"password" yaml:"password"`
	DB       int    `json:"db" yaml:"db"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			ServerID:   "",
			ServerName: "heartbeat-server",
			HTTP: httpx.HandlerConfig{
				HeartbeatPeriod: 10 * time.Second,
			},
			WS: ws.ServerConfig{
				Host:              "0.0.0.0",
				Port:              8081,
				Debug:             false,
				Path:              "/ws",
				BroadcastInterval: 30 * time.Second,
			},
		},
		Client: ClientConfig{
			Mode: "http",
			HTTP: httpx.ClientConfig{
				ServerURL:       "http://localhost:8080",
				HeartbeatPeriod: 10 * time.Second,
				Timeout:         5 * time.Second,
				RetryCount:      3,
				AutoRegister:    true,
			},
			WS: ws.ClientConfig{
				ServerURL:       "ws://localhost:8081",
				Path:            "/ws",
				HeartbeatPeriod: 10 * time.Second,
				ReconnectDelay:  5 * time.Second,
				MaxReconnects:   -1,
			},
		},
		Store: StoreConfig{
			Type:            "memory",
			KeyPrefix:       "heartbeat:",
			Timeout:         30 * time.Second,
			CleanupInterval: 10 * time.Second,
			Redis: RedisConfig{
				Host: "127.0.0.1",
				Port: 6379,
				DB:   0,
			},
		},
	}
}

// ServerFlagSet 服务端命令行参数
func ServerFlagSet(name string) *pflag.FlagSet {
	fs := pflag.NewFlagSet(name, pflag.ContinueOnError)
	// HTTP Handler 配置
	fs.Duration(name+".http.heartbeat-period", 10*time.Second, "Heartbeat period")
	// WebSocket 服务配置
	fs.String(name+".ws.host", "0.0.0.0", "WebSocket server host")
	fs.Int(name+".ws.port", 8081, "WebSocket server port")
	fs.Bool(name+".ws.debug", false, "WebSocket debug mode")
	fs.String(name+".ws.path", "/ws", "WebSocket path")
	fs.Duration(name+".ws.broadcast-interval", 30*time.Second, "Broadcast interval")
	// 存储配置
	fs.String(name+".store.type", "memory", "Store type (memory, redis)")
	fs.String(name+".store.key-prefix", "heartbeat:", "Redis key prefix")
	fs.Duration(name+".store.timeout", 30*time.Second, "Client timeout")
	fs.Duration(name+".store.cleanup-interval", 10*time.Second, "Cleanup interval")
	// Redis 配置
	fs.String(name+".store.redis.host", "127.0.0.1", "Redis host")
	fs.Int(name+".store.redis.port", 6379, "Redis port")
	fs.String(name+".store.redis.password", "", "Redis password")
	fs.Int(name+".store.redis.db", 0, "Redis database")
	return fs
}

// ClientFlagSet 客户端命令行参数
func ClientFlagSet(name string) *pflag.FlagSet {
	fs := pflag.NewFlagSet(name, pflag.ContinueOnError)
	// 通用配置
	fs.String(name+".mode", "http", "Client mode (http, ws)")
	fs.String(name+".id", "", "Client ID")
	fs.String(name+".name", "", "Client name")
	// HTTP 客户端配置
	fs.String(name+".http.server-url", "http://localhost:8080", "HTTP server URL")
	fs.Duration(name+".http.heartbeat-period", 10*time.Second, "Heartbeat period")
	fs.Duration(name+".http.timeout", 5*time.Second, "Request timeout")
	fs.Int(name+".http.retry-count", 3, "Retry count")
	fs.Bool(name+".http.auto-register", true, "Auto register on start")
	// WebSocket 客户端配置
	fs.String(name+".ws.server-url", "ws://localhost:8081", "WebSocket server URL")
	fs.String(name+".ws.path", "/ws", "WebSocket path")
	fs.Duration(name+".ws.heartbeat-period", 10*time.Second, "Heartbeat period")
	fs.Duration(name+".ws.reconnect-delay", 5*time.Second, "Reconnect delay")
	fs.Int(name+".ws.max-reconnects", -1, "Max reconnect attempts (-1 for infinite)")
	return fs
}

// ToStoreConfig 转换为存储配置
func (c *StoreConfig) ToStoreConfig() *core.StoreConfig {
	return &core.StoreConfig{
		KeyPrefix:       c.KeyPrefix,
		DefaultTimeout:  c.Timeout,
		CleanupInterval: c.CleanupInterval,
		AutoCleanup:     true,
	}
}

// ToServerInfo 转换为服务端信息
func (c *ServerConfig) ToServerInfo() *core.ServerInfo {
	return &core.ServerInfo{
		ID: c.ServerID, Name: c.ServerName,
	}
}

// ToHandlerConfig 转换为 HTTP Handler 配置
func (c *ServerConfig) ToHandlerConfig() *httpx.HandlerConfig {
	return &c.HTTP
}
