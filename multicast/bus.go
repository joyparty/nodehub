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
	queue      mq.Queue
	submitTask func(func()) error
}

// NewBus 构造函数
func NewBus(queue Queue, options ...func(*Options)) *Bus {
	opt := newOptions(options...)
	return newBus(queue, opt)
}

// NewNatsBus 使用nats构造总线
func NewNatsBus(conn *nats.Conn, options ...func(*Options)) *Bus {
	opt := newOptions(options...)
	return newBus(mq.NewNatsMQ(conn, opt.ChannelName), opt)
}

// NewRedisBus 构造函数
func NewRedisBus(client *redis.Client, options ...func(*Options)) *Bus {
	opt := newOptions(options...)
	return newBus(mq.NewRedisMQ(client, opt.ChannelName), opt)
}

func newBus(queue Queue, options *Options) *Bus {
	submitTask := ants.Submit
	if options.GoPool != nil {
		submitTask = options.GoPool.Submit
	}

	return &Bus{
		queue:      queue,
		submitTask: submitTask,
	}
}

// Publish 把消息发布到消息队列
func (bus *Bus) Publish(ctx context.Context, message *nh.Multicast) error {
	if reply := message.GetContent(); reply.GetServiceCode() == 0 || reply.GetCode() == 0 {
		return errors.New("service code or message code is empty")
	}

	if len(message.GetReceiver()) == 0 {
		return errors.New("receiver is empty")
	} else if message.GetTime() == nil {
		message.SetTime(timestamppb.Now())
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
				metrics.IncrMessageQueue(bus.queue.Topic(), msg.GetTime().AsTime())

				if msg.GetStream() == "" {
					if err := bus.submitTask(func() {
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

				select {
				case <-ctx.Done():
					return
				case w.C <- msg:
					w.Active = time.Now()
				}
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

	// GoPool goroutine pool
	//
	// 默认使用ants default pool
	GoPool GoPool
}

func newOptions(apply ...func(*Options)) *Options {
	opt := &Options{
		ChannelName: "nodehub:multicast",
	}

	for _, fn := range apply {
		fn(opt)
	}

	return opt
}

// WithChannelName 设置通道名称
func WithChannelName(name string) func(*Options) {
	return func(o *Options) {
		o.ChannelName = name
	}
}

// GoPool goroutine pool
type GoPool interface {
	Submit(task func()) error
}

// WithGoPool 设置goroutine pool
func WithGoPool(pool GoPool) func(*Options) {
	return func(opt *Options) {
		opt.GoPool = pool
	}
}
