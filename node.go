package nodehub

import (
	"context"
	"fmt"
	"nodehub/cluster"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const (
	// MDRequestID grpc metadata中的request id key
	MDRequestID = "x-req-id"
	// MDUserID grpc metadata中的user id key
	MDUserID = "x-user-id"
)

// Component 组件
type Component interface {
	// Name 组件名称，仅用于显示
	Name() string

	// Start方法注意不要阻塞程序执行
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// 如果实现了以下方法，会被自动调用
	// BeforeStart(ctx context.Context) error
	// AfterStart(ctx context.Context)
	// BeforeStop(ctx context.Context) error
	// AfterStop(ctx context.Context)
}

// Node 节点，一个节点上可以运行多个网络服务
type Node struct {
	id         string
	name       string
	components []Component
}

// NewNode 构造函数
func NewNode(name string) *Node {
	return &Node{
		id:         strconv.Itoa(int(time.Now().UnixNano())),
		name:       name,
		components: []Component{},
	}
}

// AddComponent 添加组件
//
// 组件的启动顺序与添加顺序一致
// 组件的停止顺序与添加顺序相反
func (n *Node) AddComponent(c ...Component) {
	n.components = append(n.components, c...)
}

// Serve 启动所有组件
func (n *Node) Serve(ctx context.Context) error {
	if err := n.startAll(ctx); err != nil {
		return fmt.Errorf("start all server, %w", err)
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	select {
	case <-ctx.Done():
		if err := n.stopAll(ctx); err != nil {
			return fmt.Errorf("stop all server, %w", err)
		}
	}
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

		if v, ok := c.(interface {
			AfterStop(ctx context.Context)
		}); ok {
			v.AfterStop(ctx)
		}
	}

	return nil
}

// Entry 获取服务发现条目
func (n *Node) Entry() cluster.NodeEntry {
	entry := cluster.NodeEntry{
		ID:    n.id,
		State: cluster.NodeOK,
		Name:  n.name,
	}

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
