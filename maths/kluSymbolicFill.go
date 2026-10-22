// kluSymbolicFill.go 提供稀疏 LU 的符号 fill 模拟与数值辅助（移植自算法导论 solve/ordering.go 与 lu.go）。
package maths

import (
	"errors"
	"sort"
)

// one 返回类型 T 的乘法单位元（移植自 solve/lu.go）。
func one[T Number]() T {
	var v T
	switch any(v).(type) {
	case float32:
		return any(float32(1)).(T)
	case float64:
		return any(float64(1)).(T)
	case complex64:
		return any(complex64(1)).(T)
	default:
		return any(complex128(1)).(T)
	}
}

// buildDegreeRows 构造无自环、去重的升序邻接表。
func buildDegreeRows(n int, adj [][]int) [][]int {
	rows := make([][]int, n)
	for i := 0; i < n; i++ {
		row := append([]int(nil), adj[i]...)
		sort.Ints(row)
		w := 0
		for _, v := range row {
			if v == i || (w > 0 && row[w-1] == v) {
				continue
			}
			row[w] = v
			w++
		}
		rows[i] = row[:w]
	}
	return rows
}

// aliveNeighbors 返回升序行切片里仍然存活（未消元）的邻居。
func aliveNeighbors(row []int, alive []bool) []int {
	out := make([]int, 0, len(row))
	for _, v := range row {
		if alive[v] {
			out = append(out, v)
		}
	}
	return out
}

// insertSortedInt 在升序切片里插入 v（已存在返回 false）。
func insertSortedInt(row *[]int, v int) bool {
	r := *row
	pos := sort.SearchInts(r, v)
	if pos < len(r) && r[pos] == v {
		return false
	}
	r = append(r, 0)
	copy(r[pos+1:], r[pos:])
	r[pos] = v
	*row = r
	return true
}

// checkPerm 校验 perm 是 0..n-1 的一个排列。
func checkPerm(n int, perm []int) error {
	if len(perm) != n {
		return errors.New("maths: perm 长度与节点数不符")
	}
	if !isPermutation(perm) {
		return errors.New("maths: perm 不是合法置换（越界或重复）")
	}
	return nil
}

// isPermutation 校验 perm 覆盖 0..n-1 且无重复。
func isPermutation(perm []int) bool {
	seen := make([]bool, len(perm))
	for _, v := range perm {
		if v < 0 || v >= len(perm) || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

// identityPerm 返回恒等置换 [0..n-1]。
func identityPerm(n int) []int {
	perm := make([]int, n)
	for i := range perm {
		perm[i] = i
	}
	return perm
}

// diagTooSmall 判断对角元相对矩阵量级是否视为奇异（移植自 solve/lu_block.go）。
func diagTooSmall(diagAbs, scale float64) bool {
	if scale > 0 {
		return diagAbs < Epsilon*scale
	}
	return diagAbs < Epsilon
}

// symbolicFill 做一次符号消元模拟并统计新增边数；limit ≥ 0 时一旦 fill 超过
// limit 立即提前返回（exceeded=true），用于「恒等顺序是否已坏到值得重排」的判定。
func symbolicFill(n int, adj [][]int, perm []int, limit int) (int, bool, error) {
	if len(adj) != n {
		return 0, false, errors.New("maths: 邻接表长度与节点数不符")
	}
	if err := checkPerm(n, perm); err != nil {
		return 0, false, err
	}
	graph := buildDegreeRows(n, adj)
	alive := make([]bool, n)
	for i := 0; i < n; i++ {
		alive[i] = true
	}
	fill := 0
	for _, v := range perm {
		nb := aliveNeighbors(graph[v], alive)
		for a := 0; a < len(nb); a++ {
			for b := a + 1; b < len(nb); b++ {
				u, w := nb[a], nb[b]
				if insertSortedInt(&graph[u], w) {
					insertSortedInt(&graph[w], u)
					fill++
					if limit >= 0 && fill > limit {
						return fill, true, nil
					}
				}
			}
		}
		alive[v] = false
	}
	return fill, false, nil
}
