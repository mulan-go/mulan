package session

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrTokenNotFound token不存在
	ErrTokenNotFound = errors.New("token not found")
	// ErrTokenExpired token已过期
	ErrTokenExpired = errors.New("token expired")
)

var _ Store = (*MemStore)(nil)

type memData struct {
	data   Data
	expire time.Time
}

type MemStore struct {
	data   map[string]*memData
	maxAge int
	mu     sync.RWMutex
}

func NewMemStore(maxAge ...int) *MemStore {
	s := &MemStore{
		maxAge: DefaultMaxAge,
		data:   make(map[string]*memData),
	}
	if len(maxAge) > 0 {
		s.maxAge = maxAge[0]
	}
	return s
}

func (s *MemStore) Clear(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, token)
	return nil
}

func (s *MemStore) Get(ctx context.Context, token string) (Data, error) {
	s.mu.RLock()
	data, exists := s.data[token]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrTokenNotFound
	}

	// 检查是否过期
	if !data.expire.IsZero() && data.expire.Before(time.Now()) {
		// 删除过期数据（需要写锁）
		s.mu.Lock()
		delete(s.data, token)
		s.mu.Unlock()
		return nil, ErrTokenExpired
	}

	return data.data, nil
}

func (s *MemStore) Save(ctx context.Context, v Data, lifetime ...time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	token := v.Token()
	if token == "" {
		token = v.New()
	}

	data := &memData{data: v}
	data.expire = s.calculateExpireTime(lifetime...)
	s.data[token] = data
	return nil
}

// calculateExpireTime 计算过期时间（提取公共逻辑）
func (s *MemStore) calculateExpireTime(lifetime ...time.Duration) time.Time {
	if len(lifetime) > 0 && lifetime[0] > 0 {
		return time.Now().Add(lifetime[0])
	}
	if s.maxAge > 0 {
		return time.Now().Add(time.Duration(s.maxAge) * time.Second)
	}
	return time.Time{} // 永不过期
}
