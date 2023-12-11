package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/roompb"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/servicepb"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/proto/clientpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	requestID  = &atomic.Uint32{}
	serverAddr string

	userName string
	words    string
)

func init() {
	flag.StringVar(&serverAddr, "server", "127.0.0.1:9000", "server address")
	flag.StringVar(&userName, "name", "no name", "user name")
	flag.StringVar(&words, "say", "", "words send after join")
	flag.Parse()

	logger.SetLogger(slog.Default())
}

func main() {
	conn := mustReturn(newConn())

	// show news
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				panic(fmt.Errorf("read message, %w", err))
			}

			response := &clientpb.Response{}
			mustDo(proto.Unmarshal(data, response))

			switch response.ServiceCode {
			case int32(servicepb.Services_ROOM):
				switch response.Type {
				case int32(roompb.Protocol_NEWS):
					news := &roompb.News{}
					mustDo(proto.Unmarshal(response.Data, news))

					if news.FromId == "" {
						fmt.Printf(">>> %s\n", news.Content)
					} else {
						fmt.Printf("%s: %s\n", news.FromName, news.Content)
					}
				default:
					logger.Info("receive message", "serviceCode", response.ServiceCode, "type", response.Type)
				}
			default:
				logger.Info("receive message", "serviceCode", response.ServiceCode, "type", response.Type)
			}
		}
	}()

	// join
	req := mustReturn(newRequest(&roompb.JoinRequest{
		Name: userName,
	}))
	req.ServiceCode = int32(servicepb.Services_ROOM)
	req.Method = "Join"
	mustDo(sendRequest(conn, req))

	// leave
	defer func() {
		req := mustReturn(newRequest(&emptypb.Empty{}))
		req.ServiceCode = int32(servicepb.Services_ROOM)
		req.Method = "Leave"
		sendRequest(conn, req)

		time.Sleep(1 * time.Second)
	}()

	if words != "" {
		req = mustReturn(newRequest(&roompb.SayRequest{
			Content: words,
		}))
		req.ServiceCode = int32(servicepb.Services_ROOM)
		req.Method = "Say"
		mustDo(sendRequest(conn, req))
	}

	// do nothing
	<-context.Background().Done()
}

func newConn() (*websocket.Conn, error) {
	endpoint := fmt.Sprintf("ws://%s/grpc", serverAddr)
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	return conn, err
}

func newRequest(msg proto.Message) (*clientpb.Request, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return &clientpb.Request{
		Id:   requestID.Add(1),
		Data: data,
	}, nil
}

func sendRequest(conn *websocket.Conn, req proto.Message) error {
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request, %w", err)
	}

	return conn.WriteMessage(websocket.BinaryMessage, data)
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
