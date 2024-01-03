package mq

import (
	"context"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/reactivex/rxgo/v2"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
)

type natsMQ struct {
	conn    *nats.Conn
	subject string

	observer      rxgo.Observable
	subscribeOnce *sync.Once
	done          chan struct{}
}

// NewNatsMQ 构造函数
func NewNatsMQ(conn *nats.Conn, subject string) Queue {
	return &natsMQ{
		conn:          conn,
		subject:       subject,
		subscribeOnce: &sync.Once{},
		done:          make(chan struct{}),
	}
}

func (mq *natsMQ) Publish(ctx context.Context, payload []byte) error {
	return mq.conn.Publish(mq.subject, payload)
}

func (mq *natsMQ) Subscribe(ctx context.Context, handler func([]byte)) {
	mq.subscribe()

	mq.observer.ForEach(
		func(item any) {
			handler(item.([]byte))
		},
		func(err error) {
			logger.Error("nats observer", "error", err, "subject", mq.subject)
		},
		func() {},
	)
}

func (mq *natsMQ) subscribe() {
	mq.subscribeOnce.Do(func() {
		items := make(chan rxgo.Item)
		mq.observer = rxgo.FromEventSource(items)

		sub, err := mq.conn.Subscribe(mq.subject, func(msg *nats.Msg) {
			items <- rxgo.Of(msg.Data)
		})

		if err != nil {
			mq.observer = rxgo.Defer([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
				next <- rxgo.Error(err)
			}})
			return
		}

		go func() {
			select {
			case <-mq.done:
				sub.Unsubscribe()
				close(items)
			}
		}()
	})
}

func (mq *natsMQ) Close() {
	close(mq.done)
}
