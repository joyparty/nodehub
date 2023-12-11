package gateway

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oklog/ulid/v2"
	"github.com/panjf2000/ants/v2"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/component/rpc"
	"gitlab.haochang.tv/gopkg/nodehub/event"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/notification"
	"gitlab.haochang.tv/gopkg/nodehub/proto/clientpb"
	"gitlab.haochang.tv/gopkg/nodehub/proto/gatewaypb"
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
	registry  *cluster.Registry
	notifier  notification.Subscriber
	authorize Authorize
	eventBus  event.Bus

	requestLogger logger.Logger
	server        *http.Server
	sessionHub    *sessionHub

	requestPool  *sync.Pool
	responsePool *sync.Pool

	done chan struct{}
}

// NewWebsocketProxy 构造函数
func NewWebsocketProxy(registry *cluster.Registry, listenAddr string, opt ...WebsocketProxyOption) *WebsocketProxy {
	wp := &WebsocketProxy{
		registry:   registry,
		sessionHub: newSessionHub(),
		done:       make(chan struct{}),

		authorize: func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool) {
			return "", metadata.MD{}, true
		},

		requestPool: &sync.Pool{
			New: func() any {
				return &clientpb.Request{}
			},
		},
		responsePool: &sync.Pool{
			New: func() any {
				return &clientpb.Response{}
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/grpc", wp.serveHTTP)

	wp.server = &http.Server{
		Addr:    listenAddr,
		Handler: http.HandlerFunc(mux.ServeHTTP),
	}

	for _, fn := range opt {
		fn(wp)
	}

	return wp
}

// Name 服务名称
func (wp *WebsocketProxy) Name() string {
	return "gateway"
}

// CompleteNodeEntry 补全节点信息
func (wp *WebsocketProxy) CompleteNodeEntry(entry *cluster.NodeEntry) {
	entry.Websocket = fmt.Sprintf("ws://%s/grpc", wp.server.Addr)
}

// Start 启动websocket服务器
func (wp *WebsocketProxy) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", wp.server.Addr)
	if err != nil {
		return fmt.Errorf("listen, %w", err)
	}

	go func() {
		if err := wp.server.Serve(l); err != nil && err != http.ErrServerClosed {
			logger.Error("start gateway", "error", err)

			panic(fmt.Errorf("start gateway, %w", err))
		}
	}()

	go wp.notificationLoop()

	return nil
}

// Stop 停止websocket服务器
func (wp *WebsocketProxy) Stop(ctx context.Context) error {
	if wp.authorize != nil && wp.eventBus != nil {
		wp.sessionHub.Range(func(s Session) bool {
			wp.eventBus.Publish(context.Background(), event.UserDisconnected{
				UserID: s.ID(),
			})
			return true
		})
	}

	close(wp.done)
	wp.server.Shutdown(ctx)
	wp.sessionHub.Close()
	return nil
}

func (wp *WebsocketProxy) serveHTTP(w http.ResponseWriter, r *http.Request) {
	userID, sessionMD, ok := wp.authorize(w, r)
	if !ok {
		logger.Debug("deny by authorize", "remote_addr", r.RemoteAddr)
		return
	} else if userID != "" {
		// 把user id放到request header
		sessionMD.Set(rpc.MDUserID, userID)
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

		if userID != "" && wp.eventBus != nil {
			wp.eventBus.Publish(context.Background(), event.UserDisconnected{
				UserID: userID,
			})
		}
	}()

	if userID != "" && wp.eventBus != nil {
		wp.eventBus.Publish(context.Background(), event.UserConnected{
			UserID: userID,
		})
	}

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
			md.Set(rpc.MDTransactionID, ulid.Make().String())
			ctx := metadata.NewOutgoingContext(context.Background(), md)

			if err := wp.handleUnary(ctx, sess, req); err != nil {
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

func (wp *WebsocketProxy) handleUnary(ctx context.Context, sess Session, req *clientpb.Request) (err error) {
	var (
		start    = time.Now()
		upstream *grpc.ClientConn
	)
	defer func() {
		wp.logRequest(ctx, upstream, sess, req, start, err)
	}()

	conn, desc, err := wp.registry.GetGRPCServiceConn(req.ServiceCode)
	if err != nil {
		return fmt.Errorf("get grpc conn, %w", err)
	} else if !desc.Public {
		return errRequestPrivateService
	}
	upstream = conn

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

func (wp *WebsocketProxy) logRequest(
	ctx context.Context,
	upstream *grpc.ClientConn,
	sess Session,
	req *clientpb.Request,
	start time.Time,
	err error,
) {
	if err == nil && wp.requestLogger == nil {
		return
	}

	logValues := []any{
		"reqID", req.Id,
		"remoteAddr", sess.RemoteAddr(),
		"serviceCode", req.ServiceCode,
		"method", req.Method,
		"duration", time.Since(start).String(),
	}

	if upstream != nil {
		logValues = append(logValues, "upstream", upstream.Target())
	}

	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if v := md.Get(rpc.MDTransactionID); len(v) > 0 {
			logValues = append(logValues, "transID", v[0])
		}
		if v := md.Get(rpc.MDUserID); len(v) > 0 {
			logValues = append(logValues, "userID", v[0])
		}
	}

	if err != nil {
		logValues = append(logValues, "error", err)

		if wp.requestLogger == nil {
			logger.Error("handle request", logValues...)
		} else {
			wp.requestLogger.Error("handle request", logValues...)
		}
	} else {
		wp.requestLogger.Info("handle request", logValues...)
	}
}

func (wp *WebsocketProxy) notificationLoop() {
	if wp.notifier == nil {
		return
	}

	c, err := wp.notifier.Subscribe(context.Background())
	if err != nil {
		logger.Error("subscribe notification", "error", err)

		panic(fmt.Errorf("subscribe notification, %w", err))
	}

	for {
		select {
		case <-wp.done:
			return
		case msg := <-c:
			// 只发送5分钟内的消息
			if time.Since(msg.GetTime().AsTime()) <= 5*time.Minute {
				for _, userID := range msg.GetReceiver() {
					if sess, ok := wp.sessionHub.Load(userID); ok {
						ants.Submit(func() {
							sess.Send(msg.Content)
						})
					}
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

// WebsocketProxyOption 配置选项
type WebsocketProxyOption func(*WebsocketProxy)

// WithNotifier 设置主动下发消息订阅者
func WithNotifier(n notification.Subscriber) WebsocketProxyOption {
	return func(wp *WebsocketProxy) {
		wp.notifier = n
	}
}

// Authorize 身份验证逻辑
//
// 自定义身份验证逻辑，在websocket upgrade之前调用
// 返回的metadata会在此连接的所有grpc request中携带
// 返回的userID如果不为空，则会作为会话唯一标识使用，另外也会被自动加入到metadata中
// 如果返回ok为false，会直接关闭连接
// 因此如果验证不通过之类的错误，需要在这个函数里面自行处理
type Authorize func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool)

// WithAuthorize 设置身份验证逻辑
func WithAuthorize(fn Authorize) WebsocketProxyOption {
	return func(wp *WebsocketProxy) {
		wp.authorize = fn
	}
}

// WithRequestLog 设置是否记录请求日志
func WithRequestLog(l logger.Logger) WebsocketProxyOption {
	return func(wp *WebsocketProxy) {
		wp.requestLogger = l
	}
}

// WithEventBus 设置事件总线
func WithEventBus(bus event.Bus) WebsocketProxyOption {
	return func(wp *WebsocketProxy) {
		wp.eventBus = bus
	}
}
