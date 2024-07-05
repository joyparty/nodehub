package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub"
	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/component/gateway"
	"github.com/joyparty/nodehub/event"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/multicast"
	"github.com/nats-io/nats.go"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/metadata"
)

var (
	registry *cluster.Registry

	proxyListen string
	grpcListen  string
	natsURL     string
	useTCP      bool
)

func init() {
	flag.StringVar(&proxyListen, "proxy", "127.0.0.1:9000", "proxy listen address")
	flag.StringVar(&grpcListen, "grpc", "127.0.0.1:10000", "grpc listen address")
	flag.StringVar(&natsURL, "nats", "nats://127.0.0.1:4222,nats://127.0.0.1:5222,nats://127.0.0.1:6222", "nats address")
	flag.BoolVar(&useTCP, "tcp", false, "use tcp")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	logger.SetLogger(slog.Default())

	client := mustReturn(clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379", "127.0.0.1:2479", "127.0.0.1:2579"},
	}))

	registry = mustReturn(cluster.NewRegistry(client))
}

func main() {
	var transporter gateway.Transporter
	if useTCP {
		transporter = gateway.NewTCPServer(proxyListen)
	} else {
		transporter = gateway.NewWSServer(proxyListen, "")
	}

	evBus, muBus := newNatsBus()

	uid := &atomic.Int32{}
	gwConfig := nodehub.GatewayConfig{
		Options: []gateway.Option{
			gateway.WithTransporter(transporter),
			gateway.WithRequestLogger(slog.Default()),
			gateway.WithEventBus(evBus),
			gateway.WithMulticast(muBus),
			gateway.WithInitializer(func(ctx context.Context, sess gateway.Session) (userID string, md metadata.MD, err error) {
				userID = fmt.Sprintf("%d", uid.Add(1))
				md = metadata.MD{}
				return
			}),
		},

		GRPCListen: grpcListen,
	}

	node := nodehub.NewGatewayNode(registry, gwConfig)

	logger.Info("gateway server start", "listen", proxyListen)
	mustDo(node.Serve(context.Background()))
}

func mustDo(err error) {
	if err != nil {
		panic(err)
	}
}

func mustReturn[T any](t T, er error) T {
	if er != nil {
		panic(er)
	}
	return t
}

func newNatsBus() (*event.Bus, *multicast.Bus) {
	conn := gokit.MustReturn(nats.Connect(natsURL))
	return event.NewNatsBus(conn), multicast.NewNatsBus(conn)
}
