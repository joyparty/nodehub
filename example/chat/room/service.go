package main

import (
	"context"
	"errors"
	"fmt"
	"nodehub"
	"nodehub/example/chat/proto/roompb"
	"nodehub/example/chat/proto/servicepb"
	"nodehub/logger"
	"nodehub/notification"
	"nodehub/proto/clientpb"
	"nodehub/proto/gatewaypb"
	"sync"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type roomService struct {
	roompb.UnimplementedRoomServer

	publisher notification.Publisher
	members   *sync.Map // id => name
}

func (rs *roomService) Join(ctx context.Context, req *roompb.JoinRequest) (*emptypb.Empty, error) {
	userID := mustUserID(ctx)
	rs.members.Store(userID, req.Name)

	rs.boardcast(&roompb.News{
		Content: fmt.Sprintf("%s#%s join", req.Name, userID),
	})

	return nil, nil
}

func (rs *roomService) Leave(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	userID := mustUserID(ctx)

	if v, ok := rs.members.Load(userID); ok {
		rs.members.Delete(userID)

		rs.boardcast(&roompb.News{
			Content: fmt.Sprintf("%s#%s leave", v.(string), userID),
		})
	}

	return nil, nil
}

func (rs *roomService) Say(ctx context.Context, req *roompb.SayRequest) (*emptypb.Empty, error) {
	userID := mustUserID(ctx)
	if v, ok := rs.members.Load(userID); ok {
		name := v.(string)

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
	}
	return nil, nil
}

func (rs *roomService) boardcast(news *roompb.News) {
	response, _ := clientpb.NewResponse(int32(roompb.Protocol_NEWS), news)
	response.ServiceCode = int32(servicepb.Services_ROOM)

	notify := &gatewaypb.Notification{
		Time:    timestamppb.Now(),
		Content: response,
	}

	rs.members.Range(func(key, value any) bool {
		v, _ := proto.Clone(notify).(*gatewaypb.Notification)
		v.UserId = key.(string)

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

	rs.members.Range(func(key, value any) bool {
		if value.(string) == toName {
			notify.UserId = key.(string)

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
