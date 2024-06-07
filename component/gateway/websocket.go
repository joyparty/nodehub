package gateway

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/oklog/ulid/v2"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// Upgrader websocket upgrader
var Upgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	WriteBufferPool:  &sync.Pool{},
}

// wsServer websocket网关服务器
//
// 客户端通过websocket方式连接网关，网关再转发请求到grpc后端服务
type wsServer struct {
	url      *url.URL
	listener net.Listener
	server   *http.Server
}

// NewWSServer 构造函数
func NewWSServer(listenAddr string, urlPath string) Transporter {
	return &wsServer{
		url: &url.URL{
			Scheme: "ws",
			Host:   listenAddr,
			Path:   urlPath,
		},
	}
}

// BindWSServer 绑定websocket服务器
func BindWSServer(listener net.Listener, urlPath string) Transporter {
	return &wsServer{
		listener: listener,
		url: &url.URL{
			Scheme: "ws",
			Host:   listener.Addr().String(),
			Path:   urlPath,
		},
	}
}

// CompleteNodeEntry 补全节点信息
func (ws *wsServer) CompleteNodeEntry(entry *cluster.NodeEntry) {
	entry.Entrance = ws.url.String()
}

func (ws *wsServer) Serve(ctx context.Context) (chan Session, error) {
	ch := make(chan Session)

	router := http.NewServeMux()
	router.HandleFunc(ws.url.RequestURI(), func(w http.ResponseWriter, r *http.Request) {
		sess, err := ws.newSession(w, r)
		if err != nil {
			logger.Error("initialize session", "error", err, "remoteAddr", r.RemoteAddr)
			return
		}

		defer func() {
			if v := recover(); v != nil {
				// do nothing
				_ = v
			}
		}()

		ch <- sess
	})

	ws.server = &http.Server{
		Handler: http.HandlerFunc(router.ServeHTTP),
	}

	go func() {
		defer close(ch)

		var err error
		if ws.listener != nil {
			err = ws.server.Serve(ws.listener)
		} else {
			ws.server.Addr = ws.url.Host
			err = ws.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			logger.Error("start gateway", "error", err)
			panic(fmt.Errorf("start gateway, %w", err))
		}
	}()

	return ch, nil
}

// Shutdown 停止websocket服务器
func (ws *wsServer) Shutdown(ctx context.Context) error {
	return ws.server.Shutdown(ctx)
}

func (ws *wsServer) newSession(w http.ResponseWriter, r *http.Request) (sess Session, err error) {
	wsConn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("upgrade websocket, %w", err)
	}

	wsConn.SetReadLimit(int64(MaxMessageSize))
	return newWsSession(wsConn), nil
}

type wsSession struct {
	id         string
	conn       *websocket.Conn
	md         metadata.MD
	lastRWTime gokit.ValueOf[time.Time]

	writeMux  sync.Mutex
	closeOnce sync.Once
	done      chan struct{}
}

func newWsSession(conn *websocket.Conn) *wsSession {
	ws := &wsSession{
		id:         ulid.Make().String(),
		conn:       conn,
		done:       make(chan struct{}),
		lastRWTime: gokit.NewValueOf[time.Time](),
	}

	ws.lastRWTime.Store(time.Now())
	ws.setPingPongHandler()

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
			return io.EOF
		default:
		}

		messageType, message, err := ws.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
				websocket.CloseNormalClosure,
			) {
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

func (ws *wsSession) Send(reply *nh.Reply) error {
	data, err := proto.Marshal(reply)
	if err != nil {
		return fmt.Errorf("marshal response, %w", err)
	}

	ws.writeMux.Lock()
	defer ws.writeMux.Unlock()

	ws.conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	err = ws.conn.WriteMessage(websocket.BinaryMessage, data)
	if err == nil {
		ws.lastRWTime.Store(time.Now())
	}
	return err
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
	ws.closeOnce.Do(func() {
		close(ws.done)
		_ = ws.conn.Close()
	})
	return nil
}

func (ws *wsSession) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", ws.id),
		slog.String("type", "websocket"),
		slog.String("addr", ws.RemoteAddr()),
	)
}
