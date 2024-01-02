package mq

import "context"

// Queue 消息队列
type Queue interface {
	Publish(ctx context.Context, payload []byte) error
	Subscribe(ctx context.Context, handler func([]byte))
	Close()
}
