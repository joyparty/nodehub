package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/oklog/ulid/v2"
	"gitlab.haochang.tv/gopkg/nodehub"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/component/gateway"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/metadata"
)

var (
	registry    *cluster.Registry
	proxyListen string
	grpcListen  string
	useTCP      bool
)

func init() {
	flag.StringVar(&proxyListen, "proxy", "127.0.0.1:9000", "proxy listen address")
	flag.StringVar(&grpcListen, "grpc", "127.0.0.1:10000", "grpc listen address")
	flag.BoolVar(&useTCP, "tcp", false, "use tcp")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	logger.SetLogger(slog.Default())

	client, err := clientv3.New(clientv3.Config{
		DialTimeout: 5 * time.Second,
		Endpoints:   []string{"127.0.0.1:2379"},
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
	gwConfig := nodehub.GatewayConfig{
		Options: []gateway.Option{
			gateway.WithRequestLogger(slog.Default()),
		},

		GRPCListen: grpcListen,
	}

	if useTCP {
		gwConfig.TCPListen = proxyListen
		gwConfig.TCPAuthorizer = func(sess gateway.Session) (userID string, md metadata.MD, ok bool) {
			return ulid.Make().String(), metadata.MD{}, true
		}
	} else {
		gwConfig.WSListen = proxyListen
		gwConfig.WSAuthorizer = func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool) {
			return ulid.Make().String(), metadata.MD{}, true
		}
	}

	node := nodehub.NewGatewayNode(registry, gwConfig)

	if err := registry.Put(node.Entry()); err != nil {
		panic(err)
	}

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
