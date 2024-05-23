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
	// KeepaliveInterval 网络连接保持活跃时间，默认1分钟
	//
	// 客户端在没有业务消息的情况下，需要定时向服务器端发送心跳消息
	// 服务器端如果检测到客户端连接超过这个时间还没有任何读写，就会认为此连接已断线，会触发主动断线操作
	KeepaliveInterval = 1 * time.Minute

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
type Authorizer func(ctx context.Context, sess Session) (userID string, md metadata.MD, err error)

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
				if time.Since(s.LastRWTime()) > KeepaliveInterval {
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
	nodeID        string
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
		nodeID:     nodeID.String(),
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
					_ = sess.Close()
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
		if _, ok := p.sessions.Load(ev.UserID); ok {
			p.stateTable.Store(ev.UserID, ev.ServiceCode, ev.NodeID)
		}
	})
	p.eventBus.Subscribe(ctx, func(ev event.NodeUnassign, _ time.Time) {
		p.stateTable.Remove(ev.UserID, ev.ServiceCode)
	})

	// 禁止同一个用户同时连接多个网关
	p.eventBus.Subscribe(ctx, func(ev event.UserConnected, _ time.Time) {
		if ev.GatewayID != p.nodeID {
			if sess, ok := p.sessions.Load(ev.UserID); ok {
				logger.Warn("close duplicate session, user connect to other gateway", "session", sess)
				_ = sess.Close()
			}
		}
	})

	// 处理主动下行消息
	p.multicast.Subscribe(ctx, func(msg *nh.Multicast) {
		// 只发送5分钟内的消息
		if time.Since(msg.GetTime().AsTime()) <= 5*time.Minute {
			for _, sessID := range msg.GetReceiver() {
				if sess, ok := p.sessions.Load(sessID); ok {
					if err := ants.Submit(func() {
						p.sendReply(sess, msg.Content)
					}); err != nil {
						logger.Error("submit multicast task", "error", err, "session", sess, "reply", msg.Content)
					}
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
		"gateway", p.nodeID,
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

		pass, err := p.requestInterceptor(ctx, sess, req)
		if err != nil || !pass {
			requestPool.Put(req)

			if err != nil {
				logger.Error("request interceptor",
					"error", err,
					"session", sess,
					"req", req,
				)
			}

			continue
		}

		if err := ants.Submit(func() {
			defer requestPool.Put(req)

			if err := p.handleRequest(ctx, sess, req); err != nil {
				if s, ok := status.FromError(err); ok {
					if s.Code() == codes.Unknown {
						// unknown错误，不下行详细的错误描述，避免泄露信息到客户端
						s = status.New(codes.Unknown, "unknown error")
					}

					reply, _ := nh.NewReply(int32(nh.Protocol_RPC_ERROR), &nh.RPCError{
						RequestService: req.GetServiceCode(),
						RequestMethod:  req.GetMethod(),
						Status:         s.Proto(),
					})
					reply.RequestId = req.GetId()
					p.sendReply(sess, reply)
				}
			}
		}); err != nil {
			requestPool.Put(req)

			logger.Error("submit request task", "error", err, "session", sess, "req", req)
		}
	}
}

// 以status.Error()构造的错误，都会被下行通知到客户端
func (p *Proxy) handleRequest(ctx context.Context, sess Session, req *nh.Request) (err error) {
	var (
		conn   *grpc.ClientConn
		method string
		start  = time.Now()
	)

	defer func() {
		p.logRequest(ctx, sess, req, method, start, conn, err)
	}()

	desc, ok := p.registry.GetGRPCDesc(req.GetServiceCode())
	if !ok {
		return status.Errorf(codes.NotFound, "service %d not found", req.GetServiceCode())
	} else if !desc.Public {
		return status.Errorf(codes.PermissionDenied, "request private service")
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
	defer replyPool.Put(output)

	md := sess.MetadataCopy()
	md.Set(rpc.MDTransactionID, ulid.Make().String())
	md.Set(rpc.MDSessID, sess.ID())
	md.Set(rpc.MDGateway, p.nodeID)
	ctx = metadata.NewOutgoingContext(ctx, md)

	method = path.Join(desc.Path, req.Method)
	if err = conn.Invoke(ctx, method, input, output); err != nil {
		return fmt.Errorf("invoke service: %w", err)
	}

	if req.GetNoReply() {
		return nil
	}

	output.RequestId = req.GetId()
	output.FromService = req.GetServiceCode()
	p.sendReply(sess, output)
	return nil
}

func (p *Proxy) onConnect(ctx context.Context, sess Session) error {
	userID, md, err := p.authorizer(ctx, sess)
	if err != nil {
		return fmt.Errorf("deny by authorizer, %w", err)
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

	// 断开同一个用户的其它连接
	if prev, ok := p.sessions.Load(sess.ID()); ok {
		logger.Warn("close duplicate session", "session", prev)

		_ = prev.Close()
	}

	// 放弃之前断线创造的清理任务
	if timer, ok := p.cleanJobs.Load(sess.ID()); ok {
		if !timer.Stop() {
			<-timer.C
		}
		p.cleanJobs.Delete(sess.ID())
	}

	if err := p.eventBus.Publish(ctx, event.UserConnected{
		UserID:     sess.ID(),
		GatewayID:  p.nodeID,
		RemoteAddr: sess.RemoteAddr(),
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
		UserID:     sess.ID(),
		GatewayID:  p.nodeID,
		RemoteAddr: sess.RemoteAddr(),
	})

	// 延迟5分钟之后，确认session不存在了，则清除相关数据
	p.cleanJobs.Store(sess.ID(), time.AfterFunc(5*time.Minute, func() {
		if _, ok := p.sessions.Load(sess.ID()); !ok {
			p.stateTable.CleanSession(sess.ID())
		}
		p.cleanJobs.Delete(sess.ID())
	}))
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
	method string,
	start time.Time,
	upstream *grpc.ClientConn,
	err error,
) {
	if err == nil && p.requestLogger == nil {
		return
	}

	logValues := []any{
		"gateway", p.nodeID,
		"session", sess,
		"method", method,
		"requestID", req.GetId(),
		"duration", time.Since(start).String(),
	}

	if nodeID := req.GetNodeId(); nodeID != "" {
		logValues = append(logValues, "nodeID", nodeID)
	}

	if noReply := req.GetNoReply(); noReply {
		logValues = append(logValues, "noReply", true)
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

func (p *Proxy) sendReply(sess Session, reply *nh.Reply) {
	if err := sess.Send(reply); err != nil {
		logger.Error("send reply",
			"error", err,
			"session", sess,
			"reply", reply,
		)
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
