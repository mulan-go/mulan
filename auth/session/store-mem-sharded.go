package session

import (
	"context"
	"hash/fnv"
	"time"
)

// ShardedMemStore 分片内存存储，用于提升并发写性能
type ShardedMemStore struct {
	shards    []*MemStore
	shardMask uint32
}

// NewShardedMemStore 创建分片内存存储
// shardCount 必须是2的幂（4, 8, 16, 32...）
func NewShardedMemStore(shardCount int, maxAge ...int) *ShardedMemStore {
	// 确保shardCount是2的幂
	if shardCount <= 0 || (shardCount&(shardCount-1)) != 0 {
		shardCount = 16 // 默认16个分片
	}

	s := &ShardedMemStore{
		shards:    make([]*MemStore, shardCount),
		shardMask: uint32(shardCount - 1),
	}

	// 初始化每个分片
	for i := 0; i < shardCount; i++ {
		s.shards[i] = NewMemStore(maxAge...)
	}

	return s
}

// getShard 根据token获取对应的分片
func (s *ShardedMemStore) getShard(token string) *MemStore {
	h := fnv.New32a()
	h.Write([]byte(token))
	hash := h.Sum32()
	return s.shards[hash&s.shardMask]
}

// Clear 清除token
func (s *ShardedMemStore) Clear(ctx context.Context, token string) error {
	return s.getShard(token).Clear(ctx, token)
}

// Get 获取token数据
func (s *ShardedMemStore) Get(ctx context.Context, token string) (Data, error) {
	return s.getShard(token).Get(ctx, token)
}

// Save 保存token数据
func (s *ShardedMemStore) Save(ctx context.Context, v Data, lifetime ...time.Duration) error {
	token := v.Token()
	if token == "" {
		token = v.New()
	}
	return s.getShard(token).Save(ctx, v, lifetime...)
}

var _ Store = (*ShardedMemStore)(nil)
