package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"nodehub/logger"
	"path"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

var (
	// ErrNoNodeOrDown 没有可用节点或节点已下线
	ErrNoNodeOrDown = errors.New("no node or node is down")
	// ErrNoNodeAvailable 没有可用节点
	ErrNoNodeAvailable = errors.New("no node available")
	// ErrGRPCServiceCode grpc服务代码未找到
	ErrGRPCServiceCode = errors.New("grpc service code not found")
)

// Registry 服务注册表
type Registry struct {
	client       *clientv3.Client
	keyPrefix    string
	grpcResolver *grpcResolver

	leaseID clientv3.LeaseID
}

// NewRegistry 创建服务注册表
func NewRegistry(client *clientv3.Client, opt ...func(*Registry)) (*Registry, error) {
	r := &Registry{
		client:       client,
		keyPrefix:    "/nodehub/node",
		grpcResolver: newGRPCResolver(),
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

// 向etcd生成一个10秒过期的租约
func (r *Registry) runKeeper() error {
	lease, err := r.client.Grant(r.client.Ctx(), 10) // 10 seconds
	if err != nil {
		return fmt.Errorf("grant lease, %w", err)
	}
	r.leaseID = lease.ID

	// 心跳维持
	go func() {
		ch, err := r.client.KeepAlive(r.client.Ctx(), r.leaseID)
		if err != nil {
			logger.Error("keep lease alive", "error", err)
		} else {
			for resp := range ch {
				select {
				case <-r.client.Ctx().Done():
					return
				default:
					_ = resp
				}
			}
		}

		panic(errors.New("lease keeper closed"))
	}()

	return nil
}

// 监听服务条目变更
func (r *Registry) runWatcher() {
	updateGRPCResolver := func(event mvccpb.Event_EventType, value []byte) {
		var entry NodeEntry
		if err := json.Unmarshal(value, &entry); err != nil {
			logger.Error("unmarshal entry", "error", err)
			return
		}

		switch event {
		case mvccpb.PUT:
			r.grpcResolver.Update(entry)
		case mvccpb.DELETE:
			r.grpcResolver.Remove(entry)
		}
	}

	// 处理已有条目
	resp, err := r.client.Get(r.client.Ctx(), r.keyPrefix, clientv3.WithPrefix())
	if err != nil {
		logger.Error("get exist entries", "error", err)
	}
	for _, kv := range resp.Kvs {
		updateGRPCResolver(mvccpb.PUT, kv.Value)
	}

	// 监听变更
	wCh := r.client.Watch(r.client.Ctx(), r.keyPrefix, clientv3.WithPrefix(), clientv3.WithPrevKV())
	for {
		select {
		case <-r.client.Ctx().Done():
			return
		case wResp := <-wCh:
			for _, ev := range wResp.Events {
				switch ev.Type {
				case mvccpb.PUT:
					updateGRPCResolver(ev.Type, ev.Kv.Value)
				case mvccpb.DELETE:
					updateGRPCResolver(ev.Type, ev.PrevKv.Value)
				default:
					logger.Error("unknown event type", "type", ev.Type)
				}
			}
		}
	}
}

// GetGRPCServiceConn 获取grpc服务连接
func (r *Registry) GetGRPCServiceConn(serviceCode int32) (conn *grpc.ClientConn, desc GRPCServiceDesc, err error) {
	return r.grpcResolver.GetServiceConn(serviceCode)
}

// GetGRPCNodeConn 获取指定节点的grpc服务连接
func (r *Registry) GetGRPCNodeConn(nodeID string, serviceCode int32) (conn *grpc.ClientConn, desc GRPCServiceDesc, err error) {
	return r.grpcResolver.GetNodeConn(nodeID, serviceCode)
}

// Close 关闭
func (r *Registry) Close() {
	r.grpcResolver.Close()
	r.client.Close()
}

// WithKeyPrefix 设置服务条目key前缀
func WithKeyPrefix(prefix string) func(*Registry) {
	return func(r *Registry) {
		r.keyPrefix = prefix
	}
}

// WithGRPCDialOptions 设置grpc.DialOption
func WithGRPCDialOptions(options ...grpc.DialOption) func(*Registry) {
	return func(r *Registry) {
		r.grpcResolver = newGRPCResolver(options...)
	}
}
