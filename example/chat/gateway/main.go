package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"nodehub"
	"nodehub/cluster"
	"nodehub/component/gateway"
	"nodehub/logger"
	"nodehub/notification"
	"os"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/metadata"
)

var (
	registry   *cluster.Registry
	subscriber notification.Subscriber

	listenAddr string
	redisAddr  string
)

func init() {
	flag.StringVar(&listenAddr, "listen", "127.0.0.1:9000", "listen address")
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
	subscriber = notification.NewRedisMQ(redisClient, "chat")
}

func main() {
	uid := &atomic.Int32{}
	proxy := &gateway.WebsocketProxy{
		Registry:   registry,
		ListenAddr: listenAddr,
		Notifier:   subscriber,

		Authorize: func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool) {
			userID = fmt.Sprintf("%d", uid.Add(1))
			md = metadata.MD{}
			ok = true
			return
		},
	}

	node := nodehub.NewNode("gateway")
	node.AddComponent(proxy)
	mustDo(registry.Put(node.Entry()))

	logger.Info("gateway server start", "listen", listenAddr)
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
