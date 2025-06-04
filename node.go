package nodehub

import (
	"context"
	"fmt"
	"net"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/component/gateway"
	"github.com/joyparty/nodehub/component/gateway/metrics"
	"github.com/joyparty/nodehub/component/rpc"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/oklog/ulid/v2"
	"github.com/samber/lo"
	"google.golang.org/grpc"
)

// Component 组件
type Component interface {
	// Name 组件名称，仅用于显示
	Name() string

	// Start方法注意不要阻塞程序执行
	Start(ctx context.Context) error
	Stop(ctx context.Context)

	// 如果实现了以下方法，会被自动调用
	// CompleteNodeEntry(*cluster.NodeEntry)
	// BeforeStart(ctx context.Context) error
	// AfterStart(ctx context.Context)
	// BeforeStop(ctx context.Context)
	// AfterStop(ctx context.Context)
}

// Node 节点，一个节点上可以运行多个网络服务
type Node struct {
	sync.RWMutex

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
	n.components = append(n.components, c...)
}

// GetComponents 获取组件列表
func (n *Node) GetComponents() []Component {
	return n.components
}

// Serve 启动所有组件
func (n *Node) Serve(ctx context.Context) error {
	// 确保node service一定被注册
	entry := n.Entry()
	if entry.GRPC.Endpoint != "" &&
		!lo.SomeBy(entry.GRPC.Services, func(desc cluster.GRPCServiceDesc) bool {
			return desc.Code == nh.NodeServiceCode
		}) {
		return fmt.Errorf("node grpc service not register")
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
		logger.Error("change node state", "error", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	n.stopAll(ctx)

	// 服务发现注销
	n.registry.Close()

	return nil
}

func (n *Node) startAll(ctx context.Context) error {
	type grpcServer interface {
		RegisterService(code int32, desc grpc.ServiceDesc, impl any, options ...rpc.Option) error
	}

	// 自动注入节点管理服务
	for _, c := range n.components {
		if v, ok := c.(grpcServer); ok {
			_ = v.RegisterService(nh.NodeServiceCode, nh.Node_ServiceDesc, &nodeService{node: n})
			break
		}
	}

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

func (n *Node) stopAll(ctx context.Context) {
	var wg sync.WaitGroup
	for _, c := range n.components {
		wg.Add(1)

		go func(c Component) {
			defer wg.Done()

			stopC := make(chan struct{}, 1)
			go func() {
				defer close(stopC)
				logger.Info("stop component", "name", c.Name())

				if v, ok := c.(interface {
					BeforeStop(ctx context.Context)
				}); ok {
					v.BeforeStop(ctx)
				}

				c.Stop(ctx)

				if v, ok := c.(interface {
					AfterStop(ctx context.Context)
				}); ok {
					v.AfterStop(ctx)
				}

				stopC <- struct{}{}
			}()

			select {
			case <-stopC:
				logger.Info("component stoped", "name", c.Name())
			case <-ctx.Done():
				logger.Error("component stop timeout", "name", c.Name())
			}
		}(c)
	}

	wg.Wait()
}

// Shutdown 关闭节点
func (n *Node) Shutdown() {
	n.shutdownOnce.Do(func() {
		close(n.done)
	})
}

// State 获取节点状态
func (n *Node) State() cluster.NodeState {
	n.RLock()
	defer n.RUnlock()

	return n.entry.State
}

// ChangeState 改变节点状态
func (n *Node) ChangeState(state cluster.NodeState) (err error) {
	n.Lock()
	defer n.Unlock()

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
	NodeOptions []NodeOption
	Options     []gateway.Option

	GRPCListen       string
	GRPCListener     net.Listener
	GRPCServerOption []grpc.ServerOption

	MetricsListen string
}

// NewGatewayNode 构造一个网关节点
func NewGatewayNode(registry *cluster.Registry, config GatewayConfig) *Node {
	node := NewNode("gateway", registry, config.NodeOptions...)

	options := append(config.Options, gateway.WithRegistry(registry))
	proxy, err := gateway.NewProxy(node.ID(), options...)
	if err != nil {
		panic(fmt.Errorf("create proxy, %w", err))
	}
	node.AddComponent(proxy)

	var gs *rpc.GRPCServer
	if config.GRPCListener == nil {
		gs = rpc.NewGRPCServer(config.GRPCListen, config.GRPCServerOption...)
	} else {
		gs = rpc.BindGRPCServer(config.GRPCListener, config.GRPCServerOption...)
	}
	gs.RegisterService(nh.GatewayServiceCode, nh.Gateway_ServiceDesc, proxy.NewGRPCService())
	node.AddComponent(gs)

	if addr := config.MetricsListen; addr != "" {
		node.AddComponent(metrics.NewServer(addr))
	}

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

// WithState 设置节点初始状态
func WithState(state cluster.NodeState) NodeOption {
	return func(n *Node) {
		n.entry.State = state
	}
}
