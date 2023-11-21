package nodehub

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// RegistryEvent 注册表事件
type RegistryEvent int32

const (
	// PUT 新增或更新
	PUT RegistryEvent = RegistryEvent(mvccpb.PUT)
	// DELETE 删除
	DELETE RegistryEvent = RegistryEvent(mvccpb.DELETE)
)

// NodeEntry 节点服务发现条目
type NodeEntry struct {
	// 节点ID，集群内唯一，会被用于有状态服务路由
	ID string `json:"id"`

	// 节点名称，仅用于显示
	Name string `json:"name"`

	// 节点状态
	State string `json:"state"`

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
	}
	return nil
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
	client       *clientv3.Client
	keyPrefix    string
	eventHandler func(event RegistryEvent, entry NodeEntry)

	leaseID clientv3.LeaseID
}

// NewRegistry 创建服务注册表
func NewRegistry(client *clientv3.Client, opt ...func(*Registry)) (*Registry, error) {
	r := &Registry{
		client:    client,
		keyPrefix: "/nodehub/node",
		eventHandler: func(event RegistryEvent, entry NodeEntry) {
			// do nothing
		},
	}

	for _, fn := range opt {
		fn(r)
	}

	if err := r.runKeeper(); err != nil {
		return nil, fmt.Errorf("run keeper, %w", err)
	}

	go r.runWatcher()
	return r, nil
}

// Put 注册服务
func (r *Registry) Put(entry NodeEntry) error {
	if r.leaseID == clientv3.NoLease {
		return errors.New("lease not granted")
	} else if err := entry.Validate(); err != nil {
		return fmt.Errorf("validate entry, %w", err)
	}

	value, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry, %w", err)
	}

	key := path.Join(r.keyPrefix, entry.ID)
	_, err = r.client.Put(r.client.Ctx(), key, string(value), clientv3.WithLease(r.leaseID))
	return err
}

// 向etcd生成一个10秒过期的租约，每5秒续约一次
// 租约过期后，etcd会自动删除对应的key
func (r *Registry) runKeeper() error {
	lease, err := r.client.Grant(r.client.Ctx(), 10)
	if err != nil {
		return fmt.Errorf("grant lease, %w", err)
	}
	r.leaseID = lease.ID

	go func() {
		for {
			ch, err := r.client.KeepAlive(r.client.Ctx(), r.leaseID)
			if err != nil {
				// TODO: 打日志
			} else {
				for resp := range ch {
					_ = resp
				}
			}

			time.Sleep(5 * time.Second)
		}
	}()

	return nil
}

// 监听服务条目变更
func (r *Registry) runWatcher() {
	wCh := r.client.Watch(r.client.Ctx(), r.keyPrefix, clientv3.WithPrefix(), clientv3.WithPrevKV())

	for wResp := range wCh {
		for _, ev := range wResp.Events {
			var (
				entry NodeEntry
				value []byte
			)
			switch ev.Type {
			case mvccpb.PUT:
				value = ev.Kv.Value
			case mvccpb.DELETE:
				value = ev.PrevKv.Value
			default:
				panic(fmt.Errorf("unknown event type: %v", ev.Type))
			}

			if err := json.Unmarshal(value, &entry); err != nil {
				panic(fmt.Errorf("unmarshal entry, %w", err))
			}
			r.eventHandler(RegistryEvent(ev.Type), entry)
		}
	}

	// watcher永远不该退出
	panic(errors.New("watcher closed"))
}

// WithKeyPrefix 设置服务条目key前缀
func WithKeyPrefix(prefix string) func(*Registry) {
	return func(r *Registry) {
		r.keyPrefix = prefix
	}
}

// WithEventHandler 设置服务条目变更事件处理函数
func WithEventHandler(fn func(event RegistryEvent, entry NodeEntry)) func(*Registry) {
	return func(r *Registry) {
		r.eventHandler = fn
	}
}
