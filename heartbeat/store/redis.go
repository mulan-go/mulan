package store

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mulan-go/mulan/heartbeat/core"
)

var _ core.Store = (*RedisStore)(nil)

// RedisStore Redis 存储实现
type RedisStore struct {
	client    *redis.Client
	cfg       *core.StoreConfig
	closed    bool
	closeMu   sync.RWMutex
	stopCh    chan struct{}
	cleanupWg sync.WaitGroup
}

// NewRedisStore 创建 Redis 存储
func NewRedisStore(client *redis.Client, opts ...core.StoreOption) *RedisStore {
	cfg := core.DefaultStoreConfig()
	core.ApplyOptions(cfg, opts...)

	s := &RedisStore{
		client: client,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}

	if cfg.AutoCleanup {
		s.startCleanup()
	}

	return s
}

// key 生成完整的 Redis key
func (s *RedisStore) key(clientID string) string {
	return s.cfg.KeyPrefix + clientID
}

// indexKey 返回索引 key（存储所有客户端 ID 的集合）
func (s *RedisStore) indexKey() string {
	return s.cfg.KeyPrefix + "index"
}

// startCleanup 启动自动清理协程
func (s *RedisStore) startCleanup() {
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
func (s *RedisStore) Register(ctx context.Context, client *core.ClientInfo) error {
	if s.isClosed() {
		return core.ErrStoreClosed
	}

	now := time.Now()
	client.RegisteredAt = now
	client.LastHeartbeat = now
	client.Status = core.StatusOnline

	data, err := json.Marshal(client)
	if err != nil {
		return err
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, s.key(client.ID), data, 0)
	pipe.SAdd(ctx, s.indexKey(), client.ID)
	_, err = pipe.Exec(ctx)

	return err
}

// Unregister 注销客户端
func (s *RedisStore) Unregister(ctx context.Context, clientID string) error {
	if s.isClosed() {
		return core.ErrStoreClosed
	}

	exists, err := s.client.Exists(ctx, s.key(clientID)).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return core.ErrClientNotFound
	}

	pipe := s.client.Pipeline()
	pipe.Del(ctx, s.key(clientID))
	pipe.SRem(ctx, s.indexKey(), clientID)
	_, err = pipe.Exec(ctx)

	return err
}

// Heartbeat 更新心跳
func (s *RedisStore) Heartbeat(ctx context.Context, clientID string, metadata map[string]string) error {
	if s.isClosed() {
		return core.ErrStoreClosed
	}

	data, err := s.client.Get(ctx, s.key(clientID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return core.ErrClientNotFound
		}
		return err
	}

	var client core.ClientInfo
	if err := json.Unmarshal(data, &client); err != nil {
		return err
	}

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

	newData, err := json.Marshal(&client)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, s.key(clientID), newData, 0).Err()
}

// Get 获取单个客户端信息
func (s *RedisStore) Get(ctx context.Context, clientID string) (*core.ClientInfo, error) {
	if s.isClosed() {
		return nil, core.ErrStoreClosed
	}

	data, err := s.client.Get(ctx, s.key(clientID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, core.ErrClientNotFound
		}
		return nil, err
	}

	var client core.ClientInfo
	if err := json.Unmarshal(data, &client); err != nil {
		return nil, err
	}

	return &client, nil
}

// List 获取所有客户端列表
func (s *RedisStore) List(ctx context.Context) ([]*core.ClientInfo, error) {
	if s.isClosed() {
		return nil, core.ErrStoreClosed
	}

	ids, err := s.client.SMembers(ctx, s.indexKey()).Result()
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return []*core.ClientInfo{}, nil
	}

	// 批量获取所有客户端数据
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = s.key(id)
	}

	values, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	clients := make([]*core.ClientInfo, 0, len(values))
	for _, v := range values {
		if v == nil {
			continue
		}
		var client core.ClientInfo
		if err := json.Unmarshal([]byte(v.(string)), &client); err != nil {
			continue
		}
		clients = append(clients, &client)
	}

	return clients, nil
}

// ListOnline 获取在线客户端列表
func (s *RedisStore) ListOnline(ctx context.Context) ([]*core.ClientInfo, error) {
	if s.isClosed() {
		return nil, core.ErrStoreClosed
	}

	clients, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	timeout := s.cfg.DefaultTimeout
	online := make([]*core.ClientInfo, 0, len(clients))
	for _, client := range clients {
		if !client.IsExpired(timeout) {
			online = append(online, client)
		}
	}

	return online, nil
}

// Count 获取客户端总数
func (s *RedisStore) Count(ctx context.Context) (int, error) {
	if s.isClosed() {
		return 0, core.ErrStoreClosed
	}

	count, err := s.client.SCard(ctx, s.indexKey()).Result()
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

// CountOnline 获取在线客户端数
func (s *RedisStore) CountOnline(ctx context.Context) (int, error) {
	if s.isClosed() {
		return 0, core.ErrStoreClosed
	}

	clients, err := s.ListOnline(ctx)
	if err != nil {
		return 0, err
	}

	return len(clients), nil
}

// Cleanup 清理超时客户端
func (s *RedisStore) Cleanup(ctx context.Context, timeout time.Duration) (int, error) {
	if s.isClosed() {
		return 0, core.ErrStoreClosed
	}

	clients, err := s.List(ctx)
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, client := range clients {
		if client.IsExpired(timeout) {
			if err := s.Unregister(ctx, client.ID); err == nil {
				cleaned++
			}
		}
	}

	return cleaned, nil
}

// Close 关闭存储
func (s *RedisStore) Close() error {
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
func (s *RedisStore) isClosed() bool {
	s.closeMu.RLock()
	defer s.closeMu.RUnlock()
	return s.closed
}
