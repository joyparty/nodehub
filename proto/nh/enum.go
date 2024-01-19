package nh

const (
	// NodeServiceCode 节点管理服务，每个内部节点都会内置这个grpc服务
	NodeServiceCode int32 = -1
	// GatewayServiceCode 网关服务代码，每个网关节点都会内置这个grpc服务
	GatewayServiceCode int32 = -2
)
