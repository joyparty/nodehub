package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/component/gateway/client"
	"github.com/joyparty/nodehub/example/echo/proto/authpb"
	"github.com/joyparty/nodehub/example/echo/proto/clusterpb"
	"github.com/joyparty/nodehub/example/echo/proto/echopb"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"google.golang.org/grpc/codes"
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
			Level: slog.LevelInfo,
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

	// 网关返回的RPC错误
	cli.OnReceive(0, int32(nh.ReplyCode_RPC_ERROR), func(requestID uint32, reply *nh.RPCError) {
		fmt.Printf("[%s] #%03d ERROR, call %d.%s(), code = %s, message = %s\n",
			time.Now().Format(time.RFC3339),
			requestID,
			reply.GetRequestService(),
			reply.GetRequestMethod(),
			codes.Code(reply.GetStatus().GetCode()),
			reply.GetStatus().GetMessage(),
		)
		os.Exit(1)
	})

	// echo接口返回
	cli.OnReceive(echoServiceCode, int32(echopb.ReplyCode_MSG), func(requestID uint32, reply *echopb.Msg) {
		logger.Info("receive reply", "requestID", requestID, "content", reply.GetContent())
	})

	// 收到鉴权成功消息后开始正式发送消息
	cli.OnReceive(authServiceCode, int32(authpb.Protocol_AUTHORIZE_ACK),
		func(requestID uint32, msg *authpb.AuthorizeAck) {
			for {
				cli.Call(echoServiceCode, "Send", &echopb.Msg{
					Content: "hello world!",
				})
				time.Sleep(1 * time.Second)
			}
		},
	)

	// 连接后首先发送鉴权消息
	cli.Call(0, "Authorize", &authpb.AuthorizeToken{
		Token: "0d8b750e-35e8-4f98-b032-f389d401213e",
	})

	<-context.Background().Done()
}

func newClient(endpoint string) *client.MustClient {
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

	return &client.MustClient{Client: cli}
}
