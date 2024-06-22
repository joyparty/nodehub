package mq

import (
	"context"
	"fmt"

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
	subC := make(chan *nats.Msg)
	sub, err := mq.conn.ChanSubscribe(mq.subject, subC)
	if err != nil {
		close(subC)
		return nil, fmt.Errorf("subscribe to %s, %w", mq.subject, err)
	}

	msgC := make(chan []byte)
	go func() {
		defer sub.Unsubscribe()
		defer close(msgC)

		for {
			select {
			case <-mq.done:
				return
			case <-ctx.Done():
				return
			case msg, ok := <-subC:
				if !ok {
					return
				}

				select {
				case <-mq.done:
					return
				case msgC <- msg.Data:
				}
			}
		}
	}()

	return msgC, nil
}

func (mq *natsMQ) Close() {
	close(mq.done)
}
