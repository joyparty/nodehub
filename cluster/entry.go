package cluster

import (
	"errors"
)

// NodeState 节点状态
type NodeState string

const (
	// NodeOK 正常
	NodeOK NodeState = "ok"
	// NodeLazy 只接收指定了节点ID的请求
	NodeLazy NodeState = "lazy"
	// NodeDown 下线，不接受任何请求
	NodeDown NodeState = "down"

	// AutoAllocate 自动分配
	AutoAllocate = "auto"
	// ExplicitAllocate 显式分配
	ExplicitAllocate = "explicit"
)

// NodeEntry 节点服务发现条目
type NodeEntry struct {
	// 节点ID，集群内唯一
	ID string `json:"id"`

	// 节点名称，仅用于显示
	Name string `json:"name"`

	// 节点状态
	State NodeState `json:"state"`

	// websocket连接地址
	Websocket string `json:"websocket,omitempty"`

	// grpc服务信息
	GRPC GRPCEntry `json:"grpc"`

	// git版本
	GitVersion string `json:"git_version,omitempty"`
}

// Validate 验证条目是否合法
func (e NodeEntry) Validate() error {
	if e.ID == "" {
		return errors.New("id is empty")
	} else if e.Name == "" {
		return errors.New("name is empty")
	} else if e.State == "" {
		return errors.New("state is empty")
	} else if err := e.GRPC.Validate(); err != nil {
		return err
	}
	return nil
}

// GRPCEntry gRPC服务发现条目
type GRPCEntry struct {
	// 监听地址 host:port
	Endpoint string `json:"endpoint"`

	// grpc服务列表
	Services []GRPCServiceDesc `json:"services"`
}

// Validate 验证条目是否合法
func (e GRPCEntry) Validate() error {
	if len(e.Services) == 0 {
		return nil
	} else if e.Endpoint == "" {
		return errors.New("grpc endpoint is empty")
	}

	for _, service := range e.Services {
		if err := service.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// GRPCServiceDesc gRPC服务
type GRPCServiceDesc struct {
	// 服务代码枚举值，每个服务的代码值必须唯一
	//
	// 网关会根据客户端请求消息内的service_code字段，将请求转发到对应的服务
	Code int32 `json:"code"`

	// 服务路径，网关在构造grpc请求时，用于方法地址构造
	//
	// example: /helloworld.Greeter
	Path string `json:"path"`

	// 是否允许客户端访问
	Public bool `json:"public"`

	// Stateful 是否有状态服务
	Stateful bool

	// Allocation 有状态节点分配方式
	//
	//  - auto: 自动分配，第一次请求时，如果还没有分配，会根据负载均衡策略自动选择一个可用节点
	//  - explicit: 显式分配，只有分配了节点之后，客户端才能够访问，没有分配就无法访问
	Allocation string
}

// Validate 验证条目是否合法
func (desc GRPCServiceDesc) Validate() error {
	if desc.Code == 0 {
		return errors.New("code is empty")
	} else if desc.Path == "" {
		return errors.New("path is empty")
	}

	if desc.Stateful {
		switch desc.Allocation {
		case AutoAllocate, ExplicitAllocate:
		case "":
			return errors.New("allocation is empty")
		default:
			return errors.New("allocation is invalid")
		}
	}

	return nil
}
