package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"nodehub"
	"nodehub/cluster"
	"nodehub/logger"
	"nodehub/notification"
	"nodehub/proto/clientpb"
	"nodehub/proto/gatewaypb"
	"path"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oklog/ulid/v2"
	"github.com/panjf2000/ants/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	upgrader = websocket.Upgrader{}

	errRequestPrivateService = errors.New("request private service")
)

// ServiceCode gateway服务的service code默认为1
const ServiceCode = 1

// WebsocketProxy 网关服务器
//
// 客户端通过websocket方式连接网关，网关再转发请求到grpc后端服务
type WebsocketProxy struct {
	Registry   *cluster.Registry
	ListenAddr string

	// 主动下发消息订阅者
	Notifier notification.Subscriber

	// 自定义身份验证逻辑，在websocket upgrade之前调用
	// 返回的metadata会在此连接的所有grpc request中携带
	// 返回的userID如果不为空，则会作为会话唯一标识使用，另外也会被自动加入到metadata中
	// 如果返回ok为false，会直接关闭连接
	// 因此如果验证不通过之类的错误，需要在这个函数里面自行处理
	Authorize func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool)

	sessionHub *sessionHub
	hs         *http.Server

	requestPool  *sync.Pool
	responsePool *sync.Pool
	done         chan struct{}
}

func (wp *WebsocketProxy) init() {
	wp.sessionHub = newSessionHub()
	wp.done = make(chan struct{})

	wp.requestPool = &sync.Pool{
		New: func() any {
			return &clientpb.Request{}
		},
	}

	wp.responsePool = &sync.Pool{
		New: func() any {
			return &clientpb.Response{}
		},
	}

	if wp.Authorize == nil {
		wp.Authorize = func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool) {
			return "", metadata.MD{}, true
		}
	}
}

// Name 服务名称
func (wp *WebsocketProxy) Name() string {
	return "gateway"
}

// Start 启动websocket服务器
func (wp *WebsocketProxy) Start(ctx context.Context) error {
	wp.init()

	mux := http.NewServeMux()
	mux.HandleFunc("/grpc", wp.serveHTTP)

	hs := &http.Server{
		Addr:    wp.ListenAddr,
		Handler: http.HandlerFunc(mux.ServeHTTP),
	}
	wp.hs = hs

	go func() {
		if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("start gateway", "error", err)

			panic(fmt.Errorf("start gateway, %w", err))
		}
	}()

	go wp.notificationLoop()

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
		req := wp.requestPool.Get().(*clientpb.Request)
		resetRequest(req)

		if err := sess.Recv(req); err != nil {
			wp.requestPool.Put(req)

			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("recv request", "error", err)
			}
			return
		}

		ants.Submit(func() {
			defer wp.requestPool.Put(req)

			md := sessionMD.Copy()
			// 事务ID
			md.Set(nodehub.MDTransactionID, ulid.Make().String())
			ctx := metadata.NewOutgoingContext(context.Background(), md)

			if err := wp.handleUnary(ctx, sess, req); err != nil {
				logger.Error("handle request",
					"error", err,
					"service_code", req.ServiceCode,
					"method", req.Method,
					"request_id", req.Id,
					"remote_addr", conn.RemoteAddr().String(),
				)

				if s, ok := status.FromError(err); ok {
					resp, _ := clientpb.NewResponse(int32(gatewaypb.Protocol_RPC_ERROR), &gatewaypb.RPCError{
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

func (wp *WebsocketProxy) handleUnary(ctx context.Context, sess Session, req *clientpb.Request) error {
	conn, desc, err := wp.Registry.GetGRPCServiceConn(req.ServiceCode)
	if err != nil {
		return fmt.Errorf("get grpc conn, %w", err)
	} else if !desc.Public {
		return errRequestPrivateService
	}

	input, err := newEmptyMessage(req.Data)
	if err != nil {
		return fmt.Errorf("unmarshal request data, %w", err)
	}
	output := wp.responsePool.Get().(*clientpb.Response)
	defer wp.responsePool.Put(output)
	resetResponse(output)

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

func (wp *WebsocketProxy) notificationLoop() {
	if wp.Notifier == nil {
		return
	}

	c, err := wp.Notifier.Subscribe(context.Background())
	if err != nil {
		logger.Error("subscribe notification", "error", err)

		panic(fmt.Errorf("subscribe notification, %w", err))
	}

	for {
		select {
		case <-wp.done:
			return
		case msg := <-c:
			if sess, ok := wp.sessionHub.Load(msg.GetUserId()); ok {
				// 只发送5分钟内的消息
				if time.Since(msg.GetTime().AsTime()) <= 5*time.Minute {
					ants.Submit(func() {
						sess.Send(msg.Content)
					})
				}
			}
		}
	}
}

func resetRequest(req *clientpb.Request) {
	req.Id = 0
	req.ServiceCode = 0
	req.Method = ""

	if len(req.Data) > 0 {
		req.Data = req.Data[:0]
	}
}

func resetResponse(resp *clientpb.Response) {
	resp.RequestId = 0
	resp.ServiceCode = 0
	resp.Type = 0

	if len(resp.Data) > 0 {
		resp.Data = resp.Data[:0]
	}
}

func newEmptyMessage(data []byte) (*emptypb.Empty, error) {
	msg := &emptypb.Empty{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return msg, nil
}
