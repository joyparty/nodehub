package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/oklog/ulid/v2"
	"github.com/panjf2000/ants/v2"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// Authorizer tcp授权函数
type Authorizer func(ctx context.Context, sess Session) (userID string, md metadata.MD, ok bool)

// tcpServer tcp网关服务
type tcpServer struct {
	listenAddr string
	listener   net.Listener
	authorizer Authorizer
}

// NewTCPServer 构造函数
func NewTCPServer(listenAddr string, authorizer Authorizer) Transporter {
	return &tcpServer{
		listenAddr: listenAddr,
		authorizer: authorizer,
	}
}

// CompleteNodeEntry 补全节点信息
func (ts *tcpServer) CompleteNodeEntry(entry *cluster.NodeEntry) {
	entry.Entrance = fmt.Sprintf("tcp://%s", ts.listenAddr)
}

func (ts *tcpServer) Serve(ctx context.Context) (chan Session, error) {
	l, err := net.Listen("tcp", ts.listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen, %w", err)
	}
	ts.listener = l

	ch := make(chan Session)
	go func() {
		defer close(ch)

		for {
			conn, err := l.Accept()
			if errors.Is(err, net.ErrClosed) {
				return
			} else if err != nil {
				logger.Error("tcp accept", "error", err)
				continue
			}

			if err := ants.Submit(func() {
				sess, err := ts.newSession(ctx, conn)
				if err != nil {
					logger.Error("initialize tcp session", "error", err, "remoteAddr", conn.RemoteAddr().String())
					_ = conn.Close()
					return
				}

				ch <- sess
			}); err != nil {
				logger.Error("handle tcp connection", "error", err, "remoteAddr", conn.RemoteAddr().String())
			}
		}
	}()

	return ch, nil
}

// Shutdown 停止服务
func (ts *tcpServer) Shutdown(ctx context.Context) error {
	_ = ts.listener.Close()
	return nil
}

func (ts *tcpServer) newSession(ctx context.Context, conn net.Conn) (Session, error) {
	sess := newTCPSession(conn)

	userID, md, ok := ts.authorizer(ctx, sess)
	if !ok {
		return nil, ErrDenyByAuthorizer
	} else if userID == "" {
		return nil, fmt.Errorf("user id is empty")
	} else if md == nil {
		md = metadata.MD{}
	}

	sess.SetID(userID)
	sess.SetMetadata(md)
	return sess, nil
}

type tcpSession struct {
	id         string
	conn       net.Conn
	md         metadata.MD
	lastRWTime gokit.ValueOf[time.Time]
	closeOnce  sync.Once
}

func newTCPSession(conn net.Conn) *tcpSession {
	ts := &tcpSession{
		id:         ulid.Make().String(),
		conn:       conn,
		md:         metadata.New(nil),
		lastRWTime: gokit.NewValueOf[time.Time](),
	}
	ts.lastRWTime.Store(time.Now())

	return ts
}

func (ts *tcpSession) ID() string {
	return ts.id
}

func (ts *tcpSession) SetID(id string) {
	ts.id = id
}

func (ts *tcpSession) SetMetadata(md metadata.MD) {
	ts.md = md
}

func (ts *tcpSession) MetadataCopy() metadata.MD {
	return ts.md.Copy()
}

func (ts *tcpSession) Recv(req *nh.Request) (err error) {
	defer func() {
		if errors.Is(err, net.ErrClosed) {
			err = io.EOF
		}
	}()

	msg := msgPool.Get()
	defer msgPool.Put(msg)

	for {
		if err = readMessage(ts.conn, msg); err != nil {
			return fmt.Errorf("read message, %w", err)
		}
		ts.lastRWTime.Store(time.Now())

		if msg.size > 0 {
			if err := proto.Unmarshal(msg.Bytes(), req); err != nil {
				return fmt.Errorf("unmarshal request, %w", err)
			}
			return nil
		}
	}
}

func (ts *tcpSession) Send(reply *nh.Reply) error {
	return sendReply(reply, func(data []byte) error {
		_ = ts.conn.SetWriteDeadline(time.Now().Add(writeWait))
		_, err := ts.conn.Write(data)
		if err == nil {
			ts.lastRWTime.Store(time.Now())
		}
		return err
	})
}

func (ts *tcpSession) LocalAddr() string {
	return ts.conn.LocalAddr().String()
}

func (ts *tcpSession) RemoteAddr() string {
	return ts.conn.RemoteAddr().String()
}

func (ts *tcpSession) LastRWTime() time.Time {
	return ts.lastRWTime.Load()
}

func (ts *tcpSession) Close() (err error) {
	ts.closeOnce.Do(func() {
		err = ts.conn.Close()
	})
	return
}

func (ts *tcpSession) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", ts.id),
		slog.String("type", "tcp"),
		slog.String("addr", ts.RemoteAddr()),
	)
}
