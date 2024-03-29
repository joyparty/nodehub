package gateway

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/proto/nh"
	"google.golang.org/protobuf/proto"
)

type client interface {
	send([]byte) error
	replyStream() <-chan *nh.Reply
	Close()
}

type tcpClient struct {
	conn net.Conn
	done chan struct{}
}

func newTCPClient(addr string) (*tcpClient, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &tcpClient{
		conn: conn,
		done: make(chan struct{}),
	}, nil
}

func (tc *tcpClient) send(data []byte) error {
	buf := bytes.NewBuffer(nil)

	if err := binary.Write(buf, binary.BigEndian, uint32(len(data))); err != nil {
		return fmt.Errorf("write size frame, %w", err)
	}

	if len(data) > 0 {
		if err := binary.Write(buf, binary.BigEndian, data); err != nil {
			return fmt.Errorf("write data frame, %w", err)
		}
	}

	// tc.conn.SetWriteDeadline(time.Now().Add(writeWait))
	_, err := tc.conn.Write(buf.Bytes())
	return err
}

func (tc *tcpClient) replyStream() <-chan *nh.Reply {
	ch := make(chan *nh.Reply)

	go func() {
		defer close(ch)

		for {
			select {
			case <-tc.done:
				return
			default:
			}

			sizeFrame := make([]byte, sizeLen)
			if _, err := io.ReadFull(tc.conn, sizeFrame); err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				panic(fmt.Errorf("read size frame, %w", err))
			}
			size := int(binary.BigEndian.Uint32(sizeFrame))
			if size == 0 { // ping
				continue
			}

			data := make([]byte, size)
			if _, err := io.ReadFull(tc.conn, data); err != nil {
				panic(fmt.Errorf("read data frame, %w", err))
			}

			reply := &nh.Reply{}
			if err := proto.Unmarshal(data, reply); err != nil {
				panic(fmt.Errorf("unmarshal reply, %w", err))
			}

			select {
			case <-tc.done:
				return
			case ch <- reply:
			}
		}
	}()

	return ch
}

func (tc *tcpClient) Close() {
	close(tc.done)
	_ = tc.conn.Close()
}

type wsClient struct {
	conn *websocket.Conn
	done chan struct{}
}

func newWSClient(url string) (*wsClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return &wsClient{
		conn: conn,
		done: make(chan struct{}),
	}, nil
}

func (wc *wsClient) send(data []byte) error {
	wc.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return wc.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (wc *wsClient) replyStream() <-chan *nh.Reply {
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

func (wc *wsClient) Close() {
	close(wc.done)
	wc.conn.Close()
}

// Client 网关客户端，用于测试及演示
type Client struct {
	client

	idSeq *atomic.Uint32

	// serviceCode => messageType => handler
	handlers       *gokit.MapOf[int32, *gokit.MapOf[int32, func(*nh.Reply)]]
	defaultHandler func(*nh.Reply)
}

// NewClient 创建客户端
func NewClient(dialURL string) (*Client, error) {
	l, err := url.Parse(dialURL)
	if err != nil {
		return nil, fmt.Errorf("parse dial url, %w", err)
	}

	var cc client
	switch l.Scheme {
	case "tcp":
		cc, err = newTCPClient(l.Host)
		if err != nil {
			return nil, fmt.Errorf("dial tcp, %w", err)
		}
	case "ws":
		cc, err = newWSClient(dialURL)
		if err != nil {
			return nil, fmt.Errorf("dial websocket, %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", l.Scheme)
	}

	c := &Client{
		client:         cc,
		idSeq:          &atomic.Uint32{},
		handlers:       gokit.NewMapOf[int32, *gokit.MapOf[int32, func(*nh.Reply)]](),
		defaultHandler: func(reply *nh.Reply) {},
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

	return c.send(data)
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
	ch := c.replyStream()
	for {
		select {
		case reply, ok := <-ch:
			if !ok {
				return
			}

			if handlers, ok := c.handlers.Load(reply.GetFromService()); ok {
				if handler, ok := handlers.Load(reply.GetMessageType()); ok {
					handler(reply)
					continue
				}
			}
			c.defaultHandler(reply)
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
