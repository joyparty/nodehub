package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"gitlab.haochang.tv/gopkg/nodehub/internal/mq"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
)

// RedisClient 客户端
type RedisClient = mq.RedisClient

// Bus 事件总线
type Bus interface {
	// Publish 发布事件
	//
	// Example:
	//   bus.Publish(ctx, event.UserConnected{...})
	Publish(ctx context.Context, event any) error

	// Subscribe 订阅事件
	//
	// Example:
	//
	//	 bus.Subscribe(ctx, func(ev event.UserConnected) {
	//			// ...
	//	 })
	//
	//	 bus.Subscribe(ctx, func(ev event.UserDisconnected) {
	//			// ...
	//	 })
	Subscribe(ctx context.Context, handler any) error

	// Close 关闭事件总线连接
	Close()
}

// NewBus 构造函数
func NewBus(client RedisClient) Bus {
	return &redisBus{
		mq: mq.NewRedisMQ(client, "cluster:events"),
	}
}

type redisBus struct {
	mq *mq.RedisMQ
}

func (bus *redisBus) Publish(ctx context.Context, event any) error {
	p, err := newPayload(event)
	if err != nil {
		return err
	}

	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	return bus.mq.Publish(ctx, data)
}

func (bus *redisBus) Subscribe(ctx context.Context, handler any) error {
	fn := reflect.ValueOf(handler)

	fnType := fn.Type()
	if fnType.Kind() != reflect.Func {
		return errors.New("handler must be a function")
	} else if fnType.NumIn() != 1 {
		return errors.New("handler must have one argument")
	}

	wantType := fnType.In(0)
	eventType, ok := types[deref(wantType)]
	if !ok {
		return fmt.Errorf("unknown event %s.%s", wantType.PkgPath(), wantType.Name())
	}

	bus.mq.Subscribe(ctx, func(data []byte) {
		p := payload{}
		if err := json.Unmarshal(data, &p); err != nil {
			logger.Error("unmarshal event payload", "error", err)
			return
		}

		if p.Type == eventType {
			ev := reflect.New(wantType)
			if err := json.Unmarshal(p.Detail, ev.Interface()); err != nil {
				logger.Error("unmarshal event", "error", err)
				return
			}

			fn.Call([]reflect.Value{ev.Elem()})
		}
	})

	return nil
}

func (bus *redisBus) Close() {
	bus.mq.Close()
}
