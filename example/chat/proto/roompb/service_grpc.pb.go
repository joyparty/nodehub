// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.21.12
// source: room/service.proto

package roompb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Room_Join_FullMethodName  = "/room.Room/Join"
	Room_Say_FullMethodName   = "/room.Room/Say"
	Room_Leave_FullMethodName = "/room.Room/Leave"
)

// RoomClient is the client API for Room service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type RoomClient interface {
	Join(ctx context.Context, in *JoinRequest, opts ...grpc.CallOption) (Room_JoinClient, error)
	Say(ctx context.Context, in *SayRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	Leave(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type roomClient struct {
	cc grpc.ClientConnInterface
}

func NewRoomClient(cc grpc.ClientConnInterface) RoomClient {
	return &roomClient{cc}
}

func (c *roomClient) Join(ctx context.Context, in *JoinRequest, opts ...grpc.CallOption) (Room_JoinClient, error) {
	stream, err := c.cc.NewStream(ctx, &Room_ServiceDesc.Streams[0], Room_Join_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &roomJoinClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Room_JoinClient interface {
	Recv() (*News, error)
	grpc.ClientStream
}

type roomJoinClient struct {
	grpc.ClientStream
}

func (x *roomJoinClient) Recv() (*News, error) {
	m := new(News)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *roomClient) Say(ctx context.Context, in *SayRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, Room_Say_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *roomClient) Leave(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, Room_Leave_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RoomServer is the server API for Room service.
// All implementations must embed UnimplementedRoomServer
// for forward compatibility
type RoomServer interface {
	Join(*JoinRequest, Room_JoinServer) error
	Say(context.Context, *SayRequest) (*emptypb.Empty, error)
	Leave(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	mustEmbedUnimplementedRoomServer()
}

// UnimplementedRoomServer must be embedded to have forward compatible implementations.
type UnimplementedRoomServer struct {
}

func (UnimplementedRoomServer) Join(*JoinRequest, Room_JoinServer) error {
	return status.Errorf(codes.Unimplemented, "method Join not implemented")
}
func (UnimplementedRoomServer) Say(context.Context, *SayRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Say not implemented")
}
func (UnimplementedRoomServer) Leave(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Leave not implemented")
}
func (UnimplementedRoomServer) mustEmbedUnimplementedRoomServer() {}

// UnsafeRoomServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to RoomServer will
// result in compilation errors.
type UnsafeRoomServer interface {
	mustEmbedUnimplementedRoomServer()
}

func RegisterRoomServer(s grpc.ServiceRegistrar, srv RoomServer) {
	s.RegisterService(&Room_ServiceDesc, srv)
}

func _Room_Join_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(JoinRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(RoomServer).Join(m, &roomJoinServer{stream})
}

type Room_JoinServer interface {
	Send(*News) error
	grpc.ServerStream
}

type roomJoinServer struct {
	grpc.ServerStream
}

func (x *roomJoinServer) Send(m *News) error {
	return x.ServerStream.SendMsg(m)
}

func _Room_Say_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SayRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RoomServer).Say(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Room_Say_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RoomServer).Say(ctx, req.(*SayRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Room_Leave_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RoomServer).Leave(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Room_Leave_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RoomServer).Leave(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// Room_ServiceDesc is the grpc.ServiceDesc for Room service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Room_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "room.Room",
	HandlerType: (*RoomServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Say",
			Handler:    _Room_Say_Handler,
		},
		{
			MethodName: "Leave",
			Handler:    _Room_Leave_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Join",
			Handler:       _Room_Join_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "room/service.proto",
}
