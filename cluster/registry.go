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

	watchRetryMax      int           // watch失败后的最大重试次数
	watchRetryInterval time.Duration // watch失败后的重试间隔

	allNodes *gokit.MapOf[ulid.ULID, NodeEntry]

	observable rxgo.Observable
}

// NewRegistry 创建服务注册表
func NewRegistry(client *clientv3.Client, opt ...func(*Registry)) (*Registry, error) {
	r := &Registry{
		client:             client,
		keyPrefix:          "/nodehub/node",
		grpcResolver:       newGRPCResolver(),
		leaseTTL:           10,
		watchRetryMax:      10,
		watchRetryInterval: 30 * time.Second,
		allNodes:           gokit.NewMapOf[ulid.ULID, NodeEntry](),
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
		tk := time.NewTicker(30 * time.Second)
		defer tk.Stop()

		for {
			select {
			case _, ok := <-ch:
				if !ok { // etcd down or disconnect
					logger.Error("etcd lease keeper closed")
					panic(errors.New("etcd lease keeper closed"))
				}
			case <-tk.C:
				resp, err := r.client.TimeToLive(r.client.Ctx(), lease.ID)
				if err != nil {
					logger.Error("check lease ttl", "error", err)
				} else if resp.TTL < 0 {
					logger.Error("etcd lease expired")
					panic(errors.New("etcd lease expired"))
				}
			case <-r.client.Ctx().Done():
				return
			}
		}
	}()

	return nil
}

// 监听服务条目变更
func (r *Registry) runWatcher() error {
	events := make(chan rxgo.Item)
	r.observable = rxgo.FromEventSource(events, rxgo.WithBackPressureStrategy(rxgo.Drop))

	var (
		lock     sync.Mutex
		versions = map[string]int64{} // 使用 ModRevision，键删除重建后 Version 会重置
		maxRev   int64                // 已处理的最大revision，用于检测etcd revision回退
	)

	removeNode := func(entry NodeEntry) {
		r.grpcResolver.Remove(entry)
		r.allNodes.Delete(entry.ID)

		events <- rxgo.Of(eventDeleteNode{Entry: entry})
	}

	// apply按ModRevision去重后更新节点数据，调用者必须持有lock
	apply := func(ev *clientv3.Event) {
		if ev.Kv.ModRevision > maxRev {
			maxRev = ev.Kv.ModRevision
		}

		key := string(ev.Kv.Key)
		if ver, ok := versions[key]; ok && ver >= ev.Kv.ModRevision {
			return
		}
		versions[key] = ev.Kv.ModRevision

		kv := ev.Kv
		if ev.Type == mvccpb.DELETE {
			// 版本号用ev.Kv（删除revision），条目数据用PrevKv
			kv = ev.PrevKv
		}

		var entry NodeEntry
		if err := json.Unmarshal(kv.Value, &entry); err != nil {
			logger.Error("unmarshal entry", "error", err)
			return
		}

		logger.Info("update cluster nodes", "event", ev.Type.String(), "entry", entry)

		defer func() {
			vals := []any{}
			for k, v := range r.DumpGRPCResolver() {
				vals = append(vals, k, v)
			}

			logger.Debug("grpc resolver data", vals...)
		}()

		switch ev.Type {
		case mvccpb.PUT:
			r.grpcResolver.Update(entry)
			r.allNodes.Store(entry.ID, entry)

			events <- rxgo.Of(eventUpdateNode{Entry: entry})
		case mvccpb.DELETE:
			removeNode(entry)
		}
	}

	// 全量同步，删除etcd中已不存在的节点
	resync := func() error {
		lock.Lock()
		defer lock.Unlock()

		ctx, cancel := context.WithTimeout(r.client.Ctx(), 5*time.Second)
		defer cancel()

		resp, err := r.client.Get(ctx, r.keyPrefix, clientv3.WithPrefix())
		if err != nil {
			return fmt.Errorf("get exist entries, %w", err)
		}

		// etcd从快照恢复后revision会回退，此时版本号不可信，清空后由快照重建
		// 正常情况下保留版本号，周期性同步不会产生重复的节点更新事件
		if resp.Header.Revision < maxRev {
			clear(versions)
			maxRev = resp.Header.Revision
		}

		exists := make(map[string]struct{}, len(resp.Kvs))
		for _, kv := range resp.Kvs {
			exists[string(kv.Key)] = struct{}{}
			apply(&clientv3.Event{Type: mvccpb.PUT, Kv: kv})
		}

		r.allNodes.Range(func(_ ulid.ULID, entry NodeEntry) bool {
			key := path.Join(r.keyPrefix, entry.ID.String())
			if _, ok := exists[key]; !ok {
				// 记录快照revision，避免队列中早于快照的事件把节点重新加回来
				versions[key] = resp.Header.Revision
				logger.Info("remove stale cluster node", "entry", entry, "version", resp.Header.Revision)
				removeNode(entry)
			}
			return true
		})

		return nil
	}

	// 监听变更
	go func() {
		defer close(events)

		retries := 0
		consume := func(wCh clientv3.WatchChan) error {
			for {
				ctx := r.client.Ctx()

				select {
				case <-ctx.Done():
					return ctx.Err()
				case wResp, ok := <-wCh:
					if !ok {
						return errors.New("watch channel closed")
					} else if err := wResp.Err(); err != nil {
						return fmt.Errorf("watch response, %w", err)
					}

					if wResp.Created {
						// watch建立或重建后可能错过的变更，通过全量同步补齐
						// 同步失败不重建watch，避免Get抖动占用watch重试次数，下次周期同步会重试
						if err := resync(); err != nil {
							logger.Error("resync cluster nodes", "error", err)
						}
					}

					retries = 0 // reset count

					for _, ev := range wResp.Events {
						switch ev.Type {
						case mvccpb.PUT, mvccpb.DELETE:
							lock.Lock()
							apply(ev)
							lock.Unlock()
						default:
							logger.Error("unknown event type", "type", ev.Type)
						}
					}
				}
			}
		}

		for {
			wCtx, wCancel := context.WithCancel(r.client.Ctx())
			wCh := r.client.Watch(wCtx, r.keyPrefix, clientv3.WithPrefix(), clientv3.WithPrevKV(), clientv3.WithCreatedNotify())

			err := consume(wCh)
			wCancel()

			if errors.Is(err, context.Canceled) {
				return
			}

			logger.Error("etcd watch failed", "error", err)
			// 长时间无法从etcd更新节点信息时，按设计中止服务，避免使用过期数据路由
			// 中止时长由WithWatchRetry控制（最多重试次数 x 重试间隔）
			if retries >= r.watchRetryMax {
				logger.Error("etcd watch retry limit reached", "retries", retries)
				panic(fmt.Errorf("etcd watch failed after %d retries, %w", retries, err))
			}

			retries++
			logger.Info("retry etcd watch", "retry", retries, "interval", r.watchRetryInterval)

			select {
			case <-time.After(r.watchRetryInterval):
			case <-r.client.Ctx().Done():
				return
			}
		}
	}()

	// 周期性全量同步
	go func() {
		for {
			select {
			case <-r.client.Ctx().Done():
				return
			case <-time.After(1 * time.Minute):
				if err := resync(); err != nil {
					logger.Error("resync cluster nodes", "error", err)
				}
			}
		}
	}()

	return resync()
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
func (r *Registry) SubscribeUpdate(ctx context.Context, handler func(entry NodeEntry)) {
	r.observable.
		DoOnNext(func(item any) {
			if ev, ok := item.(eventUpdateNode); ok {
				handler(ev.Entry)
			}
		}, rxgo.WithContext(ctx))
}

// SubscribeDelete 订阅节点删除
func (r *Registry) SubscribeDelete(ctx context.Context, handler func(entry NodeEntry)) {
	r.observable.
		DoOnNext(func(item any) {
			if ev, ok := item.(eventDeleteNode); ok {
				handler(ev.Entry)
			}
		}, rxgo.WithContext(ctx))
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

// WithWatchRetry 设置watch失败后的最大重试次数和重试间隔
func WithWatchRetry(maxRetries int, interval time.Duration) func(*Registry) {
	return func(r *Registry) {
		if maxRetries > 0 {
			r.watchRetryMax = maxRetries
		}

		if interval > 0 {
			r.watchRetryInterval = interval
		}
	}
}
