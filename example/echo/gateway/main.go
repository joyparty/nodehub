package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"gitlab.haochang.tv/gopkg/nodehub"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/component/gateway"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	registry *cluster.Registry
	addr     string
)

func init() {
	flag.StringVar(&addr, "addr", "127.0.0.1:9000", "listen address")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	logger.SetLogger(slog.Default())

	client, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	})
	if err != nil {
		panic(err)
	}

	registry, err = cluster.NewRegistry(client)
	if err != nil {
		panic(err)
	}

}

func main() {
	node := nodehub.NewNode("gateway", registry)
	node.AddComponent(gateway.NewWSProxy(registry, addr, gateway.WithRequestLog(slog.Default())))

	if err := registry.Put(node.Entry()); err != nil {
		panic(err)
	}

	if err := node.Serve(context.Background()); err != nil {
		panic(err)
	}
}
