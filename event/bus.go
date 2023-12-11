package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gitlab.haochang.tv/gopkg/nodehub/internal/mq"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
)

// RedisClient 客户端
type RedisClient = mq.RedisClient

// Bus 事件总线
type Bus interface {
	Publish(ctx context.Context, event any) error
	Subscribe(ctx context.Context, want ...any) (events <-chan any, err error)
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

// example: bus.Subscribe(ctx, event.UserConnected{}, event.UserDisconnected{})
func (bus *redisBus) Subscribe(ctx context.Context, want ...any) (<-chan any, error) {
	if len(want) == 0 {
		return nil, errors.New("no event specified")
	}

	unmarshalers := map[string]unmarshaler{}
	for _, w := range want {
		handler, eventType, err := newUnmarshaler(w)
		if err != nil {
			return nil, err
		}
		unmarshalers[eventType] = handler
	}

	payloadC, err := bus.mq.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("subscribe queue, %w", err)
	}

	eventC := make(chan any)
	go func() {
		defer close(eventC)

		for data := range payloadC {
			p := payload{}
			if err := json.Unmarshal(data, &p); err != nil {
				logger.Error("unmarshal event payload", "error", err)
				continue
			}

			if handler, ok := unmarshalers[p.Type]; ok {
				ev, err := handler(p)
				if err != nil {
					logger.Error("unmarshal event", "error", err)
					continue
				}

				select {
				case <-ctx.Done():
					return
				case eventC <- ev:
				}
			}
		}
	}()

	return eventC, nil
}

func (bus *redisBus) Close() {
	bus.mq.Close()
}
