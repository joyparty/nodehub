package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/joyparty/nodehub"
	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/component/rpc"
	"github.com/joyparty/nodehub/example/echo/proto/clusterpb"
	"github.com/joyparty/nodehub/example/echo/proto/echopb"
	"github.com/joyparty/nodehub/logger"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

var (
	registry *cluster.Registry
	listen   string
	balancer string
	weight   int
)

func init() {
	flag.StringVar(&listen, "listen", "127.0.0.1:9001", "listen address")
	flag.StringVar(&balancer, "balancer", "random", "balancer policy")
	flag.IntVar(&weight, "weight", 0, "weight")
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
	node := nodehub.NewNode("echo", registry)

	grpcServer, err := newGRPCServer()
	if err != nil {
		panic(err)
	}
	node.AddComponent(grpcServer)

	if err := node.Serve(context.Background()); err != nil {
		panic(err)
	}
}

func newGRPCServer() (*rpc.GRPCServer, error) {
	gs := rpc.NewGRPCServer(listen,
		grpc.ChainUnaryInterceptor(
			rpc.LogUnary(slog.Default()),
			rpc.PackReply(echopb.Echo_MethodReplyCodes),
		),
	)

	options := []rpc.Option{
		rpc.WithPublic(),
		rpc.WithBalancer(balancer),
	}
	if weight > 0 {
		options = append(options, rpc.WithWeight(weight))
	}

	err := gs.RegisterService(
		int32(clusterpb.Services_ECHO),
		echopb.Echo_ServiceDesc,
		&echoService{},
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("register service, %w", err)
	}

	return gs, nil
}

type echoService struct {
	echopb.UnimplementedEchoServer
}

func (es *echoService) Send(ctx context.Context, msg *echopb.Msg) (*echopb.Msg, error) {
	return msg, nil
}
