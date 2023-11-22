package nodehub

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"nodehub/logger"
	"path"
	"runtime"
	"sync"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// ErrNoNodeOrDown 没有可用节点或节点已下线
	ErrNoNodeOrDown = errors.New("no node or node is down")
	// ErrNoNodeAvailable 没有可用节点
	ErrNoNodeAvailable = errors.New("no node available")
	// ErrGRPCServiceCode grpc服务代码未找到
	ErrGRPCServiceCode = errors.New("grpc service code not found")
)

// RegistryEvent 注册表事件
type RegistryEvent int32

const (
	// PUT 新增或更新
	PUT RegistryEvent = RegistryEvent(mvccpb.PUT)
	// DELETE 删除
	DELETE RegistryEvent = RegistryEvent(mvccpb.DELETE)
)

// NodeState 节点状态
type NodeState string

const (
	// NodeOK 正常
	NodeOK NodeState = "ok"
	// NodeLazy 只接收指定了节点ID的请求
	NodeLazy NodeState = "lazy"
	// NodeDown 下线，不接受任何请求
	NodeDown NodeState = "down"
)

// NodeEntry 节点服务发现条目
type NodeEntry struct {
	// 节点ID，集群内唯一，会被用于有状态服务路由
	ID string `json:"id"`

	// 节点名称，仅用于显示
	Name string `json:"name"`

	// 节点状态
	State NodeState `json:"state"`

	// grpc服务信息
	GRPC GRPCEntry `json:"grpc"`

	// git版本
	GitVersion string `json:"git_version,omitempty"`
}

// Validate 验证条目是否合法
func (e NodeEntry) Validate() error {
	if e.ID == "" {
		return errors.New("id is empty")
	} else if e.Name == "" {
		return errors.New("name is empty")
	} else if e.State == "" {
		return errors.New("state is empty")
	} else if err := e.GRPC.Validate(); err != nil {
		return err
	}
	return nil
}

// GRPCEntry gRPC服务发现条目
type GRPCEntry struct {
	// 监听地址 host:port
	Endpoint string `json:"endpoint"`

	// grpc服务列表
	Services []GRPCServiceDesc `json:"services"`
}

// Validate 验证条目是否合法
func (e GRPCEntry) Validate() error {
	if len(e.Services) == 0 {
		return nil
	} else if e.Endpoint == "" {
		return errors.New("grpc endpoint is empty")
	}
	return nil
}

// GRPCServiceDesc gRPC服务
type GRPCServiceDesc struct {
	// 服务代码枚举值，每个服务的代码值必须唯一
	//
	// 网关会根据客户端请求消息内的service_code字段，将请求转发到对应的服务
	Code int32 `json:"code"`

	// 服务路径，网关在构造grpc请求时，用于方法地址构造
	//
	// example: /helloworld.Greeter
	Path string `json:"path"`

	// 是否允许客户端访问
	Public bool `json:"public"`
}

// Registry 服务注册表
type Registry struct {
	client       *clientv3.Client
	keyPrefix    string
	eventHandler func(event RegistryEvent, entry NodeEntry)

	leaseID clientv3.LeaseID
}

// NewRegistry 创建服务注册表
func NewRegistry(client *clientv3.Client, opt ...func(*Registry)) (*Registry, error) {
	r := &Registry{
		client:    client,
		keyPrefix: "/nodehub/node",
		eventHandler: func(event RegistryEvent, entry NodeEntry) {
			// do nothing
		},
	}

	for _, fn := range opt {
		fn(r)
	}

	if err := r.runKeeper(); err != nil {
		return nil, fmt.Errorf("run keeper, %w", err)
	}

	go r.runWatcher()
	return r, nil
}

// Put 注册服务
func (r *Registry) Put(entry NodeEntry) error {
	if r.leaseID == clientv3.NoLease {
		return errors.New("lease not granted")
	} else if err := entry.Validate(); err != nil {
		return fmt.Errorf("validate entry, %w", err)
	}

	value, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry, %w", err)
	}

	key := path.Join(r.keyPrefix, entry.ID)
	_, err = r.client.Put(r.client.Ctx(), key, string(value), clientv3.WithLease(r.leaseID))
	return err
}

// 向etcd生成一个10秒过期的租约
func (r *Registry) runKeeper() error {
	lease, err := r.client.Grant(r.client.Ctx(), 10) // 10 seconds
	if err != nil {
		return fmt.Errorf("grant lease, %w", err)
	}
	r.leaseID = lease.ID

	// 心跳维持
	go func() {
		ch, err := r.client.KeepAlive(r.client.Ctx(), r.leaseID)
		if err != nil {
			logger.Error("keep lease alive", "error", err)
		} else {
			for resp := range ch {
				select {
				case <-r.client.Ctx().Done():
					return
				default:
					_ = resp
				}
			}
		}

		panic(errors.New("lease keeper closed"))
	}()

	return nil
}

// 监听服务条目变更
func (r *Registry) runWatcher() {
	wCh := r.client.Watch(r.client.Ctx(), r.keyPrefix, clientv3.WithPrefix(), clientv3.WithPrevKV())

	for {
		select {
		case <-r.client.Ctx().Done():
			return
		case wResp := <-wCh:
			for _, ev := range wResp.Events {
				var (
					entry NodeEntry
					value []byte
				)
				switch ev.Type {
				case mvccpb.PUT:
					value = ev.Kv.Value
				case mvccpb.DELETE:
					value = ev.PrevKv.Value
				default:
					logger.Error("unknown event type", "type", ev.Type)
					continue
				}

				if err := json.Unmarshal(value, &entry); err != nil {
					logger.Error("unmarshal entry", "error", err)
				} else {
					r.eventHandler(RegistryEvent(ev.Type), entry)
				}
			}
		}
	}
}

// Close 关闭
func (r *Registry) Close() {
	r.client.Close()
}

// WithKeyPrefix 设置服务条目key前缀
func WithKeyPrefix(prefix string) func(*Registry) {
	return func(r *Registry) {
		r.keyPrefix = prefix
	}
}

// WithEventHandler 设置服务条目变更事件处理函数
func WithEventHandler(fn func(event RegistryEvent, entry NodeEntry)) func(*Registry) {
	return func(r *Registry) {
		r.eventHandler = fn
	}
}

// GRPCResolver grpc服务发现
type GRPCResolver struct {
	// nodeID => NodeEntry
	allNodes map[string]NodeEntry

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

// NewGRPCResolver 创建grpc服务发现
func NewGRPCResolver(dialOptions ...grpc.DialOption) *GRPCResolver {
	return &GRPCResolver{
		allNodes: make(map[string]NodeEntry),
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
func (r *GRPCResolver) Update(node NodeEntry) {
	if len(node.GRPC.Services) == 0 {
		return
	}

	r.l.Lock()
	defer r.l.Unlock()

	r.allNodes[node.ID] = node

	for _, desc := range node.GRPC.Services {
		r.services[desc.Code] = desc
	}

	r.updateOKNodes()
}

// Remove 删除条目
func (r *GRPCResolver) Remove(node NodeEntry) {
	if len(node.GRPC.Services) == 0 {
		return
	}

	r.l.Lock()
	defer r.l.Unlock()

	delete(r.allNodes, node.ID)
	r.updateOKNodes()
}

func (r *GRPCResolver) updateOKNodes() {
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

// GetConn 获取服务连接，随机选择一个可用节点
func (r *GRPCResolver) GetConn(serviceCode int32) (conn *grpc.ClientConn, desc GRPCServiceDesc, err error) {
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

// GetNodeConn 获取指定节点的服务连接
func (r *GRPCResolver) GetNodeConn(serviceCode int32, nodeID string) (conn *grpc.ClientConn, desc GRPCServiceDesc, err error) {
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

func (r *GRPCResolver) getConn(endpoint string) (*grpc.ClientConn, error) {
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
