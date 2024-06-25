package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/joyparty/gokit"
	"github.com/oklog/ulid/v2"
	"github.com/reactivex/rxgo/v2"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"

	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
)

var (
	// ErrNodeNotFoundOrDown 没有可用节点或节点已下线
	ErrNodeNotFoundOrDown = errors.New("node not found or down")
	// ErrNoNodeAvailable 没有可用节点
	ErrNoNodeAvailable = errors.New("no node available")
)

// Registry 服务注册表
type Registry struct {
	mutex sync.Mutex

	client       *clientv3.Client
	keyPrefix    string
	grpcResolver *grpcResolver

	leaseID  clientv3.LeaseID
	leaseTTL int64
	allNodes *gokit.MapOf[ulid.ULID, NodeEntry]

	observable rxgo.Observable
}

// NewRegistry 创建服务注册表
func NewRegistry(client *clientv3.Client, opt ...func(*Registry)) (*Registry, error) {
	r := &Registry{
		client:       client,
		keyPrefix:    "/nodehub/node",
		grpcResolver: newGRPCResolver(),
		leaseTTL:     10,
		allNodes:     gokit.NewMapOf[ulid.ULID, NodeEntry](),
	}

	for _, fn := range opt {
		fn(r)
	}

	if err := r.runWatcher(); err != nil {
		return nil, fmt.Errorf("run watcher, %w", err)
	}

	return r, nil
}

// Put 注册服务
func (r *Registry) Put(entry NodeEntry) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("validate entry, %w", err)
	}

	value, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry, %w", err)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	ctx, cancel := context.WithTimeout(r.client.Ctx(), 5*time.Second)
	defer cancel()

	if err := r.initLease(ctx); err != nil {
		return fmt.Errorf("run keeper, %w", err)
	}

	key := path.Join(r.keyPrefix, entry.ID.String())
	_, err = r.client.Put(ctx, key, string(value), clientv3.WithLease(r.leaseID))
	return err
}

// 向etcd生成一个10秒过期的租约
func (r *Registry) initLease(ctx context.Context) error {
	if r.leaseID != clientv3.NoLease {
		return nil
	}

	lease, err := r.client.Grant(ctx, r.leaseTTL)
	if err != nil {
		return fmt.Errorf("grant lease, %w", err)
	}

	ch, err := r.client.KeepAlive(r.client.Ctx(), lease.ID)
	if err != nil {
		return fmt.Errorf("keep lease alive, %w", err)
	}
	r.leaseID = lease.ID

	// 心跳维持
	go func() {
	Loop:
		for {
			select {
			case v, ok := <-ch:
				if !ok {
					return
				} else if v == nil { // etcd down
					break Loop
				}
			case <-r.client.Ctx().Done():
				return
			}
		}

		logger.Error("lease keeper closed")
		panic(errors.New("lease keeper closed"))
	}()

	return nil
}

// 监听服务条目变更
func (r *Registry) runWatcher() error {
	events := make(chan rxgo.Item)
	r.observable = rxgo.FromEventSource(events, rxgo.WithBackPressureStrategy(rxgo.Drop))

	var (
		lock     sync.Mutex
		versions = map[string]int64{} // etcd条目版本号
	)
	updateNodes := func(event mvccpb.Event_EventType, kv *mvccpb.KeyValue) {
		lock.Lock()
		defer lock.Unlock()

		// DELETE事件不检查版本号信息
		if event == mvccpb.PUT {
			key := string(kv.Key)
			if ver, ok := versions[key]; ok && ver >= kv.Version {
				return
			}
			versions[key] = kv.Version
		}

		var entry NodeEntry
		if err := json.Unmarshal(kv.Value, &entry); err != nil {
			logger.Error("unmarshal entry", "error", err)
			return
		}

		logger.Info("update cluster nodes", "event", event.String(), "entry", entry)

		defer func() {
			vals := []any{}
			for k, v := range r.DumpGRPCResolver() {
				vals = append(vals, k, v)
			}

			logger.Debug("grpc resolver data", vals...)
		}()

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

	// 监听变更
	go func() {
		defer close(events)

		wCh := r.client.Watch(r.client.Ctx(), r.keyPrefix, clientv3.WithPrefix(), clientv3.WithPrevKV())
		for {
			select {
			case <-r.client.Ctx().Done():
				return
			case wResp := <-wCh:
				for _, ev := range wResp.Events {
					switch ev.Type {
					case mvccpb.PUT:
						updateNodes(ev.Type, ev.Kv)
					case mvccpb.DELETE:
						updateNodes(ev.Type, ev.PrevKv)
					default:
						logger.Error("unknown event type", "type", ev.Type)
					}
				}
			}
		}
	}()

	// 处理已有条目
	updateExists := func() error {
		ctx, cancel := context.WithTimeout(r.client.Ctx(), 5*time.Second)
		defer cancel()

		resp, err := r.client.Get(ctx, r.keyPrefix, clientv3.WithPrefix())
		if err != nil {
			return fmt.Errorf("get exist entries, %w", err)
		}
		for _, kv := range resp.Kvs {
			updateNodes(mvccpb.PUT, kv)
		}
		return nil
	}

	// 延迟3秒再全量同步一次
	go func() {
		time.Sleep(3 * time.Second)
		updateExists()
	}()

	return updateExists()
}

// GetGRPCDesc 获取grpc服务描述
func (r *Registry) GetGRPCDesc(serviceCode int32) (GRPCServiceDesc, bool) {
	return r.grpcResolver.GetDesc(serviceCode)
}

// AllocGRPCNode 根据负载均衡策略给客户端会话分配可用节点
func (r *Registry) AllocGRPCNode(serviceCode int32, sess Session) (nodeID ulid.ULID, err error) {
	return r.grpcResolver.AllocNode(serviceCode, sess)
}

// PickGRPCNode 随机选择一个可用节点
func (r *Registry) PickGRPCNode(serviceCode int32) (nodeID ulid.ULID, err error) {
	return r.grpcResolver.PickNode(serviceCode)
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

// DumpGRPCResolver 导出grpc服务解析器数据
func (r *Registry) DumpGRPCResolver() map[string]any {
	return r.grpcResolver.DumpData()
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

// WithLeaseTTL 服务心跳超时，超过此时长未检测到服务心跳，即表明服务离线
//
// 单位 秒，默认10秒
func WithLeaseTTL(seconds int) func(*Registry) {
	return func(r *Registry) {
		r.leaseTTL = int64(seconds)
	}
}
