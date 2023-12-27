package cluster

import (
	"fmt"
	"hash/fnv"
	"math/big"
	"math/rand"
	"net/netip"
	"strings"
	"sync/atomic"
)

const (
	// BalancerRoundRobin 轮询
	BalancerRoundRobin = "roundRobin"
	// BalancerWeighted 加权轮询
	BalancerWeighted = "weighted"
	// BalancerIPHash 根据客户端IP地址哈希
	BalancerIPHash = "ipHash"
	// BalancerIDHash 根据客户端ID哈希
	BalancerIDHash = "idHash"
)

var (
	registeredBalancer = make(map[string]func([]NodeEntry) Balancer)
)

func init() {
	RegisterBalancer(BalancerRoundRobin, newRoundRobinBalancer)
	RegisterBalancer(BalancerWeighted, newWeightedBalancer)
	RegisterBalancer(BalancerIPHash, newIPHashBalancer)
	RegisterBalancer(BalancerIDHash, newIDHashBalancer)
}

// Session 会话
type Session interface {
	ID() string
	RemoteAddr() string
}

// Balancer 负载均衡器
type Balancer interface {
	Pick(serviceCode int32, sess Session) (NodeEntry, error)
}

// RegisterBalancer 注册负载均衡器
func RegisterBalancer(policy string, factory func([]NodeEntry) Balancer) {
	policy = strings.ToLower(policy)

	if _, ok := registeredBalancer[policy]; ok {
		panic(fmt.Errorf("balancer: balancer %s already registered", policy))
	}

	registeredBalancer[policy] = factory
}

// NewBalancer 创建负载均衡器
//
// return false 表示策略不存在，使用默认策略替代了
func NewBalancer(policy string, nodes []NodeEntry) (Balancer, bool) {
	if len(nodes) <= 1 {
		return &noBalancer{nodes}, true
	}

	if factory, ok := registeredBalancer[policy]; ok {
		return factory(nodes), true
	}
	return newRoundRobinBalancer(nodes), false
}

type noBalancer struct {
	nodes []NodeEntry
}

func (nb *noBalancer) Pick(serviceCode int32, sess Session) (NodeEntry, error) {
	if len(nb.nodes) == 0 {
		return NodeEntry{}, ErrNoNodeAvailable
	}
	return nb.nodes[0], nil
}

type roundRobinBalancer struct {
	nodes []NodeEntry
	index *atomic.Int32
}

func newRoundRobinBalancer(nodes []NodeEntry) Balancer {
	return &roundRobinBalancer{
		index: &atomic.Int32{},
		nodes: nodes,
	}
}

func (rr *roundRobinBalancer) Pick(serviceCode int32, sess Session) (NodeEntry, error) {
	// 数组内顺序获取，如果到达数组末尾，从头重新开始
	index := rr.index.Add(1) % int32(len(rr.nodes))
	return rr.nodes[index], nil
}

// 加权轮询
type weightedBalancer struct {
	nodes []NodeEntry
	sum   int
}

func newWeightedBalancer(nodes []NodeEntry) Balancer {
	sum := 0
	for _, node := range nodes {
		weight := node.Weight
		if weight == 0 {
			weight = 1
		}
		sum += weight * 100
	}

	return &weightedBalancer{
		sum:   sum,
		nodes: nodes,
	}
}

func (wb *weightedBalancer) Pick(serviceCode int32, sess Session) (NodeEntry, error) {
	r := rand.Intn(wb.sum)
	for _, node := range wb.nodes {
		weight := node.Weight
		if weight == 0 {
			weight = 1
		}
		r -= weight * 100
		if r < 0 {
			return node, nil
		}
	}

	return NodeEntry{}, ErrNoNodeAvailable
}

type ipHashBalancer struct {
	nodes []NodeEntry
}

func newIPHashBalancer(nodes []NodeEntry) Balancer {
	return &ipHashBalancer{
		nodes: nodes,
	}
}

func (ih *ipHashBalancer) Pick(serviceCode int32, sess Session) (NodeEntry, error) {
	ipInt, err := addrToint64(sess.RemoteAddr())
	if err != nil {
		return NodeEntry{}, err
	}
	index := int(ipInt) % len(ih.nodes)
	return ih.nodes[index], nil
}

func addrToint64(remoteAddr string) (int64, error) {
	addr, err := netip.ParseAddrPort(remoteAddr)
	if err != nil {
		return 0, err
	}

	result := big.NewInt(0)
	result.SetBytes(addr.Addr().AsSlice())
	return result.Int64(), nil
}

type idHashBalancer struct {
	nodes []NodeEntry
}

func newIDHashBalancer(nodes []NodeEntry) Balancer {
	return &idHashBalancer{
		nodes: nodes,
	}
}

func (ih *idHashBalancer) Pick(serviceCode int32, sess Session) (NodeEntry, error) {
	h := fnv.New64a()
	h.Write([]byte(sess.ID()))
	hash := h.Sum64()

	index := int(hash) % len(ih.nodes)

	return ih.nodes[index], nil
}
