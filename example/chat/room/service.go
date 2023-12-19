package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/joyparty/gokit"
	"gitlab.haochang.tv/gopkg/nodehub/component/rpc"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/clusterpb"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/roompb"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/notification"
	"gitlab.haochang.tv/gopkg/nodehub/proto/clientpb"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

var emptyReply = &emptypb.Empty{}

type roomService struct {
	roompb.UnimplementedRoomServer

	publisher notification.Publisher
	members   *gokit.MapOf[string, string] // id => name
}

func (rs *roomService) Join(ctx context.Context, req *roompb.JoinRequest) (*emptypb.Empty, error) {
	userID := mustUserID(ctx)

	userName := req.GetName()
	if userName == "" {
		userName = fmt.Sprintf("anonymous%s", userID)
	}
	rs.members.Store(userID, userName)

	rs.boardcast(&roompb.News{
		Content: fmt.Sprintf("ROOM: #%s join", userID),
	})

	logger.Info("join", "name", userName, "id", userID)
	return emptyReply, nil
}

func (rs *roomService) Leave(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	userID := mustUserID(ctx)

	if _, ok := rs.members.Load(userID); ok {
		rs.members.Delete(userID)

		rs.boardcast(&roompb.News{
			Content: fmt.Sprintf("ROOM: #%s leave", userID),
		})
	}

	logger.Info("leave", "userID", userID)
	return emptyReply, nil
}

func (rs *roomService) Say(ctx context.Context, req *roompb.SayRequest) (*emptypb.Empty, error) {
	userID := mustUserID(ctx)
	if name, ok := rs.members.Load(userID); ok {
		news := &roompb.News{
			FromId:   userID,
			FromName: name,
			Content:  req.Content,
		}

		if req.To == "" {
			rs.boardcast(news)
		} else {
			rs.unicast(req.To, news)
		}

		logger.Info("say", "from", name, "to", req.To, "content", req.Content)
	}
	return emptyReply, nil
}

func (rs *roomService) boardcast(news *roompb.News) {
	response, _ := clientpb.NewResponse(int32(roompb.Protocol_NEWS), news)
	response.ServiceCode = int32(clusterpb.Services_ROOM)

	receiver := []string{}
	rs.members.Range(func(id, name string) bool {
		receiver = append(receiver, id)
		return true
	})

	notify := clientpb.NewNotification(receiver, response)
	if err := rs.publisher.Publish(context.Background(), notify); err != nil {
		logger.Error("publish notification", "error", err)
	}
}

func (rs *roomService) unicast(toName string, news *roompb.News) {
	response, _ := clientpb.NewResponse(int32(roompb.Protocol_NEWS), news)
	response.ServiceCode = int32(clusterpb.Services_ROOM)

	rs.members.Range(func(id, name string) bool {
		if name == toName {
			notify := clientpb.NewNotification([]string{id}, response)
			if err := rs.publisher.Publish(context.Background(), notify); err != nil {
				logger.Error("publish notification", "error", err)
			}
			return false
		}

		return true
	})
}

func mustUserID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		panic(errors.New("no metadata"))
	}

	vals := md.Get(rpc.MDUserID)
	if len(vals) == 0 {
		panic(errors.New("no user id"))
	}

	return vals[0]
}
