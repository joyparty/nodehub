package cluster

// eventUpdateNode 更新节点
type eventUpdateNode struct {
	Entry NodeEntry
}

// eventDeleteNode 删除节点
type eventDeleteNode struct {
	Entry NodeEntry
}
