// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.21.12
// source: nodehub/gateway_service.proto

package nh

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
	Gateway_IsSessionExist_FullMethodName      = "/nodehub.Gateway/IsSessionExist"
	Gateway_SessionCount_FullMethodName        = "/nodehub.Gateway/SessionCount"
	Gateway_CloseSession_FullMethodName        = "/nodehub.Gateway/CloseSession"
	Gateway_SetServiceRoute_FullMethodName     = "/nodehub.Gateway/SetServiceRoute"
	Gateway_RemoveServiceRoute_FullMethodName  = "/nodehub.Gateway/RemoveServiceRoute"
	Gateway_ReplaceServiceRoute_FullMethodName = "/nodehub.Gateway/ReplaceServiceRoute"
	Gateway_PushMessage_FullMethodName         = "/nodehub.Gateway/PushMessage"
)

// GatewayClient is the client API for Gateway service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type GatewayClient interface {
	// 会话是否存在
	IsSessionExist(ctx context.Context, in *IsSessionExistRequest, opts ...grpc.CallOption) (*IsSessionExistResponse, error)
	// 会话数量
	SessionCount(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*SessionCountResponse, error)
	// 关闭会话连接，踢下线
	CloseSession(ctx context.Context, in *CloseSessionRequest, opts ...grpc.CallOption) (*CloseSessionResponse, error)
	// 修改状态服务路由
	SetServiceRoute(ctx context.Context, in *SetServiceRouteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// 删除状态服务路由
	RemoveServiceRoute(ctx context.Context, in *RemoveServiceRouteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// 替换状态服务路由节点
	ReplaceServiceRoute(ctx context.Context, in *ReplaceServiceRouteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// 向指定会话推送消息
	PushMessage(ctx context.Context, in *PushMessageRequest, opts ...grpc.CallOption) (*PushMessageResponse, error)
}

type gatewayClient struct {
	cc grpc.ClientConnInterface
}

func NewGatewayClient(cc grpc.ClientConnInterface) GatewayClient {
	return &gatewayClient{cc}
}

func (c *gatewayClient) IsSessionExist(ctx context.Context, in *IsSessionExistRequest, opts ...grpc.CallOption) (*IsSessionExistResponse, error) {
	out := new(IsSessionExistResponse)
	err := c.cc.Invoke(ctx, Gateway_IsSessionExist_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gatewayClient) SessionCount(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*SessionCountResponse, error) {
	out := new(SessionCountResponse)
	err := c.cc.Invoke(ctx, Gateway_SessionCount_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gatewayClient) CloseSession(ctx context.Context, in *CloseSessionRequest, opts ...grpc.CallOption) (*CloseSessionResponse, error) {
	out := new(CloseSessionResponse)
	err := c.cc.Invoke(ctx, Gateway_CloseSession_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gatewayClient) SetServiceRoute(ctx context.Context, in *SetServiceRouteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, Gateway_SetServiceRoute_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gatewayClient) RemoveServiceRoute(ctx context.Context, in *RemoveServiceRouteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, Gateway_RemoveServiceRoute_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gatewayClient) ReplaceServiceRoute(ctx context.Context, in *ReplaceServiceRouteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, Gateway_ReplaceServiceRoute_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gatewayClient) PushMessage(ctx context.Context, in *PushMessageRequest, opts ...grpc.CallOption) (*PushMessageResponse, error) {
	out := new(PushMessageResponse)
	err := c.cc.Invoke(ctx, Gateway_PushMessage_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GatewayServer is the server API for Gateway service.
// All implementations must embed UnimplementedGatewayServer
// for forward compatibility
type GatewayServer interface {
	// 会话是否存在
	IsSessionExist(context.Context, *IsSessionExistRequest) (*IsSessionExistResponse, error)
	// 会话数量
	SessionCount(context.Context, *emptypb.Empty) (*SessionCountResponse, error)
	// 关闭会话连接，踢下线
	CloseSession(context.Context, *CloseSessionRequest) (*CloseSessionResponse, error)
	// 修改状态服务路由
	SetServiceRoute(context.Context, *SetServiceRouteRequest) (*emptypb.Empty, error)
	// 删除状态服务路由
	RemoveServiceRoute(context.Context, *RemoveServiceRouteRequest) (*emptypb.Empty, error)
	// 替换状态服务路由节点
	ReplaceServiceRoute(context.Context, *ReplaceServiceRouteRequest) (*emptypb.Empty, error)
	// 向指定会话推送消息
	PushMessage(context.Context, *PushMessageRequest) (*PushMessageResponse, error)
	mustEmbedUnimplementedGatewayServer()
}

// UnimplementedGatewayServer must be embedded to have forward compatible implementations.
type UnimplementedGatewayServer struct {
}

func (UnimplementedGatewayServer) IsSessionExist(context.Context, *IsSessionExistRequest) (*IsSessionExistResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method IsSessionExist not implemented")
}
func (UnimplementedGatewayServer) SessionCount(context.Context, *emptypb.Empty) (*SessionCountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SessionCount not implemented")
}
func (UnimplementedGatewayServer) CloseSession(context.Context, *CloseSessionRequest) (*CloseSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CloseSession not implemented")
}
func (UnimplementedGatewayServer) SetServiceRoute(context.Context, *SetServiceRouteRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetServiceRoute not implemented")
}
func (UnimplementedGatewayServer) RemoveServiceRoute(context.Context, *RemoveServiceRouteRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveServiceRoute not implemented")
}
func (UnimplementedGatewayServer) ReplaceServiceRoute(context.Context, *ReplaceServiceRouteRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReplaceServiceRoute not implemented")
}
func (UnimplementedGatewayServer) PushMessage(context.Context, *PushMessageRequest) (*PushMessageResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PushMessage not implemented")
}
func (UnimplementedGatewayServer) mustEmbedUnimplementedGatewayServer() {}

// UnsafeGatewayServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to GatewayServer will
// result in compilation errors.
type UnsafeGatewayServer interface {
	mustEmbedUnimplementedGatewayServer()
}

func RegisterGatewayServer(s grpc.ServiceRegistrar, srv GatewayServer) {
	s.RegisterService(&Gateway_ServiceDesc, srv)
}

func _Gateway_IsSessionExist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(IsSessionExistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GatewayServer).IsSessionExist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Gateway_IsSessionExist_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GatewayServer).IsSessionExist(ctx, req.(*IsSessionExistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Gateway_SessionCount_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GatewayServer).SessionCount(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Gateway_SessionCount_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GatewayServer).SessionCount(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Gateway_CloseSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CloseSessionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GatewayServer).CloseSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Gateway_CloseSession_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GatewayServer).CloseSession(ctx, req.(*CloseSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Gateway_SetServiceRoute_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetServiceRouteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GatewayServer).SetServiceRoute(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Gateway_SetServiceRoute_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GatewayServer).SetServiceRoute(ctx, req.(*SetServiceRouteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Gateway_RemoveServiceRoute_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveServiceRouteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GatewayServer).RemoveServiceRoute(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Gateway_RemoveServiceRoute_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GatewayServer).RemoveServiceRoute(ctx, req.(*RemoveServiceRouteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Gateway_ReplaceServiceRoute_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReplaceServiceRouteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GatewayServer).ReplaceServiceRoute(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Gateway_ReplaceServiceRoute_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GatewayServer).ReplaceServiceRoute(ctx, req.(*ReplaceServiceRouteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Gateway_PushMessage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PushMessageRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GatewayServer).PushMessage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Gateway_PushMessage_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GatewayServer).PushMessage(ctx, req.(*PushMessageRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Gateway_ServiceDesc is the grpc.ServiceDesc for Gateway service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Gateway_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "nodehub.Gateway",
	HandlerType: (*GatewayServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "IsSessionExist",
			Handler:    _Gateway_IsSessionExist_Handler,
		},
		{
			MethodName: "SessionCount",
			Handler:    _Gateway_SessionCount_Handler,
		},
		{
			MethodName: "CloseSession",
			Handler:    _Gateway_CloseSession_Handler,
		},
		{
			MethodName: "SetServiceRoute",
			Handler:    _Gateway_SetServiceRoute_Handler,
		},
		{
			MethodName: "RemoveServiceRoute",
			Handler:    _Gateway_RemoveServiceRoute_Handler,
		},
		{
			MethodName: "ReplaceServiceRoute",
			Handler:    _Gateway_ReplaceServiceRoute_Handler,
		},
		{
			MethodName: "PushMessage",
			Handler:    _Gateway_PushMessage_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "nodehub/gateway_service.proto",
}
