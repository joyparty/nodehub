package cluster

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"sync"

	"github.com/oklog/ulid/v2"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// grpcResolver grpc服务发现
type grpcResolver struct {
	// nodeID => NodeEntry
	allNodes map[ulid.ULID]NodeEntry

	// 所有节点状态为ok的可用节点
	// serviceCode => []NodeEntry
	okNodes map[int32][]NodeEntry

	// serviceCode => GRPCServiceDesc
	services map[int32]GRPCServiceDesc

	// endpoint => *grpc.ClientConn
	conns *sync.Map

	dialOptions []grpc.DialOption

	l sync.RWMutex
}

// newGRPCResolver 创建grpc服务发现
func newGRPCResolver(dialOptions ...grpc.DialOption) *grpcResolver {
	return &grpcResolver{
		allNodes: make(map[ulid.ULID]NodeEntry),
		okNodes:  make(map[int32][]NodeEntry),
		services: make(map[int32]GRPCServiceDesc),
		conns:    new(sync.Map),
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

	r.l.Lock()
	defer r.l.Unlock()

	r.allNodes[node.ID] = node

	for _, desc := range node.GRPC.Services {
		if v, ok := r.services[desc.Code]; ok {
			if !reflect.DeepEqual(v, desc) {
				logger.Error("unexpected grpc service description", "old", v, "new", desc)
			}
		} else {
			r.services[desc.Code] = desc
		}
	}

	r.updateOKNodes()
}

// Remove 删除条目
func (r *grpcResolver) Remove(node NodeEntry) {
	if len(node.GRPC.Services) == 0 {
		return
	}

	r.l.Lock()
	defer r.l.Unlock()

	delete(r.allNodes, node.ID)
	r.updateOKNodes()
}

func (r *grpcResolver) updateOKNodes() {
	nodes := make(map[int32][]NodeEntry)

	for _, node := range r.allNodes {
		if node.State == NodeOK {
			for _, desc := range node.GRPC.Services {
				nodes[desc.Code] = append(nodes[desc.Code], node)
			}
		}
	}
	r.okNodes = nodes
}

// GetServiceConn 获取服务连接，随机选择一个可用节点
func (r *grpcResolver) GetServiceConn(serviceCode int32) (conn *grpc.ClientConn, desc GRPCServiceDesc, err error) {
	r.l.RLock()
	nodes, foundNodes := r.okNodes[serviceCode]
	desc, foundDesc := r.services[serviceCode]
	r.l.RUnlock()

	if !foundDesc {
		err = ErrGRPCServiceCode
		return
	} else if !foundNodes {
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
	r.l.RLock()
	node, foundNode := r.allNodes[nodeID]
	desc, foundDesc := r.services[serviceCode]
	r.l.RUnlock()

	if !foundDesc {
		err = ErrGRPCServiceCode
		return
	} else if !foundNode || node.State == NodeDown {
		err = ErrNoNodeOrDown
		return
	}

	conn, err = r.getConn(node.GRPC.Endpoint)
	return
}

// GetNodeConn 获取节点连接
func (r *grpcResolver) GetNodeConn(nodeID ulid.ULID) (conn *grpc.ClientConn, err error) {
	r.l.RLock()
	node, foundNode := r.allNodes[nodeID]
	r.l.RUnlock()

	if !foundNode || node.State == NodeDown {
		err = ErrNoNodeOrDown
		return
	}

	conn, err = r.getConn(node.GRPC.Endpoint)
	return
}

func (r *grpcResolver) getConn(endpoint string) (*grpc.ClientConn, error) {
	if conn, ok := r.conns.Load(endpoint); ok {
		return conn.(*grpc.ClientConn), nil
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
	r.conns.Range(func(key, value any) bool {
		_ = value.(*grpc.ClientConn).Close()
		r.conns.Delete(key)
		return true
	})
}
