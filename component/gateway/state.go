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

func (st *stateTable) Find(sessID string, serviceCode int32) (nodeID ulid.ULID, ok bool) {
	if nodes, ok := st.routes.Load(sessID); ok {
		return nodes.Load(serviceCode)
	}

	return
}

func (st *stateTable) Store(sessID string, serviceCode int32, nodeID ulid.ULID) {
	nodes, ok := st.routes.Load(sessID)
	if !ok {
		nodes, _ = st.routes.LoadOrStore(sessID, gokit.NewMapOf[int32, ulid.ULID]())
	}
	nodes.Store(serviceCode, nodeID)
}

func (st *stateTable) Remove(sessID string, serviceCode int32) {
	if nodes, ok := st.routes.Load(sessID); ok {
		nodes.Delete(serviceCode)
	}
}

func (st *stateTable) CleanNode(nodeID ulid.ULID) {
	st.routes.Range(func(sessID string, nodes *gokit.MapOf[int32, ulid.ULID]) bool {
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

// ReplaceNode 替换节点
func (st *stateTable) ReplaceNode(oldID, newID ulid.ULID) {
	st.routes.Range(func(sessID string, nodes *gokit.MapOf[int32, ulid.ULID]) bool {
		nodes.Range(func(serviceCode int32, nodeID ulid.ULID) bool {
			if nodeID.Compare(oldID) == 0 {
				nodes.Store(serviceCode, newID)
				return false
			}
			return true
		})
		return true
	})
}

func (st *stateTable) CleanSession(sessID string) {
	st.routes.Delete(sessID)
}
