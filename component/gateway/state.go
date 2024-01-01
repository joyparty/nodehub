package gateway

import (
	"github.com/joyparty/gokit"
	"github.com/oklog/ulid/v2"
)

// 有状态路由表
type stateTable struct {
	// sessionID => serviceCode => nodeID
	routes *gokit.MapOf[string, *gokit.MapOf[int32, ulid.ULID]]
}

func newStateTable() *stateTable {
	return &stateTable{
		routes: gokit.NewMapOf[string, *gokit.MapOf[int32, ulid.ULID]](),
	}
}

func (sr *stateTable) Find(sessID string, serviceCode int32) (nodeID ulid.ULID, ok bool) {
	if nodes, ok := sr.routes.Load(sessID); ok {
		if nodeID, ok := nodes.Load(serviceCode); ok {
			return nodeID, true
		}
	}

	return
}

func (sr *stateTable) Store(sessID string, serviceCode int32, nodeID ulid.ULID) {
	nodes, ok := sr.routes.Load(sessID)
	if !ok {
		nodes, _ = sr.routes.LoadOrStore(sessID, gokit.NewMapOf[int32, ulid.ULID]())
	}

	nodes.Store(serviceCode, nodeID)
}

func (sr *stateTable) Remove(sessID string, serviceCode int32) {
	if nodes, ok := sr.routes.Load(sessID); ok {
		nodes.Delete(serviceCode)
	}
}

func (sr *stateTable) CleanNode(nodeID ulid.ULID) {
	sr.routes.Range(func(sessID string, nodes *gokit.MapOf[int32, ulid.ULID]) bool {
		nodes.Range(func(serviceCode int32, id ulid.ULID) bool {
			if nodeID.Compare(id) == 0 {
				nodes.Delete(serviceCode)
				return false
			}
			return true
		})
		return true
	})
}

func (sr *stateTable) CleanSession(sessID string) {
	sr.routes.Delete(sessID)
}
