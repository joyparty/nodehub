package gateway

import (
	"context"

	"gitlab.haochang.tv/gopkg/nodehub/proto/nh"
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

func (s *gwService) SetServiceRoute(ctx context.Context, req *nh.SetServiceRouteRequest) (*emptypb.Empty, error) {
	s.stateTable.Store(req.GetSessionId(), req.GetServiceCode(), req.GetNodeId())
	return emptyReply, nil
}

func (s *gwService) RemoveServiceRoute(ctx context.Context, req *nh.RemoveServiceRouteRequest) (*emptypb.Empty, error) {
	s.stateTable.Remove(req.GetSessionId(), req.GetServiceCode())
	return emptyReply, nil
}
