package rdb

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

var Nil = redis.Nil

func New(cfg *Config) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Pass,
		DB:       cfg.DB,
		OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			return nil
		},
	})
	if cfg.Debug {
		rdb.AddHook(DebugHook{})
	}
	err := rdb.Ping(context.Background()).Err()
	if err != nil {
		return nil, err
	}
	return rdb, nil
}
