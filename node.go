package nodehub

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
)

// Node 节点，一个节点上可以运行多个网络服务
type Node struct {
	servers []Server
}

// AddServer 添加网络服务
func (n *Node) AddServer(s ...Server) {
	n.servers = append(n.servers, s...)
}

// Serve 启动所有网络服务
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
	g, ctx := errgroup.WithContext(ctx)

	for i := range n.servers {
		s := n.servers[i]
		g.Go(func() error {
			if err := s.Init(ctx); err != nil {
				return fmt.Errorf("initialize %s, %w", s.Name(), err)
			} else if err := s.BeforeStart(ctx); err != nil {
				return fmt.Errorf("before start %s, %w", s.Name(), err)
			} else if err := s.Start(ctx); err != nil {
				return fmt.Errorf("start %s, %w", s.Name(), err)
			}

			s.AfterStart(ctx)
			return nil
		})
	}
	return g.Wait()
}

func (n *Node) stopAll(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	for i := range n.servers {
		s := n.servers[i]
		g.Go(func() error {
			if err := s.BeforeStop(ctx); err != nil {
				return fmt.Errorf("before stop %s, %w", s.Name(), err)
			} else if err := s.Stop(ctx); err != nil {
				return fmt.Errorf("stop %s, %w", s.Name(), err)
			}

			s.AfterStop(ctx)
			return nil
		})
	}
	return g.Wait()
}
