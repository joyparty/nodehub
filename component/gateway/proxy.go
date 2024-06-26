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
	"github.com/joyparty/nodehub/internal/metrics"
	"github.com/joyparty/nodehub/logger"
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
	// WriteTimeout 网络连接写超时时间
	WriteTimeout = 5 * time.Second

	requestPool = gokit.NewPoolOf(func() *nh.Request {
		return &nh.Request{}
	})

	replyPool = gokit.NewPoolOf(func() *nh.Reply {
		return &nh.Reply{}
	})

	streamDesc = &grpc.StreamDesc{
		ServerStreams: true,
	}
)

// Session 连接会话
type Session interface {
	Type() string
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

// Proxy 客户端会话运行环境
type Proxy struct {
	nodeID     string
	opts       *Options
	sessions   *sessionHub
	stateTable *stateTable
	cleanJobs  *gokit.MapOf[string, *time.Timer]
	done       chan struct{}
}

// NewProxy 构造函数
func NewProxy(nodeID ulid.ULID, opt ...Option) (*Proxy, error) {
	p := &Proxy{
		nodeID:     nodeID.String(),
		opts:       newOptions(),
		sessions:   newSessionHub(),
		stateTable: newStateTable(),
		cleanJobs:  gokit.NewMapOf[string, *time.Timer](),
		done:       make(chan struct{}),
	}

	for _, fn := range opt {
		fn(p.opts)
	}

	if err := p.opts.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Name 服务名称
func (p *Proxy) Name() string {
	return "proxy"
}

// CompleteNodeEntry 补全节点信息
func (p *Proxy) CompleteNodeEntry(entry *cluster.NodeEntry) {
	p.opts.Transporter.CompleteNodeEntry(entry)
}

// Start 启动服务
func (p *Proxy) Start(ctx context.Context) error {
	p.init(ctx)

	sc, err := p.opts.Transporter.Serve(ctx)
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

				if err := p.submitTask(func() {
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

	if err := p.opts.Transporter.Shutdown(ctx); err != nil && !errors.Is(err, net.ErrClosed) {
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
	p.opts.EventBus.Subscribe(ctx, func(ev event.NodeAssign, _ time.Time) {
		if err := p.submitTask(func() {
			for _, userID := range ev.UserID {
				if _, ok := p.sessions.Load(userID); ok {
					p.stateTable.Store(userID, ev.ServiceCode, ev.NodeID)
				}
			}
		}); err != nil {
			logger.Error("handle NodeAssign event", "error", err, "event", ev)
		}
	})

	p.opts.EventBus.Subscribe(ctx, func(ev event.NodeUnassign, _ time.Time) {
		if err := p.submitTask(func() {
			for _, userID := range ev.UserID {
				p.stateTable.Remove(userID, ev.ServiceCode)
			}
		}); err != nil {
			logger.Error("handle NodeUnassign event", "error", err, "event", ev)
		}
	})

	// 禁止同一个用户同时连接多个网关
	p.opts.EventBus.Subscribe(ctx, func(ev event.UserConnected, _ time.Time) {
		if ev.GatewayID != p.nodeID {
			if sess, ok := p.sessions.Load(ev.UserID); ok {
				logger.Warn("close duplicate session, user connect to other gateway", "session", sess)
				_ = sess.Close()
			}
		}
	})

	// 处理主动下行消息
	p.opts.Multicast.Subscribe(ctx, func(msg *nh.Multicast) {
		for _, sessID := range msg.GetReceiver() {
			if sess, ok := p.sessions.Load(sessID); ok {
				if err := p.submitTask(func() {
					logger.Debug("send multicast",
						"receiver", sess.ID(),
						"service", msg.GetContent().GetServiceCode(),
						"code", msg.GetContent().GetCode(),
						"time", msg.GetTime().AsTime().Format(time.RFC3339),
					)

					p.sendReply(sess, msg.Content)
				}); err != nil {
					logger.Error("submit multicast task", "error", err, "session", sess, "reply", msg.Content)
				}
			}
		}
	})

	p.opts.Registry.SubscribeDelete(ctx, func(entry cluster.NodeEntry) {
		p.stateTable.CleanNode(entry.ID)
	})

	go p.removeZombie()
}

// Handle 处理客户端连接
func (p *Proxy) handleSession(ctx context.Context, sess Session) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger.Info("handle connection", "addr", sess.RemoteAddr())
	if err := p.onConnect(ctx, sess); err != nil {
		if !errors.Is(err, io.EOF) {
			logger.Error("initialize connect", "error", err, "addr", sess.RemoteAddr())
		}
		_ = sess.Close()
		return
	}
	defer p.onDisconnect(ctx, sess)

	metrics.IncrGatewaySession(sess.Type())
	defer metrics.DecrGatewaySession(sess.Type())

	logVars := []any{
		"session", sess,
		"gateway", p.nodeID,
	}

	logger.Info("session connected", logVars...)
	defer logger.Info("session disconnected", logVars...)

	var prevRequestID uint32

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

		if prevRequestID > 0 && req.GetId() <= prevRequestID {
			logger.Error("request id has already been used ", "session", sess, "prev", prevRequestID, "current", req.GetId())
			return
		}
		prevRequestID = req.GetId()

		if err := p.submitTask(func() {
			defer requestPool.Put(req)

			if err := p.handleRequest(ctx, sess, req); err != nil {
				if s, ok := status.FromError(err); ok {
					if s.Code() == codes.Unknown {
						// unknown错误，不下行详细的错误描述，避免泄露信息到客户端
						s = status.New(codes.Unknown, "unknown error")
					}

					reply, _ := nh.NewReply(int32(nh.ReplyCode_RPC_ERROR), &nh.RPCError{
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
		desc   cluster.GRPCServiceDesc
		method string
	)

	logRequest := p.logRequest(ctx, sess, req)
	defer func() { logRequest(conn, desc, method, err) }()

	pass, err := p.opts.RequestInterceptor(ctx, sess, req)
	if err != nil {
		return fmt.Errorf("request interceptor, %w", err)
	} else if !pass {
		return errors.New("request interceptor denied")
	}

	desc, ok := p.opts.Registry.GetGRPCDesc(req.GetServiceCode())
	if !ok {
		return status.Errorf(codes.Unimplemented, "unknown service %d", req.GetServiceCode())
	} else if !desc.Public {
		return status.Errorf(codes.PermissionDenied, "request private service")
	}

	conn, err = p.getUpstream(sess, req, desc)
	if err != nil {
		return
	}

	md := sess.MetadataCopy()
	md.Set(rpc.MDTransactionID, ulid.Make().String())
	md.Set(rpc.MDSessID, sess.ID())
	md.Set(rpc.MDGateway, p.nodeID)
	ctx = metadata.NewOutgoingContext(ctx, md)

	var cancel context.CancelFunc
	if timemout := p.opts.RequstTimeout; timemout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timemout)
		defer cancel()
	}

	input, err := newEmptyMessage(req.Data)
	if err != nil {
		return fmt.Errorf("unmarshal request data, %w", err)
	}

	output := replyPool.Get()
	nh.ResetReply(output)
	defer replyPool.Put(output)

	method = path.Join(desc.Path, req.Method)
	if err = conn.Invoke(ctx, method, input, output); err != nil {
		// 这里不要用fmt.Errorf()包装，否则fmt.Errorf()会污染status.Status.Message()，导致日志记录不必要的重复内容
		return err
	}

	if req.GetNoReply() {
		return nil
	}

	output.RequestId = req.GetId()
	output.ServiceCode = req.GetServiceCode()
	p.sendReply(sess, output)
	return nil
}

func (p *Proxy) onConnect(ctx context.Context, sess Session) error {
	userID, md, err := p.opts.Initializer(ctx, sess)
	if err != nil {
		return fmt.Errorf("deny by initializer, %w", err)
	} else if userID == "" {
		return errors.New("empty userID")
	} else if md == nil {
		md = metadata.MD{}
	}

	sess.SetID(userID)
	sess.SetMetadata(md)

	if err := p.opts.ConnectInterceptor(ctx, sess); err != nil {
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

	if err := p.opts.EventBus.Publish(ctx, event.UserConnected{
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
	p.opts.DisconnectInterceptor(ctx, sess)
	p.sessions.Delete(sess)

	// 即使出错也不中断断开流程
	_ = p.opts.EventBus.Publish(ctx, event.UserDisconnected{
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
		nodeID, err = p.opts.Registry.AllocGRPCNode(req.GetServiceCode(), sess)
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
					p.stateTable.Store(sess.ID(), req.GetServiceCode(), nodeID)
				}
			}()
			goto FINISH
		}
	}

	// 从状态路由表查询节点ID
	if v, ok := p.stateTable.Find(sess.ID(), req.GetServiceCode()); ok {
		nodeID = v
		goto FINISH
	}

	// 非自动分配策略，没有找到节点就中断请求
	if desc.Allocation != cluster.AutoAllocate && nodeID.Time() == 0 {
		err = status.Error(codes.Aborted, "no node allocated")
		return
	}

	// 自动分配策略，根据负载均衡策略选择一个节点发送
	nodeID, err = p.opts.Registry.AllocGRPCNode(req.GetServiceCode(), sess)
	if err != nil {
		err = status.Errorf(codes.Aborted, "pick grpc node, %v", err)
		return
	}
	defer func() {
		if err == nil {
			p.stateTable.Store(sess.ID(), req.GetServiceCode(), nodeID)
		}
	}()

FINISH:
	conn, err = p.opts.Registry.GetGRPCConn(nodeID)
	if err != nil {
		err = status.Errorf(codes.Aborted, "get grpc conn, %v", err)
	}
	return
}

func (p *Proxy) logRequest(ctx context.Context, sess Session, req *nh.Request) func(*grpc.ClientConn, cluster.GRPCServiceDesc, string, error) {
	start := time.Now()

	return func(upstream *grpc.ClientConn, desc cluster.GRPCServiceDesc, method string, err error) {
		// 如果method为空，说明还没有到达请求阶段
		if method != "" {
			metrics.IncrGRPCRequests(method, err, time.Since(start))
		}

		if err == nil && p.opts.RequestLogger == nil {
			return
		}

		logValues := []any{
			"gateway", p.nodeID,
			"session", sess,
			"req", req,
			"duration", time.Since(start).String(),
		}

		if name := desc.Name; name != "" {
			logValues = append(logValues, "service", name)
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
			if s, ok := status.FromError(err); ok {
				logValues = append(logValues, "error", s.Message(), "grpcCode", s.Code())
			} else {
				logValues = append(logValues, "error", err)
			}

			if p.opts.RequestLogger == nil {
				logger.Error("handle request", logValues...)
			} else {
				p.opts.RequestLogger.Error("handle request", logValues...)
			}
		} else {
			p.opts.RequestLogger.Info("handle request", logValues...)
		}
	}
}

func (p *Proxy) submitTask(task func()) error {
	if pool := p.opts.GoPool; pool != nil {
		return pool.Submit(task)
	}
	return ants.Submit(task)
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

// 定时移除心跳超时的连接
func (p *Proxy) removeZombie() {
	for {
		select {
		case <-p.done:
			return
		case <-time.After(10 * time.Second):
			p.sessions.Range(func(s Session) bool {
				if time.Since(s.LastRWTime()) > p.opts.KeepaliveInterval {
					logger.Info("remove heartbeat timeout session", "session", s)
					_ = s.Close()
				}
				return true
			})
		}
	}
}

var emptyMessage = &emptypb.Empty{}

func newEmptyMessage(data []byte) (*emptypb.Empty, error) {
	if len(data) == 0 {
		return emptyMessage, nil
	}

	msg := &emptypb.Empty{}
	err := proto.Unmarshal(data, msg)
	return msg, err
}

// sessionHub 会话集合
type sessionHub struct {
	sessions *gokit.MapOf[string, Session]
	closed   *atomic.Bool
}

func newSessionHub() *sessionHub {
	hub := &sessionHub{
		sessions: gokit.NewMapOf[string, Session](),
		closed:   &atomic.Bool{},
	}

	return hub
}

func (h *sessionHub) Count() int {
	var count int
	h.sessions.Range(func(_ string, _ Session) bool {
		count++
		return true
	})
	return count
}

func (h *sessionHub) Store(sess Session) {
	h.sessions.Store(sess.ID(), sess)
}

func (h *sessionHub) Load(id string) (Session, bool) {
	return h.sessions.Load(id)
}

func (h *sessionHub) Delete(sess Session) {
	h.sessions.CompareAndDelete(sess.ID(), sess)
}

func (h *sessionHub) Range(f func(s Session) bool) {
	h.sessions.Range(func(_ string, value Session) bool {
		return f(value)
	})
}

func (h *sessionHub) Close() {
	if h.closed.CompareAndSwap(false, true) {
		h.sessions.Range(func(id string, sess Session) bool {
			_ = sess.Close()
			h.sessions.Delete(id)
			return true
		})
	}
}
