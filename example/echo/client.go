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
	"github.com/joyparty/nodehub/component/gateway"
	"github.com/joyparty/nodehub/example/echo/proto/authpb"
	"github.com/joyparty/nodehub/example/echo/proto/clusterpb"
	"github.com/joyparty/nodehub/example/echo/proto/echopb"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

var (
	serverAddr string
	useTCP     bool
	useQUIC    bool

	echoServiceCode = int32(clusterpb.Services_ECHO)
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

	client := newEchoClient(endpoint)
	defer client.Close()

	client.OnReceive(int32(echopb.ReplyCode_MSG), func(requestID uint32, reply *echopb.Msg) {
		fmt.Printf("[%s] #%03d receive: %s\n", time.Now().Format(time.RFC3339), requestID, reply.GetContent())
	})

	// 收到鉴权成功消息后开始正式发送消息
	client.Client.OnReceive(
		int32(clusterpb.Services_AUTH),
		int32(authpb.Protocol_AUTHORIZE_ACK),
		func(requestID uint32, msg *authpb.AuthorizeAck) {
			for {
				gokit.Must(
					client.Call("Send", &echopb.Msg{
						Content: "hello world!",
					}),
				)
				time.Sleep(1 * time.Second)
			}
		},
	)

	// 连接后首先发送鉴权消息
	gokit.Must(
		client.Client.Call(0, "Authorize", &authpb.AuthorizeToken{
			Token: "0d8b750e-35e8-4f98-b032-f389d401213e",
		}),
	)

	<-context.Background().Done()
}

type echoClient struct {
	*gateway.Client
}

func newEchoClient(endpoint string) *echoClient {
	var client *gateway.Client
	if strings.HasPrefix(endpoint, "quic://") {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"quic-echo-example"},
		}
		client = gokit.MustReturn(gateway.NewQUICClient(endpoint, tlsConfig, nil))
	} else {
		client = gokit.MustReturn(gateway.NewClient(endpoint))
	}

	ec := &echoClient{
		Client: client,
	}

	ec.Client.OnReceive(0, int32(nh.Protocol_RPC_ERROR), func(requestID uint32, reply *nh.RPCError) {
		fmt.Printf("[%s] #%03d ERROR, call %d.%s(), code = %s, message = %s\n",
			time.Now().Format(time.RFC3339),
			requestID,
			reply.GetRequestService(),
			reply.GetRequestMethod(),
			codes.Code(reply.GetStatus().GetCode()),
			reply.GetStatus().GetMessage(),
		)
		os.Exit(1) // revive:disable-line:deep-exit
	})

	return ec
}

func (c *echoClient) Call(method string, arg proto.Message, options ...gateway.CallOption) error {
	return c.Client.Call(echoServiceCode, method, arg, options...)
}

func (c *echoClient) OnReceive(messageType int32, handler any) {
	c.Client.OnReceive(echoServiceCode, messageType, handler)
}
