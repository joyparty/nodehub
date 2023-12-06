package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"time"

	"gitlab.haochang.tv/gopkg/nodehub"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/component/rpc"
	serverpb "gitlab.haochang.tv/gopkg/nodehub/example/echo/proto/server"
	pb "gitlab.haochang.tv/gopkg/nodehub/example/echo/proto/server/echo"
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

type server struct {
	*rpc.GRPCServer
}

func newGRPCServer() (*server, error) {
	s := rpc.NewGRPCServer(addr, grpc.UnaryInterceptor(rpc.LogUnary(slog.Default())))
	if err := s.RegisterPublicService(int32(serverpb.Services_ECHO), &pb.Echo_ServiceDesc, &echoService{}); err != nil {
		return nil, fmt.Errorf("register service, %w", err)
	}

	return &server{
		GRPCServer: s,
	}, nil
}

type echoService struct {
	pb.UnimplementedEchoServer
}

func (es *echoService) Send(ctx context.Context, msg *pb.Msg) (*clientpb.Response, error) {
	fmt.Printf("%s [%s]: receive msg: %s\n", time.Now().Format(time.RFC3339), addr, msg.GetMessage())

	return clientpb.NewResponse(int32(pb.Protocol_MSG), msg)
}
