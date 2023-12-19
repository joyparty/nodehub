package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"gitlab.haochang.tv/gopkg/nodehub"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/component/rpc"
	"gitlab.haochang.tv/gopkg/nodehub/example/echo/proto/clusterpb"
	"gitlab.haochang.tv/gopkg/nodehub/example/echo/proto/echopb"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/proto/clientpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

var (
	registry *cluster.Registry
	addr     string
)

func init() {
	flag.StringVar(&addr, "addr", "127.0.0.1:9001", "listen address")
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
	grpcServer, err := newGRPCServer()
	if err != nil {
		panic(err)
	}

	node := nodehub.NewNode("echo", registry)
	node.AddComponent(grpcServer)

	if err := node.Serve(context.Background()); err != nil {
		panic(err)
	}
}

func newGRPCServer() (*rpc.GRPCServer, error) {
	gs := rpc.NewGRPCServer(addr, grpc.UnaryInterceptor(rpc.LogUnary(slog.Default())))

	err := gs.RegisterService(
		int32(clusterpb.Services_ECHO),
		echopb.Echo_ServiceDesc,
		&echoService{},
		rpc.WithPublic(),
		rpc.WithUnordered(),
	)
	if err != nil {
		return nil, fmt.Errorf("register service, %w", err)
	}

	return gs, nil
}

type echoService struct {
	echopb.UnimplementedEchoServer
}

func (es *echoService) Send(ctx context.Context, msg *echopb.Msg) (*clientpb.Response, error) {
	fmt.Printf("%s [%s]: receive msg: %s\n", time.Now().Format(time.RFC3339), addr, msg.GetMessage())

	return clientpb.NewResponse(int32(echopb.Protocol_MSG), msg)
}
