package mq

import (
	"context"

	"github.com/nats-io/nats.go"
)

type natsMQ struct {
	conn    *nats.Conn
	subject string
	done    chan struct{}
}

// NewNatsMQ 构造函数
func NewNatsMQ(conn *nats.Conn, subject string) Queue {
	return &natsMQ{
		conn:    conn,
		subject: subject,
		done:    make(chan struct{}),
	}
}

func (mq *natsMQ) Topic() string {
	return mq.subject
}

func (mq *natsMQ) Publish(ctx context.Context, payload []byte) error {
	return mq.conn.Publish(mq.subject, payload)
}

func (mq *natsMQ) Subscribe(ctx context.Context) (<-chan []byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	msgC := make(chan []byte, 100)

	sub, err := mq.conn.Subscribe(mq.subject, func(msg *nats.Msg) {
		select {
		case <-ctx.Done():
		case msgC <- msg.Data:
		}
	})
	if err != nil {
		cancel()
		return nil, err
	}

	go func() {
		select {
		case <-mq.done:
		case <-ctx.Done():
		}

		sub.Unsubscribe()
		cancel()
		close(msgC)
	}()

	return msgC, nil
}

func (mq *natsMQ) Close() {
	close(mq.done)
}
