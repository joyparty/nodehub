package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"sync"

	"github.com/redis/go-redis/v9"
	"gitlab.haochang.tv/gopkg/nodehub"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/component/rpc"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/roompb"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/servicepb"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/notification"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

var (
	registry  *cluster.Registry
	publisher notification.Publisher

	listenAddr string
	redisAddr  string
)

func init() {
	flag.StringVar(&listenAddr, "listen", "127.0.0.1:9100", "listen address")
	flag.StringVar(&redisAddr, "redis", "127.0.0.1:6379", "redis address")
	flag.Parse()

	logger.SetLogger(slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	))

	client := mustReturn(clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	}))

	registry = mustReturn(cluster.NewRegistry(client))

	redisClient := redis.NewClient(&redis.Options{
		Network: "tcp",
		Addr:    redisAddr,
	})
	publisher = notification.NewRedisMQ(redisClient, "chat")
}

func main() {
	server := mustReturn(newGRPCServer())

	node := nodehub.NewNode("room")
	node.AddComponent(server)
	mustDo(registry.Put(node.Entry()))

	logger.Info("room server start", "listen", listenAddr)
	mustDo(node.Serve(context.Background()))
}

func newGRPCServer() (*rpc.GRPCServer, error) {
	service := &roomService{
		publisher: publisher,
		members:   &sync.Map{},
	}

	server := rpc.NewGRPCServer(listenAddr, grpc.UnaryInterceptor(rpc.LogUnary(slog.Default())))
	if err := server.RegisterPublicService(int32(servicepb.Services_ROOM), &roompb.Room_ServiceDesc, service); err != nil {
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
