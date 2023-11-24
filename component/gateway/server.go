package gateway

import (
	"context"
	"fmt"
	"net/http"
	"nodehub"
	"nodehub/logger"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{}
)

// Server 网关服务器
//
// 客户端通过websocket方式连接网关，网关再转发请求到grpc后端服务
type Server struct {
	Resolver   *nodehub.GRPCResolver
	SessionHub *sessionHub
	ListenAddr string

	hs *http.Server
}

// Name 服务名称
func (s *Server) Name() string {
	return "gateway"
}

// Start 启动websocket服务器
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.serveHTTP)

	hs := &http.Server{
		Addr:    s.ListenAddr,
		Handler: http.HandlerFunc(mux.ServeHTTP),
	}

	go func() {
		logger.Info("start gateway", "addr", s.ListenAddr)
		if err := hs.ListenAndServe(); err != nil {
			logger.Error("start gateway", "error", err)

			panic(fmt.Errorf("start gateway, %w", err))
		}
	}()

	return nil
}

// Stop 停止websocket服务器
func (s *Server) Stop(ctx context.Context) error {
	return nil
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("upgrade websocket", "error", err)
		return
	}

	sess := newWsSession(conn)
	s.SessionHub.Store(sess)
}
