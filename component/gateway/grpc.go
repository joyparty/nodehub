package gateway

import (
	"context"

	"github.com/oklog/ulid/v2"
	"gitlab.haochang.tv/gopkg/nodehub/proto/nh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var emptyReply = &emptypb.Empty{}

type gwService struct {
	nh.UnimplementedGatewayServer

	sessionHub *sessionHub
	stateTable *stateTable
}

func (s *gwService) IsSessionExist(ctx context.Context, req *nh.IsSessionExistRequest) (*nh.IsSessionExistResponse, error) {
	_, exist := s.sessionHub.Load(req.GetSessionId())

	return &nh.IsSessionExistResponse{
		SessionId: req.GetSessionId(),
		Exist:     exist,
	}, nil
}

// 会话数量
func (s *gwService) SessionCount(context.Context, *emptypb.Empty) (*nh.SessionCountResponse, error) {
	return &nh.SessionCountResponse{
		Count: s.sessionHub.Count(),
	}, nil
}

func (s *gwService) CloseSession(ctx context.Context, req *nh.CloseSessionRequest) (*nh.CloseSessionResponse, error) {
	if sess, ok := s.sessionHub.Load(req.GetSessionId()); ok {
		if err := sess.Close(); err != nil {
			return nil, err
		}
		s.sessionHub.Delete(sess.ID())

		return &nh.CloseSessionResponse{
			SessionId: req.GetSessionId(),
			Success:   true,
		}, nil
	}

	return &nh.CloseSessionResponse{
		SessionId: req.GetSessionId(),
	}, nil
}

func (s *gwService) SetServiceRoute(ctx context.Context, req *nh.SetServiceRouteRequest) (*emptypb.Empty, error) {
	nodeID, _ := ulid.Parse(req.GetNodeId())
	s.stateTable.Store(req.GetSessionId(), req.GetServiceCode(), nodeID)
	return emptyReply, nil
}

func (s *gwService) RemoveServiceRoute(ctx context.Context, req *nh.RemoveServiceRouteRequest) (*emptypb.Empty, error) {
	s.stateTable.Remove(req.GetSessionId(), req.GetServiceCode())
	return emptyReply, nil
}

func (s *gwService) PushMessage(ctx context.Context, req *nh.PushMessageRequest) (*nh.PushMessageResponse, error) {
	sess, ok := s.sessionHub.Load(req.GetSessionId())
	if !ok {
		return &nh.PushMessageResponse{
			SessionId: req.GetSessionId(),
		}, nil
	}

	if req.GetReply().GetFromService() == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid content, from_service is required")
	} else if req.GetReply().GetMessageType() == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid content, message_type is required")
	}

	if err := sess.Send(req.GetReply()); err != nil {
		return nil, err
	}
	return &nh.PushMessageResponse{
		SessionId: req.GetSessionId(),
		Success:   true,
	}, nil
}
