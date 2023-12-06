package gateway

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oklog/ulid/v2"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/proto/clientpb"
	"google.golang.org/protobuf/proto"
)

var (
	// DefaultHeartbeatDuration 心跳消息发送时间间隔，连接在超过这个时间没有收发消息，就会发送心跳消息
	DefaultHeartbeatDuration = 1 * time.Minute

	// DefaultHeartbeatTimeout 心跳消息超时时间，默认为心跳消息发送时间间隔的3倍
	DefaultHeartbeatTimeout = 3 * DefaultHeartbeatDuration

	// ErrSessionClosed 会话已关闭
	ErrSessionClosed = errors.New("session closed")

	writeWait = 5 * time.Second
)

// Session 连接会话
type Session interface {
	ID() string
	SetID(string)
	Recv(*clientpb.Request) error
	Send(*clientpb.Response) error
	LocalAddr() string
	RemoteAddr() string
	LastRWTime() time.Time
	Close() error
}

type wsPayload struct {
	messageType int
	data        []byte
}

type wsSession struct {
	id   string
	conn *websocket.Conn

	// 不允许并发写，因此用队列方式排队写
	sendC chan wsPayload

	// 最后一次读写时间
	lastRWTime *atomic.Value

	done   chan struct{}
	closed *atomic.Bool
}

func newWsSession(conn *websocket.Conn) *wsSession {
	ws := &wsSession{
		id:         ulid.Make().String(),
		conn:       conn,
		sendC:      make(chan wsPayload, 3), // 为什么是三？因为事不过三
		done:       make(chan struct{}),
		lastRWTime: &atomic.Value{},
		closed:     &atomic.Bool{},
	}

	ws.lastRWTime.Store(time.Now())
	ws.setPingPongHandler()

	go ws.sendLoop()
	go ws.heartbeatLoop()
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

func (ws *wsSession) heartbeatLoop() {
	for {
		select {
		case <-ws.done:
			return
		case <-time.After(1 * time.Second):
			if time.Since(ws.LastRWTime()) > DefaultHeartbeatDuration {
				select {
				case <-ws.done:
				case ws.sendC <- wsPayload{
					messageType: websocket.PingMessage,
					data:        nil,
				}:
				}
			}
		}
	}
}

func (ws *wsSession) ID() string {
	return ws.id
}

func (ws *wsSession) SetID(id string) {
	ws.id = id
}

func (ws *wsSession) Recv(req *clientpb.Request) error {
	for {
		select {
		case <-ws.done:
			return ErrSessionClosed
		default:
		}

		messageType, message, err := ws.conn.ReadMessage()
		if err != nil {
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

func (ws *wsSession) Send(resp *clientpb.Response) error {
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
	for {
		select {
		case <-ws.done:
			return
		case payload := <-ws.sendC:
			ws.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.conn.WriteMessage(payload.messageType, payload.data); err != nil {
				args := []any{
					"error", err,
					"clientID", ws.ID(),
					"remoteAddr", ws.RemoteAddr(),
				}

				if payload.messageType == websocket.BinaryMessage {
					message := &clientpb.Response{}
					if err := proto.Unmarshal(payload.data, message); err == nil {
						args = append(args,
							"requestID", message.GetRequestId(),
							"serviceCode", message.GetServiceCode(),
							"type", message.GetType(),
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
	return ws.lastRWTime.Load().(time.Time)
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

// sessionHub 会话集合
type sessionHub struct {
	clients *sync.Map
	done    chan struct{}
	closed  *atomic.Bool
}

func newSessionHub() *sessionHub {
	hub := &sessionHub{
		clients: &sync.Map{},
		done:    make(chan struct{}),
		closed:  &atomic.Bool{},
	}

	go hub.removeZombie()
	return hub
}

func (h *sessionHub) Store(c Session) {
	h.clients.Store(c.ID(), c)
}

func (h *sessionHub) Load(id string) (Session, bool) {
	if c, ok := h.clients.Load(id); ok {
		return c.(Session), true
	}
	return nil, false
}

func (h *sessionHub) Delete(id string) {
	h.clients.Delete(id)
}

func (h *sessionHub) Range(f func(s Session) bool) {
	h.clients.Range(func(_, value any) bool {
		return f(value.(Session))
	})
}

func (h *sessionHub) Close() {
	if h.closed.CompareAndSwap(false, true) {
		close(h.done)

		h.Range(func(s Session) bool {
			s.Close()

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
				if time.Since(s.LastRWTime()) > DefaultHeartbeatTimeout {
					h.Delete(s.ID())
					s.Close()
				}
				return true
			})
		}
	}
}
