package rpc

import (
	"context"
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
	services map[int32]cluster.GRPCServiceDesc
}

// NewGRPCServer 构造函数
func NewGRPCServer(endpoint string, opts ...grpc.ServerOption) *GRPCServer {
	return &GRPCServer{
		endpoint: endpoint,
		server:   grpc.NewServer(opts...),
		services: make(map[int32]cluster.GRPCServiceDesc),
	}
}

// RegisterService 注册服务
func (gs *GRPCServer) RegisterService(code int32, desc grpc.ServiceDesc, impl any, config ...Config) error {
	sd := cluster.GRPCServiceDesc{
		Code: code,
		Path: fmt.Sprintf("/%s", desc.ServiceName),
	}
	for _, c := range config {
		sd = c(sd)
	}

	if err := sd.Validate(); err != nil {
		return err
	} else if _, ok := gs.services[sd.Code]; ok {
		return fmt.Errorf("service code %d already registered", sd.Code)
	}

	gs.services[sd.Code] = sd
	gs.server.RegisterService(&desc, impl)
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
		Services: lo.Values(gs.services),
	}
}

// Config 配置
type Config func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc

// WithPublic 设置服务为公开
func WithPublic() Config {
	return func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc {
		desc.Public = true
		return desc
	}
}

// WithStateful 设置服务为有状态
func WithStateful() Config {
	return func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc {
		desc.Stateful = true
		desc.Allocation = cluster.ExplicitAllocate
		return desc
	}
}

// WithAllocation 设置节点分配方式
func WithAllocation(allocation string) Config {
	return func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc {
		desc.Allocation = allocation
		return desc
	}
}

// WithUnordered 设置此服务的所有接口请求不需要保证时序性
func WithUnordered() Config {
	return func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc {
		desc.Unordered = true
		return desc
	}
}
