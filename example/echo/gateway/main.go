package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"log/slog"
	"math/big"
	"net"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub"
	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/component/gateway"
	"github.com/joyparty/nodehub/event"
	"github.com/joyparty/nodehub/example/echo/proto/authpb"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/multicast"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/nats-io/nats.go"
	"github.com/oklog/ulid/v2"
	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

var (
	registry    *cluster.Registry
	proxyListen string
	reuse       bool
	grpcListen  string
	natsURL     string
	useTCP      bool
	useQUIC     bool
)

func init() {
	flag.StringVar(&proxyListen, "proxy", "127.0.0.1:9000", "proxy listen address")
	flag.BoolVar(&reuse, "reuse", false, "reuse port")
	flag.StringVar(&grpcListen, "grpc", "127.0.0.1:10000", "grpc listen address")
	flag.StringVar(&natsURL, "nats", "nats://127.0.0.1:4222,nats://127.0.0.1:5222,nats://127.0.0.1:6222", "nats address")
	flag.BoolVar(&useTCP, "tcp", false, "use tcp")
	flag.BoolVar(&useQUIC, "quic", false, "use quic")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	logger.SetLogger(slog.Default())

	client, err := clientv3.New(clientv3.Config{
		DialTimeout: 5 * time.Second,
		Endpoints:   []string{"127.0.0.1:2379", "127.0.0.1:2479", "127.0.0.1:2579"},
	})
	if err != nil {
		panic(err)
	}

	registry, err = cluster.NewRegistry(client)
	if err != nil {
		panic(err)
	}
}

func main() {
	var transporter gateway.Transporter

	if useTCP {
		transporter = gateway.BindTCPServer(
			gokit.MustReturn(newTCPListener(proxyListen, reuse)),
		)
	} else if useQUIC {
		transporter = gateway.BindQUICServer(
			gokit.MustReturn(newPacketConn(proxyListen, reuse)),
			generateTLSConfig(),
			nil,
		)
	} else {
		transporter = gateway.BindWSServer(
			gokit.MustReturn(newTCPListener(proxyListen, reuse)),
			"",
		)
	}

	evBus, muBus := newNatsBus()
	gwConfig := nodehub.GatewayConfig{
		Options: []gateway.Option{
			gateway.WithRequestLogger(slog.Default()),
			gateway.WithTransporter(transporter),
			gateway.WithEventBus(evBus),
			gateway.WithMulticast(muBus),
			gateway.WithInitializer(initializer),
		},

		GRPCListener: gokit.MustReturn(newTCPListener(grpcListen, reuse)),

		// 启动prometheus指标服务
		MetricsListen: "127.0.0.1:12345",
	}

	node := nodehub.NewGatewayNode(registry, gwConfig)

	// 自动关闭新上线节点
	// registry.SubscribeUpdate(func(entry cluster.NodeEntry) {
	// 	if entry.ID == node.ID() || entry.State != cluster.NodeOK {
	// 		return
	// 	}
	// 	time.Sleep(3 * time.Second)

	// 	client, err := registry.GetNodeClient(entry.ID)
	// 	if errors.Is(err, cluster.ErrNodeNotFoundOrDown) {
	// 		return
	// 	} else if err != nil {
	// 		panic(err)
	// 	}

	// 	logger.Info("send ChangeState command", "nodeID", entry.ID)
	// 	if _, err := client.ChangeState(context.Background(), &nh.ChangeStateRequest{
	// 		State: string(cluster.NodeLazy),
	// 	}); err != nil {
	// 		panic(err)
	// 	}
	// })

	// registry.SubscribeUpdate(func(entry cluster.NodeEntry) {
	// 	if entry.ID == node.ID() || entry.State != cluster.NodeLazy {
	// 		return
	// 	}
	// 	time.Sleep(3 * time.Second)

	// 	client, err := registry.GetNodeClient(entry.ID)
	// 	if errors.Is(err, cluster.ErrNodeNotFoundOrDown) {
	// 		return
	// 	} else if err != nil {
	// 		panic(err)
	// 	}

	// 	logger.Info("send shutdown command", "nodeID", entry.ID)
	// 	if _, err := client.Shutdown(context.Background(), &emptypb.Empty{}); err != nil {
	// 		panic(err)
	// 	}
	// })

	if err := node.Serve(context.Background()); err != nil {
		panic(err)
	}
}

func newNatsBus() (*event.Bus, *multicast.Bus) {
	conn := gokit.MustReturn(nats.Connect(natsURL))
	return event.NewNatsBus(conn), multicast.NewNatsBus(conn)
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-echo-example"},
	}
}

func newTCPListener(addr string, reuse bool) (net.Listener, error) {
	if !reuse {
		return net.Listen("tcp", addr)
	}

	lc := &net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEADDR, 1)
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
		},
	}

	return lc.Listen(context.Background(), "tcp", addr)
}

func newPacketConn(addr string, reuse bool) (net.PacketConn, error) {
	if !reuse {
		return net.ListenPacket("udp", addr)
	}

	lc := &net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEADDR, 1)
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
		},
	}

	return lc.ListenPacket(context.Background(), "udp", addr)
}

// 客户端在连接之后的5秒内需要发送鉴权消息，鉴权失败或超时都会断开连接
func initializer(ctx context.Context, sess gateway.Session) (userID string, md metadata.MD, err error) {
	validToken := func(req *nh.Request) error {
		if req.GetMethod() != "Authorize" {
			return errors.New("invalid method")
		}

		token := &authpb.AuthorizeToken{}
		if err := proto.Unmarshal(req.GetData(), token); err != nil {
			return err
		}

		if token.GetToken() != "0d8b750e-35e8-4f98-b032-f389d401213e" {
			return errors.New("invalid token")
		}

		userID = ulid.Make().String()
		md = metadata.New(nil)
		return nil
	}

	var requestID uint32 = 1
	errC := make(chan error, 1)
	go func() {
		defer close(errC)

		req := &nh.Request{}
		if err := sess.Recv(req); err != nil {
			errC <- err
			return
		}
		atomic.StoreUint32(&requestID, req.GetId())

		if err := validToken(req); err != nil {
			errC <- err
			return
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	select {
	case err = <-errC:
	case <-ctx.Done():
		err = ctx.Err()
	}

	if err == nil {
		reply := gokit.MustReturn(authpb.PackAuthorizeAck(authpb.AuthorizeAck_builder{
			UserId: userID,
		}.Build()))
		// 确保Reply的RequestId和Request的Id一致
		reply.SetRequestId(atomic.LoadUint32(&requestID))

		gokit.Must(sess.Send(reply))
	}

	return
}
