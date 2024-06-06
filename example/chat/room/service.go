package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/component/rpc"
	"github.com/joyparty/nodehub/event"
	"github.com/joyparty/nodehub/example/chat/proto/roompb"
	"github.com/joyparty/nodehub/logger"
	"github.com/reactivex/rxgo/v2"
	"google.golang.org/protobuf/types/known/emptypb"
)

var emptyReply = &emptypb.Empty{}

type roomService struct {
	roompb.UnimplementedRoomServer

	eventBus *event.Bus
	members  *gokit.MapOf[string, string] // id => name
	event    chan rxgo.Item
	observer rxgo.Observable
}

// NewRoomService 构造函数
func NewRoomService(eventBus *event.Bus) roompb.RoomServer {
	event := make(chan rxgo.Item)

	return &roomService{
		eventBus: eventBus,
		members:  gokit.NewMapOf[string, string](),
		event:    event,
		observer: rxgo.FromEventSource(event),
	}
}

func (rs *roomService) Join(req *roompb.JoinRequest, stream roompb.Room_JoinServer) error {
	userID := rpc.SessionIDInContext(stream.Context())
	rs.members.Store(userID, req.GetName())
	defer rs.members.Delete(userID)

	ctx, cancel := context.WithCancel(stream.Context())
	cancel = sync.OnceFunc(cancel)
	defer cancel()

	rxgo.Of(eventJoin{
		UserID: userID,
		Name:   req.GetName(),
	}).SendContext(ctx, rs.event)

	// 订阅网络离线事件，执行cancel ctx
	rs.eventBus.Subscribe(ctx, func(ev event.UserDisconnected, _ time.Time) {
		if ev.UserID == userID {
			cancel()
		}
	})

	// 收到自己的离开事件，执行cancel ctx
	rs.observer.DoOnNext(
		func(item any) {
			if v, ok := item.(eventLeave); ok {
				if v.UserID == userID {
					cancel()
				}
			}
		},
		rxgo.WithContext(ctx),
	)

	<-rs.observer.DoOnNext(
		func(item any) {
			var news *roompb.News

			switch event := item.(type) {
			case eventSay:
				// 忽略不是发给自己的消息
				if event.ToUserID != "" && event.ToUserID != userID {
					return
				}

				news = &roompb.News{
					FromId:   event.UserID,
					FromName: event.UserName,
					Content:  event.Content,
				}
			case eventJoin:
				news = &roompb.News{
					Content: fmt.Sprintf("ROOM: %s join", event.Name),
				}
			case eventLeave:
				news = &roompb.News{
					Content: fmt.Sprintf("ROOM: %s leave", event.Name),
				}
			default:
				logger.Error("unknown event", "event", event)
				return
			}

			if err := stream.Send(news); err != nil {
				logger.Error("send news", "error", err)
			}
		},
		rxgo.WithContext(ctx),
	)

	logger.Info("stream completed", "userID", userID, "name", req.GetName())
	return nil
}

func (rs *roomService) Leave(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	userID := rpc.SessionIDInContext(ctx)
	if name, ok := rs.members.Load(userID); ok {
		rxgo.Of(eventLeave{
			UserID: userID,
			Name:   name,
		}).SendContext(ctx, rs.event)
	}

	return emptyReply, nil
}

func (rs *roomService) Say(ctx context.Context, req *roompb.SayRequest) (*emptypb.Empty, error) {
	userID := rpc.SessionIDInContext(ctx)
	if name, ok := rs.members.Load(userID); ok {
		event := eventSay{
			UserID:   userID,
			UserName: name,
			Content:  req.GetContent(),
		}

		if toName := req.GetTo(); toName != "" {
			rs.members.Range(func(id, name string) bool {
				if name == toName {
					event.ToUserID = id
					event.ToUserName = name

					return false
				}

				return true
			})
		}

		rxgo.Of(event).SendContext(ctx, rs.event)
	}

	return emptyReply, nil
}

type eventSay struct {
	UserID     string
	UserName   string
	ToUserID   string
	ToUserName string
	Content    string
}

type eventLeave struct {
	UserID string
	Name   string
}

type eventJoin struct {
	UserID string
	Name   string
}
