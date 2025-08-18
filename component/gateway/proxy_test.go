package gateway

import (
	"crypto/rand"
	"sync"
	"testing"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/proto"
)

func BenchmarkMessage(b *testing.B) {
	requestPool := sync.Pool{
		New: func() any {
			return &nh.Request{}
		},
	}

	testData := make([]byte, 1024)
	_ = gokit.MustReturn(rand.Read(testData))

	payload := gokit.MustReturn(proto.Marshal(nh.Request_builder{
		Id:          1,
		ServiceCode: 2,
		Method:      "foobar",
		Data:        testData,
		NodeId:      ulid.Make().String(),
	}.Build()))

	handle := func(req *nh.Request) {
		gokit.Must(proto.Unmarshal(payload, req))

		_ = req.GetData()
	}

	b.ReportAllocs()
	b.ResetTimer()

	b.Run("no pool", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				req := &nh.Request{}
				handle(req)

				_ = req
			}
		})
	})

	b.Run("pool", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				req := requestPool.Get().(*nh.Request)
				handle(req)

				req.Reset()
				requestPool.Put(req)
			}
		})
	})
}
