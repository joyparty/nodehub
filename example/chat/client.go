package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/component/gateway"
	"github.com/joyparty/nodehub/example/chat/proto/clusterpb"
	"github.com/joyparty/nodehub/example/chat/proto/roompb"
	"github.com/joyparty/nodehub/proto/nh"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	serverAddr string
	name       string
	say        string
	useTCP     bool

	chatServiceCode = int32(clusterpb.Services_ROOM)
)

func init() {
	flag.StringVar(&serverAddr, "server", "127.0.0.1:9000", "server address")
	flag.StringVar(&name, "name", "", "user name")
	flag.StringVar(&say, "say", "", "words send after join")
	flag.BoolVar(&useTCP, "tcp", false, "use tcp")
	flag.Parse()
}

func main() {
	var gwClient *gateway.Client
	if useTCP {
		gwClient = gokit.MustReturn(gateway.NewClient(fmt.Sprintf("tcp://%s", serverAddr)))
	} else {
		gwClient = gokit.MustReturn(gateway.NewClient(fmt.Sprintf("ws://%s", serverAddr)))
	}

	client := newChatClient(gwClient)
	// defer client.Close()

	client.OnReceive(int32(roompb.ReplyCode_NEWS), func(requestID uint32, reply *roompb.News) {
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
			gateway.WithServerStream(),
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
	*gateway.Client
}

func newChatClient(gc *gateway.Client) *chatClient {
	cc := &chatClient{
		Client: gc,
	}

	cc.Client.OnReceive(0, int32(nh.Protocol_RPC_ERROR), func(requestID uint32, reply *nh.RPCError) {
		fmt.Printf("[%s] #%03d ERROR, call %d.%s(), code = %s, message = %s\n",
			time.Now().Format(time.RFC3339),
			requestID,
			reply.GetRequestService(),
			reply.GetRequestMethod(),
			codes.Code(reply.Status.Code),
			reply.Status.Message,
		)
		os.Exit(1) // revive:disable-line:deep-exit
	})

	return cc
}

func (c *chatClient) Call(method string, arg proto.Message, options ...gateway.CallOption) error {
	return c.Client.Call(chatServiceCode, method, arg, options...)
}

func (c *chatClient) OnReceive(messageType int32, handler any) {
	c.Client.OnReceive(chatServiceCode, messageType, handler)
}
