package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"path"
	"sync/atomic"
	"time"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/component/rpc"
	"github.com/joyparty/nodehub/event"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/multicast"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/oklog/ulid/v2"
	"github.com/panjf2000/ants/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	// DefaultHeartbeatDuration 心跳消息发送时间间隔，连接在超过这个时间没有收发消息，就会发送心跳消息
	DefaultHeartbeatDuration = 1 * time.Minute

	// DefaultHeartbeatTimeout 心跳消息超时时间，默认为心跳消息发送时间间隔的3倍
	DefaultHeartbeatTimeout = 3 * DefaultHeartbeatDuration

	// MaxMessageSize 客户端消息最大长度，默认64KB
	MaxMessageSize = 64 * 1024

	requestPool = gokit.NewPoolOf(func() *nh.Request {
		return &nh.Request{}
	})

	replyPool = gokit.NewPoolOf(func() *nh.Reply {
		return &nh.Reply{}
	})
)

// Authorizer 身份认证
//
// 返回的metadata会在此连接的所有grpc request中携带
// 返回的userID会作为会话唯一标识使用，也会被自动加入到metadata中
// 如果返回ok为false，会直接关闭连接
// 因此如果验证不通过之类的错误，需要在这个函数里面自行处理
type Authorizer func(ctx context.Context, sess Session) (userID string, md metadata.MD, ok bool)

// Transporter 网关传输层接口
type Transporter interface {
	CompleteNodeEntry(entry *cluster.NodeEntry)
	Serve(ctx context.Context) (chan Session, error)
	Shutdown(ctx context.Context) error
}

// Session 连接会话
type Session interface {
	ID() string
	SetID(string)
	SetMetadata(metadata.MD)
	MetadataCopy() metadata.MD
	Recv(*nh.Request) error
	Send(*nh.Reply) error
	LocalAddr() string
	RemoteAddr() string
	LastRWTime() time.Time
	Close() error
	LogValue() slog.Value
}

// sessionHub 会话集合
type sessionHub struct {
	sessions *gokit.MapOf[string, Session]
	count    *atomic.Int32
	done     chan struct{}
	closed   *atomic.Bool
}

func newSessionHub() *sessionHub {
	hub := &sessionHub{
		sessions: gokit.NewMapOf[string, Session](),
		count:    &atomic.Int32{},
		done:     make(chan struct{}),
		closed:   &atomic.Bool{},
	}

	go hub.removeZombie()
	return hub
}

func (h *sessionHub) Count() int32 {
	return h.count.Load()
}

func (h *sessionHub) Store(c Session) {
	h.sessions.Store(c.ID(), c)
	h.count.Add(1)
}

func (h *sessionHub) Load(id string) (Session, bool) {
	if c, ok := h.sessions.Load(id); ok {
		return c.(Session), true
	}
	return nil, false
}

func (h *sessionHub) Delete(id string) {
	if _, ok := h.sessions.Load(id); ok {
		h.sessions.Delete(id)
		h.count.Add(-1)
	}
}

func (h *sessionHub) Range(f func(s Session) bool) {
	h.sessions.Range(func(_ string, value Session) bool {
		return f(value)
	})
}

func (h *sessionHub) Close() {
	if h.closed.CompareAndSwap(false, true) {
		close(h.done)

		h.Range(func(s Session) bool {
			_ = s.Close()

			h.Delete(s.ID())
			return true
		})
	}
}

// 定时移除心跳超时的客户端
func (h *sessionHub) removeZombie() {
	for {
		select {
		case <-h.done:
			return
		case <-time.After(10 * time.Second):
			h.Range(func(s Session) bool {
				if time.Since(s.LastRWTime()) > DefaultHeartbeatTimeout {
					logger.Info("remove heartbeat timeout session", "session", s)

					h.Delete(s.ID())
					_ = s.Close()
				}
				return true
			})
		}
	}
}

// Proxy 客户端会话运行环境
type Proxy struct {
	nodeID        ulid.ULID
	transporter   Transporter
	authorizer    Authorizer
	registry      *cluster.Registry
	eventBus      *event.Bus
	multicast     multicast.Subscriber
	requestLogger logger.Logger

	sessions   *sessionHub
	stateTable *stateTable
	cleanJobs  *gokit.MapOf[string, *time.Timer]
	done       chan struct{}

	requestInterceptor    RequestInterceptor
	connectInterceptor    ConnectInterceptor
	disconnectInterceptor DisconnectInterceptor
}

// NewProxy 构造函数
func NewProxy(nodeID ulid.ULID, opt ...Option) (*Proxy, error) {
	p := &Proxy{
		nodeID:     nodeID,
		sessions:   newSessionHub(),
		stateTable: newStateTable(),
		cleanJobs:  gokit.NewMapOf[string, *time.Timer](),
		done:       make(chan struct{}),

		requestInterceptor:    defaultRequestInterceptor,
		connectInterceptor:    defaultConnectInterceptor,
		disconnectInterceptor: defaultDisconnectInterceptor,
	}

	for _, fn := range opt {
		fn(p)
	}

	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Proxy) validate() error {
	if p.registry == nil {
		return errors.New("registry is nil")
	} else if p.transporter == nil {
		return errors.New("transporter is nil")
	} else if p.authorizer == nil {
		return errors.New("authorizer is nil")
	} else if p.eventBus == nil {
		return errors.New("eventBus is nil")
	} else if p.multicast == nil {
		return errors.New("multicast subscriber is nil")
	}

	return nil
}

// Name 服务名称
func (p *Proxy) Name() string {
	return "proxy"
}

// CompleteNodeEntry 补全节点信息
func (p *Proxy) CompleteNodeEntry(entry *cluster.NodeEntry) {
	p.transporter.CompleteNodeEntry(entry)
}

// Start 启动服务
func (p *Proxy) Start(ctx context.Context) error {
	p.init(ctx)

	sc, err := p.transporter.Serve(ctx)
	if err != nil {
		return fmt.Errorf("start transporter, %w", err)
	}

	go func() {
		for {
			select {
			case <-p.done:
				return
			case sess, ok := <-sc:
				if !ok {
					return
				}

				if err := ants.Submit(func() {
					p.handleSession(ctx, sess)
				}); err != nil {
					logger.Error("handle session", "error", err, "session", sess)
				}
			}
		}
	}()

	return nil
}

// Stop 停止服务
func (p *Proxy) Stop(ctx context.Context) {
	close(p.done)
	p.sessions.Close()

	if err := p.transporter.Shutdown(ctx); err != nil && !errors.Is(err, net.ErrClosed) {
		logger.Error("shutdown gateway transporter", "error", err)
	}
}

// NewGRPCService 网关管理服务
func (p *Proxy) NewGRPCService() nh.GatewayServer {
	return &gwService{
		sessionHub: p.sessions,
		stateTable: p.stateTable,
	}
}

func (p *Proxy) init(ctx context.Context) {
	// 有状态路由更新
	p.eventBus.Subscribe(ctx, func(ev event.NodeAssign, _ time.Time) {
		if _, ok := p.sessions.Load(ev.SessionID); ok {
			p.stateTable.Store(ev.SessionID, ev.ServiceCode, ev.NodeID)
		}
	})
	p.eventBus.Subscribe(ctx, func(ev event.NodeUnassign, _ time.Time) {
		p.stateTable.Remove(ev.SessionID, ev.ServiceCode)
	})

	// 禁止同一个用户同时连接多个网关
	p.eventBus.Subscribe(ctx, func(ev event.UserConnected, _ time.Time) {
		if ev.GatewayID != p.nodeID.String() {
			if sess, ok := p.sessions.Load(ev.SessionID); ok {
				p.sessions.Delete(sess.ID())
				sess.Close()
			}
		}
	})

	// 处理主动下行消息
	p.multicast.Subscribe(ctx, func(msg *nh.Multicast) {
		// 只发送5分钟内的消息
		if time.Since(msg.GetTime().AsTime()) <= 5*time.Minute {
			for _, sessID := range msg.GetReceiver() {
				if sess, ok := p.sessions.Load(sessID); ok {
					ants.Submit(func() {
						sess.Send(msg.Content)
					})
				}
			}
		}
	})

	p.registry.SubscribeDelete(func(entry cluster.NodeEntry) {
		p.stateTable.CleanNode(entry.ID)
	})
}

// Handle 处理客户端连接
func (p *Proxy) handleSession(ctx context.Context, sess Session) {
	logVars := []any{
		"session", sess,
		"gateway", p.nodeID.String(),
	}

	logger.Info("session connected", logVars...)
	defer logger.Info("session disconnected", logVars...)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := p.onConnect(ctx, sess); err != nil {
		logger.Error("on connect", "error", err, "session", sess)
		_ = sess.Close()
		return
	}
	defer p.onDisconnect(ctx, sess)

	taskC := make(chan requestTask)
	defer close(taskC)
	go p.runTasks(ctx, taskC)

	tracer := newRequestTracer()
	defer tracer.Release()

	for {
		select {
		case <-p.done:
			return
		default:
		}

		req := requestPool.Get()
		nh.ResetRequest(req)

		if err := sess.Recv(req); err != nil {
			requestPool.Put(req)

			if !errors.Is(err, io.EOF) {
				logger.Error("recv request", "error", err, "session", sess)
			}

			return
		}

		if pass, err := p.requestInterceptor(ctx, sess, req); err != nil {
			requestPool.Put(req)

			logger.Error("request interceptor",
				"error", err,
				"session", sess,
				"req", req,
			)
		} else if pass {
			run, pipeline := p.buildRequest(ctx, tracer, sess, req)
			task := func() {
				defer requestPool.Put(req)
				run()
			}

			if pipeline == "" {
				// 允许无序执行，并发处理
				ants.Submit(task)
			} else {
				// 需要保证时序性，投递到队列处理
				select {
				case <-p.done:
					return
				case taskC <- requestTask{
					Pipeline: pipeline,
					Task:     task,
				}:
				}
			}
		}
	}
}

func (p *Proxy) onConnect(ctx context.Context, sess Session) error {
	userID, md, ok := p.authorizer(ctx, sess)
	if !ok {
		return errors.New("deny by authorizer")
	} else if userID == "" {
		return errors.New("empty userID")
	} else if md == nil {
		md = metadata.MD{}
	}

	sess.SetID(userID)
	sess.SetMetadata(md)

	if err := p.connectInterceptor(ctx, sess); err != nil {
		return err
	}

	// 放弃之前断线创造的清理任务
	if timer, ok := p.cleanJobs.Load(sess.ID()); ok {
		if !timer.Stop() {
			<-timer.C
		}
		p.cleanJobs.Delete(sess.ID())
	}

	if err := p.eventBus.Publish(ctx, event.UserConnected{
		SessionID: sess.ID(),
		GatewayID: p.nodeID.String(),
	}); err != nil {
		return fmt.Errorf("publish event, %w", err)
	}

	p.sessions.Store(sess)
	return nil
}

func (p *Proxy) onDisconnect(ctx context.Context, sess Session) {
	defer sess.Close()
	p.disconnectInterceptor(ctx, sess)
	p.sessions.Delete(sess.ID())

	// 即使出错也不中断断开流程
	_ = p.eventBus.Publish(ctx, event.UserDisconnected{
		SessionID: sess.ID(),
		GatewayID: p.nodeID.String(),
	})

	// 延迟5分钟之后，确认session不存在了，则清除相关数据
	p.cleanJobs.Store(sess.ID(), time.AfterFunc(5*time.Minute, func() {
		if _, ok := p.sessions.Load(sess.ID()); !ok {
			p.stateTable.CleanSession(sess.ID())
		}
		p.cleanJobs.Delete(sess.ID())
	}))
}

//revive:disable-next-line:cyclomatic High complexity score but easy to understand
func (p *Proxy) buildRequest(ctx context.Context, tracer *requestTracer, sess Session, req *nh.Request) (exec func(), pipeline string) {
	// 以status.Error()构造的错误，都会被下行通知到客户端
	var err error
	desc, ok := p.registry.GetGRPCDesc(req.ServiceCode)
	if !ok {
		err = status.Errorf(codes.NotFound, "service %d not found", req.ServiceCode)
	} else if !desc.Public {
		err = status.Errorf(codes.PermissionDenied, "request private service")
	}

	var (
		doRequest func() error
		start     = time.Now()
	)
	if err != nil {
		pipeline = ""

		doRequest = func() error {
			p.logRequest(ctx, sess, req, start, nil, err)
			return err
		}
	} else {
		pipeline = desc.Pipeline

		doRequest = func() (err error) {
			var conn *grpc.ClientConn
			defer func() {
				p.logRequest(ctx, sess, req, start, conn, err)
			}()

			if lastID, lastReply, ok := tracer.LastReplyOf(pipeline); ok {
				if req.GetId() < lastID {
					return status.Errorf(codes.Aborted, "request id %d is less than last request id %d", req.GetId(), lastID)
				} else if req.GetId() == lastID {
					if req.GetNoReply() {
						return nil
					}
					return sess.Send(lastReply)
				}
			}

			conn, err = p.getUpstream(sess, req, desc)
			if err != nil {
				return
			}

			input, err := newEmptyMessage(req.Data)
			if err != nil {
				return fmt.Errorf("unmarshal request data: %w", err)
			}

			output := replyPool.Get()
			nh.ResetReply(output)

			defer func() {
				if err != nil ||
					!tracer.Save(pipeline, req, output) {
					replyPool.Put(output)
				}
			}()

			md := sess.MetadataCopy()
			md.Set(rpc.MDTransactionID, ulid.Make().String())
			md.Set(rpc.MDSessID, sess.ID())
			md.Set(rpc.MDGateway, p.nodeID.String())
			ctx = metadata.NewOutgoingContext(ctx, md)

			method := path.Join(desc.Path, req.Method)
			if err = conn.Invoke(ctx, method, input, output); err != nil {
				return fmt.Errorf("invoke service: %w", err)
			}

			if req.GetNoReply() {
				return nil
			}

			output.RequestId = req.GetId()
			output.FromService = req.GetServiceCode()
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

				reply, _ := nh.NewReply(int32(nh.Protocol_RPC_ERROR), &nh.RPCError{
					RequestService: req.GetServiceCode(),
					RequestMethod:  req.GetMethod(),
					Status:         s.Proto(),
				})
				reply.RequestId = req.GetId()

				_ = sess.Send(reply)
			}
		}
	}

	return
}

func (p *Proxy) getUpstream(sess Session, req *nh.Request, desc cluster.GRPCServiceDesc) (conn *grpc.ClientConn, err error) {
	var nodeID ulid.ULID
	// 无状态服务，根据负载均衡策略选择一个节点发送
	if !desc.Stateful {
		nodeID, err = p.registry.AllocGRPCNode(req.ServiceCode, sess)
		if err != nil {
			err = status.Errorf(codes.Aborted, "pick grpc node, %v", err)
			return
		}

		goto FINISH
	}

	if desc.Allocation == cluster.ClientAllocate {
		// 每次客户端指定了节点，记录下来，后续使用
		if v := req.GetNodeId(); v != "" {
			nodeID, err = ulid.Parse(v)
			if err != nil {
				err = status.Errorf(codes.InvalidArgument, "invalid request.NodeId, %s", v)
				return
			}

			defer func() {
				if err == nil {
					p.stateTable.Store(sess.ID(), req.ServiceCode, nodeID)
				}
			}()
			goto FINISH
		}
	}

	// 从状态路由表查询节点ID
	if v, ok := p.stateTable.Find(sess.ID(), req.ServiceCode); ok {
		nodeID = v
		goto FINISH
	}

	// 非自动分配策略，没有找到节点就中断请求
	if desc.Allocation != cluster.AutoAllocate && nodeID.Time() == 0 {
		err = status.Error(codes.Aborted, "no node allocated")
		return
	}

	// 自动分配策略，根据负载均衡策略选择一个节点发送
	nodeID, err = p.registry.AllocGRPCNode(req.ServiceCode, sess)
	if err != nil {
		err = status.Errorf(codes.Aborted, "pick grpc node, %v", err)
		return
	}
	defer func() {
		if err == nil {
			p.stateTable.Store(sess.ID(), req.ServiceCode, nodeID)
		}
	}()

FINISH:
	conn, err = p.registry.GetGRPCConn(nodeID)
	if err != nil {
		err = status.Errorf(codes.Aborted, "get grpc conn, %v", err)
	}
	return
}

func (p *Proxy) logRequest(
	ctx context.Context,
	sess Session,
	req *nh.Request,
	start time.Time,
	upstream *grpc.ClientConn,
	err error,
) {
	if err == nil && p.requestLogger == nil {
		return
	}

	logValues := []any{
		"session", sess,
		"req", req,
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
	}

	if err != nil {
		logValues = append(logValues, "error", err)

		if p.requestLogger == nil {
			logger.Error("handle request", logValues...)
		} else {
			p.requestLogger.Error("handle request", logValues...)
		}
	} else {
		p.requestLogger.Info("handle request", logValues...)
	}
}

type requestTask struct {
	Pipeline string
	Task     func()
}

// 把每个service的请求分发到不同的worker处理
// 确保对同一个service的请求是顺序处理的
func (p *Proxy) runTasks(ctx context.Context, reqC <-chan requestTask) {
	type worker struct {
		C          chan func()
		ActiveTime time.Time
	}

	workers := map[string]*worker{}
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
		case task, ok := <-reqC:
			if !ok {
				return
			}

			w, ok := workers[task.Pipeline]
			if !ok {
				w = &worker{
					C: make(chan func(), 100),
				}
				workers[task.Pipeline] = w

				go func() {
					for fn := range w.C {
						fn() // 错误会被打印到请求日志中，这里就不需要再处理
					}
				}()
			}

			w.C <- task.Task
			w.ActiveTime = time.Now()
		}
	}
}

// Option 网关配置选项
type Option func(p *Proxy)

// WithRegistry 设置服务注册中心
func WithRegistry(registry *cluster.Registry) Option {
	return func(p *Proxy) {
		p.registry = registry
	}
}

// WithTransporter 设置传输层
func WithTransporter(transporter Transporter) Option {
	return func(p *Proxy) {
		p.transporter = transporter
	}
}

// WithAuthorizer 设置身份验证
func WithAuthorizer(authorizer Authorizer) Option {
	return func(p *Proxy) {
		p.authorizer = authorizer
	}
}

// RequestInterceptor 请求拦截器
//
// 每次收到客户端请求都会执行，return pass=false会中断当次请求
type RequestInterceptor func(ctx context.Context, sess Session, req *nh.Request) (pass bool, err error)

var defaultRequestInterceptor = func(ctx context.Context, sess Session, req *nh.Request) (pass bool, err error) {
	return true, nil
}

// WithRequestInterceptor 设置请求拦截器
func WithRequestInterceptor(interceptor RequestInterceptor) Option {
	return func(p *Proxy) {
		p.requestInterceptor = interceptor
	}
}

// ConnectInterceptor 在连接创建之后执行自定义操作，返回错误会中断连接
type ConnectInterceptor func(ctx context.Context, sess Session) error

var defaultConnectInterceptor = func(ctx context.Context, sess Session) error {
	return nil
}

// WithConnectInterceptor 设置连接拦截器
func WithConnectInterceptor(interceptor ConnectInterceptor) Option {
	return func(p *Proxy) {
		p.connectInterceptor = interceptor
	}
}

// DisconnectInterceptor 在连接断开前执行自定操作
type DisconnectInterceptor func(ctx context.Context, sess Session)

var defaultDisconnectInterceptor = func(ctx context.Context, sess Session) {}

// WithDisconnectInterceptor 设置断开连接拦截器
func WithDisconnectInterceptor(interceptor DisconnectInterceptor) Option {
	return func(p *Proxy) {
		p.disconnectInterceptor = interceptor
	}
}

// WithEventBus 设置事件总线
func WithEventBus(bus *event.Bus) Option {
	return func(p *Proxy) {
		p.eventBus = bus
	}
}

// WithMulticast 设置广播组件
func WithMulticast(multicast multicast.Subscriber) Option {
	return func(p *Proxy) {
		p.multicast = multicast
	}
}

// WithRequestLogger 设置请求日志记录器
func WithRequestLogger(logger logger.Logger) Option {
	return func(p *Proxy) {
		p.requestLogger = logger
	}
}

func newEmptyMessage(data []byte) (msg *emptypb.Empty, err error) {
	msg = &emptypb.Empty{}
	if len(data) > 0 {
		err = proto.Unmarshal(data, msg)
	}
	return
}

// 记录每个时序性管道的最后一次请求和响应结果
// 如果request id小于最后一次的request id，说明顺序存在问题
// 如果request id等于最后一次的request id，直接返回最后一次的响应结果
type requestTracer struct {
	lastID    *gokit.MapOf[string, uint32]
	lastReply *gokit.MapOf[string, *nh.Reply]
}

func newRequestTracer() *requestTracer {
	return &requestTracer{
		lastID:    gokit.NewMapOf[string, uint32](),
		lastReply: gokit.NewMapOf[string, *nh.Reply](),
	}
}

func (r *requestTracer) Save(pipeline string, req *nh.Request, reply *nh.Reply) bool {
	// 只有具备时序性的管道请求才会保存
	// 非时序性请求，可能出现后发先至的情况，在处理过程中request id并不能保证递增，即使记录也没有意义
	if pipeline == "" {
		return false
	}

	r.lastID.Store(pipeline, req.GetId())

	prev, ok := r.lastReply.Swap(pipeline, reply)
	// 把保存的上一次响应结果放回对象池
	if ok {
		replyPool.Put(prev)
	}
	return true
}

func (r *requestTracer) LastReplyOf(pipeline string) (uint32, *nh.Reply, bool) {
	if id, ok := r.lastID.Load(pipeline); ok {
		if reply, ok := r.lastReply.Load(pipeline); ok {
			return id, reply, true
		}
	}

	return 0, nil, false
}

func (r *requestTracer) Release() {
	r.lastReply.Range(func(key string, value *nh.Reply) bool {
		replyPool.Put(value)
		r.lastReply.Delete(key)
		return true
	})
}
