package session

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var _ Store = (*RedisStore)(nil)

type RedisStore struct {
	client    redis.UniversalClient
	keyPrefix string
	maxAge    int
}

func NewRedisStore(client redis.UniversalClient, maxAge ...int) (*RedisStore, error) {
	s := &RedisStore{
		keyPrefix: DefaultKeyPrefix,
		maxAge:    DefaultMaxAge,
		client:    client,
	}
	if len(maxAge) > 0 {
		s.maxAge = maxAge[0]
	}
	return s, client.Ping(context.Background()).Err()
}

func (s *RedisStore) Clear(ctx context.Context, token string) error {
	return s.client.Del(ctx, s.getKey(token)).Err()
}

func (s *RedisStore) Get(ctx context.Context, token string) (Data, error) {
	key := s.getKey(token)
	result := s.client.HGetAll(ctx, key)

	if result.Err() != nil {
		return nil, result.Err()
	}

	vals := result.Val()
	if len(vals) == 0 {
		return nil, ErrTokenNotFound
	}

	// 创建新的DefaultData来接收扫描结果
	data := &DefaultData{}
	if err := result.Scan(data); err != nil {
		zap.L().Error("Failed to scan redis data",
			zap.String("key", key),
			zap.Error(err))
		return nil, err
	}

	return data, nil
}

func (s *RedisStore) Save(ctx context.Context, v Data, lifetime ...time.Duration) error {
	token := v.Token()
	if token == "" {
		token = v.New()
	}

	key := s.getKey(token)
	expiration := s.calculateExpireTime(lifetime...)

	// 使用Pipeline提高性能
	pipe := s.client.Pipeline()
	pipe.HSet(ctx, key, v)
	if expiration > 0 {
		pipe.Expire(ctx, key, expiration)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		zap.L().Error("Failed to save session",
			zap.String("key", key),
			zap.Error(err))
		return err
	}

	return nil
}

// getKey 获取完整的Redis key
func (s *RedisStore) getKey(token string) string {
	return s.keyPrefix + token
}

// calculateExpireTime 计算过期时间
func (s *RedisStore) calculateExpireTime(lifetime ...time.Duration) time.Duration {
	if len(lifetime) > 0 && lifetime[0] > 0 {
		return lifetime[0]
	}
	if s.maxAge > 0 {
		return time.Duration(s.maxAge) * time.Second
	}
	return 0 // 永不过期
}
