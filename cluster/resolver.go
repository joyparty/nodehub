package cluster

import (
	"fmt"
	"math/rand"

	"github.com/joyparty/gokit"
	"github.com/oklog/ulid/v2"
	"github.com/samber/lo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// grpcResolver grpc服务发现
type grpcResolver struct {
	allNodes *gokit.MapOf[ulid.ULID, NodeEntry]

	// serviceCode => GRPCServiceDesc
	services *gokit.MapOf[int32, GRPCServiceDesc]

	serviceNodes *gokit.MapOf[int32, []NodeEntry]

	// 每个服务的负载均衡器
	// serviceCode => Balancer
	serviceBalancer *gokit.MapOf[int32, Balancer]

	// endpoint => *grpc.ClientConn
	conns *gokit.MapOf[string, *grpc.ClientConn]

	dialOptions []grpc.DialOption
}

// newGRPCResolver 创建grpc服务发现
func newGRPCResolver(dialOptions ...grpc.DialOption) *grpcResolver {
	return &grpcResolver{
		allNodes:        gokit.NewMapOf[ulid.ULID, NodeEntry](),
		services:        gokit.NewMapOf[int32, GRPCServiceDesc](),
		serviceNodes:    gokit.NewMapOf[int32, []NodeEntry](),
		serviceBalancer: gokit.NewMapOf[int32, Balancer](),
		conns:           gokit.NewMapOf[string, *grpc.ClientConn](),
		dialOptions: append([]grpc.DialOption{
			// 内部服务节点之间不需要加密
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}, dialOptions...),
	}
}

// Update 更新条目
func (r *grpcResolver) Update(node NodeEntry) {
	if len(node.GRPC.Services) == 0 {
		return
	}

	r.allNodes.Store(node.ID, node)

	for _, desc := range node.GRPC.Services {
		// code为负数的是框架内置服务，不需要服务发现
		if desc.Code < 0 {
			continue
		}

		// 网关可以一直开启不重启，所以允许新的节点配置覆盖已有配置
		r.services.Store(desc.Code, desc)
		r.updateServiceNodes(desc.Code)
		r.updateBalancer(desc.Code)
	}
}

// Remove 删除条目
func (r *grpcResolver) Remove(node NodeEntry) {
	if len(node.GRPC.Services) == 0 {
		return
	}

	r.allNodes.Delete(node.ID)
	for _, desc := range node.GRPC.Services {
		r.updateServiceNodes(desc.Code)
		r.updateBalancer(desc.Code)
	}

	if conn, ok := r.conns.LoadAndDelete(node.GRPC.Endpoint); ok {
		conn.Close()
	}
}

// updateServiceNodes 更新服务可用节点
func (r *grpcResolver) updateServiceNodes(serviceCode int32) {
	nodes := []NodeEntry{}
	r.allNodes.Range(func(_ ulid.ULID, node NodeEntry) bool {
		if node.State == NodeOK &&
			lo.SomeBy(node.GRPC.Services, func(desc GRPCServiceDesc) bool {
				return desc.Code == serviceCode
			}) {
			nodes = append(nodes, node)
		}
		return true
	})

	if len(nodes) == 0 {
		r.serviceNodes.Delete(serviceCode)
	} else {
		r.serviceNodes.Store(serviceCode, nodes)
	}
}

func (r *grpcResolver) updateBalancer(serviceCode int32) {
	// 私有服务不使用负载均衡
	desc, ok := r.services.Load(serviceCode)
	if !ok || !desc.Public {
		return
	}

	nodes, _ := r.serviceNodes.Load(serviceCode)

	// balancer不考虑在使用过程中增减节点，每次节点变更都重新生成相关服务的负载均衡器
	if len(nodes) == 0 {
		r.serviceBalancer.Delete(serviceCode)
	} else {
		r.serviceBalancer.Store(serviceCode, NewBalancer(serviceCode, nodes))
	}
}

// GetDesc 获取服务描述
func (r *grpcResolver) GetDesc(serviceCode int32) (GRPCServiceDesc, bool) {
	return r.services.Load(serviceCode)
}

// AllocNode 根据负载均衡策略分配可用节点
func (r *grpcResolver) AllocNode(serviceCode int32, sess Session) (nodeID ulid.ULID, err error) {
	balancer, foundBalancer := r.serviceBalancer.Load(serviceCode)
	if !foundBalancer {
		err = ErrNoNodeAvailable
		return
	}

	node, err := balancer.Pick(sess)
	if err != nil {
		return
	}
	return node.ID, nil
}

// PickNode 随机选择可用节点
func (r *grpcResolver) PickNode(serviceCode int32) (nodeID ulid.ULID, err error) {
	nodes, _ := r.serviceNodes.Load(serviceCode)
	if l := len(nodes); l == 0 {
		err = ErrNoNodeAvailable
	} else if l == 1 {
		nodeID = nodes[0].ID
	} else {
		nodeID = nodes[rand.Intn(l)].ID
	}
	return
}

// GetConn 获取节点连接
func (r *grpcResolver) GetConn(nodeID ulid.ULID) (conn *grpc.ClientConn, err error) {
	node, foundNode := r.allNodes.Load(nodeID)
	if !foundNode || node.State == NodeDown {
		err = ErrNodeNotFoundOrDown
		return
	}

	conn, err = r.getConn(node.GRPC.Endpoint)
	return
}

func (r *grpcResolver) getConn(endpoint string) (*grpc.ClientConn, error) {
	if conn, ok := r.conns.Load(endpoint); ok {
		return conn, nil
	}

	conn, err := grpc.NewClient(endpoint, r.dialOptions...)
	if err != nil {
		return nil, fmt.Errorf("dial grpc, %w", err)
	}

	r.conns.Store(endpoint, conn)
	return conn, nil
}

func (r *grpcResolver) Close() {
	r.conns.Range(func(key string, value *grpc.ClientConn) bool {
		_ = value.Close()
		r.conns.Delete(key)
		return true
	})
}
