package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"nodehub/cluster"
	"nodehub/logger"
	clientpb "nodehub/proto/client"
	"path"

	"github.com/gorilla/websocket"
	"github.com/panjf2000/ants/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	upgrader = websocket.Upgrader{}

	errRequestPrivateNode = errors.New("request private node")
)

// WebsocketProxy 网关服务器
//
// 客户端通过websocket方式连接网关，网关再转发请求到grpc后端服务
type WebsocketProxy struct {
	Registry   *cluster.Registry
	ListenAddr string

	sessionHub *sessionHub
	hs         *http.Server
}

// Name 服务名称
func (wp *WebsocketProxy) Name() string {
	return "gateway"
}

// Start 启动websocket服务器
func (wp *WebsocketProxy) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy", wp.serveHTTP)

	hs := &http.Server{
		Addr:    wp.ListenAddr,
		Handler: http.HandlerFunc(mux.ServeHTTP),
	}
	wp.hs = hs

	wp.sessionHub = newSessionHub()

	go func() {
		logger.Info("start gateway", "addr", wp.ListenAddr)
		if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("start gateway", "error", err)

			panic(fmt.Errorf("start gateway, %w", err))
		}
	}()

	return nil
}

// Stop 停止websocket服务器
func (wp *WebsocketProxy) Stop(ctx context.Context) error {
	wp.hs.Shutdown(ctx)
	wp.sessionHub.Close()
	return nil
}

// TODO: 提供注入身份验证机制
func (wp *WebsocketProxy) serveHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("upgrade websocket", "error", err)
		return
	}

	sess := newWsSession(conn)
	defer sess.Close()

	wp.sessionHub.Store(sess)

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

// TODO: 把requestID和userID放到metadata里面
func (wp *WebsocketProxy) handleRequest(sess Session, req *clientpb.Request) error {
	conn, desc, err := wp.Registry.GetGRPCServiceConn(req.ServiceCode)
	if err != nil {
		return fmt.Errorf("get grpc conn, %w", err)
	} else if !desc.Public {
		return errRequestPrivateNode
	}

	input := &emptypb.Empty{}
	if err := proto.Unmarshal(req.Data, input); err != nil {
		return fmt.Errorf("unmarshal request data, %w", err)
	}

	output := &clientpb.Response{}
	apiPath := path.Join(desc.Path, req.Method)
	if err := grpc.Invoke(context.Background(), apiPath, input, output, conn); err != nil {
		return fmt.Errorf("invoke grpc, %w", err)
	}

	// google.protobuf.Empty类型，不需要下行数据
	if proto.Size(output) == 0 {
		return nil
	}

	output.RequestId = req.Id
	output.ServiceCode = req.ServiceCode

	return sess.Send(output)
}
