package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"

	"github.com/joyparty/gokit"
	"github.com/oklog/ulid/v2"
	"github.com/reactivex/rxgo/v2"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"gitlab.haochang.tv/gopkg/nodehub/proto/nh"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

var (
	// ErrNodeNotFoundOrDown 没有可用节点或节点已下线
	ErrNodeNotFoundOrDown = errors.New("node not found or down")
	// ErrNoNodeAvailable 没有可用节点
	ErrNoNodeAvailable = errors.New("no node available")
)

// Registry 服务注册表
type Registry struct {
	client       *clientv3.Client
	keyPrefix    string
	grpcResolver *grpcResolver

	leaseID  clientv3.LeaseID
	allNodes *gokit.MapOf[ulid.ULID, NodeEntry]

	observable rxgo.Observable
}

// NewRegistry 创建服务注册表
func NewRegistry(client *clientv3.Client, opt ...func(*Registry)) (*Registry, error) {
	r := &Registry{
		client:       client,
		keyPrefix:    "/nodehub/node",
		grpcResolver: newGRPCResolver(),
		allNodes:     gokit.NewMapOf[ulid.ULID, NodeEntry](),
	}

	for _, fn := range opt {
		fn(r)
	}

	if err := r.runKeeper(); err != nil {
		return nil, fmt.Errorf("run keeper, %w", err)
	}

	events := r.runWatcher()
	r.observable = rxgo.FromEventSource(events, rxgo.WithBackPressureStrategy(rxgo.Drop))

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

	key := path.Join(r.keyPrefix, entry.ID.String())
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
			for {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
				case <-r.client.Ctx().Done():
					return
				}
			}
		}

		panic(errors.New("lease keeper closed"))
	}()

	return nil
}

// 监听服务条目变更
func (r *Registry) runWatcher() <-chan rxgo.Item {
	events := make(chan rxgo.Item)

	go func() {
		defer close(events)

		updateNodes := func(event mvccpb.Event_EventType, value []byte) {
			var entry NodeEntry
			if err := json.Unmarshal(value, &entry); err != nil {
				logger.Error("unmarshal entry", "error", err)
				return
			}

			logger.Info("update cluster nodes", "event", event.String(), "entry", entry)

			switch event {
			case mvccpb.PUT:
				r.grpcResolver.Update(entry)
				r.allNodes.Store(entry.ID, entry)

				events <- rxgo.Of(eventUpdateNode{Entry: entry})
			case mvccpb.DELETE:
				r.grpcResolver.Remove(entry)
				r.allNodes.Delete(entry.ID)

				events <- rxgo.Of(eventDeleteNode{Entry: entry})
			}
		}

		// 处理已有条目
		resp, err := r.client.Get(r.client.Ctx(), r.keyPrefix, clientv3.WithPrefix())
		if err != nil {
			logger.Error("get exist entries", "error", err)
		}
		for _, kv := range resp.Kvs {
			updateNodes(mvccpb.PUT, kv.Value)
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
						updateNodes(ev.Type, ev.Kv.Value)
					case mvccpb.DELETE:
						updateNodes(ev.Type, ev.PrevKv.Value)
					default:
						logger.Error("unknown event type", "type", ev.Type)
					}
				}
			}
		}
	}()

	return events
}

// GetGRPCDesc 获取grpc服务描述
func (r *Registry) GetGRPCDesc(serviceCode int32) (GRPCServiceDesc, bool) {
	return r.grpcResolver.GetDesc(serviceCode)
}

// PickGRPCNode 随机选择一个可用GRPC服务节点
func (r *Registry) PickGRPCNode(serviceCode int32, sess Session) (nodeID ulid.ULID, err error) {
	return r.grpcResolver.PickNode(serviceCode, sess)
}

// GetGRPCConn 获取指定节点的grpc连接
func (r *Registry) GetGRPCConn(nodeID ulid.ULID) (conn *grpc.ClientConn, err error) {
	return r.grpcResolver.GetConn(nodeID)
}

// GetGatewayClient 获取网关grpc服务客户端
func (r *Registry) GetGatewayClient(nodeID ulid.ULID) (nh.GatewayClient, error) {
	entry, ok := r.allNodes.Load(nodeID)
	if !ok {
		return nil, ErrNodeNotFoundOrDown
	}

	conn, err := r.grpcResolver.getConn(entry.GRPC.Endpoint)
	if err != nil {
		return nil, err
	}

	return nh.NewGatewayClient(conn), nil
}

// GetNodeClient 获取节点grpc服务客户端
func (r *Registry) GetNodeClient(nodeID ulid.ULID) (nh.NodeClient, error) {
	entry, ok := r.allNodes.Load(nodeID)
	if !ok {
		return nil, ErrNodeNotFoundOrDown
	}

	conn, err := r.grpcResolver.getConn(entry.GRPC.Endpoint)
	if err != nil {
		return nil, err
	}

	return nh.NewNodeClient(conn), nil
}

// ForeachNodes 遍历所有节点
//
// 如果f返回false，则停止遍历
func (r *Registry) ForeachNodes(f func(NodeEntry) bool) {
	r.allNodes.Range(func(_ ulid.ULID, v NodeEntry) bool {
		return f(v)
	})
}

// SubscribeUpdate 订阅节点更新
func (r *Registry) SubscribeUpdate(handler func(entry NodeEntry)) {
	r.observable.
		DoOnNext(func(item any) {
			if ev, ok := item.(eventUpdateNode); ok {
				handler(ev.Entry)
			}
		})
}

// SubscribeDelete 订阅节点删除
func (r *Registry) SubscribeDelete(handler func(entry NodeEntry)) {
	r.observable.
		DoOnNext(func(item any) {
			if ev, ok := item.(eventDeleteNode); ok {
				handler(ev.Entry)
			}
		})
}

// Close 关闭
func (r *Registry) Close() {
	if r.leaseID != clientv3.NoLease {
		r.client.Revoke(r.client.Ctx(), r.leaseID)
	}
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
