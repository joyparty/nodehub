package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"nodehub"
	"nodehub/logger"
	clientpb "nodehub/proto/client"
	"path"

	"github.com/gorilla/websocket"
	"github.com/panjf2000/ants/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

var (
	upgrader = websocket.Upgrader{}

	errRequestPrivateNode = errors.New("request private node")
)

// WebsocketProxy 网关服务器
//
// 客户端通过websocket方式连接网关，网关再转发请求到grpc后端服务
type WebsocketProxy struct {
	Resolver   *nodehub.GRPCResolver
	SessionHub *sessionHub
	ListenAddr string

	hs *http.Server
}

// Name 服务名称
func (wp *WebsocketProxy) Name() string {
	return "gateway"
}

// Start 启动websocket服务器
func (wp *WebsocketProxy) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wp.serveHTTP)

	hs := &http.Server{
		Addr:    wp.ListenAddr,
		Handler: http.HandlerFunc(mux.ServeHTTP),
	}

	go func() {
		logger.Info("start gateway", "addr", wp.ListenAddr)
		if err := hs.ListenAndServe(); err != nil {
			logger.Error("start gateway", "error", err)

			panic(fmt.Errorf("start gateway, %w", err))
		}
	}()

	return nil
}

// Stop 停止websocket服务器
func (wp *WebsocketProxy) Stop(ctx context.Context) error {
	return nil
}

func (wp *WebsocketProxy) serveHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("upgrade websocket", "error", err)
		return
	}

	sess := newWsSession(conn)
	defer sess.Close()

	wp.SessionHub.Store(sess)

	for {
		req := &clientpb.Request{}
		if err := sess.Recv(req); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("recv request", "error", err)
			}
			return
		}

		ants.Submit(func() {
			if err := wp.handleRequest(sess, req); err != nil {
				// TODO: 下行错误响应

				logger.Error("handle request",
					"error", err,
					"service_code", req.ServiceCode,
					"method", req.Method,
					"request_id", req.Id,
					"remote_addr", conn.RemoteAddr().String(),
				)
			}
		})
	}
}

func (wp *WebsocketProxy) handleRequest(sess Session, req *clientpb.Request) error {
	conn, desc, err := wp.Resolver.GetConn(req.ServiceCode)
	if err != nil {
		return fmt.Errorf("get grpc conn, %w", err)
	} else if !desc.Public {
		return errRequestPrivateNode
	}

	resp := &clientpb.Response{}
	apiPath := path.Join(desc.Path, req.Method)
	if err := grpc.Invoke(context.Background(), apiPath, req, resp, conn); err != nil {
		return fmt.Errorf("invoke grpc, %w", err)
	}

	// google.protobuf.Empty类型，不需要下行数据
	if proto.Size(resp) == 0 {
		return nil
	}

	resp.RequestId = req.Id
	resp.ServiceCode = req.ServiceCode

	return sess.Send(resp)
}
