package metrics

import (
	"context"
	"fmt"
	"net/http"

	"github.com/joyparty/nodehub/cluster"
	in "github.com/joyparty/nodehub/internal/metrics"
	"github.com/joyparty/nodehub/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server is a prometheus metrics server
type Server struct {
	addr string
	s    *http.Server
}

// NewServer 构造函数
func NewServer(addr string) *Server {
	return &Server{
		addr: addr,
	}
}

// Name implements nodehub.Component interface.
func (s *Server) Name() string {
	return "metrics"
}

// CompleteNodeEntry 服务发现条目配置
func (s *Server) CompleteNodeEntry(entry *cluster.NodeEntry) {
	entry.Metrics = fmt.Sprintf("http://%s/metrics", s.addr)
}

// Start implements nodehub.Component interface.
func (s *Server) Start(ctx context.Context) error {
	reg := in.Init()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	s.s = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	go func() {
		if err := s.s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("start metrics server", "error", err)

			panic(fmt.Errorf("start metrics server, %w", err))
		}
	}()
	return nil
}

// Stop implements nodehub.Component interface.
func (s *Server) Stop(ctx context.Context) {
	_ = s.s.Shutdown(ctx)
}
