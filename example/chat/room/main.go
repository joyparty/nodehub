package main

import (
	"context"
	"flag"
	"log/slog"
	"nodehub"
	"nodehub/cluster"
	"nodehub/component/rpc"
	"nodehub/example/chat/proto/roompb"
	"nodehub/example/chat/proto/servicepb"
	"nodehub/logger"
	"nodehub/notification"
	"os"
	"sync"

	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
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

	server := rpc.NewGRPCServer(listenAddr)
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
