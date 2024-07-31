package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/component/gateway/client"
	"github.com/joyparty/nodehub/example/echo/proto/authpb"
	"github.com/joyparty/nodehub/example/echo/proto/clusterpb"
	"github.com/joyparty/nodehub/example/echo/proto/echopb"
	"github.com/joyparty/nodehub/logger"
	"google.golang.org/grpc/status"
)

var (
	serverAddr string
	useTCP     bool
	useQUIC    bool

	echoServiceCode = int32(clusterpb.Services_ECHO)
	authServiceCode = int32(clusterpb.Services_AUTH)
)

func init() {
	flag.StringVar(&serverAddr, "server", "127.0.0.1:9000", "server address")
	flag.BoolVar(&useTCP, "tcp", false, "use tcp")
	flag.BoolVar(&useQUIC, "quic", false, "use quic")
	flag.Parse()

	logger.SetLogger(
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})),
	)
}

func main() {
	var endpoint string
	if useTCP {
		endpoint = fmt.Sprintf("tcp://%s", serverAddr)
	} else if useQUIC {
		endpoint = fmt.Sprintf("quic://%s", serverAddr)
	} else {
		endpoint = fmt.Sprintf("ws://%s/", serverAddr)
	}

	cli := newClient(endpoint)
	defer cli.Close()

	// 鉴权请求
	authReply := &authpb.AuthorizeAck{}
	err := cli.Call(context.Background(), 0, "Authorize",
		&authpb.AuthorizeToken{Token: "0d8b750e-35e8-4f98-b032-f389d401213e"},
		authReply,
	)
	if err != nil {
		handleError(err)
	}
	logger.Info("auth success", "user_id", authReply.GetUserId())

	var wg sync.WaitGroup
	for {

		for i := 0; i < 3; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				echoReply := &echopb.Msg{}
				err := cli.Call(context.Background(), echoServiceCode, "Send",
					&echopb.Msg{Content: "hello world!"},
					echoReply,
					client.WithStream("test:echo"),
				)

				if err != nil {
					handleError(err)
				} else {
					logger.Info("echo back", "content", echoReply.GetContent())
				}
			}()
		}

		wg.Wait()
		time.Sleep(1 * time.Second)
	}
}

func handleError(err error) {
	if s, ok := status.FromError(err); ok {
		fmt.Printf("RPCError, code = %s, message = %s\n", s.Code(), s.Message())
	} else {
		fmt.Printf("ERROR: %v\n", err)
	}

	os.Exit(1)
}

func newClient(endpoint string) *client.Client {
	var cli *client.Client
	if strings.HasPrefix(endpoint, "quic://") {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"quic-echo-example"},
		}
		cli = gokit.MustReturn(client.NewQUIC(endpoint, tlsConfig, nil))
	} else {
		cli = gokit.MustReturn(client.New(endpoint))
	}

	return cli
}
