package multicast

import (
	"context"
	"errors"

	"github.com/joyparty/nodehub/internal/mq"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

// Channel 通道名称
var Channel = "nodehub:multicast"

// Queue 消息队列
type Queue = mq.Queue

// Bus push message总线
type Bus struct {
	queue mq.Queue
}

// NewBus 构造函数
func NewBus(queue Queue) *Bus {
	return &Bus{
		queue: queue,
	}
}

// NewNatsBus 使用nats构造总线
func NewNatsBus(conn *nats.Conn) *Bus {
	return &Bus{
		queue: mq.NewNatsMQ(conn, Channel),
	}
}

// NewRedisBus 构造函数
//
// client 可以使用 *redis.Client 或者 *redis.ClusterClient
//
// 当使用ClusterClient时，会采用sharded channel
func NewRedisBus(client *redis.Client) *Bus {
	return &Bus{
		queue: mq.NewRedisMQ(client, Channel),
	}
}

// Publish 把消息发布到消息队列
func (bus *Bus) Publish(ctx context.Context, message *nh.Multicast) error {
	if message.GetContent().GetServiceCode() == 0 {
		return errors.New("invalid content, from_service is required")
	}

	payload, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	return bus.queue.Publish(ctx, payload)
}

// Subscribe 从消息队列订阅消息
func (bus *Bus) Subscribe(ctx context.Context, handler func(*nh.Multicast)) error {
	bus.queue.Subscribe(ctx, func(payload []byte) {
		n := &nh.Multicast{}
		if err := proto.Unmarshal(payload, n); err != nil {
			logger.Error("unmarshal notification", "error", err)
			return
		}
		handler(n)
	})

	return nil
}

// Close 关闭消息队列
func (bus *Bus) Close() {
	bus.queue.Close()
}
