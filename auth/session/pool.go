package session

import (
	"context"
	"sync"
)

// SessionPool Session对象池，减少内存分配
var sessionPool = sync.Pool{
	New: func() any {
		return &Session{
			keyPrefix: DefaultKeyPrefix,
			maxAge:    DefaultMaxAge,
		}
	},
}

// getSession 从池中获取Session
func getSession(ctx context.Context, store Store, data Data, maxAge ...int) *Session {
	s := sessionPool.Get().(*Session)
	// 重置Session状态
	s.ctx = ctx
	s.store = store
	s.data = data
	s.token = data.Token()
	s.loaded = false
	s.IsNil = false
	if len(maxAge) > 0 {
		s.maxAge = maxAge[0]
	} else {
		s.maxAge = DefaultMaxAge
	}
	return s
}

// putSession 将Session放回池中
func putSession(s *Session) {
	s.ctx = nil
	s.store = nil
	s.data = nil
	s.token = ""
	s.loaded = false
	sessionPool.Put(s)
}

// NewSessionWithPool 使用对象池创建Session（可选）
func NewSessionWithPool(ctx context.Context, store Store, data Data, maxAge ...int) *Session {
	return getSession(ctx, store, data, maxAge...)
}

// ReleaseSession 释放Session到池中（可选，需要手动调用）
func ReleaseSession(s *Session) { putSession(s) }
