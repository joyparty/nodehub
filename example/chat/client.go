package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joyparty/gokit"
	"gitlab.haochang.tv/gopkg/nodehub/component/gateway"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/clusterpb"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/roompb"
	"gitlab.haochang.tv/gopkg/nodehub/proto/clientpb"
	"gitlab.haochang.tv/gopkg/nodehub/proto/gatewaypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	serverAddr string
	name       string
	say        string
	node       string

	chatServiceCode = int32(clusterpb.Services_ROOM)
)

func init() {
	flag.StringVar(&serverAddr, "server", "127.0.0.1:9000", "server address")
	flag.StringVar(&name, "name", "", "user name")
	flag.StringVar(&say, "say", "", "words send after join")
	flag.StringVar(&node, "node", "", "node id")
	flag.Parse()
}

func main() {
	client := &chatClient{
		WSClient: gokit.MustReturn(gateway.NewWSClient(fmt.Sprintf("ws://%s/grpc", serverAddr))),
	}
	// defer client.Close()

	client.WSClient.SetDefaultHandler(func(resp *clientpb.Response) {
		fmt.Printf("[%s] response: %s\n", time.Now().Format(time.RFC3339), resp.String())
	})

	client.WSClient.OnReceive(gateway.ServiceCode, int32(gatewaypb.Protocol_RPC_ERROR), func(requestID uint32, reply *gatewaypb.RPCError) {
		fmt.Printf("[%s] #%03d ERROR, call %d.%s(), code = %s, message = %s\n",
			time.Now().Format(time.RFC3339),
			requestID,
			reply.GetRequestService(),
			reply.GetRequestMethod(),
			codes.Code(reply.Status.Code),
			reply.Status.Message,
		)
		os.Exit(1)
	})

	client.OnReceive(int32(roompb.Protocol_NEWS), func(requestID uint32, reply *roompb.News) {
		if reply.FromId == "" {
			fmt.Printf(">>> %s\n", reply.Content)
		} else {
			fmt.Printf("%s: %s\n", reply.FromName, reply.Content)
		}
	})

	gokit.Must(
		client.Call("Join",
			&roompb.JoinRequest{Name: name},
			gateway.WithNoReply(),
			gateway.WithNode(node),
		),
	)

	defer func() {
		client.Call("Leave",
			&emptypb.Empty{},
			gateway.WithNoReply(),
		)
		time.Sleep(1 * time.Second)
	}()

	if say != "" {
		gokit.Must(
			client.Call("Say",
				&roompb.SayRequest{Content: say},
				gateway.WithNoReply(),
			),
		)
		time.Sleep(1 * time.Second)
		return
	}

	<-context.Background().Done()
}

type chatClient struct {
	*gateway.WSClient
}

func (c *chatClient) Call(method string, arg proto.Message, options ...gateway.CallOption) error {
	return c.WSClient.Call(chatServiceCode, method, arg, options...)
}

func (c *chatClient) OnReceive(messageType int32, handler any) {
	c.WSClient.OnReceive(chatServiceCode, messageType, handler)
}
