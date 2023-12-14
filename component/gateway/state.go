package gateway

import (
	"github.com/joyparty/gokit"
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

func (sr *stateTable) Find(sessID string, serviceCode int32) (nodeID string, ok bool) {
	if nodes, ok := sr.routes.Load(sessID); ok {
		if nodeID, ok := nodes.Load(serviceCode); ok {
			return nodeID, true
		}
	}

	return
}

func (sr *stateTable) Store(sessID string, serviceCode int32, nodeID string) {
	nodes, ok := sr.routes.Load(sessID)
	if !ok {
		nodes, _ = sr.routes.LoadOrStore(sessID, gokit.NewMapOf[int32, string]())
	}

	nodes.Store(serviceCode, nodeID)
}

func (sr *stateTable) Remove(sessID string, serviceCode int32) {
	if nodes, ok := sr.routes.Load(sessID); ok {
		nodes.Delete(serviceCode)
	}
}

func (sr *stateTable) Clean(sessID string) {
	sr.routes.Delete(sessID)
}
