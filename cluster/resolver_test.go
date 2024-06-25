package cluster

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/joyparty/gokit"
	"github.com/oklog/ulid/v2"
	"github.com/samber/lo"
)

func TestGRPCResolver(t *testing.T) {
	resolver := newGRPCResolver()
	entries := genTestEntries(20)

	t.Run("init", func(t *testing.T) {
		updateResolver(resolver, entries, nil)
		testResolver(t, resolver, entries)
	})

	// 删除3个节点
	t.Run("deleteThree", func(t *testing.T) {
		updateResolver(resolver, entries[3:], entries[:3])
		testResolver(t, resolver, entries[3:])
	})

	// 把所有节点设置为down
	t.Run("allDown", func(t *testing.T) {
		for i := 0; i < len(entries); i++ {
			entries[i].State = NodeDown
		}
		updateResolver(resolver, entries, nil)
		testResolver(t, resolver, entries)
	})

	// 把所有节点设置为ok
	t.Run("allOK", func(t *testing.T) {
		for i := 0; i < len(entries); i++ {
			entries[i].State = NodeOK
		}
		updateResolver(resolver, entries, nil)
		testResolver(t, resolver, entries)
	})

	t.Run("deleteAll", func(t *testing.T) {
		updateResolver(resolver, nil, entries)
		testResolver(t, resolver, []NodeEntry{})
	})
}

func updateResolver(resolver *grpcResolver, update []NodeEntry, remove []NodeEntry) {
	var wg sync.WaitGroup
	for i := 0; i < len(update); i++ {
		entry := update[i]

		wg.Add(1)
		go func() {
			defer wg.Done()
			resolver.Update(entry)
		}()
	}

	for i := 0; i < len(remove); i++ {
		entry := remove[i]
		wg.Add(1)

		go func() {
			defer wg.Done()
			resolver.Remove(entry)
		}()
	}
	wg.Wait()
}

func testResolver(t *testing.T, resolver *grpcResolver, entries []NodeEntry) {
	if expected, actual := len(entries), resolver.allNodes.Count(); expected != int(actual) {
		t.Fatalf("expected %d nodes, got %d", expected, actual)
	}
	// t.Logf("allNodes %d", resolver.allNodes.Count())

	for serviceCode, expected := range getExpectedOKNodes(entries) {
		actual, _ := resolver.okNodes.Load(serviceCode)
		if !compareNodes(expected, actual) {
			// printJSON(expected)
			// printJSON(actual)

			t.Fatalf("ok nodes not equal, expected %d nodes, got %d", len(expected), len(actual))
		}
	}
}

func getExpectedOKNodes(entries []NodeEntry) map[int32][]NodeEntry {
	okNodes := map[int32][]NodeEntry{}

	for _, entry := range entries {
		for _, service := range entry.GRPC.Services {
			if _, ok := okNodes[service.Code]; !ok {
				okNodes[service.Code] = []NodeEntry{}
			}

			if entry.State == NodeOK {
				okNodes[service.Code] = append(okNodes[service.Code], entry)
			}
		}
	}

	return okNodes
}

func getExpectedServices(entries []NodeEntry) []int32 {
	services := []int32{}
	lo.ForEach(entries, func(entry NodeEntry, _ int) {
		lo.ForEach(entry.GRPC.Services, func(desc GRPCServiceDesc, _ int) {
			services = append(services, desc.Code)
		})
	})

	return lo.Uniq(services)
}

func genTestEntries(count int) []NodeEntry {
	entries := make([]NodeEntry, 0, count)

	for i := 0; i < count; i++ {
		entry := NodeEntry{
			ID:    ulid.Make(),
			Name:  fmt.Sprintf("node-%d", i),
			State: NodeOK,
			GRPC:  genTestGRPCEntries(),
		}

		entries = append(entries, entry)
	}

	return entries
}

func genTestGRPCEntries() GRPCEntry {
	used := map[int32]struct{}{}

	services := []GRPCServiceDesc{}
	for i, j := 0, rand.Intn(5)+1; i < j; i++ {
		serviceCode := rand.Int31n(10)
		serviceCode++ // 确保不为0

		if _, ok := used[serviceCode]; ok {
			continue
		}
		used[serviceCode] = struct{}{}

		services = append(services, GRPCServiceDesc{
			Name:     fmt.Sprintf("service-%d", serviceCode),
			Code:     serviceCode,
			Path:     fmt.Sprintf("/service.%d/", serviceCode),
			Balancer: BalancerRandom,
		})
	}

	return GRPCEntry{
		Endpoint: fmt.Sprintf("127.0.0.1:%d", rand.Intn(10000)),
		Services: services,
	}
}

func compareNodes(x []NodeEntry, y []NodeEntry) bool {
	sort.Slice(x, func(i, j int) bool {
		return x[i].ID.Compare(x[j].ID) < 0
	})
	sort.Slice(y, func(i, j int) bool {
		return y[i].ID.Compare(y[j].ID) < 0
	})

	return slices.CompareFunc(x, y, func(x NodeEntry, y NodeEntry) int {
		return x.ID.Compare(y.ID)
	}) == 0
}

func printJSON(v any) {
	data := gokit.MustReturn(json.MarshalIndent(v, "", "  "))
	fmt.Fprintln(os.Stderr, string(data))
}
