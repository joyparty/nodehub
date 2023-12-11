package notification

import (
	"context"

	"gitlab.haochang.tv/gopkg/nodehub/proto/clientpb"
)

// Publisher 把消息发布到消息队列
type Publisher interface {
	Publish(ctx context.Context, message *clientpb.Notification) error
}

// Subscriber 从消息队列订阅消息
type Subscriber interface {
	// Subscribe(ctx context.Context) (<-chan *clientpb.Notification, error)
	Subscribe(ctx context.Context, handler func(*clientpb.Notification)) error
}
