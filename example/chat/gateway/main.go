package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/joyparty/gokit"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"gitlab.haochang.tv/gopkg/nodehub"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/component/gateway"
	"gitlab.haochang.tv/gopkg/nodehub/event"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/multicast"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/metadata"
)

var (
	registry *cluster.Registry

	websocketListen string
	grpcListen      string
	redisAddr       string
	natsAddr        string
)

func init() {
	flag.StringVar(&websocketListen, "websocket", "127.0.0.1:9000", "websocket listen address")
	flag.StringVar(&grpcListen, "grpc", "127.0.0.1:10000", "grpc listen address")
	flag.StringVar(&redisAddr, "redis", "127.0.0.1:6379", "redis address")
	flag.StringVar(&natsAddr, "nats", "127.0.0.1:4222", "nats address")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	logger.SetLogger(slog.Default())

	client := mustReturn(clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	}))

	registry = mustReturn(cluster.NewRegistry(client))
}

func main() {
	// evBus, muBus := newRedisBus()
	evBus, muBus := newNatsBus()

	uid := &atomic.Int32{}
	node := nodehub.NewGatewayNode(registry, nodehub.GatewayConfig{
		WSProxyListen: websocketListen,
		WSProxyOption: []gateway.WSProxyOption{
			gateway.WithMulticastSubscriber(muBus),
			gateway.WithEventBus(evBus),
			gateway.WithRequestLog(slog.Default()),
			gateway.WithAuthorize(func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool) {
				userID = fmt.Sprintf("%d", uid.Add(1))
				md = metadata.MD{}
				ok = true
				return
			}),
		},
		GRPCListen: grpcListen,
	})

	logger.Info("gateway server start", "listen", websocketListen)
	mustDo(node.Serve(context.Background()))
}

func mustDo(err error) {
	if err != nil {
		panic(err)
	}
}

func mustReturn[T any](t T, er error) T {
	if er != nil {
		panic(er)
	}
	return t
}

func newRedisBus() (*event.Bus, *multicast.Bus) {
	client := redis.NewClient(&redis.Options{
		Network: "tcp",
		Addr:    redisAddr,
	})
	return event.NewRedisBus(client), multicast.NewRedisBus(client)
}

func newNatsBus() (*event.Bus, *multicast.Bus) {
	dsn := fmt.Sprintf("nats://%s", natsAddr)
	conn := gokit.MustReturn(nats.Connect(dsn))
	return event.NewNatsBus(conn), multicast.NewNatsBus(conn)
}
