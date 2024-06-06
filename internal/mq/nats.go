package mq

import (
	"context"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/reactivex/rxgo/v2"
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

	mq.observer.
		DoOnNext(
			func(item any) {
				handler(item.([]byte))
			},
			rxgo.WithContext(ctx),
		)
}

func (mq *natsMQ) subscribe() {
	mq.subscribeOnce.Do(func() {
		msgC := make(chan *nats.Msg, 64)
		sub, err := mq.conn.ChanSubscribe(mq.subject, msgC)
		if err != nil {
			mq.observer = rxgo.Defer([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
				next <- rxgo.Error(err)
			}}, rxgo.WithErrorStrategy(rxgo.ContinueOnError))
			return
		}

		items := make(chan rxgo.Item)
		mq.observer = rxgo.FromEventSource(items, rxgo.WithErrorStrategy(rxgo.ContinueOnError))

		go func() {
			defer sub.Unsubscribe()
			defer close(items)

			for {
				select {
				case <-mq.done:
					return
				case msg, ok := <-msgC:
					if !ok {
						return
					}

					select {
					case <-mq.done:
						return
					case items <- rxgo.Of(msg.Data):
					}
				}
			}
		}()
	})
}

func (mq *natsMQ) Close() {
	close(mq.done)
}
