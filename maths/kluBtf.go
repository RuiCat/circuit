// BTF 块三角化:按矩阵有向结构图的强连通分量把行列重排成块上三角
// (非零元只对角块与其右上块),块内再做最小度重排以抑制 fill。

package maths

import "container/heap"

// btfOrder 返回块三角化的置换与各块起始下标;结构只有一个强连通分量时
// 返回 (nil, nil)(调用方退回原有的恒等/最小度择优)。
func (lu *kluSparse[T]) btfOrder(m Matrix[T]) ([]int, []int) {
	return computeBtfOrder(m, lu.n)
}

// computeBtfOrder 是 btfOrder 的包级实现,供稀疏 LU 与分块 LU 共用。
func computeBtfOrder[T Number](m Matrix[T], n int) ([]int, []int) {
	adj := make([][]int, n)
	for i := 0; i < n; i++ {
		forEachNonzero(m, i, func(c int, _ T) {
			if c >= 0 && c < n {
				adj[i] = append(adj[i], c)
			}
		})
	}
	comps := kluSccComponents(adj)
	if len(comps) <= 1 {
		return nil, nil
	}
	compOf := make([]int, n)
	for ci, comp := range comps {
		for _, v := range comp {
			compOf[v] = ci
		}
	}
	order, ok := blockTopologicalOrder(comps, compOf, m, n)
	if !ok {
		return nil, nil // 块图异常(理论不可达):退回无 BTF
	}
	perm := make([]int, 0, n)
	starts := make([]int, 0, len(order))
	for _, ci := range order {
		starts = append(starts, len(perm))
		members := comps[ci]
		local := blockAdjacency(m, compOf, ci, members)
		for _, k := range MinimumDegreeOrder(len(members), local) {
			perm = append(perm, members[k])
		}
	}
	return perm, starts
}

// blockTopologicalOrder 在 SCC 缩合图上做确定性拓扑排序(Kahn,小 id 优先);
// 缩合图有环或排序不全 → (nil,false)。
func blockTopologicalOrder[T Number](comps [][]int, compOf []int, m Matrix[T], n int) ([]int, bool) {
	k := len(comps)
	type edgeKey struct{ from, to int }
	seen := make(map[edgeKey]struct{}, k)
	indeg := make([]int, k)
	edges := make([]edgeKey, 0, k)
	for u := 0; u < n; u++ {
		cu := compOf[u]
		forEachNonzero(m, u, func(c int, _ T) {
			if c < 0 || c >= n {
				return
			}
			cv := compOf[c]
			if cv == cu {
				return
			}
			key := edgeKey{cu, cv}
			if _, dup := seen[key]; dup {
				return
			}
			seen[key] = struct{}{}
			edges = append(edges, key)
			indeg[cv]++
		})
	}
	adj := make([][]int, k)
	for _, e := range edges {
		adj[e.from] = append(adj[e.from], e.to)
	}
	h := &blockHeap{}
	for i := 0; i < k; i++ {
		if indeg[i] == 0 {
			heap.Push(h, i)
		}
	}
	order := make([]int, 0, k)
	for h.Len() > 0 {
		u := heap.Pop(h).(int)
		order = append(order, u)
		for _, v := range adj[u] {
			indeg[v]--
			if indeg[v] == 0 {
				heap.Push(h, v)
			}
		}
	}
	return order, len(order) == k
}

// blockAdjacency 返回第 ci 个 SCC 内部(对称化)的局部邻接表,供块内最小度重排。
func blockAdjacency[T Number](m Matrix[T], compOf []int, ci int, members []int) [][]int {
	rank := make(map[int]int, len(members))
	for k, v := range members {
		rank[v] = k
	}
	adj := make([][]int, len(members))
	for k, u := range members {
		forEachNonzero(m, u, func(c int, _ T) {
			if c < 0 || c >= len(compOf) || compOf[c] != ci {
				return
			}
			r := rank[c]
			adj[k] = append(adj[k], r)
			adj[r] = append(adj[r], k) // 对称化:最小度按无向结构重排
		})
	}
	return adj
}

// blockHeap 是「小 id 优先」的整数堆(拓扑排序的确定性来源)。
type blockHeap []int

func (h blockHeap) Len() int           { return len(h) }
func (h blockHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h blockHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *blockHeap) Push(x any)        { *h = append(*h, x.(int)) }
func (h *blockHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
