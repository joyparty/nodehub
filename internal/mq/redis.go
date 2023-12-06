package mq

import (
	"context"

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

// RedisMQ 是基于 Redis 的消息队列
type RedisMQ struct {
	client  RedisClient
	channel string
	sharded bool
	done    chan struct{}
}

// NewRedisMQ 构造函数
//
// client 可以使用 *redis.Client 或者 *redis.ClusterClient
//
// 当使用ClusterClient时，会采用sharded channel
func NewRedisMQ(client RedisClient, channel string) *RedisMQ {
	// 如果是集群客户端，使用shared pubsub
	_, shared := client.(*redis.ClusterClient)

	return &RedisMQ{
		client:  client,
		channel: channel,
		sharded: shared,
		done:    make(chan struct{}),
	}
}

// Publish 把消息发布到消息队列
func (mq *RedisMQ) Publish(ctx context.Context, payload []byte) error {
	var result *redis.IntCmd
	if mq.sharded {
		result = mq.client.SPublish(ctx, mq.channel, payload)
	} else {
		result = mq.client.Publish(ctx, mq.channel, payload)
	}
	return result.Err()
}

// Subscribe 从消息队列订阅消息
func (mq *RedisMQ) Subscribe(ctx context.Context) (<-chan []byte, error) {
	var pubsub *redis.PubSub

	if mq.sharded {
		pubsub = mq.client.SSubscribe(ctx, mq.channel)
	} else {
		pubsub = mq.client.Subscribe(ctx, mq.channel)
	}

	mc := pubsub.Channel()
	nc := make(chan []byte)

	go func() {
		defer func() {
			close(nc)

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

				data := []byte(msg.Payload)

				select {
				case <-ctx.Done():
					return
				case <-mq.done:
					return
				case nc <- data:
				}
			}
		}
	}()

	return nc, nil
}

// Close 关闭消息队列
func (mq *RedisMQ) Close() {
	close(mq.done)
}
