package main

import (
	"context"
	"fmt"
	"log/slog"
	"nodehub"
	"nodehub/component/gateway"
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
	registry   *nodehub.Registry
	resolver   *nodehub.GRPCResolver
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

	resolver = nodehub.NewGRPCResolver()

	registry, err = nodehub.NewRegistry(etcdClient,
		nodehub.WithEventHandler(func(event nodehub.RegistryEvent, entry nodehub.NodeEntry) {
			fmt.Printf("event: %d, entry: %+v\n", event, entry)

			switch event {
			case nodehub.PUT:
				resolver.Update(entry)
			case nodehub.DELETE:
				resolver.Remove(entry)
			}
		}),
	)
	if err != nil {
		panic(err)
	}

}

func main() {
	node := &nodehub.Node{}

	node.AddComponent(&gateway.WebsocketProxy{
		Resolver:   resolver,
		ListenAddr: "127.0.0.1:9000",
	})

	entry := nodehub.NodeEntry{
		ID:    fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:  "gateway",
		State: nodehub.NodeOK,
	}
	if err := registry.Put(entry); err != nil {
		panic(err)
	}

	if err := node.Serve(context.Background()); err != nil {
		panic(err)
	}
}

type server struct {
	*rpc.GRPCServer
}

func newGRPCServer() (*server, error) {
	s := rpc.NewGRPCServer("127.0.0.1:9000")
	if err := s.RegisterService(
		int32(serverpb.Services_ECHO),
		&pb.Echo_ServiceDesc,
		&echoService{},
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
