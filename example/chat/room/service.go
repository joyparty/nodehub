package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/joyparty/gokit"
	"gitlab.haochang.tv/gopkg/nodehub"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/roompb"
	"gitlab.haochang.tv/gopkg/nodehub/example/chat/proto/servicepb"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/notification"
	"gitlab.haochang.tv/gopkg/nodehub/proto/clientpb"
	"gitlab.haochang.tv/gopkg/nodehub/proto/gatewaypb"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var emptyReply = &emptypb.Empty{}

type roomService struct {
	roompb.UnimplementedRoomServer

	publisher notification.Publisher
	members   *gokit.MapOf[string, string] // id => name
}

func (rs *roomService) Join(ctx context.Context, req *roompb.JoinRequest) (*emptypb.Empty, error) {
	userID := mustUserID(ctx)
	rs.members.Store(userID, req.Name)

	rs.boardcast(&roompb.News{
		Content: fmt.Sprintf("%s#%s join", req.Name, userID),
	})

	logger.Info("join", "name", req.Name, "id", userID)
	return emptyReply, nil
}

func (rs *roomService) Leave(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	userID := mustUserID(ctx)

	if name, ok := rs.members.Load(userID); ok {
		rs.members.Delete(userID)

		rs.boardcast(&roompb.News{
			Content: fmt.Sprintf("%s#%s leave", name, userID),
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
	response.ServiceCode = int32(servicepb.Services_ROOM)

	notify := &gatewaypb.Notification{
		Time:    timestamppb.Now(),
		Content: response,
	}

	rs.members.Range(func(id, name string) bool {
		v, _ := proto.Clone(notify).(*gatewaypb.Notification)
		v.UserId = id

		if err := rs.publisher.Publish(context.Background(), v); err != nil {
			logger.Error("publish notification", "error", err)
		}
		return true
	})
}

func (rs *roomService) unicast(toName string, news *roompb.News) {
	response, _ := clientpb.NewResponse(int32(roompb.Protocol_NEWS), news)
	response.ServiceCode = int32(servicepb.Services_ROOM)

	notify := &gatewaypb.Notification{
		Time:    timestamppb.Now(),
		Content: response,
	}

	rs.members.Range(func(id, name string) bool {
		if name == toName {
			notify.UserId = id

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

	vals := md.Get(nodehub.MDUserID)
	if len(vals) == 0 {
		panic(errors.New("no user id"))
	}

	return vals[0]
}
