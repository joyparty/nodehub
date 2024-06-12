package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/internal/codec"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/quic-go/quic-go"
	"google.golang.org/protobuf/proto"
)

var writeTimeout = 5 * time.Second

type connection interface {
	send(service int32, data []byte) error
	ping() error
	replyStream() <-chan *nh.Reply
	Close()
}

type tcpConn struct {
	conn net.Conn
	done chan struct{}
}

func newTCPConn(addr string) (*tcpConn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &tcpConn{
		conn: conn,
		done: make(chan struct{}),
	}, nil
}

func (tc *tcpConn) send(service int32, data []byte) error {
	return codec.SendBytes(data, func(data []byte) error {
		_, err := tc.conn.Write(data)
		return err
	})
}

func (tc *tcpConn) ping() error {
	return tc.send(0, nil)
}

func (tc *tcpConn) replyStream() <-chan *nh.Reply {
	ch := make(chan *nh.Reply)

	go func() {
		defer close(ch)

		msg := codec.GetMessage()
		defer codec.PutMessage(msg)

		for {
			select {
			case <-tc.done:
				return
			default:
			}

			if err := codec.ReadMessage(tc.conn, msg); err != nil {
				if !errors.Is(err, net.ErrClosed) {
					logger.Error("read tcp", "error", err)
				}
				return
			}

			reply := &nh.Reply{}
			gokit.Must(proto.Unmarshal(msg.Bytes(), reply))

			select {
			case <-tc.done:
				return
			case ch <- reply:
			}
		}
	}()

	return ch
}

func (tc *tcpConn) Close() {
	close(tc.done)
	_ = tc.conn.Close()
}

type quicConn struct {
	conn    quic.Connection
	streams []quic.Stream
	done    chan struct{}
}

func newQUICConn(addr string, tlsConfig *tls.Config, quicConfig *quic.Config) (*quicConn, error) {
	conn, err := quic.DialAddr(context.Background(), addr, tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}

	streams := []quic.Stream{}
	for i := 0; i < 3; i++ {
		stream, err := conn.OpenStream()
		if err != nil {
			return nil, fmt.Errorf("open stream, %w", err)
		}
		streams = append(streams, stream)
	}

	qc := &quicConn{
		conn:    conn,
		streams: streams,
		done:    make(chan struct{}),
	}

	return qc, nil
}

func (qc *quicConn) send(service int32, data []byte) error {
	s := qc.streams[int(service)%len(qc.streams)]

	return codec.SendBytes(data, func(data []byte) error {
		_, err := s.Write(data)
		return err
	})
}

func (qc *quicConn) ping() error {
	return qc.send(0, nil)
}

func (qc *quicConn) replyStream() <-chan *nh.Reply {
	c := make(chan *nh.Reply)

	for _, s := range qc.streams {
		go func(s quic.Stream) {
			msg := codec.GetMessage()
			defer codec.PutMessage(msg)

			for {
				select {
				case <-qc.done:
					return
				default:
				}

				if err := codec.ReadMessage(s, msg); err != nil {
					logger.Error("read quic", "error", err)
					return
				}

				reply := &nh.Reply{}
				gokit.Must(proto.Unmarshal(msg.Bytes(), reply))

				select {
				case <-qc.done:
					return
				case c <- reply:
				}
			}
		}(s)
	}

	return c
}

func (qc *quicConn) Close() {
	close(qc.done)
	_ = qc.conn.CloseWithError(0, "")
}

type wsConn struct {
	conn *websocket.Conn
	done chan struct{}
}

func newWSConn(url string) (*wsConn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return &wsConn{
		conn: conn,
		done: make(chan struct{}),
	}, nil
}

func (wc *wsConn) send(service int32, data []byte) error {
	wc.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return wc.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (wc *wsConn) ping() error {
	return wc.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout))
}

func (wc *wsConn) replyStream() <-chan *nh.Reply {
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
			if err != nil {
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure,
					websocket.CloseNormalClosure) {
					logger.Error("read websocket", "error", err)
				}
				return
			}

			switch messageType {
			case websocket.BinaryMessage:
				reply := &nh.Reply{}
				gokit.Must(proto.Unmarshal(data, reply))

				select {
				case <-wc.done:
					return
				case ch <- reply:
				}
			default:
				logger.Error("unexpected websocket message type", "type", messageType)
			}
		}
	}()

	return ch
}

func (wc *wsConn) Close() {
	close(wc.done)
	wc.conn.Close()
}

// Client 网关客户端，用于测试及演示
type Client struct {
	idSeq *atomic.Uint32
	conn  connection

	// serviceCode => messageType => handler
	handlers       *gokit.MapOf[int32, *gokit.MapOf[int32, func(*nh.Reply)]]
	defaultHandler func(*nh.Reply)
}

// New 创建客户端
func New(dialURL string) (*Client, error) {
	l, err := url.Parse(dialURL)
	if err != nil {
		return nil, fmt.Errorf("parse dial url, %w", err)
	}

	var cc connection
	switch l.Scheme {
	case "tcp":
		cc, err = newTCPConn(l.Host)
		if err != nil {
			return nil, fmt.Errorf("dial tcp, %w", err)
		}
	case "ws":
		cc, err = newWSConn(dialURL)
		if err != nil {
			return nil, fmt.Errorf("dial websocket, %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", l.Scheme)
	}

	c := &Client{
		conn:     cc,
		idSeq:    &atomic.Uint32{},
		handlers: gokit.NewMapOf[int32, *gokit.MapOf[int32, func(*nh.Reply)]](),
		defaultHandler: func(reply *nh.Reply) {
			fmt.Printf("%s REPLY: requestID=%d service=%d code=%d\n",
				time.Now().Format(time.RFC3339),
				reply.GetRequestId(),
				reply.GetServiceCode(),
				reply.GetCode(),
			)
		},
	}

	go c.run()
	return c, nil
}

// NewQUIC 创建QUIC客户端
func NewQUIC(dialURL string, tlsConfig *tls.Config, quicConfig *quic.Config) (*Client, error) {
	l, err := url.Parse(dialURL)
	if err != nil {
		return nil, fmt.Errorf("parse dial url, %w", err)
	} else if l.Scheme != "quic" {
		return nil, fmt.Errorf("unsupported scheme: %s", l.Scheme)
	}

	qc, err := newQUICConn(l.Host, tlsConfig, quicConfig)
	if err != nil {
		return nil, fmt.Errorf("dial quic, %w", err)
	}

	c := &Client{
		conn:     qc,
		idSeq:    &atomic.Uint32{},
		handlers: gokit.NewMapOf[int32, *gokit.MapOf[int32, func(*nh.Reply)]](),
		defaultHandler: func(reply *nh.Reply) {
			fmt.Printf("%s REPLY: requestID=%d service=%d code=%d\n",
				time.Now().Format(time.RFC3339),
				reply.GetRequestId(),
				reply.GetServiceCode(),
				reply.GetCode(),
			)
		},
	}

	go c.run()
	return c, nil
}

// SetDefaultHandler 设置默认消息处理器
func (c *Client) SetDefaultHandler(handler func(reply *nh.Reply)) {
	c.defaultHandler = handler
}

// Call 发起远程调用
func (c *Client) Call(serviceCode int32, method string, arg proto.Message, options ...CallOption) error {
	req, err := c.newRequest(arg, options...)
	if err != nil {
		return fmt.Errorf("build request message, %w", err)
	}
	req.ServiceCode = serviceCode
	req.Method = method

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request message, %w", err)
	}

	return c.conn.send(serviceCode, data)
}

// CallStream 调用server-side stream接口
func (c *Client) CallStream(serviceCode int32, method string, arg proto.Message, options ...CallOption) error {
	options = append(options, WithServerStream())
	return c.Call(serviceCode, method, arg, options...)
}

// OnReceive 注册消息处理器
//
// Example:
//
//	client.OnReceive(gateway.ServiceCode, int32(gatewaypb.Protocol_RPC_ERROR), func(requestID uint32, msg *gatewaypb.RPCError) {
//		// ...
//	})
func (c *Client) OnReceive(serviceCode int32, messageType int32, handler any) {
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

	serviceHandlers, ok := c.handlers.Load(serviceCode)
	if !ok {
		serviceHandlers, _ = c.handlers.LoadOrStore(serviceCode, gokit.NewMapOf[int32, func(*nh.Reply)]())
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

func (c *Client) newRequest(msg proto.Message, options ...CallOption) (*nh.Request, error) {
	req := &nh.Request{
		Id: c.idSeq.Add(1),
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

func (c *Client) run() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if err := c.conn.ping(); err != nil {
				logger.Error("ping gateway", "error", err)
			}
		}
	}()

	ch := c.conn.replyStream()

Loop:
	for {
		select {
		case reply, ok := <-ch:
			if !ok {
				break Loop
			}
			logger.Debug("receive reply",
				"requestID", reply.GetRequestId(),
				"fromService", reply.GetServiceCode(),
				"code", reply.GetCode(),
			)

			if handlers, ok := c.handlers.Load(reply.GetServiceCode()); ok {
				if handler, ok := handlers.Load(reply.GetCode()); ok {
					go handler(reply)
					continue
				}
			}
			c.defaultHandler(reply)
		}
	}

	logger.Info("gateway connection closed")
}

// Close 关闭客户端
func (c *Client) Close() {
	c.conn.Close()
}

// MustClient 使用must方法处理错误的客户端
type MustClient struct {
	*Client
}

// Call 发送请求
func (c *MustClient) Call(serviceCode int32, method string, arg proto.Message, options ...CallOption) {
	gokit.Must(c.Client.Call(serviceCode, method, arg, options...))
}

// CallStream 调用stream接口
func (c *MustClient) CallStream(serviceCode int32, method string, arg proto.Message, options ...CallOption) {
	gokit.Must(c.Client.CallStream(serviceCode, method, arg, options...))
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

// WithServerStream 发起server-side stream方法请求
func WithServerStream() CallOption {
	return func(req *nh.Request) {
		req.ServerStream = true
	}
}
