package mq

import (
	"context"

	"github.com/reactivex/rxgo/v2"
)

// Queue 消息队列
type Queue interface {
	Publish(ctx context.Context, payload []byte) error
	Subscribe(ctx context.Context, handler func([]byte))
	Observable() rxgo.Observable
	Close()
}
