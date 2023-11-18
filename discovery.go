package nodehub

import clientv3 "go.etcd.io/etcd/client/v3"

// NodeEntry 节点服务发现条目
type NodeEntry struct {
	// 节点ID，集群内唯一，会被用于有状态服务路由
	ID string `json:"id"`

	// 节点名称，仅用于显示
	Name string `json:"name"`

	// grpc服务信息
	GRPC GRPCEntry `json:"grpc"`

	// git版本
	GitVersion string `json:"git_version,omitempty"`
}

// GRPCEntry gRPC服务发现条目
type GRPCEntry struct {
	// 监听地址 host:port
	Endpoint string `json:"endpoint"`

	// grpc服务列表
	Services []GRPCService `json:"services"`
}

// GRPCService gRPC服务
type GRPCService struct {
	// 服务代码枚举值，每个服务的代码值必须唯一
	//
	// 网关会根据客户端请求消息内的service_code字段，将请求转发到对应的服务
	Code int32 `json:"code"`

	// 服务路径，网关在构造grpc请求时，用于方法地址构造
	//
	// example: /helloworld.Greeter
	Path string `json:"path"`

	// 是否有状态服务
	Stateful bool `json:"stateful"`

	// 是否允许客户端访问
	Public bool `json:"public"`

	// 是否允许匿名访问
	AllowAnonymous bool `json:"allow_anonymous"`
}

// Registry 服务注册表
type Registry struct {
	client    *clientv3.Client
	keyPrefix string
}

// Register 注册服务
func (r *Registry) Register(entry NodeEntry) error {
	return nil
}
