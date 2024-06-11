package cluster

import (
	"errors"

	"github.com/oklog/ulid/v2"
)

// NodeState 节点状态
type NodeState string

const (
	// NodeOK 正常
	NodeOK NodeState = "ok"
	// NodeLazy 不接受新的请求
	NodeLazy NodeState = "lazy"
	// NodeDown 下线，不接受任何请求
	NodeDown NodeState = "down"

	// AutoAllocate 自动分配，第一次请求时，如果还没有分配，会根据负载均衡策略自动选择一个可用节点
	AutoAllocate = "auto"
	// ServerAllocate 服务端分配，只有服务器端分配好节点之后，客户端才能够访问
	ServerAllocate = "server"
	// ClientAllocate 客户端分配，客户端请求时附带的nodeID会被记录下来，后续即使不指定nodeID，也会被分配到同一个节点
	ClientAllocate = "client"
)

// NodeEntry 节点服务发现条目
type NodeEntry struct {
	// 节点ID，集群内唯一
	ID ulid.ULID `json:"id"`

	// 节点名称，仅用于显示
	Name string `json:"name"`

	// 节点状态
	State NodeState `json:"state"`

	// 网关入口URL
	//
	// Example:
	//	 - tcp://0.0.0.0:8222
	//	 - ws://0.0.0.0:8222/grpc
	Entrance string `json:"entrance,omitempty"`

	// grpc服务信息
	GRPC GRPCEntry `json:"grpc"`

	// git版本
	GitVersion string `json:"git_version,omitempty"`
}

// Validate 验证条目是否合法
func (e NodeEntry) Validate() error {
	if e.ID.Time() == 0 {
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

	for _, desc := range e.Services {
		if err := desc.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// GRPCServiceDesc gRPC服务
type GRPCServiceDesc struct {
	// 服务名称
	//
	// example: helloworld.Greeter
	Name string `json:"name"`

	// 服务代码枚举值，每个服务的代码值必须唯一
	//
	// 网关会根据客户端请求消息内的service_code字段，将请求转发到对应的服务
	Code int32 `json:"code"`

	// 服务路径，网关在构造grpc请求时，用于方法地址构造
	//
	// example: /helloworld.Greeter
	Path string `json:"path"`

	// 是否允许客户端访问
	Public bool `json:"public,omitempty"`

	// 负载均衡策略
	Balancer string `json:"balancer"`

	// Weight 节点权重，用于负载均衡
	Weight int `json:"weight,omitempty"`

	// Stateful 是否有状态服务
	Stateful bool `json:"stateful,omitempty"`

	// Allocation 有状态节点分配方式
	Allocation string `json:"allocation,omitempty"`
}

// Validate 验证条目是否合法
func (desc GRPCServiceDesc) Validate() error {
	if desc.Code == 0 {
		return errors.New("code is empty")
	} else if desc.Path == "" {
		return errors.New("path is empty")
	} else if desc.Balancer == "" {
		return errors.New("balancer is empty")
	}

	if desc.Stateful {
		switch desc.Allocation {
		case AutoAllocate, ServerAllocate, ClientAllocate:
		case "":
			return errors.New("allocation is empty")
		default:
			return errors.New("allocation is invalid")
		}
	}

	return nil
}
