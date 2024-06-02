package gateway

import (
	"context"

	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/oklog/ulid/v2"
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
		Exist: exist,
	}, nil
}

// 会话数量
func (s *gwService) SessionCount(context.Context, *emptypb.Empty) (*nh.SessionCountResponse, error) {
	return &nh.SessionCountResponse{
		Count: int32(s.sessionHub.Count()),
	}, nil
}

func (s *gwService) CloseSession(ctx context.Context, req *nh.CloseSessionRequest) (*nh.CloseSessionResponse, error) {
	if sess, ok := s.sessionHub.Load(req.GetSessionId()); ok {
		if err := sess.Close(); err != nil {
			return nil, err
		}

		return &nh.CloseSessionResponse{
			Success: true,
		}, nil
	}

	return &nh.CloseSessionResponse{}, nil
}

func (s *gwService) SetServiceRoute(ctx context.Context, req *nh.SetServiceRouteRequest) (*emptypb.Empty, error) {
	if _, ok := s.sessionHub.Load(req.GetSessionId()); ok {
		nodeID, err := ulid.Parse(req.GetNodeId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid node id, %v", err)
		}

		s.stateTable.Store(req.GetSessionId(), req.GetServiceCode(), nodeID)
	}
	return emptyReply, nil
}

func (s *gwService) RemoveServiceRoute(ctx context.Context, req *nh.RemoveServiceRouteRequest) (*emptypb.Empty, error) {
	s.stateTable.Remove(req.GetSessionId(), req.GetServiceCode())
	return emptyReply, nil
}

func (s *gwService) ReplaceServiceRoute(ctx context.Context, req *nh.ReplaceServiceRouteRequest) (*emptypb.Empty, error) {
	oldID, err := ulid.Parse(req.GetOldNodeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid node id, %v", err)
	}

	newID, err := ulid.Parse(req.GetNewNodeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid node id, %v", err)
	}

	s.stateTable.ReplaceNode(oldID, newID)
	return emptyReply, nil
}

func (s *gwService) SendReply(ctx context.Context, req *nh.SendReplyRequest) (*nh.SendReplyResponse, error) {
	sess, ok := s.sessionHub.Load(req.GetSessionId())
	if !ok {
		return &nh.SendReplyResponse{}, nil
	}

	if req.GetReply().GetFromService() == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid reply, from_service is empty")
	} else if req.GetReply().GetMessageType() == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid reply, message_type is empty")
	}

	if err := sess.Send(req.GetReply()); err != nil {
		return nil, err
	}
	return &nh.SendReplyResponse{
		Success: true,
	}, nil
}

// NewGatewayClient 网关管理接口客户端
func NewGatewayClient(registry *cluster.Registry, nodeID ulid.ULID) (nh.GatewayClient, error) {
	conn, err := registry.GetGRPCConn(nodeID)
	if err != nil {
		return nil, err
	}

	return nh.NewGatewayClient(conn), nil
}
