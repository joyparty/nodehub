package nodehub

import (
	"context"
)

// Server 网络服务
type Server interface {
	Name() string

	Init(ctx context.Context) error

	BeforeStart(ctx context.Context) error
	Start(ctx context.Context) error
	AfterStart(ctx context.Context)

	BeforeStop(ctx context.Context) error
	Stop(ctx context.Context) error
	AfterStop(ctx context.Context)
}
