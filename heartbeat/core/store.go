package core

import (
	"context"
	"errors"
	"time"
)

// 存储相关错误
var (
	ErrClientNotFound = errors.New("client not found")
	ErrClientExists   = errors.New("client already exists")
	ErrStoreClosed    = errors.New("store is closed")
)

// Store 存储接口
type Store interface {
	// Register 注册客户端
	Register(ctx context.Context, client *ClientInfo) error

	// Unregister 注销客户端
	Unregister(ctx context.Context, clientID string) error

	// Heartbeat 更新心跳
	Heartbeat(ctx context.Context, clientID string, metadata map[string]string) error

	// Get 获取单个客户端信息
	Get(ctx context.Context, clientID string) (*ClientInfo, error)

	// List 获取所有客户端列表
	List(ctx context.Context) ([]*ClientInfo, error)

	// ListOnline 获取在线客户端列表
	ListOnline(ctx context.Context) ([]*ClientInfo, error)

	// Count 获取客户端总数
	Count(ctx context.Context) (int, error)

	// CountOnline 获取在线客户端数
	CountOnline(ctx context.Context) (int, error)

	// Cleanup 清理超时客户端
	Cleanup(ctx context.Context, timeout time.Duration) (int, error)

	// Close 关闭存储
	Close() error
}

// StoreOption 存储配置选项
type StoreOption func(*StoreConfig)

// StoreConfig 存储配置
type StoreConfig struct {
	// KeyPrefix Redis key 前缀
	KeyPrefix string

	// DefaultTimeout 默认超时时间
	DefaultTimeout time.Duration

	// CleanupInterval 清理间隔
	CleanupInterval time.Duration

	// AutoCleanup 是否自动清理
	AutoCleanup bool
}

// DefaultStoreConfig 默认存储配置
func DefaultStoreConfig() *StoreConfig {
	return &StoreConfig{
		KeyPrefix:       "heartbeat:",
		DefaultTimeout:  30 * time.Second,
		CleanupInterval: 10 * time.Second,
		AutoCleanup:     true,
	}
}

// WithKeyPrefix 设置 key 前缀
func WithKeyPrefix(prefix string) StoreOption {
	return func(c *StoreConfig) {
		c.KeyPrefix = prefix
	}
}

// WithDefaultTimeout 设置默认超时
func WithDefaultTimeout(timeout time.Duration) StoreOption {
	return func(c *StoreConfig) {
		c.DefaultTimeout = timeout
	}
}

// WithCleanupInterval 设置清理间隔
func WithCleanupInterval(interval time.Duration) StoreOption {
	return func(c *StoreConfig) {
		c.CleanupInterval = interval
	}
}

// WithAutoCleanup 设置是否自动清理
func WithAutoCleanup(auto bool) StoreOption {
	return func(c *StoreConfig) {
		c.AutoCleanup = auto
	}
}

// ApplyOptions 应用配置选项
func ApplyOptions(cfg *StoreConfig, opts ...StoreOption) {
	for _, opt := range opts {
		opt(cfg)
	}
}
