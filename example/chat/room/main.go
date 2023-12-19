package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/joyparty/gokit"
	"github.com/redis/go-redis/v9"
	"gitlab.haochang.tv/gopkg/nodehub"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/component/rpc"
	"gitlab.haochang.tv/gopkg/nodehub/event"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/clusterpb"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/roompb"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/notification"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

var (
	registry  *cluster.Registry
	publisher notification.Publisher
	eventBus  event.Bus

	listenAddr string
	redisAddr  string
)

func init() {
	flag.StringVar(&listenAddr, "listen", "127.0.0.1:9100", "listen address")
	flag.StringVar(&redisAddr, "redis", "127.0.0.1:6379", "redis address")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	logger.SetLogger(slog.Default())

	client := mustReturn(clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	}))

	registry = mustReturn(cluster.NewRegistry(client))

	redisClient := redis.NewClient(&redis.Options{
		Network: "tcp",
		Addr:    redisAddr,
	})
	publisher = notification.NewRedisMQ(redisClient, "chat")
	eventBus = event.NewBus(redisClient)
}

func main() {
	server := mustReturn(newGRPCServer())

	node := nodehub.NewNode("room", registry)
	node.AddComponent(server)

	logger.Info("room server start", "listen", listenAddr)
	mustDo(node.Serve(context.Background()))
}

func newGRPCServer() (*rpc.GRPCServer, error) {
	service := &roomService{
		publisher: publisher,
		members:   gokit.NewMapOf[string, string](),
	}

	if err := eventBus.Subscribe(context.Background(), func(ev event.UserConnected) {
		service.boardcast(&roompb.News{
			Content: fmt.Sprintf("EVENT: #%s connected", ev.UserID),
		})
	}); err != nil {
		panic(err)
	}

	if err := eventBus.Subscribe(context.Background(), func(ev event.UserDisconnected) {
		service.members.Delete(ev.UserID)

		service.boardcast(&roompb.News{
			Content: fmt.Sprintf("EVENT: #%s disconnected", ev.UserID),
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
		rpc.WithAllocation(cluster.ClientAllocate),
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
