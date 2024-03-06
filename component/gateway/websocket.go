package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
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
	listenAddr     string
	authorizer     WSAuthorizer
	server         *http.Server
	sessionHandler SessionHandler
}

// NewWSServer 构造函数
func NewWSServer(listenAddr string, authorizer WSAuthorizer) Transporter {
	return &wsServer{
		listenAddr: listenAddr,
		authorizer: authorizer,
	}
}

// CompleteNodeEntry 补全节点信息
func (ws *wsServer) CompleteNodeEntry(entry *cluster.NodeEntry) {
	entry.Entrance = fmt.Sprintf("ws://%s/grpc", ws.listenAddr)
}

// SetSessionHandler 设置会话处理函数
func (ws *wsServer) SetSessionHandler(handler SessionHandler) {
	ws.sessionHandler = handler
}

// Start 启动websocket服务器
func (ws *wsServer) Start(ctx context.Context) error {
	router := http.NewServeMux()
	router.HandleFunc("/grpc", func(w http.ResponseWriter, r *http.Request) {
		ws.handle(w, r.WithContext(ctx))
	})

	ws.server = &http.Server{
		Addr:    ws.listenAddr,
		Handler: http.HandlerFunc(router.ServeHTTP),
	}

	go func() {
		if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("start gateway", "error", err)
			panic(fmt.Errorf("start gateway, %w", err))
		}
	}()

	return nil
}

// Stop 停止websocket服务器
func (ws *wsServer) Stop(ctx context.Context) error {
	return ws.server.Shutdown(ctx)
}

func (ws *wsServer) handle(w http.ResponseWriter, r *http.Request) {
	sess, err := ws.newSession(w, r)
	if err != nil {
		logger.Error("initialize session", "error", err, "remoteAddr", r.RemoteAddr)
		return
	}

	ws.sessionHandler(r.Context(), sess)
}

func (ws *wsServer) newSession(w http.ResponseWriter, r *http.Request) (Session, error) {
	userID, md, ok := ws.authorizer(w, r)
	if !ok {
		return nil, errors.New("deny by authorize")
	} else if userID == "" {
		return nil, errors.New("empty userID")
	} else if md == nil {
		md = metadata.MD{}
	}

	wsConn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("upgrade websocket, %w", err)
	}
	wsConn.SetReadLimit(int64(MaxPayloadSize))

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
			return io.EOF
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
