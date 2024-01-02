package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"gitlab.haochang.tv/gopkg/nodehub/internal/mq"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
)

// RedisClient 客户端
type RedisClient = mq.RedisClient

// Queue 消息队列
type Queue = mq.Queue

// NewBus 构造函数
func NewBus(queue Queue) *Bus {
	return &Bus{
		queue: queue,
	}
}

// NewRedisBus 构造函数
func NewRedisBus(client RedisClient) *Bus {
	return &Bus{
		queue: mq.NewRedisMQ(client, "cluster:events"),
	}
}

// Bus 事件总线
type Bus struct {
	queue mq.Queue
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
func (bus *Bus) Subscribe(ctx context.Context, handler any) error {
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

	bus.queue.Subscribe(ctx, func(data []byte) {
		p := payload{}
		if err := json.Unmarshal(data, &p); err != nil {
			logger.Error("unmarshal event payload", "error", err)
			return
		}

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
	})

	return nil
}

// Close 关闭事件总线连接
func (bus *Bus) Close() {
	bus.queue.Close()
}
