package nodehub

import (
	"context"

	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/oklog/ulid/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// 节点管理服务，每个节点都会自动注册
type nodeService struct {
	nh.UnimplementedNodeServer

	node *Node
}

// 改变节点状态
func (ns *nodeService) ChangeState(_ context.Context, req *nh.ChangeStateRequest) (*emptypb.Empty, error) {
	state := req.GetState()
	if state == "" {
		return nil, status.Error(codes.InvalidArgument, "state is empty")
	}

	return nh.EmptyReply, ns.node.ChangeState(cluster.NodeState(state))
}

// 关闭服务
func (ns *nodeService) Shutdown(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	logger.Info("node shutdown by rpc command")
	ns.node.Shutdown()

	return nh.EmptyReply, nil
}

// NewNodeClient 节点管理服务客户端
func NewNodeClient(registry *cluster.Registry, nodeID ulid.ULID) (nh.NodeClient, error) {
	conn, err := registry.GetGRPCConn(nodeID)
	if err != nil {
		return nil, err
	}

	return nh.NewNodeClient(conn), nil
}
