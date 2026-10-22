// kluScc.go 提供 BTF 分块所需的强连通分量（Tarjan）。
package maths

import "sort"

// kluSccComponents 返回有向图的强连通分量。
// 每个分量内节点按升序，分量之间按最小节点升序排列（保证 BTF 结果确定）。
// adj[v] 中越界列被忽略；重复边不影响结果。
func kluSccComponents(adj [][]int) [][]int {
	n := len(adj)
	index := make([]int, n)
	low := make([]int, n)
	onStack := make([]bool, n)
	for i := range index {
		index[i] = -1
	}
	stack := make([]int, 0, n)
	comps := make([][]int, 0)
	next := 0
	var strongConnect func(v int)
	strongConnect = func(v int) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range adj[v] {
			if w < 0 || w >= n {
				continue
			}
			if index[w] == -1 {
				strongConnect(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
				continue
			}
			if onStack[w] && index[w] < low[v] {
				low[v] = index[w]
			}
		}
		if low[v] != index[v] {
			return
		}
		comp := make([]int, 0)
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			comp = append(comp, w)
			if w == v {
				break
			}
		}
		sort.Ints(comp)
		comps = append(comps, comp)
	}
	for v := 0; v < n; v++ {
		if index[v] == -1 {
			strongConnect(v)
		}
	}
	sort.Slice(comps, func(i, j int) bool { return comps[i][0] < comps[j][0] })
	return comps
}
