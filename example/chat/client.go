package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/component/gateway/client"
	"github.com/joyparty/nodehub/example/chat/proto/clusterpb"
	"github.com/joyparty/nodehub/example/chat/proto/roompb"
	"github.com/joyparty/nodehub/proto/nh"
	"google.golang.org/grpc/status"
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
	var cli *client.Client
	if useTCP {
		cli = gokit.MustReturn(client.New(fmt.Sprintf("tcp://%s", serverAddr)))
	} else {
		cli = gokit.MustReturn(client.New(fmt.Sprintf("ws://%s", serverAddr)))
	}
	defer cli.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli.Subscribe(ctx, chatServiceCode, int32(roompb.ReplyCode_NEWS), func(news *roompb.News, reply *nh.Reply) {
		if news.GetFromId() == "" {
			fmt.Printf(">>> %s\n", news.GetContent())
		} else {
			fmt.Printf("%s: %s\n", news.GetFromName(), news.GetContent())
		}
	})

	if err := cli.CallNoReply(ctx, chatServiceCode, "Join", roompb.JoinRequest_builder{Name: name}.Build()); err != nil {
		handleError(err)
	}

	defer func() {
		cli.CallNoReply(context.Background(), chatServiceCode, "Leave", &emptypb.Empty{})
	}()

	if say == "" {
		<-context.Background().Done()
	}

	if err := cli.CallNoReply(ctx, chatServiceCode, "Say", roompb.SayRequest_builder{Content: say}.Build()); err != nil {
		handleError(err)
	}
	time.Sleep(1 * time.Second)
}

func handleError(err error) {
	if s, ok := status.FromError(err); ok {
		fmt.Printf("RPCError, code = %s, message = %s\n", s.Code(), s.Message())
	} else {
		fmt.Printf("ERROR: %v\n", err)
	}

	os.Exit(1)
}
