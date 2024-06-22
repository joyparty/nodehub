package multicast

import (
	"context"
	"time"

	"github.com/joyparty/nodehub/internal/metrics"
	"github.com/joyparty/nodehub/internal/mq"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

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
func NewNatsBus(conn *nats.Conn, options ...func(*Options)) *Bus {
	opt := newOptions()
	for _, fn := range options {
		fn(opt)
	}

	return &Bus{
		queue: mq.NewNatsMQ(conn, opt.ChannelName),
	}
}

// NewRedisBus 构造函数
func NewRedisBus(client *redis.Client, options ...func(*Options)) *Bus {
	opt := newOptions()
	for _, fn := range options {
		fn(opt)
	}

	return &Bus{
		queue: mq.NewRedisMQ(client, opt.ChannelName),
	}
}

// Publish 把消息发布到消息队列
func (bus *Bus) Publish(ctx context.Context, message *nh.Multicast) error {
	payload, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	return bus.queue.Publish(ctx, payload)
}

// Subscribe 从消息队列订阅消息
func (bus *Bus) Subscribe(ctx context.Context, handler func(*nh.Multicast)) error {
	msgC, err := bus.queue.Subscribe(ctx)
	if err != nil {
		return err
	}

	go func() {
		for msg := range msgC {
			n := &nh.Multicast{}
			if err := proto.Unmarshal(msg, n); err != nil {
				logger.Error("unmarshal multicast message", "error", err)
			} else {
				handler(n)

				metrics.IncrMessageQueue(bus.queue.Topic(), time.Since(n.Time.AsTime()))
			}
		}
	}()
	return nil
}

// Close 关闭消息队列
func (bus *Bus) Close() {
	bus.queue.Close()
}

// Options 配置
type Options struct {
	// ChannelName 通道名称，默认为nodehub:multicast
	//
	// 不同的总线实现内有不同的含义，在nats里面是topic，redis里面是channel
	ChannelName string
}

func newOptions() *Options {
	return &Options{
		ChannelName: "nodehub:multicast",
	}
}

// WithChannelName 设置通道名称
func WithChannelName(name string) func(*Options) {
	return func(o *Options) {
		o.ChannelName = name
	}
}
