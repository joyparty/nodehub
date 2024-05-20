package gateway

import (
	"github.com/joyparty/gokit"
	"github.com/oklog/ulid/v2"
)

// 有状态路由表
type stateTable struct {
	// sessionID => serviceCode => nodeID
	routes *gokit.MapOf[string, *gokit.MapOf[int32, string]]
}

func newStateTable() *stateTable {
	return &stateTable{
		routes: gokit.NewMapOf[string, *gokit.MapOf[int32, string]](),
	}
}

func (st *stateTable) Find(sessID string, serviceCode int32) (nodeID ulid.ULID, ok bool) {
	if nodes, ok := st.routes.Load(sessID); ok {
		if v, ok := nodes.Load(serviceCode); ok {
			nodeID, _ := ulid.Parse(v)
			return nodeID, true
		}
	}

	return
}

func (st *stateTable) Store(sessID string, serviceCode int32, nodeID ulid.ULID) {
	nodes, ok := st.routes.Load(sessID)
	if !ok {
		nodes, _ = st.routes.LoadOrStore(sessID, gokit.NewMapOf[int32, string]())
	}
	nodes.Store(serviceCode, nodeID.String())
}

func (st *stateTable) Remove(sessID string, serviceCode int32) {
	if nodes, ok := st.routes.Load(sessID); ok {
		nodes.Delete(serviceCode)
	}
}

func (st *stateTable) CleanNode(nodeID ulid.ULID) {
	nodeid := nodeID.String()

	st.routes.Range(func(sessID string, nodes *gokit.MapOf[int32, string]) bool {
		nodes.Range(func(serviceCode int32, id string) bool {
			if nodeid == id {
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
	oldid := oldID.String()
	newid := newID.String()

	st.routes.Range(func(sessID string, nodes *gokit.MapOf[int32, string]) bool {
		nodes.Range(func(serviceCode int32, nodeID string) bool {
			if nodeID == oldid {
				nodes.Store(serviceCode, newid)
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
