package main

import (
	"context"
	"fmt"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/component/rpc"
	"github.com/joyparty/nodehub/example/chat/proto/clusterpb"
	"github.com/joyparty/nodehub/example/chat/proto/roompb"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/multicast"
	"github.com/joyparty/nodehub/proto/nh"
	"google.golang.org/protobuf/types/known/emptypb"
)

var emptyReply = &emptypb.Empty{}

type roomService struct {
	roompb.UnimplementedRoomServer

	publisher multicast.Publisher
	members   *gokit.MapOf[string, string] // id => name
}

func (rs *roomService) Join(ctx context.Context, req *roompb.JoinRequest) (*emptypb.Empty, error) {
	userID := rpc.SessionIDInContext(ctx)

	userName := req.GetName()
	if userName == "" {
		userName = fmt.Sprintf("anonymous%s", userID)
	}
	rs.members.Store(userID, userName)

	rs.boardcast(roompb.News_builder{
		Content: fmt.Sprintf("ROOM: #%s join", userID),
	}.Build())

	logger.Info("join", "name", userName, "id", userID)
	return emptyReply, nil
}

func (rs *roomService) Leave(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	userID := rpc.SessionIDInContext(ctx)

	if _, ok := rs.members.Load(userID); ok {
		rs.members.Delete(userID)

		rs.boardcast(roompb.News_builder{
			Content: fmt.Sprintf("ROOM: #%s leave", userID),
		}.Build())
	}

	logger.Info("leave", "userID", userID)
	return emptyReply, nil
}

func (rs *roomService) Say(ctx context.Context, req *roompb.SayRequest) (*emptypb.Empty, error) {
	userID := rpc.SessionIDInContext(ctx)
	if name, ok := rs.members.Load(userID); ok {
		news := roompb.News_builder{
			FromId:   userID,
			FromName: name,
			Content:  req.GetContent(),
		}.Build()

		if req.GetTo() == "" {
			rs.boardcast(news)
		} else {
			rs.unicast(req.GetTo(), news)
		}

		logger.Info("say", "from", name, "to", req.GetTo(), "content", req.GetContent())
	}
	return emptyReply, nil
}

func (rs *roomService) boardcast(news *roompb.News) {
	response, _ := nh.NewReply(int32(roompb.ReplyCode_NEWS), news)
	response.SetServiceCode(int32(clusterpb.Services_ROOM))

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
	response, _ := nh.NewReply(int32(roompb.ReplyCode_NEWS), news)
	response.SetServiceCode(int32(clusterpb.Services_ROOM))

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
