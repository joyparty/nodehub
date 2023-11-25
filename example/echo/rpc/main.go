package main

import (
	"context"
	"fmt"
	"log/slog"
	"nodehub"
	"nodehub/cluster"
	"nodehub/component/rpc"
	serverpb "nodehub/example/echo/proto/server"
	pb "nodehub/example/echo/proto/server/echo"
	"nodehub/logger"
	clientpb "nodehub/proto/client"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"
)

var (
	etcdClient *clientv3.Client
	registry   *cluster.Registry
)

func init() {
	logger.SetLogger(slog.Default())

	var err error
	etcdClient, err = clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	})
	if err != nil {
		panic(err)
	}

	registry, err = cluster.NewRegistry(etcdClient)
	if err != nil {
		panic(err)
	}

}

func main() {
	node := &nodehub.Node{}

	grpcServer, err := newGRPCServer()
	if err != nil {
		panic(err)
	}
	node.AddComponent(grpcServer)

	entry := cluster.NodeEntry{
		ID:    fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:  "echo",
		State: cluster.NodeOK,
		GRPC:  grpcServer.ToEntry(),
	}
	if err := registry.Put(entry); err != nil {
		panic(err)
	}

	logger.Info("start echo service", "listen", "127.0.0.1:9001")
	if err := node.Serve(context.Background()); err != nil {
		panic(err)
	}
}

type server struct {
	*rpc.GRPCServer
}

func newGRPCServer() (*server, error) {
	s := rpc.NewGRPCServer("127.0.0.1:9001")
	if err := s.RegisterService(
		int32(serverpb.Services_ECHO),
		&pb.Echo_ServiceDesc,
		&echoService{},
		rpc.WithPublic(),
	); err != nil {
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
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal msg, %w", err)
	}

	return &clientpb.Response{
		Route: int32(pb.Protocol_MSG),
		Data:  data,
	}, nil
}
