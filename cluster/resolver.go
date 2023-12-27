package cluster

import (
	"fmt"
	"runtime"

	"github.com/joyparty/gokit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// grpcResolver grpc服务发现
type grpcResolver struct {
	allNodes *gokit.MapOf[string, NodeEntry]

	// 每个服务的负载均衡器
	// serviceCode => Balancer
	balancer *gokit.MapOf[int32, Balancer]

	// serviceCode => GRPCServiceDesc
	services *gokit.MapOf[int32, GRPCServiceDesc]

	// endpoint => *grpc.ClientConn
	conns *gokit.MapOf[string, *grpc.ClientConn]

	dialOptions []grpc.DialOption
}

// newGRPCResolver 创建grpc服务发现
func newGRPCResolver(dialOptions ...grpc.DialOption) *grpcResolver {
	return &grpcResolver{
		allNodes: gokit.NewMapOf[string, NodeEntry](),
		balancer: gokit.NewMapOf[int32, Balancer](),
		services: gokit.NewMapOf[int32, GRPCServiceDesc](),
		conns:    gokit.NewMapOf[string, *grpc.ClientConn](),
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

		if _, ok := r.services.Load(desc.Code); !ok {
			r.services.Store(desc.Code, desc)
		}

		r.resetBalancer(desc.Code)
	}
}

// Remove 删除条目
func (r *grpcResolver) Remove(node NodeEntry) {
	if len(node.GRPC.Services) == 0 {
		return
	}

	r.allNodes.Delete(node.ID)
	for _, desc := range node.GRPC.Services {
		r.resetBalancer(desc.Code)
	}
}

func (r *grpcResolver) resetBalancer(serviceCode int32) {
	// 找出所有状态为OK的节点
	nodes := []NodeEntry{}
	r.allNodes.Range(func(_ string, entry NodeEntry) bool {
		if entry.State == NodeOK {
			for _, desc := range entry.GRPC.Services {
				if desc.Code == serviceCode {
					nodes = append(nodes, entry)
					break
				}
			}
		}
		return true
	})

	// balancer不考虑在使用过程中增减节点，每次节点变更都重新生成相关服务的负载均衡器
	if len(nodes) == 0 {
		r.balancer.Delete(serviceCode)
	} else {
		r.balancer.Store(serviceCode, NewBalancer(serviceCode, nodes))
	}
}

// GetDesc 获取服务描述
func (r *grpcResolver) GetDesc(serviceCode int32) (GRPCServiceDesc, bool) {
	return r.services.Load(serviceCode)
}

// PickNode 随机选择一个可用节点
func (r *grpcResolver) PickNode(serviceCode int32, sess Session) (nodeID string, err error) {
	balancer, foundBalancer := r.balancer.Load(serviceCode)
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

// GetConn 获取节点连接
func (r *grpcResolver) GetConn(nodeID string) (conn *grpc.ClientConn, err error) {
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

	conn, err := grpc.Dial(endpoint, r.dialOptions...)
	if err != nil {
		return nil, fmt.Errorf("dial grpc, %w", err)
	}
	runtime.SetFinalizer(conn, func(conn *grpc.ClientConn) {
		_ = conn.Close()
	})

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
