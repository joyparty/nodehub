package main

import (
	"context"
	"fmt"

	"github.com/joyparty/gokit"
	"gitlab.haochang.tv/gopkg/nodehub/component/rpc"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/clusterpb"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/roompb"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/multicast"
	"gitlab.haochang.tv/gopkg/nodehub/proto/nh"
	"google.golang.org/protobuf/types/known/emptypb"
)

var emptyReply = &emptypb.Empty{}

type roomService struct {
	roompb.UnimplementedRoomServer

	publisher multicast.Publisher
	members   *gokit.MapOf[string, string] // id => name
}

func (rs *roomService) Join(ctx context.Context, req *roompb.JoinRequest) (*emptypb.Empty, error) {
	userID := rpc.UserIDInContext(ctx)

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
	userID := rpc.UserIDInContext(ctx)

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
	userID := rpc.UserIDInContext(ctx)
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
	response, _ := nh.NewReply(int32(roompb.Protocol_NEWS), news)
	response.FromService = int32(clusterpb.Services_ROOM)

	receiver := []string{}
	rs.members.Range(func(id, name string) bool {
		receiver = append(receiver, id)
		return true
	})

	notify := nh.NewMulticast(receiver, response)
	if err := rs.publisher.Publish(context.Background(), notify); err != nil {
		logger.Error("publish notification", "error", err)
	}
}

func (rs *roomService) unicast(toName string, news *roompb.News) {
	response, _ := nh.NewReply(int32(roompb.Protocol_NEWS), news)
	response.FromService = int32(clusterpb.Services_ROOM)

	rs.members.Range(func(id, name string) bool {
		if name == toName {
			notify := nh.NewMulticast([]string{id}, response)
			if err := rs.publisher.Publish(context.Background(), notify); err != nil {
				logger.Error("publish notification", "error", err)
			}
			return false
		}

		return true
	})
}
