package service

import (
	"context"
)

type Servicer interface {
	Serve() error
	Shutdown(context.Context) error
	Close() error
}
