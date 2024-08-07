package multicast

import (
	"context"
	"errors"
	"time"

	"github.com/joyparty/nodehub/internal/metrics"
	"github.com/joyparty/nodehub/internal/mq"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/nats-io/nats.go"
	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	if len(message.Receiver) == 0 {
		return errors.New("receiver is empty")
	} else if message.Time == nil {
		message.Time = timestamppb.Now()
	}

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
		type worker struct {
			C      chan *nh.Multicast
			Active time.Time
		}

		workers := map[string]*worker{}
		defer func() {
			for k, w := range workers {
				close(w.C)
				delete(workers, k)
			}
		}()

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for k, w := range workers {
					if time.Since(w.Active) > 5*time.Minute {
						close(w.C)
						delete(workers, k)
					}
				}
			case data, ok := <-msgC:
				if !ok {
					return
				}

				msg := &nh.Multicast{}
				if err := proto.Unmarshal(data, msg); err != nil {
					logger.Error("unmarshal multicast message", "error", err)
					continue
				}
				metrics.IncrMessageQueue(bus.queue.Topic(), time.Since(msg.Time.AsTime()))

				if msg.GetStream() == "" {
					if err := ants.Submit(func() {
						handler(msg)
					}); err != nil {
						logger.Error("submit handler", "error", err)
					}
					continue
				}

				w, ok := workers[msg.GetStream()]
				if !ok {
					w = &worker{
						C: make(chan *nh.Multicast, 100),
					}
					workers[msg.GetStream()] = w

					go func() {
						for msg := range w.C {
							handler(msg)
						}
					}()
				}

				w.C <- msg
				w.Active = time.Now()
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
