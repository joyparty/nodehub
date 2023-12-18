package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joyparty/gokit"
	"gitlab.haochang.tv/gopkg/nodehub/component/gateway"
	"gitlab.haochang.tv/gopkg/nodehub/example/echo/proto/echopb"
	"gitlab.haochang.tv/gopkg/nodehub/example/echo/proto/servicepb"
	"gitlab.haochang.tv/gopkg/nodehub/proto/clientpb"
	"gitlab.haochang.tv/gopkg/nodehub/proto/gatewaypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

var (
	serverAddr      string
	echoServiceCode = int32(servicepb.Services_ECHO)
)

func init() {
	flag.StringVar(&serverAddr, "server", "127.0.0.1:9000", "server address")
	flag.Parse()
}

func main() {
	endpoint := fmt.Sprintf("ws://%s/grpc", serverAddr)
	client := &echoClient{
		WSClient: gokit.MustReturn(gateway.NewWSClient(endpoint)),
	}
	defer client.Close()

	client.WSClient.SetDefaultHandler(func(resp *clientpb.Response) {
		fmt.Printf("[%s] response: %s\n", time.Now().Format(time.RFC3339), resp.String())
	})

	client.WSClient.OnReceive(gateway.ServiceCode, int32(gatewaypb.Protocol_RPC_ERROR), func(requestID uint32, reply *gatewaypb.RPCError) {
		fmt.Printf("[%s] #%03d ERROR, call %d.%s(), code = %s, message = %s\n",
			time.Now().Format(time.RFC3339),
			requestID,
			reply.ServiceCode,
			reply.Method,
			codes.Code(reply.Status.Code),
			reply.Status.Message,
		)
		os.Exit(1)
	})

	client.OnReceive(int32(echopb.Protocol_MSG), func(requestID uint32, reply *echopb.Msg) {
		fmt.Printf("[%s] #%03d receive: %s\n", time.Now().Format(time.RFC3339), requestID, reply.Message)
	})

	for {
		gokit.Must(
			client.Call("Send", &echopb.Msg{
				Message: "hello world!",
			}),
		)
		time.Sleep(1 * time.Second)
	}
}

type echoClient struct {
	*gateway.WSClient
}

func (c *echoClient) Call(method string, arg proto.Message, options ...gateway.CallOption) error {
	return c.WSClient.Call(echoServiceCode, method, arg, options...)
}

func (c *echoClient) OnReceive(messageType int32, handler any) {
	c.WSClient.OnReceive(echoServiceCode, messageType, handler)
}
