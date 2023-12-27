package cluster

import (
	"fmt"
	"hash/fnv"
	"math/big"
	"math/rand"
	"net/netip"
	"strings"
)

const (
	// BalancerRandom 随机
	BalancerRandom = "random"
	// BalancerRoundRobin 加权轮询
	BalancerRoundRobin = "roundRobin"
	// BalancerIPHash 根据客户端IP地址哈希
	BalancerIPHash = "ipHash"
	// BalancerIDHash 根据客户端ID哈希
	BalancerIDHash = "idHash"
)

var (
	registeredBalancer = make(map[string]func([]NodeEntry) Balancer)
)

func init() {
	RegisterBalancer(BalancerRandom, newRandomBalancer)
	RegisterBalancer(BalancerRoundRobin, newRoundRobinBalancer)
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
func NewBalancer(policy string, nodes []NodeEntry) Balancer {
	if len(nodes) <= 1 {
		return &noBalancer{nodes}
	}

	if factory, ok := registeredBalancer[policy]; ok {
		return factory(nodes)
	}

	// 如果返回某个默认的balancer，可能会导致非预期的结果
	// 如果不返回balancer，resolver那边最终会导致获取节点时返回无节点可用错误，误导排查方向
	// 返回指定错误的balancer，可以在获取节点时得知真正的错误
	return &errorBalancer{
		err: fmt.Errorf("balancer %s not implemented", policy),
	}
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

type errorBalancer struct {
	err error
}

func (eb *errorBalancer) Pick(serviceCode int32, sess Session) (NodeEntry, error) {
	return NodeEntry{}, eb.err
}

type randomBalancer struct {
	nodes []NodeEntry
}

func newRandomBalancer(nodes []NodeEntry) Balancer {
	return &randomBalancer{
		nodes: nodes,
	}
}

func (b *randomBalancer) Pick(serviceCode int32, sess Session) (NodeEntry, error) {
	return b.nodes[rand.Intn(len(b.nodes))], nil
}

// 加权轮询
type roundRobinBalancer struct {
	nodes []NodeEntry
	sum   int
}

func newRoundRobinBalancer(nodes []NodeEntry) Balancer {
	sum := 0
	for i, node := range nodes {
		if node.Weight == 0 {
			node.Weight = 1
			nodes[i] = node
		}
		sum += node.Weight * 10
	}

	return &roundRobinBalancer{
		sum:   sum,
		nodes: nodes,
	}
}

func (b *roundRobinBalancer) Pick(serviceCode int32, sess Session) (NodeEntry, error) {
	r := rand.Intn(b.sum)
	for _, node := range b.nodes {
		r -= node.Weight * 10
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

func (b *ipHashBalancer) Pick(serviceCode int32, sess Session) (NodeEntry, error) {
	ipInt, err := addrToint64(sess.RemoteAddr())
	if err != nil {
		return NodeEntry{}, err
	}
	index := int(ipInt) % len(b.nodes)
	return b.nodes[index], nil
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

func (b *idHashBalancer) Pick(serviceCode int32, sess Session) (NodeEntry, error) {
	h := fnv.New64a()
	h.Write([]byte(sess.ID()))
	hash := h.Sum64()

	index := int(hash) % len(b.nodes)
	return b.nodes[index], nil
}
