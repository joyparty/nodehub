package gateway

import (
	"errors"
	"fmt"
	"nodehub/logger"
	clientpb "nodehub/proto/client"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

var (
	// DefaultHeartbeatDuration 心跳消息发送时间间隔，连接在超过这个时间没有收发消息，就会发送心跳消息
	DefaultHeartbeatDuration = 1 * time.Minute

	// DefaultHeartbeatTimeout 心跳消息超时时间，默认为心跳消息发送时间间隔的3倍
	DefaultHeartbeatTimeout = 3 * DefaultHeartbeatDuration

	// ErrSessionClosed 会话已关闭
	ErrSessionClosed = errors.New("session closed")
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
	id   *atomic.Value
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
		id:         &atomic.Value{},
		conn:       conn,
		sendC:      make(chan wsPayload, 128),
		done:       make(chan struct{}),
		lastRWTime: &atomic.Value{},
		closed:     &atomic.Bool{},
	}

	ws.id.Store(uuid.New().String())
	ws.lastRWTime.Store(time.Now())

	go ws.sendLoop()
	go ws.heartbeatLoop()
	return ws
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
	return ws.id.Load().(string)
}

func (ws *wsSession) SetID(id string) {
	ws.id.Store(id)
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
							"route", message.GetRoute(),
						)
					}
				}

				logger.Error("send message", args...)
			} else {
				ws.lastRWTime.Store(time.Now())
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
	h.clients.Store(c.ID, c)
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
