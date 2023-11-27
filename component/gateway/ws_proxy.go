package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"nodehub"
	"nodehub/cluster"
	"nodehub/logger"
	clientpb "nodehub/proto/client"
	gatewaypb "nodehub/proto/gateway"
	"path"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/panjf2000/ants/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	upgrader = websocket.Upgrader{}

	errRequestPrivateNode = errors.New("request private node")
)

// ServiceCode gateway服务的service code默认为1
const ServiceCode = 1

// WebsocketProxy 网关服务器
//
// 客户端通过websocket方式连接网关，网关再转发请求到grpc后端服务
type WebsocketProxy struct {
	Registry   *cluster.Registry
	ListenAddr string

	// 自定义身份验证逻辑，在websocket upgrade之前调用
	// 返回的metadata会在此连接的所有grpc request中携带
	// 返回的userID如果不为空，则会作为会话唯一标识使用，另外也会被自动加入到metadata中
	// 如果返回ok为false，会直接关闭连接
	// 因此如果验证不通过之类的错误，需要在这个函数里面自行处理
	Authorize func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool)

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
	mux.HandleFunc("/grpc", wp.serveHTTP)

	hs := &http.Server{
		Addr:    wp.ListenAddr,
		Handler: http.HandlerFunc(mux.ServeHTTP),
	}
	wp.hs = hs

	wp.sessionHub = newSessionHub()

	if wp.Authorize == nil {
		wp.Authorize = func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool) {
			return "", metadata.MD{}, true
		}
	}

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

func (wp *WebsocketProxy) serveHTTP(w http.ResponseWriter, r *http.Request) {
	userID, sessionMD, ok := wp.Authorize(w, r)
	if !ok {
		logger.Debug("deny by authorize", "remote_addr", r.RemoteAddr)
		return
	} else if userID != "" {
		// 把user id放到request header
		sessionMD.Set(nodehub.MDUserID, userID)
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("upgrade websocket", "error", err)
		return
	}

	sess := newWsSession(conn)
	if userID != "" {
		sess.SetID(userID)
	}

	wp.sessionHub.Store(sess)
	defer func() {
		wp.sessionHub.Delete(sess.ID())
		sess.Close()
	}()

	for {
		req := &clientpb.Request{}
		if err := sess.Recv(req); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("recv request", "error", err)
			}
			return
		}

		ants.Submit(func() {
			if err := wp.handleUnary(sess, req, sessionMD.Copy()); err != nil {
				logger.Error("handle request",
					"error", err,
					"service_code", req.ServiceCode,
					"method", req.Method,
					"request_id", req.Id,
					"remote_addr", conn.RemoteAddr().String(),
				)

				if s, ok := status.FromError(err); ok {
					resp, _ := nodehub.PackClientResponse(int32(gatewaypb.Protocol_RPC_ERROR), &gatewaypb.RPCError{
						ServiceCode: req.ServiceCode,
						Method:      req.Method,
						Status:      s.Proto(),
					})
					resp.RequestId = req.Id
					resp.ServiceCode = ServiceCode

					sess.Send(resp)
				}
			}
		})
	}
}

func (wp *WebsocketProxy) handleUnary(sess Session, req *clientpb.Request, md metadata.MD) error {
	conn, desc, err := wp.Registry.GetGRPCServiceConn(req.ServiceCode)
	if err != nil {
		return fmt.Errorf("get grpc conn, %w", err)
	} else if !desc.Public {
		return errRequestPrivateNode
	}

	input, err := nodehub.NewEmptyMessage(req.Data)
	if err != nil {
		return fmt.Errorf("unmarshal request data, %w", err)
	}
	output := &clientpb.Response{}

	// 把request id放到request header
	md.Set(nodehub.MDRequestID, strconv.Itoa(int(req.Id)))
	ctx := metadata.NewIncomingContext(context.Background(), md)

	apiPath := path.Join(desc.Path, req.Method)
	if err := grpc.Invoke(ctx, apiPath, input, output, conn); err != nil {
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
