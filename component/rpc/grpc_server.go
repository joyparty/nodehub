package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/samber/lo"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"google.golang.org/grpc"
)

const (
	// MDUserID grpc metadata中的user id key
	MDUserID = "x-user-id"
	// MDTransactionID grpc metadata中的transaction id key，用于跟踪请求
	MDTransactionID = "x-trans-id"
)

// GRPCServer grpc服务
type GRPCServer struct {
	endpoint string
	server   *grpc.Server
	services []grpcServiceDesc
}

type grpcServiceDesc struct {
	Code   int32
	Desc   *grpc.ServiceDesc
	Public bool
}

// NewGRPCServer 构造函数
func NewGRPCServer(endpoint string, opts ...grpc.ServerOption) *GRPCServer {
	return &GRPCServer{
		endpoint: endpoint,
		server:   grpc.NewServer(opts...),
	}
}

// RegisterPublicService 注册服务
func (gs *GRPCServer) RegisterPublicService(code int32, desc *grpc.ServiceDesc, impl any) error {
	return gs.registerService(impl, grpcServiceDesc{
		Code:   code,
		Desc:   desc,
		Public: true,
	})
}

// RegisterPrivateService 注册内部服务
func (gs *GRPCServer) RegisterPrivateService(code int32, desc *grpc.ServiceDesc, impl any) error {
	return gs.registerService(impl, grpcServiceDesc{
		Code:   code,
		Desc:   desc,
		Public: false,
	})
}

func (gs *GRPCServer) registerService(impl any, service grpcServiceDesc) error {
	if service.Code == 0 {
		return errors.New("code must not be 0")
	}

	for _, v := range gs.services {
		if v.Code == service.Code {
			return fmt.Errorf("code %d already registered", service.Code)
		}
	}

	gs.services = append(gs.services, service)
	gs.server.RegisterService(service.Desc, impl)
	return nil
}

// Name 服务名称
func (gs *GRPCServer) Name() string {
	return "grpc"
}

// Start 启动服务
func (gs *GRPCServer) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", gs.endpoint)
	if err != nil {
		return fmt.Errorf("listen tcp, %w", err)
	}

	go func() {
		if err := gs.server.Serve(l); err != nil {
			logger.Error("start grpc", "error", err)

			panic(fmt.Errorf("start grpc, %w", err))
		}
	}()

	return nil
}

// Stop 停止服务
func (gs *GRPCServer) Stop(ctx context.Context) error {
	gs.server.Stop()
	return nil
}

// CompleteNodeEntry 设置条目中的grpc信息
func (gs *GRPCServer) CompleteNodeEntry(entry *cluster.NodeEntry) {
	entry.GRPC = cluster.GRPCEntry{
		Endpoint: gs.endpoint,
		Services: lo.Map(gs.services, func(s grpcServiceDesc, _ int) cluster.GRPCServiceDesc {
			return cluster.GRPCServiceDesc{
				Code:   s.Code,
				Path:   fmt.Sprintf("/%s", s.Desc.ServiceName),
				Public: s.Public,
			}
		}),
	}
}
