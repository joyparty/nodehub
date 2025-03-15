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
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

var (
	registry *cluster.Registry

	listenAddr string
	natsURL    string
)

func init() {
	flag.StringVar(&listenAddr, "listen", "127.0.0.1:9100", "listen address")
	flag.StringVar(&natsURL, "nats", "nats://127.0.0.1:4222,nats://127.0.0.1:5222,nats://127.0.0.1:6222", "nats address")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	logger.SetLogger(slog.Default())

	client := gokit.MustReturn(clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379", "127.0.0.1:2479", "127.0.0.1:2579"},
	}))

	registry = gokit.MustReturn(cluster.NewRegistry(client))
}

func main() {
	server := gokit.MustReturn(newGRPCServer())

	node := nodehub.NewNode("room", registry)
	node.AddComponent(server)

	logger.Info("room server start", "listen", listenAddr)
	gokit.Must(node.Serve(context.Background()))
}

func newGRPCServer() (*rpc.GRPCServer, error) {
	// evBus, muBus := newRedisBus()
	evBus, muBus := newNatsBus()

	service := &roomService{
		publisher: muBus,
		members:   gokit.NewMapOf[string, string](),
	}

	evBus.Subscribe(context.Background(), func(ev event.UserConnected, _ time.Time) {
		service.boardcast(&roompb.News{
			Content: fmt.Sprintf("EVENT: #%s connected", ev.UserID),
		})
	})

	evBus.Subscribe(context.Background(), func(ev event.UserDisconnected, _ time.Time) {
		service.members.Delete(ev.UserID)

		service.boardcast(&roompb.News{
			Content: fmt.Sprintf("EVENT: #%s disconnected", ev.UserID),
		})
	})

	server := rpc.NewGRPCServer(listenAddr,
		grpc.ChainUnaryInterceptor(
			rpc.LogUnary(slog.Default()),
			rpc.PackReply(roompb.Room_MethodReplyCodes),
		),
	)

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

func newNatsBus() (*event.Bus, *multicast.Bus) {
	conn := gokit.MustReturn(nats.Connect(natsURL))
	return event.NewNatsBus(conn), multicast.NewNatsBus(conn)
}
