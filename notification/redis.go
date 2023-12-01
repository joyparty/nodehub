package notification

import (
	"context"
	"nodehub/internal/mq"
	"nodehub/logger"
	"nodehub/proto/gatewaypb"

	"google.golang.org/protobuf/proto"
)

// RedisClient 实现了PubSub方法的客户端接口
type RedisClient = mq.RedisClient

// RedisMQ 是基于 Redis 的消息队列
type RedisMQ struct {
	core *mq.RedisMQ
}

// NewRedisMQ 构造函数
//
// client 可以使用 *redis.Client 或者 *redis.ClusterClient
//
// 当使用ClusterClient时，会采用sharded channel
func NewRedisMQ(client RedisClient, channel string) *RedisMQ {
	return &RedisMQ{
		core: mq.NewRedisMQ(client, channel),
	}
}

// Publish 把消息发布到消息队列
func (rq *RedisMQ) Publish(ctx context.Context, message *gatewaypb.Notification) error {
	payload, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	return rq.core.Publish(ctx, payload)
}

// Subscribe 从消息队列订阅消息
func (rq *RedisMQ) Subscribe(ctx context.Context) (<-chan *gatewaypb.Notification, error) {
	payloadC, err := rq.core.Subscribe(ctx)
	if err != nil {
		return nil, err
	}

	resultC := make(chan *gatewaypb.Notification)
	go func() {
		defer close(resultC)

		for payload := range payloadC {
			n := &gatewaypb.Notification{}
			if err := proto.Unmarshal(payload, n); err != nil {
				logger.Error("unmarshal notification", "error", err)
				continue
			}
			select {
			case <-ctx.Done():
				return
			case resultC <- n:
			}
		}
	}()

	return resultC, nil
}

// Close 关闭消息队列
func (rq *RedisMQ) Close() {
	rq.core.Close()
}
