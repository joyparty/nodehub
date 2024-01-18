package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joyparty/gokit"
	"github.com/oklog/ulid/v2"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/proto/nh"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

var (
	// ErrSessionClosed 会话已关闭
	ErrSessionClosed = errors.New("session closed")

	writeWait = 5 * time.Second
	upgrader  = websocket.Upgrader{}
)

// WSServer websocket网关服务器
//
// 客户端通过websocket方式连接网关，网关再转发请求到grpc后端服务
type WSServer struct {
	playground *Playground
	authorizer WSAuthorizer
	server     *http.Server
}

// NewWSServer 构造函数
func NewWSServer(
	playground *Playground,
	listenAddr string,
	authorizer WSAuthorizer,
) *WSServer {
	ws := &WSServer{
		playground: playground,
		authorizer: authorizer,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/grpc", ws.handle)

	ws.server = &http.Server{
		Addr:    listenAddr,
		Handler: http.HandlerFunc(mux.ServeHTTP),
	}
	return ws
}

// Name 服务名称
func (ws *WSServer) Name() string {
	return "wsproxy"
}

// CompleteNodeEntry 补全节点信息
func (ws *WSServer) CompleteNodeEntry(entry *cluster.NodeEntry) {
	entry.Entrance = fmt.Sprintf("ws://%s/grpc", ws.server.Addr)
}

// Start 启动websocket服务器
func (ws *WSServer) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", ws.server.Addr)
	if err != nil {
		return fmt.Errorf("listen, %w", err)
	}

	go func() {
		if err := ws.server.Serve(l); err != nil && err != http.ErrServerClosed {
			logger.Error("start gateway", "error", err)

			panic(fmt.Errorf("start gateway, %w", err))
		}
	}()

	return nil
}

// Stop 停止websocket服务器
func (ws *WSServer) Stop(ctx context.Context) error {
	ws.server.Shutdown(ctx)
	ws.playground.Close()
	return nil
}

func (ws *WSServer) handle(w http.ResponseWriter, r *http.Request) {
	sess, err := ws.newSession(w, r)
	if err != nil {
		logger.Error("initialize session", "error", err, "remoteAddr", r.RemoteAddr)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ws.playground.Handle(ctx, sess)
}

func (ws *WSServer) newSession(w http.ResponseWriter, r *http.Request) (Session, error) {
	userID, md, ok := ws.authorizer(w, r)
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
	sess.SetMetadata(md)

	return sess, nil
}

// WSAuthorizer websocket方式身份验证逻辑
//
// 自定义身份验证逻辑，在websocket upgrade之前调用
// 返回的metadata会在此连接的所有grpc request中携带
// 返回的userID如果不为空，则会作为会话唯一标识使用，另外也会被自动加入到metadata中
// 如果返回ok为false，会直接关闭连接
// 因此如果验证不通过之类的错误，需要在这个函数里面自行处理
type WSAuthorizer func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool)

type wsPayload struct {
	messageType int
	data        []byte
}

type wsSession struct {
	id   string
	conn *websocket.Conn
	md   metadata.MD

	// 不允许并发写，因此用队列方式排队写
	sendC chan wsPayload

	// 最后一次读写时间
	lastRWTime gokit.ValueOf[time.Time]

	done   chan struct{}
	closed *atomic.Bool
}

func newWsSession(conn *websocket.Conn) *wsSession {
	ws := &wsSession{
		id:         ulid.Make().String(),
		conn:       conn,
		sendC:      make(chan wsPayload, 3), // 为什么是三？因为事不过三
		done:       make(chan struct{}),
		lastRWTime: gokit.NewValueOf[time.Time](),
		closed:     &atomic.Bool{},
	}

	ws.lastRWTime.Store(time.Now())
	ws.setPingPongHandler()

	go ws.sendLoop()
	return ws
}

// 接收到ping pong消息后，也更新活跃时间
func (ws *wsSession) setPingPongHandler() {
	pongHandler := ws.conn.PongHandler()
	ws.conn.SetPongHandler(func(appData string) error {
		ws.lastRWTime.Store(time.Now())

		return pongHandler(appData)
	})

	pingHandler := ws.conn.PingHandler()
	ws.conn.SetPingHandler(func(appData string) error {
		ws.lastRWTime.Store(time.Now())

		return pingHandler(appData)
	})
}

func (ws *wsSession) ID() string {
	return ws.id
}

func (ws *wsSession) SetID(id string) {
	ws.id = id
}

func (ws *wsSession) SetMetadata(md metadata.MD) {
	ws.md = md
}

func (ws *wsSession) MetadataCopy() metadata.MD {
	return ws.md.Copy()
}

func (ws *wsSession) Recv(req *nh.Request) error {
	for {
		select {
		case <-ws.done:
			return ErrSessionClosed
		default:
		}

		messageType, message, err := ws.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return io.EOF
			}
			return err
		}
		ws.lastRWTime.Store(time.Now())

		// 只处理二进制消息
		if messageType == websocket.BinaryMessage {
			if err := proto.Unmarshal(message, req); err != nil {
				return fmt.Errorf("unmarshal request, %w", err)
			}
			return nil
		}
	}
}

func (ws *wsSession) Send(resp *nh.Reply) error {
	data, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response, %w", err)
	}

	select {
	case <-ws.done:
		return ErrSessionClosed
	case ws.sendC <- wsPayload{
		messageType: websocket.BinaryMessage,
		data:        data,
	}:
	}
	return nil
}

func (ws *wsSession) sendLoop() {
	ticker := time.NewTicker(DefaultHeartbeatDuration / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ws.done:
			return
		case <-ticker.C:
			if time.Since(ws.LastRWTime()) > DefaultHeartbeatDuration {
				ws.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
			}
		case payload := <-ws.sendC:
			ws.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.conn.WriteMessage(payload.messageType, payload.data); err != nil {
				args := []any{
					"error", err,
					"sessID", ws.ID(),
					"remoteAddr", ws.RemoteAddr(),
				}

				if payload.messageType == websocket.BinaryMessage {
					message := &nh.Reply{}
					if err := proto.Unmarshal(payload.data, message); err == nil {
						args = append(args,
							"requestID", message.GetRequestId(),
							"fromService", message.GetFromService(),
							"messageType", message.GetMessageType(),
						)
					}
				}

				logger.Error("send message", args...)
			} else {
				// 只有发送二进制消息才更新活跃时间，发送Ping消息不更新
				if payload.messageType == websocket.BinaryMessage {
					ws.lastRWTime.Store(time.Now())
				}
			}
		}
	}
}

func (ws *wsSession) LastRWTime() time.Time {
	return ws.lastRWTime.Load()
}

func (ws *wsSession) LocalAddr() string {
	return ws.conn.LocalAddr().String()
}

func (ws *wsSession) RemoteAddr() string {
	return ws.conn.RemoteAddr().String()
}

func (ws *wsSession) Close() error {
	if ws.closed.CompareAndSwap(false, true) {
		close(ws.done)
		return ws.conn.Close()
	}
	return nil
}

// WSClient websocket客户端，用于测试及演示
type WSClient struct {
	conn   *websocket.Conn
	wMutex *sync.Mutex
	idSeq  *atomic.Uint32
	done   chan struct{}

	// serviceCode => messageType => handler
	handlers       *gokit.MapOf[int32, *gokit.MapOf[int32, func(*nh.Reply)]]
	defaultHandler func(*nh.Reply)
}

// NewWSClient 创建websocket客户端
func NewWSClient(wsURL string) (*WSClient, error) {
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("response %d, %w", resp.StatusCode, err)
	}

	c := &WSClient{
		conn:   conn,
		wMutex: &sync.Mutex{},
		idSeq:  &atomic.Uint32{},
		done:   make(chan struct{}),

		handlers:       gokit.NewMapOf[int32, *gokit.MapOf[int32, func(*nh.Reply)]](),
		defaultHandler: func(reply *nh.Reply) {},
	}
	go c.run()
	return c, nil
}

func (wc *WSClient) run() {
	ch := wc.responseStream()
	for {
		select {
		case <-wc.done:
			return
		case reply, ok := <-ch:
			if !ok {
				return
			}

			if handlers, ok := wc.handlers.Load(reply.FromService); ok {
				if handler, ok := handlers.Load(reply.GetMessageType()); ok {
					handler(reply)
				}
			} else {
				wc.defaultHandler(reply)
			}
		}
	}
}

// Close 关闭客户端
func (wc *WSClient) Close() {
	close(wc.done)
	wc.conn.Close()
}

// Call 发起远程调用
func (wc *WSClient) Call(serviceCode int32, method string, arg proto.Message, options ...CallOption) error {
	req, err := wc.newRequest(arg, options...)
	if err != nil {
		return fmt.Errorf("build request message, %w", err)
	}
	req.ServiceCode = serviceCode
	req.Method = method

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request message, %w", err)
	}

	wc.wMutex.Lock()
	defer wc.wMutex.Unlock()

	wc.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	return wc.conn.WriteMessage(websocket.BinaryMessage, data)
}

// OnReceive 注册消息处理器
//
// Example:
//
//	client.OnReceive(gateway.ServiceCode, int32(gatewaypb.Protocol_RPC_ERROR), func(requestID uint32, msg *gatewaypb.RPCError) {
//		// ...
//	})
func (wc *WSClient) OnReceive(serviceCode int32, messageType int32, handler any) {
	fn := reflect.ValueOf(handler)
	fnType := fn.Type()

	if fnType.Kind() != reflect.Func {
		panic(fmt.Errorf("handler must be a function"))
	} else if fnType.NumIn() != 2 {
		panic(fmt.Errorf("handler must have two arguments"))
	}

	firstArg := fnType.In(0)
	if firstArg.Kind() != reflect.Uint32 {
		panic(fmt.Errorf("handler's first argument must be uint32"))
	}

	secondArg := fnType.In(1)
	if !secondArg.Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
		panic(fmt.Errorf("handler's second argument must be proto.Message"))
	}

	serviceHandlers, ok := wc.handlers.Load(serviceCode)
	if !ok {
		serviceHandlers, _ = wc.handlers.LoadOrStore(serviceCode, gokit.NewMapOf[int32, func(*nh.Reply)]())
	}
	serviceHandlers.Store(messageType, func(resp *nh.Reply) {
		msg := reflect.New(secondArg.Elem())

		if err := proto.Unmarshal(resp.Data, msg.Interface().(proto.Message)); err != nil {
			panic(fmt.Errorf("unmarshal response message, %w", err))
		}

		fn.Call([]reflect.Value{
			reflect.ValueOf(resp.GetRequestId()),
			msg,
		})
	})
}

// SetDefaultHandler 设置默认消息处理器
func (wc *WSClient) SetDefaultHandler(handler func(reply *nh.Reply)) {
	wc.defaultHandler = handler
}

func (wc *WSClient) responseStream() <-chan *nh.Reply {
	ch := make(chan *nh.Reply)

	go func() {
		defer close(ch)

		for {
			select {
			case <-wc.done:
				return
			default:
			}

			messageType, data, err := wc.conn.ReadMessage()
			if err != nil && !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				panic(fmt.Errorf("read websocket, %w", err))
			}

			switch messageType {
			case websocket.BinaryMessage:
				resp := &nh.Reply{}
				if err := proto.Unmarshal(data, resp); err != nil {
					panic(fmt.Errorf("unmarshal response message, %w", err))
				}

				select {
				case <-wc.done:
					return
				case ch <- resp:
				}
			default:
				panic(fmt.Errorf("unexpected websocket message type: %d", messageType))
			}
		}
	}()

	return ch
}

func (wc *WSClient) newRequest(msg proto.Message, options ...CallOption) (*nh.Request, error) {
	req := &nh.Request{
		Id: wc.idSeq.Add(1),
	}

	for _, opt := range options {
		opt(req)
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	req.Data = data
	return req, nil
}

// CallOption 调用选项
type CallOption func(req *nh.Request)

// WithNode 指定节点
func WithNode(nodeID string) CallOption {
	return func(req *nh.Request) {
		req.NodeId = nodeID
	}
}

// WithNoReply 不需要回复
func WithNoReply() CallOption {
	return func(req *nh.Request) {
		req.NoReply = true
	}
}
