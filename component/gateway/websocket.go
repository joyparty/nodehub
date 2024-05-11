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

var (
	// Upgrader websocket upgrader
	Upgrader = websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		WriteBufferPool:  &sync.Pool{},
	}

	writeWait = 5 * time.Second
)

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

	closeOnce sync.Once
	done      chan struct{}
}

func newWsSession(conn *websocket.Conn) *wsSession {
	ws := &wsSession{
		id:         ulid.Make().String(),
		conn:       conn,
		sendC:      make(chan wsPayload, 3), // 为什么是三？因为事不过三
		done:       make(chan struct{}),
		lastRWTime: gokit.NewValueOf[time.Time](),
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

func (ws *wsSession) Send(resp *nh.Reply) error {
	data, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response, %w", err)
	}

	select {
	case <-ws.done:
		return io.EOF
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
					"session", ws,
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
