package mq

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisMQ 是基于 Redis 的消息队列
type redisMQ struct {
	client  *redis.Client
	channel string
	done    chan struct{}
}

// NewRedisMQ 构造函数
//
// client 可以使用 *redis.Client 或者 *redis.ClusterClient
//
// 当使用ClusterClient时，会采用sharded channel
func NewRedisMQ(client *redis.Client, channel string) Queue {
	return &redisMQ{
		client:  client,
		channel: channel,
		done:    make(chan struct{}),
	}
}

func (mq *redisMQ) Topic() string {
	return mq.channel
}

// Publish 把消息发布到消息队列
func (mq *redisMQ) Publish(ctx context.Context, payload []byte) error {
	return mq.client.Publish(ctx, mq.channel, payload).Err()
}

func (mq *redisMQ) Subscribe(ctx context.Context) (<-chan []byte, error) {
	msgC := make(chan []byte)

	go func() {
		pubsub := mq.client.Subscribe(ctx, mq.channel)

		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			pubsub.Unsubscribe(ctx)
			pubsub.Close()
			close(msgC)
		}()

		for {
			select {
			case <-mq.done:
				return
			case <-ctx.Done():
				return
			case msg, ok := <-pubsub.Channel():
				if !ok {
					return
				}

				select {
				case <-mq.done:
					return
				case msgC <- []byte(msg.Payload):
				default:
					// drop
				}
			}
		}
	}()

	return msgC, nil
}

// Close 关闭消息队列
func (mq *redisMQ) Close() {
	close(mq.done)
}
