package session

import (
	"context"
	"time"
)

type Store interface {
	Clear(ctx context.Context, v string) error
	Get(ctx context.Context, v string) (Data, error)
	Save(ctx context.Context, v Data, lifetime ...time.Duration) error
}
