package multicast

import (
	"context"

	"github.com/joyparty/nodehub/proto/nh"
)

// Publisher 把消息发布到消息队列
type Publisher interface {
	Publish(ctx context.Context, message *nh.Multicast) error
}

// Subscriber 从消息队列订阅消息
type Subscriber interface {
	Subscribe(ctx context.Context, handler func(*nh.Multicast)) error
}
