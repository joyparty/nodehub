package mq

import (
	"context"
	"sync"

	"github.com/reactivex/rxgo/v2"
	"github.com/redis/go-redis/v9"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
)

// RedisClient 实现了PubSub方法的客户端接口
type RedisClient interface {
	Publish(ctx context.Context, channel string, message any) *redis.IntCmd
	SPublish(ctx context.Context, channel string, message any) *redis.IntCmd
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
	SSubscribe(ctx context.Context, channels ...string) *redis.PubSub
}

// redisMQ 是基于 Redis 的消息队列
type redisMQ struct {
	client  RedisClient
	channel string
	sharded bool
	done    chan struct{}

	observer      rxgo.Observable
	subscribeOnce *sync.Once
}

// NewRedisMQ 构造函数
//
// client 可以使用 *redis.Client 或者 *redis.ClusterClient
//
// 当使用ClusterClient时，会采用sharded channel
func NewRedisMQ(client RedisClient, channel string) Queue {
	// 如果是集群客户端，使用shared pubsub
	_, shared := client.(*redis.ClusterClient)

	return &redisMQ{
		client:        client,
		channel:       channel,
		sharded:       shared,
		done:          make(chan struct{}),
		subscribeOnce: &sync.Once{},
	}
}

// Publish 把消息发布到消息队列
func (mq *redisMQ) Publish(ctx context.Context, payload []byte) error {
	var result *redis.IntCmd
	if mq.sharded {
		result = mq.client.SPublish(ctx, mq.channel, payload)
	} else {
		result = mq.client.Publish(ctx, mq.channel, payload)
	}
	return result.Err()
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
		var (
			pubsub *redis.PubSub
			ctx    = context.Background()
		)

		if mq.sharded {
			pubsub = mq.client.SSubscribe(ctx, mq.channel)
		} else {
			pubsub = mq.client.Subscribe(ctx, mq.channel)
		}

		items := make(chan rxgo.Item)
		mq.observer = rxgo.FromEventSource(items, rxgo.WithErrorStrategy(rxgo.ContinueOnError))

		mc := pubsub.Channel()
		go func() {
			defer func() {
				close(items)

				if mq.sharded {
					pubsub.SUnsubscribe(ctx)
				} else {
					pubsub.Unsubscribe(ctx)
				}
				pubsub.Close()
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
