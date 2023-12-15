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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	upgrader = websocket.Upgrader{}

	requestPool = &sync.Pool{
		New: func() any {
			return &clientpb.Request{}
		},
	}

	responsePool = &sync.Pool{
		New: func() any {
			return &clientpb.Response{}
		},
	}
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
	stateTable    *stateTable

	done chan struct{}
}

// NewWebsocketProxy 构造函数
func NewWebsocketProxy(registry *cluster.Registry, listenAddr string, opt ...WebsocketProxyOption) *WebsocketProxy {
	wp := &WebsocketProxy{
		registry:   registry,
		sessionHub: newSessionHub(),
		stateTable: newStateTable(),
		done:       make(chan struct{}),
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

	// 有状态路由更新
	if wp.eventBus != nil {
		wp.eventBus.Subscribe(ctx, func(ev event.NodeAssign) {
			if _, ok := wp.sessionHub.Load(ev.UserID); ok {
				wp.stateTable.Store(ev.UserID, ev.ServiceCode, ev.NodeID)
			}
		})
		wp.eventBus.Subscribe(ctx, func(ev event.NodeUnassign) {
			if _, ok := wp.sessionHub.Load(ev.UserID); ok {
				wp.stateTable.Remove(ev.UserID, ev.ServiceCode)
			}
		})
	}

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

func (wp *WebsocketProxy) serveHTTP(w http.ResponseWriter, r *http.Request) { // revive:disable-line
	sess, err := wp.newSession(w, r)
	if err != nil {
		logger.Error("initialize session", "error", err)
		return
	}
	logger.Info("client connected", "sessionID", sess.ID(), "remoteAddr", sess.RemoteAddr())

	// 初始化
	wp.sessionHub.Store(sess)

	if wp.eventBus != nil && wp.authorize != nil {
		wp.eventBus.Publish(context.Background(), event.UserConnected{
			UserID: sess.ID(),
		})
	}

	// 结束清理
	defer func() {
		logger.Info("client disconnected", "sessionID", sess.ID(), "remoteAddr", sess.RemoteAddr())

		wp.sessionHub.Delete(sess.ID())
		sess.Close()

		if wp.eventBus != nil && wp.authorize != nil {
			wp.eventBus.Publish(context.Background(), event.UserDisconnected{
				UserID: sess.ID(),
			})
		}

		sessID := sess.ID()
		ants.Submit(func() {
			// 延迟5分钟之后，确认session不存在了，则清除相关数据
			time.Sleep(5 * time.Minute)
			if _, ok := wp.sessionHub.Load(sessID); !ok {
				wp.stateTable.Clean(sessID)
			}
		})
	}()

	type request struct {
		service int32
		fn      func()
	}

	requestC := make(chan request)
	defer close(requestC)

	// 把每个service的请求分发到不同的worker处理
	// 确保对同一个service的请求是顺序处理的
	go func() {
		type worker struct {
			C          chan func()
			ActiveTime time.Time
		}

		workers := map[int32]*worker{}
		defer func() {
			for _, w := range workers {
				close(w.C)
			}
		}()

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-wp.done:
				return
			case <-ticker.C:
				// 清除不活跃的worker
				for key, w := range workers {
					if time.Since(w.ActiveTime) > 5*time.Minute {
						close(w.C)
						delete(workers, key)
					}
				}
			case req, ok := <-requestC:
				if !ok {
					return
				}

				w, ok := workers[req.service]
				if !ok {
					w = &worker{
						C: make(chan func(), 100),
					}
					workers[req.service] = w

					go func() {
						for fn := range w.C {
							fn() // 错误会被打印到请求日志中，这里就不需要再处理
						}
					}()
				}

				w.C <- req.fn
				w.ActiveTime = time.Now()
			}
		}
	}()

	for {
		req := requestPool.Get().(*clientpb.Request)
		clientpb.ResetRequest(req)

		if err := sess.Recv(req); err != nil {
			requestPool.Put(req)

			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("recv request", "error", err)
			}
			return
		}

		exec, unordered := wp.newUnaryRequest(context.Background(), req, sess)
		fn := func() {
			exec()
			requestPool.Put(req)
		}

		if unordered {
			// 允许无序执行，并发处理
			ants.Submit(fn)
		} else {
			// 需要保证时序性，投递到worker处理
			select {
			case <-wp.done:
				return
			case requestC <- request{
				service: req.ServiceCode,
				fn:      fn,
			}:
			}
		}
	}
}

func (wp *WebsocketProxy) newSession(w http.ResponseWriter, r *http.Request) (Session, error) {
	var (
		userID string
		md     = metadata.MD{}
		ok     bool
	)

	if wp.authorize != nil {
		userID, md, ok = wp.authorize(w, r)
		if !ok {
			return nil, errors.New("deny by authorize")
		} else if md == nil {
			md = metadata.MD{}
		}
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("upgrade websocket, %w", err)
	}

	sess := newWsSession(wsConn)
	if userID != "" {
		sess.SetID(userID)
		md.Set(rpc.MDUserID, userID)
	}

	if len(md) > 0 {
		sess.SetMetadata(md)
	}
	return sess, nil
}

func (wp *WebsocketProxy) newUnaryRequest(ctx context.Context, req *clientpb.Request, sess Session) (exec func(), unordered bool) {
	// 以status.Error()构造的错误，都会被下行通知到客户端
	var err error
	desc, ok := wp.registry.GetGRPCDesc(req.ServiceCode)
	if !ok {
		err = status.Errorf(codes.NotFound, "grpc service code(%d) not found", req.ServiceCode)
	} else if !desc.Public {
		err = status.Error(codes.PermissionDenied, "request private service")
	}

	var doRequest func() error
	startTime := time.Now()
	if err != nil {
		unordered = true // 不需要保证时序性

		doRequest = func() error {
			wp.logRequest(ctx, nil, sess, req, startTime, err)
			return err
		}
	} else {
		unordered = desc.Unordered

		doRequest = func() (err error) {
			var conn *grpc.ClientConn
			defer func() {
				wp.logRequest(ctx, conn, sess, req, startTime, err)
			}()

			conn, err = wp.getUpstream(sess, req, desc)
			if err != nil {
				return
			}

			input, err := newEmptyMessage(req.Data)
			if err != nil {
				return fmt.Errorf("unmarshal request data, %w", err)
			}

			output := responsePool.Get().(*clientpb.Response)
			defer responsePool.Put(output)
			clientpb.ResetResponse(output)

			md := sess.MetadataCopy()
			md.Set(rpc.MDTransactionID, ulid.Make().String()) // 事务ID
			ctx = metadata.NewOutgoingContext(ctx, md)

			method := path.Join(desc.Path, req.Method)
			if err := grpc.Invoke(ctx, method, input, output, conn); err != nil {
				return fmt.Errorf("call grpc, %w", err)
			}

			// google.protobuf.Empty类型，不需要下行数据
			if proto.Size(output) == 0 {
				return nil
			}
			output.RequestId = req.Id
			output.ServiceCode = req.ServiceCode

			return sess.Send(output)
		}
	}

	exec = func() {
		if err := doRequest(); err != nil {
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
	}
	return
}

func (wp *WebsocketProxy) getUpstream(sess Session, req *clientpb.Request, desc cluster.GRPCServiceDesc) (conn *grpc.ClientConn, err error) {
	var nodeID string
	// 无状态服务，根据负载均衡策略选择一个节点发送
	if !desc.Stateful {
		nodeID, err = wp.registry.PickGRPCNode(req.ServiceCode)
		if err != nil {
			err = status.Errorf(codes.Unavailable, "pick grpc node, %v", err)
			return
		}

		goto FINISH
	}

	// 从请求参数或者状态路由表内获取节点ID
	if nodeID = req.GetNodeId(); nodeID == "" {
		if v, ok := wp.stateTable.Find(sess.ID(), req.ServiceCode); ok {
			nodeID = v
		}
	}

	if nodeID != "" {
		goto FINISH
	}

	// 显式分配策略，如果没有找到节点就中止处理
	if desc.Allocation == cluster.ExplicitAllocate {
		err = status.Error(codes.PermissionDenied, "no node allocated")
		return
	}

	// 自动分配策略，根据负载均衡策略选择一个节点发送
	nodeID, err = wp.registry.PickGRPCNode(req.ServiceCode)
	if err != nil {
		err = status.Errorf(codes.Unavailable, "pick grpc node, %v", err)
		return
	}
	defer func() {
		if err == nil {
			wp.stateTable.Store(sess.ID(), req.ServiceCode, nodeID)
		}
	}()

FINISH:
	conn, err = wp.registry.GetGRPCConn(nodeID)
	if err != nil {
		err = status.Errorf(codes.Unavailable, "get grpc conn, %v", err)
	}
	return
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

	wp.notifier.Subscribe(context.Background(), func(msg *clientpb.Notification) {
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
	})
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
