package main

import (
	"context"
	"log/slog"
	"nodehub"
	"nodehub/cluster"
	"nodehub/component/gateway"
	"nodehub/logger"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	registry *cluster.Registry
)

func init() {
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
	node := nodehub.NewNode("gateway")

	node.AddComponent(&gateway.WebsocketProxy{
		Registry:   registry,
		ListenAddr: "127.0.0.1:9000",
	})

	if err := registry.Put(node.Entry()); err != nil {
		panic(err)
	}

	if err := node.Serve(context.Background()); err != nil {
		panic(err)
	}
}
