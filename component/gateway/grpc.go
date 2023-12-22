package gateway

import (
	"context"

	"gitlab.haochang.tv/gopkg/nodehub/proto/nodehubpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

var emptyReply = &emptypb.Empty{}

type gwService struct {
	nodehubpb.UnimplementedGatewayServer
	sessionHub *sessionHub
	stateTable *stateTable
}

func (s *gwService) IsSessionExist(ctx context.Context, req *nodehubpb.IsSessionExistRequest) (*nodehubpb.IsSessionExistResponse, error) {
	_, exist := s.sessionHub.Load(req.GetSessionId())

	return &nodehubpb.IsSessionExistResponse{
		SessionId: req.GetSessionId(),
		Exist:     exist,
	}, nil
}

func (s *gwService) SetServiceRoute(ctx context.Context, req *nodehubpb.SetServiceRouteRequest) (*emptypb.Empty, error) {
	s.stateTable.Store(req.GetSessionId(), req.GetServiceCode(), req.GetNodeId())
	return emptyReply, nil
}

func (s *gwService) RemoveServiceRoute(ctx context.Context, req *nodehubpb.RemoveServiceRouteRequest) (*emptypb.Empty, error) {
	s.stateTable.Remove(req.GetSessionId(), req.GetServiceCode())
	return emptyReply, nil
}
