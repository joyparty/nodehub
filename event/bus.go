package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/joyparty/nodehub/internal/metrics"
	"github.com/joyparty/nodehub/internal/mq"
	"github.com/joyparty/nodehub/logger"
	"github.com/nats-io/nats.go"
	"github.com/reactivex/rxgo/v2"
	"github.com/redis/go-redis/v9"
)

// Queue 消息队列
type Queue = mq.Queue

// NewBus 构造函数
func NewBus(queue Queue) *Bus {
	return &Bus{
		queue: queue,
	}
}

// NewNatsBus 使用nats构造事件总线
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

// Bus 事件总线
type Bus struct {
	queue mq.Queue

	observeOnce sync.Once
	observable  rxgo.Observable // type: payload
}

// Publish 发布事件
//
// Example:
//
//	bus.Publish(ctx, event.UserConnected{...})
func (bus *Bus) Publish(ctx context.Context, event any) error {
	p, err := newPayload(event)
	if err != nil {
		panic(fmt.Errorf("publish event, %w", err))
	}

	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	return bus.queue.Publish(ctx, data)
}

// Subscribe 订阅事件
//
// Example:
//
//	 bus.Subscribe(ctx, func(ev event.UserConnected, t time.Time) {
//			// ...
//	 })
//
//	 bus.Subscribe(ctx, func(ev event.UserDisconnected, t time.Time) {
//			// ...
//	 })
func (bus *Bus) Subscribe(ctx context.Context, handler any) {
	fn := reflect.ValueOf(handler)

	fnType := fn.Type()
	if fnType.Kind() != reflect.Func {
		// 设置事件订阅handler一般是在启动阶段，如果这里出错了，很可能会导致关键的流程故障，
		// 假设程序员忘记捕获错误，就会导致难以排查的故障
		// 因此干脆panic中断进程，让程序员把代码改对了再执行
		panic(errors.New("handler must be a function"))
	} else if fnType.NumIn() != 2 {
		panic(errors.New("handler must have two argument"))
	}

	firstArg := fnType.In(0)
	eventType, ok := types[deref(firstArg)]
	if !ok {
		panic(fmt.Errorf("first argument(%s.%s) must be registered event", firstArg.PkgPath(), firstArg.Name()))
	}

	secondArg := fnType.In(1)
	if _, ok := reflect.New(secondArg).Elem().Interface().(time.Time); !ok {
		panic(errors.New("second argument must be time.Time"))
	}

	bus.observe()
	bus.observable.ForEach(
		func(item any) {
			p := item.(payload)

			if p.Type == eventType {
				ev := reflect.New(firstArg)
				if err := json.Unmarshal(p.Detail, ev.Interface()); err != nil {
					logger.Error("unmarshal event", "error", err)
					return
				}

				fn.Call([]reflect.Value{
					ev.Elem(),
					reflect.ValueOf(p.GetTime()),
				})
			}
		},
		func(err error) {
			logger.Error("handle cluster event", "error", err)
		},
		func() {},

		rxgo.WithContext(ctx),
	)
}

func (bus *Bus) observe() {
	bus.observeOnce.Do(func() {
		bus.observable = rxgo.Create([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
			msgC, err := bus.queue.Subscribe(context.Background())
			if err != nil {
				logger.Error("subscribe cluster events", "error", err)
				panic(fmt.Errorf("subscribe cluster events, %w", err))
			}

			for msg := range msgC {
				p := payload{}
				if err := json.Unmarshal(msg, &p); err != nil {
					next <- rxgo.Error(err)
				} else {
					next <- rxgo.Of(p)

					metrics.IncrMessageQueue(bus.queue.Topic(), time.Since(p.GetTime()))
				}
			}
		}}, rxgo.WithErrorStrategy(rxgo.ContinueOnError))
	})
}

// Close 关闭事件总线连接
func (bus *Bus) Close() {
	bus.queue.Close()
}

// Options 配置
type Options struct {
	// ChannelName 通道名称，默认为nodehub:events
	//
	// 不同的总线实现内有不同的含义，在nats里面是topic，redis里面是channel
	ChannelName string
}

func newOptions() *Options {
	return &Options{
		ChannelName: "nodehub:events",
	}
}

// WithChannelName 设置通道名称
func WithChannelName(name string) func(*Options) {
	return func(o *Options) {
		o.ChannelName = name
	}
}
