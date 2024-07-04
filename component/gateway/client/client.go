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
	"github.com/reactivex/rxgo/v2"
	"google.golang.org/grpc/status"
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

// Client 客户端
type Client struct {
	seq  atomic.Uint32
	conn connection

	await         *gokit.MapOf[uint32, chan *nh.Reply]
	replyC        chan rxgo.Item
	replyObserver rxgo.Observable
	done          chan struct{}
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

	return newClient(cc), nil
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

	return newClient(qc), nil
}

func newClient(conn connection) *Client {
	replyC := make(chan rxgo.Item)
	c := &Client{
		conn:          conn,
		await:         gokit.NewMapOf[uint32, chan *nh.Reply](),
		replyC:        replyC,
		replyObserver: rxgo.FromEventSource(replyC),
		done:          make(chan struct{}),
	}

	go c.pingLoop()
	go c.receiveLoop()
	return c
}

// Call 发送请求
func (cli *Client) Call(ctx context.Context, serviceCode int32, method string, input, output proto.Message, options ...CallOption) error {
	req, err := cli.send(serviceCode, method, input, options...)
	if err != nil {
		return fmt.Errorf("send request, %w", err)
	} else if req.GetNoReply() {
		return nil
	}

	ch := make(chan *nh.Reply, 1)

	cli.await.Store(req.GetId(), ch)
	defer cli.await.Delete(req.GetId())

	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case reply := <-ch:
		// rpc error
		if reply.GetServiceCode() == 0 && reply.GetCode() == int32(nh.ReplyCode_RPC_ERROR) {
			rpcError := &nh.RPCError{}
			if err := proto.Unmarshal(reply.GetData(), rpcError); err != nil {
				return fmt.Errorf("unmarshal rpc error, %w", err)
			}

			return status.FromProto(rpcError.GetStatus()).Err()
		}

		if err := proto.Unmarshal(reply.GetData(), output); err != nil {
			return fmt.Errorf("unmarshal reply, %w", err)
		}

		return nil
	}
}

// CallNoReply 发送无需回复的请求
func (cli *Client) CallNoReply(ctx context.Context, serviceCode int32, method string, input proto.Message, options ...CallOption) error {
	options = append(options, WithNoReply())

	return cli.Call(ctx, serviceCode, method, input, nil, options...)
}

// Subscribe 订阅指定类型消息
//
// Example:
//
//	client.Subscribe(ctx, gateway.ServiceCode, int32(gatewaypb.Protocol_RPC_ERROR), func(msg *gatewaypb.RPCError, reply *nh.Reply) {
//		// ...
//	})
func (cli *Client) Subscribe(ctx context.Context, serviceCode int32, messageCode int32, callback any) {
	handler := cli.newHandler(callback)

	cli.replyObserver.
		Filter(func(item any) bool {
			reply, ok := item.(*nh.Reply)
			return ok &&
				reply.GetServiceCode() == serviceCode &&
				reply.GetCode() == messageCode
		}).
		DoOnNext(func(item any) {
			handler(item.(*nh.Reply))
		}, rxgo.WithContext(ctx))
}

// Close 关闭客户端
func (cli *Client) Close() {
	cli.conn.Close()
	close(cli.done)
	close(cli.replyC)
}

func (cli *Client) newHandler(callback any) func(*nh.Reply) {
	handler := reflect.ValueOf(callback)
	handlerType := handler.Type()

	if handlerType.Kind() != reflect.Func {
		panic(errors.New("callback must be a function"))
	} else if handlerType.NumIn() != 2 {
		panic(errors.New("callback must have to arguments"))
	}

	firstArg := handlerType.In(0)
	if !firstArg.Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
		panic(errors.New("callback's first argument must be proto.Message"))
	}

	sencondArg := handlerType.In(1)
	if sencondArg != reflect.TypeOf((*nh.Reply)(nil)) {
		panic(errors.New("callback's second argument must be *nh.Reply"))
	}

	return func(reply *nh.Reply) {
		msg := reflect.New(firstArg.Elem())
		gokit.Must(proto.Unmarshal(reply.Data, msg.Interface().(proto.Message)))

		handler.Call([]reflect.Value{
			msg,
			reflect.ValueOf(reply),
		})
	}
}

func (cli *Client) send(serviceCode int32, method string, input proto.Message, options ...CallOption) (*nh.Request, error) {
	data, err := proto.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal request message, %w", err)
	}
	req := &nh.Request{
		Id:          cli.seq.Add(1),
		Data:        data,
		ServiceCode: serviceCode,
		Method:      method,
	}

	for _, opt := range options {
		opt(req)
	}

	data, err = proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request payload, %w", err)
	}

	return req, cli.conn.send(serviceCode, data)
}

func (cli *Client) receiveLoop() {
	for reply := range cli.conn.replyStream() {
		logger.Debug("receive reply",
			"requestID", reply.GetRequestId(),
			"fromService", reply.GetServiceCode(),
			"code", reply.GetCode(),
		)

		if ch, ok := cli.await.Load(reply.GetRequestId()); ok {
			ch <- reply
			continue
		}

		select {
		case <-cli.done:
			return
		case cli.replyC <- rxgo.Of(reply):
		}
	}
}

func (cli *Client) pingLoop() {
	tk := time.NewTicker(10 * time.Second)
	defer tk.Stop()

	for range tk.C {
		select {
		case <-cli.done:
			return
		default:
		}

		if err := cli.conn.ping(); err != nil {
			logger.Error("send ping", "error", err)
		}
	}
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
