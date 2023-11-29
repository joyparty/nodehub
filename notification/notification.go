package notification

import (
	"context"
	gatewaypb "nodehub/proto/gateway"
)

// Publisher 把消息发布到消息队列
type Publisher interface {
	Publish(ctx context.Context, message *gatewaypb.Notification) error
}

// Subscriber 从消息队列订阅消息
type Subscriber interface {
	Subscribe(ctx context.Context) (<-chan *gatewaypb.Notification, error)
}
