package nodehub

import (
	"context"
	"fmt"
	"os/signal"
	"sync"
	"syscall"

	"github.com/oklog/ulid/v2"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/component/gateway"
	"gitlab.haochang.tv/gopkg/nodehub/component/rpc"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/proto/nh"
	"google.golang.org/grpc"
)

// Component 组件
type Component interface {
	// Name 组件名称，仅用于显示
	Name() string

	// Start方法注意不要阻塞程序执行
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// 如果实现了以下方法，会被自动调用
	// CompleteNodeEntry(*cluster.NodeEntry)
	// BeforeStart(ctx context.Context) error
	// AfterStart(ctx context.Context)
	// BeforeStop(ctx context.Context) error
	// AfterStop(ctx context.Context)
}

// Node 节点，一个节点上可以运行多个网络服务
type Node struct {
	entry      *cluster.NodeEntry
	registry   *cluster.Registry
	components []Component

	shutdownOnce sync.Once
	done         chan struct{}
}

// NewNode 构造函数
func NewNode(name string, registry *cluster.Registry, option ...NodeOption) *Node {
	node := &Node{
		entry: &cluster.NodeEntry{
			ID:    ulid.Make(),
			Name:  name,
			State: cluster.NodeOK,
		},
		registry:   registry,
		components: []Component{},
		done:       make(chan struct{}),
	}

	for _, opt := range option {
		opt(node)
	}
	return node
}

// AddComponent 添加组件
//
// 组件的启动顺序与添加顺序一致
// 组件的停止顺序与添加顺序相反
func (n *Node) AddComponent(c ...Component) {
	type grpcServer interface {
		RegisterService(code int32, desc grpc.ServiceDesc, impl any, options ...rpc.Option) error
	}

	for i := range c {
		// 自动注入节点管理服务
		if v, ok := c[i].(grpcServer); ok {
			v.RegisterService(rpc.NodeServiceCode, nh.Node_ServiceDesc, &nodeService{node: n})
			break
		}
	}
	n.components = append(n.components, c...)
}

// Serve 启动所有组件
func (n *Node) Serve(ctx context.Context) error {
	// 确保node service一定被注册
	entry := n.Entry()
	if entry.GRPC.Endpoint != "" {
		var found bool
		for _, desc := range entry.GRPC.Services {
			if desc.Code == rpc.NodeServiceCode {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("node grpc service not register")
		}
	}

	if err := n.startAll(ctx); err != nil {
		return fmt.Errorf("start all server, %w", err)
	}

	// 服务发现注册
	if err := n.registry.Put(n.Entry()); err != nil {
		return fmt.Errorf("register node, %w", err)
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	select {
	case <-n.done:
	case <-ctx.Done():
	}

	if err := n.ChangeState(cluster.NodeDown); err != nil {
		return fmt.Errorf("cannot change node state, %w", err)
	}

	if err := n.stopAll(ctx); err != nil {
		return fmt.Errorf("stop all server, %w", err)
	}
	// 服务发现注销
	n.registry.Close()

	return nil
}

func (n *Node) startAll(ctx context.Context) error {
	for _, c := range n.components {
		if v, ok := c.(interface {
			BeforeStart(ctx context.Context) error
		}); ok {
			if err := v.BeforeStart(ctx); err != nil {
				return fmt.Errorf("before start %s, %w", c.Name(), err)
			}
		}

		if err := c.Start(ctx); err != nil {
			return fmt.Errorf("start %s, %w", c.Name(), err)
		}
		logger.Info("start component", "name", c.Name())

		if v, ok := c.(interface {
			AfterStart(ctx context.Context)
		}); ok {
			v.AfterStart(ctx)
		}
	}

	return nil
}

func (n *Node) stopAll(ctx context.Context) error {
	for i := len(n.components) - 1; i >= 0; i-- {
		c := n.components[i]

		if v, ok := c.(interface {
			BeforeStop(ctx context.Context) error
		}); ok {
			if err := v.BeforeStop(ctx); err != nil {
				return fmt.Errorf("before stop %s, %w", c.Name(), err)
			}
		}

		if err := c.Stop(ctx); err != nil {
			return fmt.Errorf("stop %s, %w", c.Name(), err)
		}
		logger.Info("stop component", "name", c.Name())

		if v, ok := c.(interface {
			AfterStop(ctx context.Context)
		}); ok {
			v.AfterStop(ctx)
		}
	}

	return nil
}

// Shutdown 关闭节点
func (n *Node) Shutdown() {
	n.shutdownOnce.Do(func() {
		close(n.done)
	})
}

// State 获取节点状态
func (n *Node) State() cluster.NodeState {
	return n.entry.State
}

// ChangeState 改变节点状态
func (n *Node) ChangeState(state cluster.NodeState) (err error) {
	if n.entry.State == state {
		return nil
	}

	old := n.entry.State
	defer func() {
		if err != nil {
			n.entry.State = old
		}
	}()

	n.entry.State = state
	return n.registry.Put(n.Entry())
}

// ID 获取节点ID
func (n *Node) ID() ulid.ULID {
	return n.entry.ID
}

// Entry 获取服务发现条目
func (n *Node) Entry() cluster.NodeEntry {
	entry := *n.entry

	// 把条目信息交给实现了这个接口的组件修改
	for i := range n.components {
		if v, ok := n.components[i].(interface {
			CompleteNodeEntry(entry *cluster.NodeEntry)
		}); ok {
			v.CompleteNodeEntry(&entry)
		}
	}
	return entry
}

// GatewayConfig 网关配置
type GatewayConfig struct {
	Options []gateway.Option

	WebsocketListen  string
	Authorizer       gateway.Authorizer
	GRPCListen       string
	GRPCServerOption []grpc.ServerOption
}

// NewGatewayNode 构造一个网关节点
func NewGatewayNode(registry *cluster.Registry, config GatewayConfig) *Node {
	node := NewNode("gateway", registry)

	playground := gateway.NewPlayground(node.ID(), registry, config.Options...)
	gw := gateway.NewWSProxy(playground, config.WebsocketListen, config.Authorizer)
	node.AddComponent(gw)

	gs := rpc.NewGRPCServer(config.GRPCListen, config.GRPCServerOption...)
	gs.RegisterService(rpc.GatewayServiceCode, nh.Gateway_ServiceDesc, playground.NewGRPCService())
	node.AddComponent(gs)

	return node
}

// NodeOption 节点选项
type NodeOption func(*Node)

// WithGitVersion 设置git版本
func WithGitVersion(version string) NodeOption {
	return func(n *Node) {
		n.entry.GitVersion = version
	}
}
