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
	registeredBalancer = make(map[string]BalancerFactory)
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

// BalancerFactory 负载均衡器工厂
type BalancerFactory func(serviceCode int32, nodes []NodeEntry) Balancer

// Balancer 负载均衡器
type Balancer interface {
	Pick(sess Session) (NodeEntry, error)
}

// RegisterBalancer 注册负载均衡器
func RegisterBalancer(policy string, factory BalancerFactory) {
	policy = strings.ToLower(policy)
	if _, ok := registeredBalancer[policy]; ok {
		panic(fmt.Errorf("balancer: balancer %s already registered", policy))
	}

	registeredBalancer[policy] = factory
}

// NewBalancer 创建负载均衡器
func NewBalancer(serviceCode int32, nodes []NodeEntry) Balancer {
	if len(nodes) <= 1 {
		return &noBalancer{nodes}
	}

	var policy string
	for _, desc := range nodes[0].GRPC.Services {
		policy = desc.Balancer
		break
	}

	if factory, ok := registeredBalancer[strings.ToLower(policy)]; ok {
		return factory(serviceCode, nodes)
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

func (nb *noBalancer) Pick(sess Session) (NodeEntry, error) {
	if len(nb.nodes) == 0 {
		return NodeEntry{}, ErrNoNodeAvailable
	}
	return nb.nodes[0], nil
}

type errorBalancer struct {
	err error
}

func (eb *errorBalancer) Pick(sess Session) (NodeEntry, error) {
	return NodeEntry{}, eb.err
}

type randomBalancer struct {
	nodes []NodeEntry
}

func newRandomBalancer(serviceCode int32, nodes []NodeEntry) Balancer {
	return &randomBalancer{
		nodes: nodes,
	}
}

func (b *randomBalancer) Pick(sess Session) (NodeEntry, error) {
	return b.nodes[rand.Intn(len(b.nodes))], nil
}

type weightedNode struct {
	entry  NodeEntry
	weight int
}

// 加权轮询
type roundRobinBalancer struct {
	nodes []weightedNode
	sum   int
}

func newRoundRobinBalancer(serviceCode int32, nodes []NodeEntry) Balancer {
	wnodes := make([]weightedNode, 0, len(nodes))
	sum := 0
	for _, node := range nodes {
		for _, desc := range node.GRPC.Services {
			if desc.Code == serviceCode {
				weight := desc.Weight
				if weight <= 0 {
					weight = 1
				}
				weight = weight * 10

				sum += weight
				wnodes = append(wnodes, weightedNode{
					entry:  node,
					weight: weight,
				})
				break
			}
		}
	}

	return &roundRobinBalancer{
		sum:   sum,
		nodes: wnodes,
	}
}

func (b *roundRobinBalancer) Pick(sess Session) (NodeEntry, error) {
	r := rand.Intn(b.sum)
	for _, node := range b.nodes {
		r -= node.weight
		if r < 0 {
			return node.entry, nil
		}
	}

	return NodeEntry{}, ErrNoNodeAvailable
}

type ipHashBalancer struct {
	nodes []NodeEntry
}

func newIPHashBalancer(serviceCode int32, nodes []NodeEntry) Balancer {
	return &ipHashBalancer{
		nodes: nodes,
	}
}

func (b *ipHashBalancer) Pick(sess Session) (NodeEntry, error) {
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

func newIDHashBalancer(serviceCode int32, nodes []NodeEntry) Balancer {
	return &idHashBalancer{
		nodes: nodes,
	}
}

func (b *idHashBalancer) Pick(sess Session) (NodeEntry, error) {
	h := fnv.New64a()
	h.Write([]byte(sess.ID()))
	hash := h.Sum64()

	index := int(hash) % len(b.nodes)
	return b.nodes[index], nil
}
