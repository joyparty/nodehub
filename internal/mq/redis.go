package mq

import (
	"context"
	"sync"

	"github.com/joyparty/nodehub/logger"
	"github.com/reactivex/rxgo/v2"
	"github.com/redis/go-redis/v9"
)

// redisMQ 是基于 Redis 的消息队列
type redisMQ struct {
	client  *redis.Client
	channel string
	done    chan struct{}

	observer      rxgo.Observable
	subscribeOnce *sync.Once
}

// NewRedisMQ 构造函数
//
// client 可以使用 *redis.Client 或者 *redis.ClusterClient
//
// 当使用ClusterClient时，会采用sharded channel
func NewRedisMQ(client *redis.Client, channel string) Queue {
	return &redisMQ{
		client:        client,
		channel:       channel,
		done:          make(chan struct{}),
		subscribeOnce: &sync.Once{},
	}
}

// Publish 把消息发布到消息队列
func (mq *redisMQ) Publish(ctx context.Context, payload []byte) error {
	return mq.client.Publish(ctx, mq.channel, payload).Err()
}

// Subscribe 从消息队列订阅消息
func (mq *redisMQ) Subscribe(ctx context.Context, handler func([]byte)) {
	mq.subscribe()

	mq.observer.ForEach(
		func(item any) {
			handler(item.([]byte))
		},
		func(err error) {
			logger.Error("redis observer error", "error", err, "channel", mq.channel)
		},
		func() {},
	)
}

func (mq *redisMQ) subscribe() {
	mq.subscribeOnce.Do(func() {
		ctx := context.Background()
		pubsub := mq.client.Subscribe(ctx, mq.channel)
		items := make(chan rxgo.Item)
		mq.observer = rxgo.FromEventSource(items, rxgo.WithErrorStrategy(rxgo.ContinueOnError))

		mc := pubsub.Channel()
		go func() {
			defer func() {
				pubsub.Unsubscribe(ctx)
				pubsub.Close()

				close(items)
			}()

			for {
				select {
				case <-mq.done:
					return
				case msg, ok := <-mc:
					if !ok {
						logger.Error("redis MQ consumer unexpected closed", "channel", mq.channel)
						return
					}

					select {
					case <-mq.done:
						return
					case items <- rxgo.Of([]byte(msg.Payload)):
					}
				}
			}
		}()
	})
}

// Close 关闭消息队列
func (mq *redisMQ) Close() {
	close(mq.done)
}
