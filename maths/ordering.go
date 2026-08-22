package maths

// minimumDegreeOrder 计算矩阵符号图的最小度（Minimum Degree）重排。
// 目标：通过行列重排最小化 LU 分解的 fill-in（对结构对称的 MNA 矩阵）。
// 每步消除当前度数（未消除邻居数）最小的节点，并将其邻居两两相连（fill-in）。
// 返回 perm：perm[i] = 原矩阵第 i 行/列在重排后矩阵中的位置。
// 时间复杂度 O(n² + Σdeg²)，适合数千节点的电路矩阵；CPU8（2210 节点）亚秒级。
func MinimumDegreeOrder(n int, adj [][]int) []int {
	// 符号图：邻接集合（去重）
	graph := make([]map[int]struct{}, n)
	for i := 0; i < n; i++ {
		graph[i] = make(map[int]struct{}, len(adj[i]))
		for _, j := range adj[i] {
			if i != j {
				graph[i][j] = struct{}{}
			}
		}
	}
	alive := make([]bool, n)
	degree := make([]int, n)
	for i := 0; i < n; i++ {
		alive[i] = true
		degree[i] = len(graph[i])
	}
	perm := make([]int, n)
	filled := 0
	for k := 0; k < n; k++ {
		// 选度数最小的未消除节点
		best, bestDeg := -1, int(^uint(0)>>1)
		for i := 0; i < n; i++ {
			if alive[i] && degree[i] < bestDeg {
				best, bestDeg = i, degree[i]
			}
		}
		if best == -1 {
			break
		}
		perm[k] = best
		alive[best] = false
		// 消除 best：其未消除邻居两两相连（产生 fill-in）
		nb := make([]int, 0, len(graph[best]))
		for j := range graph[best] {
			if alive[j] {
				nb = append(nb, j)
			}
		}
		for a := 0; a < len(nb); a++ {
			for b := a + 1; b < len(nb); b++ {
				if _, ok := graph[nb[a]][nb[b]]; !ok {
					graph[nb[a]][nb[b]] = struct{}{}
					graph[nb[b]][nb[a]] = struct{}{}
					filled++
				}
			}
		}
		// 更新受影响节点的度数
		for _, j := range nb {
			d := 0
			for u := range graph[j] {
				if alive[u] {
					d++
				}
			}
			degree[j] = d
		}
	}
	return perm
}

// SymbolicFillCount 按给定消元顺序 perm 做符号 fill-in 估计：
// 模拟消除过程（邻居两两相连），返回产生的 fill-in 数。
// 用于在多种候选顺序间选择 fill-in 最少者（符号估计与实际 LU 的 fill-in 趋势一致）。
func SymbolicFillCount(n int, adj [][]int, perm []int) int {
	graph := make([]map[int]struct{}, n)
	for i := 0; i < n; i++ {
		graph[i] = make(map[int]struct{}, len(adj[i]))
		for _, j := range adj[i] {
			if i != j {
				graph[i][j] = struct{}{}
			}
		}
	}
	alive := make([]bool, n)
	for i := 0; i < n; i++ {
		alive[i] = true
	}
	fill := 0
	for _, v := range perm {
		nb := make([]int, 0, len(graph[v]))
		for u := range graph[v] {
			if alive[u] {
				nb = append(nb, u)
			}
		}
		for a := 0; a < len(nb); a++ {
			for b := a + 1; b < len(nb); b++ {
				if _, ok := graph[nb[a]][nb[b]]; !ok {
					graph[nb[a]][nb[b]] = struct{}{}
					graph[nb[b]][nb[a]] = struct{}{}
					fill++
				}
			}
		}
		alive[v] = false
	}
	return fill
}

// reorderSymmetric 按 perm 对结构对称矩阵做行列重排：A'[i][j] = A[perm[i]][perm[j]]。
// perm 由 minimumDegreeOrder 生成。返回重排后的稀疏矩阵。
func ReorderSymmetric[T Number](A Matrix[T], perm []int) Matrix[T] {
	n := A.Rows()
	inv := make([]int, n)
	for i, v := range perm {
		inv[v] = i
	}
	re := NewSparseMatrix[T](n, n)
	for i := 0; i < n; i++ {
		cols, vals := A.GetRow(perm[i])
		for idx, j := range cols {
			re.Set(i, inv[j], vals.Get(idx))
		}
	}
	return re
}
