package gateway

import (
	"bufio"
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
	"github.com/joyparty/nodehub/internal/codec"
	"github.com/joyparty/nodehub/internal/metrics"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/oklog/ulid/v2"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// tcpServer tcp网关服务
type tcpServer struct {
	listenAddr string
	listener   net.Listener
}

// NewTCPServer 构造函数
func NewTCPServer(listenAddr string) Transporter {
	return &tcpServer{
		listenAddr: listenAddr,
	}
}

// BindTCPServer 绑定TCP服务器
func BindTCPServer(listener net.Listener) Transporter {
	return &tcpServer{
		listenAddr: listener.Addr().String(),
		listener:   listener,
	}
}

// CompleteNodeEntry 补全节点信息
func (ts *tcpServer) CompleteNodeEntry(entry *cluster.NodeEntry) {
	entry.Entrance = fmt.Sprintf("tcp://%s", ts.listenAddr)
}

func (ts *tcpServer) Serve(ctx context.Context) (chan Session, error) {
	if ts.listener == nil {
		l, err := net.Listen("tcp", ts.listenAddr)
		if err != nil {
			return nil, fmt.Errorf("listen, %w", err)
		}
		ts.listener = l
	}

	ch := make(chan Session)
	go func() {
		defer close(ch)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn, err := ts.listener.Accept()
			if errors.Is(err, net.ErrClosed) {
				return
			} else if err != nil {
				logger.Error("tcp accept", "error", err)
				continue
			}

			ch <- newTCPSession(conn)
		}
	}()

	return ch, nil
}

// Shutdown 停止服务
func (ts *tcpServer) Shutdown(ctx context.Context) error {
	return ts.listener.Close()
}

type tcpSession struct {
	id         string
	conn       net.Conn
	md         metadata.MD
	lastRWTime gokit.ValueOf[time.Time]
	closeOnce  sync.Once

	r *bufio.Reader
}

func newTCPSession(conn net.Conn) *tcpSession {
	ts := &tcpSession{
		id:         ulid.Make().String(),
		conn:       conn,
		md:         metadata.New(nil),
		lastRWTime: gokit.NewValueOf[time.Time](),

		r: bufio.NewReader(conn),
	}
	ts.lastRWTime.Store(time.Now())

	return ts
}

func (ts *tcpSession) Type() string {
	return "tcp"
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

	msg := codec.GetMessage()
	defer codec.PutMessage(msg)

	for {
		if err = codec.ReadMessage(ts.r, msg); err != nil {
			return fmt.Errorf("read message, %w", err)
		}
		ts.lastRWTime.Store(time.Now())

		if msg.Len() > 0 {
			metrics.IncrPayloadSize(ts.Type(), msg.Len())

			if err := proto.Unmarshal(msg.Bytes(), req); err != nil {
				return fmt.Errorf("unmarshal request, %w", err)
			}
			return nil
		}
	}
}

func (ts *tcpSession) Send(reply *nh.Reply) error {
	return codec.SendReply(reply, func(data []byte) error {
		_ = ts.conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
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
