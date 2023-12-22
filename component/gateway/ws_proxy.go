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
	"github.com/joyparty/gokit"
	"github.com/oklog/ulid/v2"
	"github.com/panjf2000/ants/v2"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/component/rpc"
	"gitlab.haochang.tv/gopkg/nodehub/event"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/notification"
	"gitlab.haochang.tv/gopkg/nodehub/proto/nodehubpb"
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
			return &nodehubpb.Request{}
		},
	}

	replyPool = &sync.Pool{
		New: func() any {
			return &nodehubpb.Reply{}
		},
	}
)

// WSProxy 网关服务器
//
// 客户端通过websocket方式连接网关，网关再转发请求到grpc后端服务
type WSProxy struct {
	nodeID    string
	registry  *cluster.Registry
	notifier  notification.Subscriber
	authorize Authorize
	eventBus  event.Bus

	requestLogger logger.Logger
	server        *http.Server
	sessionHub    *sessionHub
	stateTable    *stateTable
	cleanJobs     *gokit.MapOf[string, *time.Timer]

	done chan struct{}
}

// NewWSProxy 构造函数
func NewWSProxy(nodeID string, registry *cluster.Registry, listenAddr string, opt ...WSProxyOption) *WSProxy {
	wp := &WSProxy{
		nodeID:   nodeID,
		registry: registry,
		authorize: func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool) {
			w.WriteHeader(http.StatusUnauthorized)

			return "", nil, false
		},
		sessionHub: newSessionHub(),
		stateTable: newStateTable(),
		cleanJobs:  gokit.NewMapOf[string, *time.Timer](),
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
func (wp *WSProxy) Name() string {
	return "wsproxy"
}

// NewGRPCService 创建grpc服务
func (wp *WSProxy) NewGRPCService() nodehubpb.GatewayServer {
	return &gwService{
		sessionHub: wp.sessionHub,
		stateTable: wp.stateTable,
	}
}

// CompleteNodeEntry 补全节点信息
func (wp *WSProxy) CompleteNodeEntry(entry *cluster.NodeEntry) {
	entry.Websocket = fmt.Sprintf("ws://%s/grpc", wp.server.Addr)
}

// Start 启动websocket服务器
func (wp *WSProxy) Start(ctx context.Context) error {
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
			// 即使离线，状态路由也会继续留存一段时间，所以仍然接受删除操作
			wp.stateTable.Remove(ev.UserID, ev.ServiceCode)
		})
	}

	wp.registry.SubscribeDelete(func(entry cluster.NodeEntry) {
		wp.stateTable.CleanNode(entry.ID)
	})

	return nil
}

// Stop 停止websocket服务器
func (wp *WSProxy) Stop(ctx context.Context) error {
	if wp.eventBus != nil {
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

func (wp *WSProxy) serveHTTP(w http.ResponseWriter, r *http.Request) {
	sess, err := wp.newSession(w, r)
	if err != nil {
		logger.Error("initialize session", "error", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wp.onConnect(ctx, sess)
	defer wp.onDisconnect(ctx, sess)

	requestQueue := make(chan serviceReqeust)
	defer close(requestQueue)
	go wp.runRequestQueue(ctx, requestQueue)

	for {
		select {
		case <-wp.done:
			return
		default:
		}

		req := requestPool.Get().(*nodehubpb.Request)
		nodehubpb.ResetRequest(req)

		if err := sess.Recv(req); err != nil {
			requestPool.Put(req)

			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("recv request", "error", err)
			}
			return
		}

		exec, unordered := wp.newUnaryRequest(ctx, req, sess)
		fn := func() {
			exec()
			requestPool.Put(req)
		}

		if unordered {
			// 允许无序执行，并发处理
			ants.Submit(fn)
		} else {
			// 需要保证时序性，投递到队列处理
			select {
			case <-wp.done:
				return
			case requestQueue <- serviceReqeust{
				Service: req.ServiceCode,
				Request: fn,
			}:
			}
		}
	}
}

func (wp *WSProxy) newSession(w http.ResponseWriter, r *http.Request) (Session, error) {
	userID, md, ok := wp.authorize(w, r)
	if !ok {
		return nil, errors.New("deny by authorize")
	} else if userID == "" {
		return nil, errors.New("empty userID")
	} else if md == nil {
		md = metadata.MD{}
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("upgrade websocket, %w", err)
	}

	sess := newWsSession(wsConn)
	sess.SetID(userID)

	md.Set(rpc.MDUserID, userID)
	md.Set(rpc.MDGateway, wp.nodeID)
	sess.SetMetadata(md)

	return sess, nil
}

func (wp *WSProxy) onConnect(ctx context.Context, sess Session) {
	wp.sessionHub.Store(sess)

	// 放弃之前断线创造的清理任务
	if timer, ok := wp.cleanJobs.Load(sess.ID()); ok {
		if !timer.Stop() {
			<-timer.C
		}
		wp.cleanJobs.Delete(sess.ID())
	}

	if wp.eventBus != nil {
		wp.eventBus.Publish(ctx, event.UserConnected{
			UserID: sess.ID(),
		})
	}

	logger.Info("client connected", "sessionID", sess.ID(), "remoteAddr", sess.RemoteAddr())
}

func (wp *WSProxy) onDisconnect(ctx context.Context, sess Session) {
	sessID := sess.ID()
	wp.sessionHub.Delete(sessID)
	sess.Close()

	if wp.eventBus != nil {
		wp.eventBus.Publish(ctx, event.UserDisconnected{
			UserID: sessID,
		})
	}

	// 延迟5分钟之后，确认session不存在了，则清除相关数据
	wp.cleanJobs.Store(sessID, time.AfterFunc(5*time.Minute, func() {
		if _, ok := wp.sessionHub.Load(sessID); !ok {
			wp.stateTable.CleanSession(sessID)
		}
		wp.cleanJobs.Delete(sessID)
	}))

	logger.Info("client disconnected", "sessionID", sessID, "remoteAddr", sess.RemoteAddr())
}

func (wp *WSProxy) newUnaryRequest(ctx context.Context, req *nodehubpb.Request, sess Session) (exec func(), unordered bool) {
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

			output := replyPool.Get().(*nodehubpb.Reply)
			defer replyPool.Put(output)
			nodehubpb.ResetReply(output)

			md := sess.MetadataCopy()
			md.Set(rpc.MDTransactionID, ulid.Make().String()) // 事务ID
			ctx = metadata.NewOutgoingContext(ctx, md)

			method := path.Join(desc.Path, req.Method)
			if err := grpc.Invoke(ctx, method, input, output, conn); err != nil {
				return fmt.Errorf("call grpc, %w", err)
			}

			if req.GetNoReply() {
				return nil
			}

			output.RequestId = req.Id
			output.FromService = req.ServiceCode
			return sess.Send(output)
		}
	}

	exec = func() {
		if err := doRequest(); err != nil {
			if s, ok := status.FromError(err); ok {
				// unknown错误，不下行详细的错误描述，避免泄露信息到客户端
				if s.Code() == codes.Unknown {
					s = status.New(codes.Unknown, "unknown error")
				}

				resp, _ := nodehubpb.NewReply(int32(nodehubpb.Protocol_RPC_ERROR), &nodehubpb.RPCError{
					RequestService: req.ServiceCode,
					RequestMethod:  req.Method,
					Status:         s.Proto(),
				})
				resp.RequestId = req.Id

				sess.Send(resp)
			}
		}
	}
	return
}

func (wp *WSProxy) getUpstream(sess Session, req *nodehubpb.Request, desc cluster.GRPCServiceDesc) (conn *grpc.ClientConn, err error) {
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

	if desc.Allocation == cluster.ClientAllocate {
		// 每次客户端指定了节点，记录下来，后续使用
		if nodeID = req.GetNodeId(); nodeID != "" {
			defer func() {
				if err == nil {
					wp.stateTable.Store(sess.ID(), req.ServiceCode, nodeID)
				}
			}()
			goto FINISH
		}
	}

	// 从状态路由表查询节点ID
	if v, ok := wp.stateTable.Find(sess.ID(), req.ServiceCode); ok {
		nodeID = v
		goto FINISH
	}

	// 非自动分配策略，没有找到节点就中断请求
	if desc.Allocation != cluster.AutoAllocate && nodeID == "" {
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

func (wp *WSProxy) logRequest(
	ctx context.Context,
	upstream *grpc.ClientConn,
	sess Session,
	req *nodehubpb.Request,
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

	if nodeID := req.GetNodeId(); nodeID != "" {
		logValues = append(logValues, "nodeID", nodeID)
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

type serviceReqeust struct {
	Service int32
	Request func()
}

// 把每个service的请求分发到不同的worker处理
// 确保对同一个service的请求是顺序处理的
func (wp *WSProxy) runRequestQueue(ctx context.Context, queue <-chan serviceReqeust) {
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
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 清除不活跃的worker
			for key, w := range workers {
				if time.Since(w.ActiveTime) > 5*time.Minute {
					close(w.C)
					delete(workers, key)
				}
			}
		case item, ok := <-queue:
			if !ok {
				return
			}

			w, ok := workers[item.Service]
			if !ok {
				w = &worker{
					C: make(chan func(), 100),
				}
				workers[item.Service] = w

				go func() {
					for fn := range w.C {
						fn() // 错误会被打印到请求日志中，这里就不需要再处理
					}
				}()
			}

			w.C <- item.Request
			w.ActiveTime = time.Now()
		}
	}
}

func (wp *WSProxy) notificationLoop() {
	if wp.notifier == nil {
		return
	}

	wp.notifier.Subscribe(context.Background(), func(msg *nodehubpb.Notification) {
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

func newEmptyMessage(data []byte) (msg *emptypb.Empty, err error) {
	msg = &emptypb.Empty{}
	if len(data) > 0 {
		err = proto.Unmarshal(data, msg)
	}
	return
}

// WSProxyOption 配置选项
type WSProxyOption func(*WSProxy)

// WithNotifier 设置主动下发消息订阅者
func WithNotifier(n notification.Subscriber) WSProxyOption {
	return func(wp *WSProxy) {
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
func WithAuthorize(fn Authorize) WSProxyOption {
	return func(wp *WSProxy) {
		wp.authorize = fn
	}
}

// WithRequestLog 设置是否记录请求日志
func WithRequestLog(l logger.Logger) WSProxyOption {
	return func(wp *WSProxy) {
		wp.requestLogger = l
	}
}

// WithEventBus 设置事件总线
func WithEventBus(bus event.Bus) WSProxyOption {
	return func(wp *WSProxy) {
		wp.eventBus = bus
	}
}
