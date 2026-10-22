// 稀疏 LU 的静态重排:结构签名(值无关) + 恒等/最小度顺序择fill 少者,
// 并用成本护栏避免为本来就稀疏的结构白付重排开销。

package maths

import "sort"

// structureSignature 计算矩阵结构签名(只混入每行的非零列号,不含元素值):
// FNV-1a。结构不变 → 签名不变,静态重排与消元计划可复用;结构变化 → 重算。
func structureSignature[T Number](m Matrix[T]) uint64 {
	const offset = 14695981039346656037
	const prime = 1099511628211
	h := uint64(offset)
	mix := func(v uint64) {
		h ^= v
		h *= prime
	}
	// 逐行直读非零(不分配),稠密输入也不会退化成 O(n²) 的 GetRow 拷贝。
	for i := 0; i < m.Rows(); i++ {
		count := 0
		forEachNonzero(m, i, func(c int, _ T) {
			count++
			mix(uint64(c) + 1)
		})
		mix(uint64(count)) // 行分隔与行内非零数(前后 m 的行长度也要能区分)
		_ = count
	}
	mix(uint64(m.Rows()))
	return h
}

// symmetricAdjacency 由矩阵结构构造无向邻接表(结构对称化:非对称输入
// 也按 A∪Aᵀ 处理,重排只改变消元顺序、不改变解);每行升序、不含自环。
func symmetricAdjacency[T Number](m Matrix[T]) [][]int {
	n := m.Rows()
	edges := make([][2]int, 0, 4*n)
	for i := 0; i < n; i++ {
		forEachNonzero(m, i, func(j int, _ T) {
			if j == i || j < 0 || j >= n {
				return
			}
			edges = append(edges, [2]int{i, j})
		})
	}
	rows := make([][]int, n)
	for i := range rows {
		rows[i] = make([]int, 0, 8)
	}
	for _, e := range edges {
		rows[e[0]] = append(rows[e[0]], e[1])
		rows[e[1]] = append(rows[e[1]], e[0])
	}
	for i := 0; i < n; i++ {
		row := rows[i]
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

// computeStaticPerm 计算静态重排置换:先看恒等顺序的符号 fill 预测,只有
// 它 > 3n(结构本身不稀疏、值得付重排开销)才与最小度重排比较并取更少者。
// 结构已变时同时作废消元计划并重置骨架安全阀。
func (lu *kluSparse[T]) computeStaticPerm(m Matrix[T]) []int {
	n := lu.n
	lu.plan = nil
	lu.skelOff = false
	lu.staleStreak = 0
	lu.blocks = nil
	if lu.useBTF {
		if perm, blocks := lu.btfOrder(m); perm != nil {
			lu.blocks = blocks
			return perm
		}
	}
	adj := symmetricAdjacency(m)
	perm := identityPerm(n)
	// 只需判定恒等顺序的 fill 是否已超线性量级(>3n);超过即提前结束模拟,
	// 不再跑完整 O(填充) 的符号消元(1 万节点上这是首次分解的最大单项)。
	_, exceeded, err := symbolicFill(n, adj, perm, 3*n)
	if err != nil {
		return perm
	}
	if exceeded {
		// 这类结构上最小度几乎总优于自然顺序,直接采用 MD,
		// 不再做第二次符号模拟。
		perm = MinimumDegreeOrder(n, adj)
	}
	return perm
}
