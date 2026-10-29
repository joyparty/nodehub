package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/reactivex/rxgo/v2"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type watchRequest struct {
	ctx context.Context
	rev int64
	ch  chan clientv3.WatchResponse
}

type registryWatchStub struct {
	clientv3.KV
	clientv3.Watcher
	snapshots chan *clientv3.GetResponse
	requests  chan watchRequest
}

// Get 返回预设快照，nil 用于模拟读取失败。
func (s *registryWatchStub) Get(ctx context.Context, _ string, _ ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	select {
	case resp := <-s.snapshots:
		if resp == nil {
			return nil, errors.New("snapshot unavailable")
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Watch 记录订阅并由测试控制响应。
func (s *registryWatchStub) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	ch := make(chan clientv3.WatchResponse, 1)
	s.requests <- watchRequest{ctx: ctx, rev: clientv3.OpGet(key, opts...).Rev(), ch: ch}
	return ch
}

// Close 无需关闭外部连接。
func (s *registryWatchStub) Close() error { return nil }

func receiveWatchTest[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for watcher")
		var zero T
		return zero
	}
}

func TestRegistryWatchRecovery(t *testing.T) {
	for _, failure := range []string{"closed", "canceled", "compacted"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			node := NodeEntry{ID: ulid.Make(), Name: "test", State: NodeOK}
			removed := NodeEntry{ID: ulid.Make(), Name: "removed", State: NodeOK}
			kv := func(entry NodeEntry, revision, version int64) *mvccpb.KeyValue {
				value, err := json.Marshal(entry)
				if err != nil {
					t.Fatal(err)
				}
				return &mvccpb.KeyValue{Key: []byte("/nodehub/node/" + entry.ID.String()), Value: value, ModRevision: revision, Version: version}
			}
			stub := &registryWatchStub{snapshots: make(chan *clientv3.GetResponse, 3), requests: make(chan watchRequest, 2)}
			stub.snapshots <- &clientv3.GetResponse{Header: &etcdserverpb.ResponseHeader{Revision: 20}, Kvs: []*mvccpb.KeyValue{kv(node, 10, 9), kv(removed, 11, 1)}}
			stub.snapshots <- nil // 重连时第一次全量读取失败，仍应延迟重试。
			node.State = NodeLazy
			stub.snapshots <- &clientv3.GetResponse{Header: &etcdserverpb.ResponseHeader{Revision: 40}, Kvs: []*mvccpb.KeyValue{kv(node, 30, 1)}}
			client := clientv3.NewCtxClient(context.Background())
			client.KV, client.Watcher = stub, stub
			t.Cleanup(func() { _ = client.Close() })
			r, err := NewRegistry(client)
			if err != nil {
				t.Fatal(err)
			}
			first := receiveWatchTest(t, stub.requests)
			if first.rev != 21 {
				t.Fatalf("initial watch revision = %d, want 21", first.rev)
			}
			failedAt := time.Now()
			switch failure {
			case "closed":
				close(first.ch)
			case "canceled":
				first.ch <- clientv3.WatchResponse{Canceled: true}
			case "compacted":
				first.ch <- clientv3.WatchResponse{CompactRevision: 30}
			}
			second := receiveWatchTest(t, stub.requests)
			if time.Since(failedAt) < 2*time.Second || second.rev != 41 || first.ctx.Err() == nil {
				t.Fatal("recovery must delay retries, cancel the old watch and use the new snapshot revision")
			}
			if actual, ok := r.allNodes.Load(node.ID); !ok || actual.State != NodeLazy {
				t.Fatal("resync missed a recreated key whose Version reset to 1")
			}
			if _, ok := r.allNodes.Load(removed.ID); ok {
				t.Fatal("resync retained a node deleted while disconnected")
			}
			updates := r.observable.Observe(rxgo.WithBufferedChannel(1))
			node.State = NodeOK
			second.ch <- clientv3.WatchResponse{Events: []*clientv3.Event{{Type: mvccpb.PUT, Kv: kv(node, 41, 2)}}}
			item := receiveWatchTest(t, updates)
			if event, ok := item.V.(eventUpdateNode); !ok || event.Entry.State != NodeOK {
				t.Fatal("recovered watch failed to process the next update")
			}
			_ = client.Close()
			select {
			case _, ok := <-updates:
				if ok {
					t.Fatal("expected event stream to close on cancellation")
				}
			case <-time.After(time.Second):
				t.Fatal("watch did not exit after client cancellation")
			}
		})
	}
}
