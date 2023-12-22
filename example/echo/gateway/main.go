package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/oklog/ulid/v2"
	"gitlab.haochang.tv/gopkg/nodehub"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/component/gateway"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/metadata"
)

var (
	registry        *cluster.Registry
	websocketListen string
	grpcListen      string
)

func init() {
	flag.StringVar(&websocketListen, "websocket", "127.0.0.1:9000", "websocket listen address")
	flag.StringVar(&grpcListen, "grpc", "127.0.0.1:10000", "grpc listen address")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	logger.SetLogger(slog.Default())

	client, err := clientv3.New(clientv3.Config{
		DialTimeout: 5 * time.Second,
		Endpoints:   []string{"127.0.0.1:2379"},
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
	node := nodehub.NewGatewayNode(registry, nodehub.GatewayConfig{
		WSProxyListen: websocketListen,
		WSProxyOption: []gateway.WSProxyOption{
			gateway.WithRequestLog(slog.Default()),
			gateway.WithAuthorize(func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool) {
				return ulid.Make().String(), metadata.MD{}, true
			}),
		},
		GRPCListen: grpcListen,
	})

	if err := registry.Put(node.Entry()); err != nil {
		panic(err)
	}

	if err := node.Serve(context.Background()); err != nil {
		panic(err)
	}
}
