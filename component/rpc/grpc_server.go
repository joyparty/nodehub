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
	// MDSessID grpc metadata中的session id key
	MDSessID = "x-sess"
	// MDTransactionID grpc metadata中的transaction id key，用于跟踪请求
	MDTransactionID = "x-trans"
	// MDGateway grpc metadata中的gateway key，用于标识请求来自哪个网关
	MDGateway = "x-gw"
)

const (
	// NodeServiceCode 节点管理服务，每个内部节点都会内置这个grpc服务
	NodeServiceCode int32 = -1
	// GatewayServiceCode 网关服务代码，每个网关节点都会内置这个grpc服务
	GatewayServiceCode int32 = -2
)

// GRPCServer grpc服务
type GRPCServer struct {
	listenAddr string
	server     *grpc.Server
	services   map[int32]cluster.GRPCServiceDesc
}

// NewGRPCServer 构造函数
func NewGRPCServer(listenAddr string, opts ...grpc.ServerOption) *GRPCServer {
	return &GRPCServer{
		listenAddr: listenAddr,
		server:     grpc.NewServer(opts...),
		services:   make(map[int32]cluster.GRPCServiceDesc),
	}
}

// RegisterService 注册服务
func (gs *GRPCServer) RegisterService(code int32, desc grpc.ServiceDesc, impl any, options ...Option) error {
	sd := cluster.GRPCServiceDesc{
		Code:     code,
		Path:     fmt.Sprintf("/%s", desc.ServiceName),
		Balancer: cluster.BalancerRandom,
	}
	for _, opt := range options {
		sd = opt(sd)
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
	l, err := net.Listen("tcp", gs.listenAddr)
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
	gs.server.GracefulStop()
	return nil
}

// CompleteNodeEntry 设置条目中的grpc信息
func (gs *GRPCServer) CompleteNodeEntry(entry *cluster.NodeEntry) {
	entry.GRPC = cluster.GRPCEntry{
		Endpoint: gs.listenAddr,
		Services: lo.Values(gs.services),
	}
}

// Option 配置
type Option func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc

// WithPublic 设置服务为公开
func WithPublic() Option {
	return func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc {
		desc.Public = true
		return desc
	}
}

// WithStateful 设置服务为有状态
func WithStateful() Option {
	return func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc {
		desc.Stateful = true
		desc.Allocation = cluster.ServerAllocate
		return desc
	}
}

// WithAllocation 设置节点分配方式
func WithAllocation(allocation string) Option {
	return func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc {
		desc.Allocation = allocation
		return desc
	}
}

// WithPipeline 设置管道名称
//
// 设置了管道名称的请求会按照时序性顺序处理，没有设置管道的请求会并发处理
//
// 多个服务可以声明同一个管道名称，这样请求会被分配到同一个管道中
func WithPipeline(pipeline string) Option {
	return func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc {
		desc.Pipeline = pipeline
		return desc
	}
}

// WithBalancer 设置负载均衡策略
func WithBalancer(balancer string) Option {
	return func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc {
		desc.Balancer = balancer
		return desc
	}
}

// WithWeight 设置节点权重
func WithWeight(weight int) Option {
	return func(desc cluster.GRPCServiceDesc) cluster.GRPCServiceDesc {
		desc.Weight = weight
		return desc
	}
}
