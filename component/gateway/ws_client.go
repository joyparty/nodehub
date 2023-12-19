package gateway

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joyparty/gokit"
	"gitlab.haochang.tv/gopkg/nodehub/proto/clientpb"
	"google.golang.org/protobuf/proto"
)

// WSClient websocket客户端，用于测试及演示
type WSClient struct {
	conn   *websocket.Conn
	wMutex *sync.Mutex
	idSeq  *atomic.Uint32
	done   chan struct{}

	// serviceCode => messageType => handler
	handlers       *gokit.MapOf[int32, *gokit.MapOf[int32, func(*clientpb.Response)]]
	defaultHandler func(*clientpb.Response)
}

// NewWSClient 创建websocket客户端
func NewWSClient(wsURL string) (*WSClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}

	c := &WSClient{
		conn:   conn,
		wMutex: &sync.Mutex{},
		idSeq:  &atomic.Uint32{},
		done:   make(chan struct{}),

		handlers:       gokit.NewMapOf[int32, *gokit.MapOf[int32, func(*clientpb.Response)]](),
		defaultHandler: func(resp *clientpb.Response) {},
	}
	go c.run()
	return c, nil
}

func (c *WSClient) run() {
	ch := c.responseStream()
	for {
		select {
		case <-c.done:
			return
		case resp, ok := <-ch:
			if !ok {
				return
			}

			if handlers, ok := c.handlers.Load(resp.ServiceCode); ok {
				if handler, ok := handlers.Load(resp.Type); ok {
					handler(resp)
				}
			} else {
				c.defaultHandler(resp)
			}
		}
	}
}

// Close 关闭客户端
func (c *WSClient) Close() {
	close(c.done)
	c.conn.Close()
}

// Call 发起远程调用
func (c *WSClient) Call(serviceCode int32, method string, arg proto.Message, options ...CallOption) error {
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

	c.wMutex.Lock()
	defer c.wMutex.Unlock()

	c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

// OnReceive 注册消息处理器
//
// Example:
//
//	client.OnReceive(gateway.ServiceCode, int32(gatewaypb.Protocol_RPC_ERROR), func(requestID uint32, msg *gatewaypb.RPCError) {
//		// ...
//	})
func (c *WSClient) OnReceive(serviceCode int32, messageType int32, handler any) {
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
		serviceHandlers, _ = c.handlers.LoadOrStore(serviceCode, gokit.NewMapOf[int32, func(*clientpb.Response)]())
	}
	serviceHandlers.Store(messageType, func(resp *clientpb.Response) {
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
func (c *WSClient) SetDefaultHandler(handler func(reply *clientpb.Response)) {
	c.defaultHandler = handler
}

func (c *WSClient) responseStream() <-chan *clientpb.Response {
	ch := make(chan *clientpb.Response)

	go func() {
		defer close(ch)

		for {
			select {
			case <-c.done:
				return
			default:
			}

			messageType, data, err := c.conn.ReadMessage()
			if err != nil && !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				panic(fmt.Errorf("read websocket, %w", err))
			}

			switch messageType {
			case websocket.BinaryMessage:
				resp := &clientpb.Response{}
				if err := proto.Unmarshal(data, resp); err != nil {
					panic(fmt.Errorf("unmarshal response message, %w", err))
				}

				select {
				case <-c.done:
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

func (c *WSClient) newRequest(msg proto.Message, options ...CallOption) (*clientpb.Request, error) {
	req := &clientpb.Request{
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

// CallOption 调用选项
type CallOption func(req *clientpb.Request)

// WithNode 指定节点
func WithNode(nodeID string) CallOption {
	return func(req *clientpb.Request) {
		req.NodeId = nodeID
	}
}

// WithNoReply 不需要回复
func WithNoReply() CallOption {
	return func(req *clientpb.Request) {
		req.NoReply = true
	}
}
