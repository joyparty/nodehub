package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joyparty/gokit"
	"gitlab.haochang.tv/gopkg/nodehub/component/gateway"
	serverpb "gitlab.haochang.tv/gopkg/nodehub/example/echo/proto/server"
	pb "gitlab.haochang.tv/gopkg/nodehub/example/echo/proto/server/echo"
	"gitlab.haochang.tv/gopkg/nodehub/proto/clientpb"
	"gitlab.haochang.tv/gopkg/nodehub/proto/gatewaypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

var (
	serverAddr      string
	echoServiceCode = int32(serverpb.Services_ECHO)
)

func init() {
	flag.StringVar(&serverAddr, "server", "127.0.0.1:9000", "server address")
	flag.Parse()
}

func main() {
	endpoint := fmt.Sprintf("ws://%s/grpc", serverAddr)
	client := &echoClient{
		Client: gokit.MustReturn(gateway.NewClient(endpoint)),
	}
	defer client.Close()

	client.Client.SetDefaultHandler(func(resp *clientpb.Response) {
		fmt.Printf("[%s] response: %s\n", time.Now().Format(time.RFC3339), resp.String())
	})

	client.Client.OnReceive(gateway.ServiceCode, int32(gatewaypb.Protocol_RPC_ERROR), func(requestID uint32, reply *gatewaypb.RPCError) {
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

	client.OnReceive(int32(pb.Protocol_MSG), func(requestID uint32, reply *pb.Msg) {
		fmt.Printf("[%s] #%03d receive: %s\n", time.Now().Format(time.RFC3339), requestID, reply.Message)
	})

	for {
		gokit.Must(
			client.Call("Send", &pb.Msg{
				Message: "hello world!",
			}),
		)
		time.Sleep(1 * time.Second)
	}
}

type echoClient struct {
	*gateway.Client
}

func (c *echoClient) Call(method string, arg proto.Message, options ...gateway.CallOption) error {
	return c.Client.Call(echoServiceCode, method, arg, options...)
}

func (c *echoClient) OnReceive(messageType int32, handler any) {
	c.Client.OnReceive(echoServiceCode, messageType, handler)
}
