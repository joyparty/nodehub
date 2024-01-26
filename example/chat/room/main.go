package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub"
	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/component/rpc"
	"github.com/joyparty/nodehub/event"
	"github.com/joyparty/nodehub/example/chat/proto/clusterpb"
	"github.com/joyparty/nodehub/example/chat/proto/roompb"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/multicast"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

var (
	registry *cluster.Registry

	listenAddr string
	redisAddr  string
	natsAddr   string
)

func init() {
	flag.StringVar(&listenAddr, "listen", "127.0.0.1:9100", "listen address")
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
	server := mustReturn(newGRPCServer())

	node := nodehub.NewNode("room", registry)
	node.AddComponent(server)

	logger.Info("room server start", "listen", listenAddr)
	mustDo(node.Serve(context.Background()))
}

func newGRPCServer() (*rpc.GRPCServer, error) {
	// evBus, muBus := newRedisBus()
	evBus, muBus := newNatsBus()

	service := &roomService{
		publisher: muBus,
		members:   gokit.NewMapOf[string, string](),
	}

	if err := evBus.Subscribe(context.Background(), func(ev event.UserConnected, _ time.Time) {
		service.boardcast(&roompb.News{
			Content: fmt.Sprintf("EVENT: #%s connected", ev.SessionID),
		})
	}); err != nil {
		panic(err)
	}

	if err := evBus.Subscribe(context.Background(), func(ev event.UserDisconnected, _ time.Time) {
		service.members.Delete(ev.SessionID)

		service.boardcast(&roompb.News{
			Content: fmt.Sprintf("EVENT: #%s disconnected", ev.SessionID),
		})
	}); err != nil {
		panic(err)
	}

	server := rpc.NewGRPCServer(listenAddr, grpc.UnaryInterceptor(rpc.LogUnary(slog.Default())))
	err := server.RegisterService(
		int32(clusterpb.Services_ROOM),
		roompb.Room_ServiceDesc,
		service,
		rpc.WithPublic(),
		rpc.WithStateful(),
		rpc.WithAllocation(cluster.AutoAllocate),
	)
	if err != nil {
		return nil, err
	}

	return server, nil
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
