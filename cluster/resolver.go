package cluster

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"

	"github.com/joyparty/gokit"
	"github.com/oklog/ulid/v2"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// grpcResolver grpc服务发现
type grpcResolver struct {
	allNodes *gokit.MapOf[ulid.ULID, NodeEntry]

	// 所有节点状态为ok的可用节点
	// serviceCode => []NodeEntry
	okNodes *gokit.MapOf[int32, []NodeEntry]

	// serviceCode => GRPCServiceDesc
	services *gokit.MapOf[int32, GRPCServiceDesc]

	// endpoint => *grpc.ClientConn
	conns *gokit.MapOf[string, *grpc.ClientConn]

	dialOptions []grpc.DialOption
}

// newGRPCResolver 创建grpc服务发现
func newGRPCResolver(dialOptions ...grpc.DialOption) *grpcResolver {
	return &grpcResolver{
		allNodes: gokit.NewMapOf[ulid.ULID, NodeEntry](),
		okNodes:  gokit.NewMapOf[int32, []NodeEntry](),
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
		if v, ok := r.services.Load(desc.Code); ok {
			if !reflect.DeepEqual(v, desc) {
				logger.Error("unexpected grpc service description", "old", v, "new", desc)
			}
		} else {
			r.services.Store(desc.Code, desc)
		}

		r.updateOKNodes(desc.Code)
	}
}

// Remove 删除条目
func (r *grpcResolver) Remove(node NodeEntry) {
	if len(node.GRPC.Services) == 0 {
		return
	}

	r.allNodes.Delete(node.ID)
	for _, desc := range node.GRPC.Services {
		r.updateOKNodes(desc.Code)
	}
}

func (r *grpcResolver) updateOKNodes(serviceCode int32) {
	nodes := []NodeEntry{}

	r.allNodes.Range(func(_ ulid.ULID, entry NodeEntry) bool {
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

	if len(nodes) == 0 {
		r.okNodes.Delete(serviceCode)
	} else {
		r.okNodes.Store(serviceCode, nodes)
	}
}

// GetServiceConn 获取服务连接，随机选择一个可用节点
func (r *grpcResolver) GetServiceConn(serviceCode int32) (conn *grpc.ClientConn, desc GRPCServiceDesc, err error) {
	desc, foundDesc := r.services.Load(serviceCode)
	if !foundDesc {
		err = ErrGRPCServiceCode
		return
	}

	nodes, foundNodes := r.okNodes.Load(serviceCode)
	if !foundNodes {
		err = ErrNoNodeAvailable
		return
	}

	var node NodeEntry
	if l := len(nodes); l == 1 {
		node = nodes[0]
	} else {
		node = nodes[rand.Intn(l)]
	}
	conn, err = r.getConn(node.GRPC.Endpoint)
	return
}

// GetServiceNodeConn 获取服务连接，指定节点
func (r *grpcResolver) GetServiceNodeConn(serviceCode int32, nodeID ulid.ULID) (conn *grpc.ClientConn, desc GRPCServiceDesc, err error) {
	desc, foundDesc := r.services.Load(serviceCode)
	if !foundDesc {
		err = ErrGRPCServiceCode
		return
	}

	node, foundNode := r.allNodes.Load(nodeID)
	if !foundNode || node.State == NodeDown {
		err = ErrNoNodeOrDown
		return
	}

	conn, err = r.getConn(node.GRPC.Endpoint)
	return
}

// GetNodeConn 获取节点连接
func (r *grpcResolver) GetNodeConn(nodeID ulid.ULID) (conn *grpc.ClientConn, err error) {
	node, foundNode := r.allNodes.Load(nodeID)
	if !foundNode || node.State == NodeDown {
		err = ErrNoNodeOrDown
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
