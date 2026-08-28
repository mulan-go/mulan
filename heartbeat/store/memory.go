package store

import (
	"context"
	"sync"
	"time"

	"github.com/mulan-go/mulan/heartbeat/core"
)

var _ core.Store = (*MemoryStore)(nil)

// MemoryStore 内存存储实现
type MemoryStore struct {
	clients   sync.Map // map[string]*core.ClientInfo
	cfg       *core.StoreConfig
	closed    bool
	closeMu   sync.RWMutex
	stopCh    chan struct{}
	cleanupWg sync.WaitGroup
}

// NewMemoryStore 创建内存存储
func NewMemoryStore(opts ...core.StoreOption) *MemoryStore {
	cfg := core.DefaultStoreConfig()
	core.ApplyOptions(cfg, opts...)

	s := &MemoryStore{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}

	if cfg.AutoCleanup {
		s.startCleanup()
	}

	return s
}

// startCleanup 启动自动清理协程
func (s *MemoryStore) startCleanup() {
	s.cleanupWg.Add(1)
	go func() {
		defer s.cleanupWg.Done()
		ticker := time.NewTicker(s.cfg.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_, _ = s.Cleanup(context.Background(), s.cfg.DefaultTimeout)
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Register 注册客户端
func (s *MemoryStore) Register(ctx context.Context, client *core.ClientInfo) error {
	if s.isClosed() {
		return core.ErrStoreClosed
	}

	now := time.Now()
	client.RegisteredAt = now
	client.LastHeartbeat = now
	client.Status = core.StatusOnline

	s.clients.Store(client.ID, client)
	return nil
}

// Unregister 注销客户端
func (s *MemoryStore) Unregister(ctx context.Context, clientID string) error {
	if s.isClosed() {
		return core.ErrStoreClosed
	}

	if _, ok := s.clients.Load(clientID); !ok {
		return core.ErrClientNotFound
	}

	s.clients.Delete(clientID)
	return nil
}

// Heartbeat 更新心跳
func (s *MemoryStore) Heartbeat(ctx context.Context, clientID string, metadata map[string]string) error {
	if s.isClosed() {
		return core.ErrStoreClosed
	}

	val, ok := s.clients.Load(clientID)
	if !ok {
		return core.ErrClientNotFound
	}

	client := val.(*core.ClientInfo)
	client.LastHeartbeat = time.Now()
	client.Status = core.StatusOnline

	// 合并 metadata
	if metadata != nil {
		if client.Metadata == nil {
			client.Metadata = make(map[string]string)
		}
		for k, v := range metadata {
			client.Metadata[k] = v
		}
	}

	return nil
}

// Get 获取单个客户端信息
func (s *MemoryStore) Get(ctx context.Context, clientID string) (*core.ClientInfo, error) {
	if s.isClosed() {
		return nil, core.ErrStoreClosed
	}

	val, ok := s.clients.Load(clientID)
	if !ok {
		return nil, core.ErrClientNotFound
	}

	return val.(*core.ClientInfo), nil
}

// List 获取所有客户端列表
func (s *MemoryStore) List(ctx context.Context) ([]*core.ClientInfo, error) {
	if s.isClosed() {
		return nil, core.ErrStoreClosed
	}

	var clients []*core.ClientInfo
	s.clients.Range(func(_, value interface{}) bool {
		clients = append(clients, value.(*core.ClientInfo))
		return true
	})

	return clients, nil
}

// ListOnline 获取在线客户端列表
func (s *MemoryStore) ListOnline(ctx context.Context) ([]*core.ClientInfo, error) {
	if s.isClosed() {
		return nil, core.ErrStoreClosed
	}

	var clients []*core.ClientInfo
	timeout := s.cfg.DefaultTimeout

	s.clients.Range(func(_, value interface{}) bool {
		client := value.(*core.ClientInfo)
		if !client.IsExpired(timeout) {
			clients = append(clients, client)
		}
		return true
	})

	return clients, nil
}

// Count 获取客户端总数
func (s *MemoryStore) Count(ctx context.Context) (int, error) {
	if s.isClosed() {
		return 0, core.ErrStoreClosed
	}

	count := 0
	s.clients.Range(func(_, _ interface{}) bool {
		count++
		return true
	})

	return count, nil
}

// CountOnline 获取在线客户端数
func (s *MemoryStore) CountOnline(ctx context.Context) (int, error) {
	if s.isClosed() {
		return 0, core.ErrStoreClosed
	}

	count := 0
	timeout := s.cfg.DefaultTimeout

	s.clients.Range(func(_, value interface{}) bool {
		client := value.(*core.ClientInfo)
		if !client.IsExpired(timeout) {
			count++
		}
		return true
	})

	return count, nil
}

// Cleanup 清理超时客户端
func (s *MemoryStore) Cleanup(ctx context.Context, timeout time.Duration) (int, error) {
	if s.isClosed() {
		return 0, core.ErrStoreClosed
	}

	cleaned := 0
	var toDelete []string

	s.clients.Range(func(key, value interface{}) bool {
		client := value.(*core.ClientInfo)
		if client.IsExpired(timeout) {
			toDelete = append(toDelete, key.(string))
			cleaned++
		}
		return true
	})

	for _, id := range toDelete {
		s.clients.Delete(id)
	}

	return cleaned, nil
}

// Close 关闭存储
func (s *MemoryStore) Close() error {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return nil
	}
	s.closed = true
	s.closeMu.Unlock()

	close(s.stopCh)
	s.cleanupWg.Wait()

	return nil
}

// isClosed 检查是否已关闭
func (s *MemoryStore) isClosed() bool {
	s.closeMu.RLock()
	defer s.closeMu.RUnlock()
	return s.closed
}
