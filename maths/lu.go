package maths

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// NewLU 创建一个稠密矩阵 LU 分解求解器。
func NewLU[T Number](n int) (LU[T], error) {
	if n < 1 {
		return nil, errors.New("lu dimension must be positive")
	}
	return &luDense[T]{
		baseLU: baseLU[T]{
			n:        n,
			L:        NewDenseMatrix[T](n, n),
			U:        NewDenseMatrix[T](n, n),
			Y:        NewDenseVector[T](n),
			P:        make([]int, n),
			pinverse: make([]int, n),
		},
	}, nil
}

// NewLUSparse 创建一个稀疏矩阵 LU 分解求解器。
func NewLUSparse[T Number](n int) (LU[T], error) {
	if n < 1 {
		return nil, errors.New("lu sparse dimension must be positive")
	}
	return &luSparse[T]{
		baseLU: baseLU[T]{
			n:        n,
			L:        newRowSparseMatrix[T](n, n),
			U:        newRowSparseMatrix[T](n, n),
			Y:        NewDenseVector[T](n),
			P:        make([]int, n),
			pinverse: make([]int, n),
		},
	}, nil
}

// baseLU 保存 LU 分解的通用字段。
type baseLU[T Number] struct {
	n        int       // 矩阵维度
	L        Matrix[T] // 下三角矩阵 L
	U        Matrix[T] // 上三角矩阵 U
	Y        Vector[T] // 求解过程中的中间向量 (Ly = Pb)
	P        []int     // 置换矩阵的表示，P[i] = j 表示原始矩阵的第 j 行在置换后位于第 i 行
	pinverse []int     // P 的逆置换
}

// Dim 返回矩阵的维度。
func (lu *baseLU[T]) Dim() int {
	return lu.n
}

// init 初始化 LU 分解器。
// 它将 L 置为单位矩阵，U 置为输入矩阵的副本，并初始化置换矩阵 P。
func (lu *baseLU[T]) init(matrix Matrix[T]) {
	lu.L.Zero()
	lu.U.Zero()
	matrix.Copy(lu.U)
	for i := 0; i < lu.n; i++ {
		lu.P[i] = i
		lu.pinverse[i] = i
		lu.L.Set(i, i, T(1)) // L 的对角线元素为 1
	}
}

// updatePermutation 更新置换矩阵 P 及其逆。
func (lu *baseLU[T]) updatePermutation(k, maxRow int) {
	lu.P[k], lu.P[maxRow] = lu.P[maxRow], lu.P[k]
	lu.pinverse[lu.P[k]] = k
	lu.pinverse[lu.P[maxRow]] = maxRow
}

// luDense 实现稠密矩阵的 LU 分解。
type luDense[T Number] struct {
	baseLU[T]
}

// Decompose 对稠密矩阵执行 LU 分解。
func (lu *luDense[T]) Decompose(matrix Matrix[T]) error {
	if !matrix.IsSquare() {
		return errors.New("lu dense decompose: input must be square matrix")
	}
	if matrix.Rows() != lu.n {
		return errors.New("lu dense decompose: matrix dimension mismatch")
	}

	lu.init(matrix)

	// Doolittle 分解算法
	for k := 0; k < lu.n; k++ {
		// --- 部分主元选择 (Partial Pivoting) ---
		// 在当前列 k 中，从对角线元素开始向下寻找绝对值最大的元素，以保证数值稳定性。
		maxRow := k
		pivotVal := lu.U.Get(k, k)
		// 防止 NaN/Inf 主元绕过奇异矩阵检测，污染解向量
		if !IsValidFloat64(Abs(pivotVal)) {
			return fmt.Errorf("lu dense decompose: pivot contains NaN/Inf at k=%d", k)
		}
		maxAbsVal := Abs(pivotVal)
		for i := k + 1; i < lu.n; i++ {
			if v := Abs(lu.U.Get(i, k)); v > maxAbsVal {
				maxAbsVal = v
				maxRow = i
			}
		}

		if maxAbsVal < Epsilon {
			// 如果主元过小，矩阵可能是奇异的或接近奇异的，分解失败。
			return errors.New("lu dense decompose: matrix is singular or nearly singular")
		}

		// 如果找到了更大的主元，则进行行交换。
		if maxRow != k {
			// 交换 U 的 k 行和 maxRow 行。
			lu.U.SwapRows(k, maxRow)
			// 交换 L 中 k 行之前的部分。
			for j := 0; j < k; j++ {
				val1 := lu.L.Get(k, j)
				val2 := lu.L.Get(maxRow, j)
				lu.L.Set(k, j, val2)
				lu.L.Set(maxRow, j, val1)
			}
			// 更新置换信息。
			lu.updatePermutation(k, maxRow)
		}

		// --- 消元过程 ---
		pivotVal = lu.U.Get(k, k)
		// 对 k 列下方的所有行进行操作。
		for i := k + 1; i < lu.n; i++ {
			// 计算乘数因子，并存入 L 矩阵。
			factor := lu.U.Get(i, k) / pivotVal
			lu.L.Set(i, k, factor)
			// 将 U 矩阵的 (i, k) 元素置零。
			var zero T
			lu.U.Set(i, k, zero)

			// 更新 U 矩阵中第 i 行的剩余元素。
			for j := k + 1; j < lu.n; j++ {
				newVal := lu.U.Get(i, j) - factor*lu.U.Get(k, j)
				lu.U.Set(i, j, newVal)
			}
		}
	}
	return nil
}

// SolveReuse 使用 LU 分解结果求解 Ax=b。
// 该方法分为两步：
// 1. 前向替换 (Forward Substitution): 求解 Ly = Pb
// 2. 后向回代 (Backward Substitution): 求解 Ux = y
func (lu *luDense[T]) SolveReuse(b, x Vector[T]) error {
	if b.Length() != lu.n || x.Length() != lu.n {
		return errors.New("lu dense solve: vector dimension mismatch")
	}

	// --- 1. 前向替换: Ly = Pb ---
	// 注意 b 向量需要根据置换矩阵 P 进行重排。
	lu.Y.Zero()
	for i := 0; i < lu.n; i++ {
		sum := b.Get(lu.P[i])
		for j := 0; j < i; j++ {
			sum -= lu.L.Get(i, j) * lu.Y.Get(j)
		}
		lu.Y.Set(i, sum)
	}

	// --- 2. 后向回代: Ux = y ---
	x.Zero()
	for i := lu.n - 1; i >= 0; i-- {
		sum := lu.Y.Get(i)
		for j := i + 1; j < lu.n; j++ {
			sum -= lu.U.Get(i, j) * x.Get(j)
		}
		diagVal := lu.U.Get(i, i)
		if Abs(diagVal) < Epsilon {
			return errors.New("lu dense solve: division by zero (U diagonal is zero)")
		}
		x.Set(i, sum/diagVal)
	}

	return nil
}

// luSparseMinDim 是符号骨架的启用维度下限：与 computeStaticPerm 的 n<500 → identityPerm 口径一致，
// 小矩阵 fill-in 温和、复用收益不足，且既有数值路径依赖其原顺序。
const luSparseMinDim = 500

// luSparseStaleLimit 是骨架连续失效次数上限：达到即停用骨架（避免「重放 + 回退动态路径」双开销
// 变成回归）；结构签名变化（新结构）时重新启用。
const luSparseStaleLimit = 3

// errSkeletonStale 表示数值阶段发现骨架记录的消元计划与实际数值路径不符（主元行变了、
// 或需要骨架没有的槽位）：骨架作废，调用方回退动态路径。
var errSkeletonStale = errors.New("lu sparse: skeleton is stale")

// luSparse 实现稀疏矩阵的 LU 分解。
type luSparse[T Number] struct {
	baseLU[T]
	perm        []int          // 静态最小度重排序：U 的第 i 行 = 原矩阵 perm[i] 行（宽行排最后，抑制 fill-in）
	permSig     uint64         // 矩阵结构签名（每行列号集合，见 structureSignature）：结构不变时复用 perm，避免每步重复最小度排序
	skel        *luSkeleton[T] // 符号骨架：与 perm 同键（结构签名）复用的 U 槽位 + 消元计划；nil = 无可用骨架
	pivotID     []int          // 最近一次动态分解记录的主元行身份序列（供 captureSkeleton 固化骨架）
	staleStreak int            // 骨架连续失效次数（安全阀，见 luSparseStaleLimit）
	skelOff     bool           // 骨架已被安全阀停用，直到结构签名变化
}

// computeStaticPerm 计算静态重排置换，按**符号填充数预测**在自然顺序与最小度重排之间择优
// （判据与成本护栏见函数内注释；决策只在结构签名变化时发生一次，之后由 permSig 复用）。
func (lu *luSparse[T]) computeStaticPerm(matrix Matrix[T]) []int {
	// 小矩阵（<500 节点）fill-in 温和，保持原顺序以维持既有数值路径
	// （消元顺序改变会引入不同舍入误差，破坏边沿敏感的时序电路行为）。
	if lu.n < 500 {
		return identityPerm(lu.n)
	}
	// 结构签名：每行列号集合的 FNV hash（先排序再混入；电路每步矩阵结构不变，值变化不影响签名）。
	sig := lu.structureSignature(matrix)
	// 结构未变：复用缓存的 perm，避免每步重复最小度排序（CPU8 实测占 22% CPU）。
	// 签名不可用（哨兵）时结构是否变化无法判定，一律走「不复用」分支；
	// 此时不复用分支只做一次签名扫描，邻接表推迟到确实需要重排时才构建。
	if sig != STRUCTURE_SIGNATURE_UNAVAILABLE && sig == lu.permSig && lu.perm != nil {
		return lu.perm
	}
	// 结构已变（或签名不可用）：perm 与符号骨架同时作废，并重新启用骨架安全阀。
	lu.skel = nil
	lu.skelOff = false
	lu.staleStreak = 0
	adj := make([][]int, lu.n)
	for i := 0; i < lu.n; i++ {
		cols, _ := matrix.GetRow(i)
		adj[i] = cols
	}
	var perm []int
	// 择优判据（UD8 修正）：按**符号填充数预测**在「自然顺序」与「最小度重排」之间取更少者。
	//
	// 旧判据 maxDeg > 10*avgDeg 只看度数比，会漏掉两类真实结构：
	//   - 度数比 ≈4 的 hub（电源/时钟总线连大量节点，但每行度相近）；
	//   - 度数比 ≈1.0 的随机稀疏（无 hub，但自然顺序填充爆炸）。
	// 父体实测（五种 ≥500 结构）：填充预测与实际 U nnz 的优劣排序完全一致——
	// hub600 31778→4599（−85.5%）、hub800 51120→5971（−88.3%）、
	// random600 107486→85443 且分解 1.47s→895ms；band/tridiag 预测打平、保持自然顺序。
	//
	// 成本护栏：先算一次自然顺序的填充预测——填充已经很小（≤ 3n）说明该结构本身稀疏，
	// 最小度不可能显著更优（带状/三对角的下界本就在 ~2n 量级），直接保持自然顺序，
	// 从而不为常见带状电路多付最小度重排与第二次预测的开销。
	// 决策只在**结构签名变化**时发生一次，之后由 permSig 复用（既有缓存语义不变）。
	perm = identityPerm(lu.n)
	if fillID := SymbolicFillCount(lu.n, adj, perm); fillID > 3*lu.n {
		if mdPerm := MinimumDegreeOrder(lu.n, adj); SymbolicFillCount(lu.n, adj, mdPerm) < fillID {
			perm = mdPerm
		}
	}
	lu.perm = perm
	lu.permSig = sig
	return perm
}

// identityPerm 返回恒等置换 [0,1,...,n-1]。
func identityPerm(n int) []int {
	perm := make([]int, n)
	for i := range perm {
		perm[i] = i
	}
	return perm
}

// Decompose 对稀疏矩阵执行 LU 分解。
// 两个阶段分离：
//   - 符号阶段（symbolicStage，见 computeStaticPerm）：结构签名 → 复用/重算静态重排 perm；
//     复用命中时连符号骨架（U 槽位 + 消元计划）一起复用，符号工作完全跳过。
//   - 数值阶段：有骨架时走 numericReplay（结构只读、只写值槽）；骨架失效则走 decomposeWithPerm
//     动态路径，并在成功后固化新骨架。
//
// 若静态重排顺序触发数值奇异（近奇异矩阵对消元顺序敏感），回退原顺序重试一次。
func (lu *luSparse[T]) Decompose(matrix Matrix[T]) error {
	if !matrix.IsSquare() {
		return errors.New("lu sparse decompose: input must be square matrix")
	}
	if matrix.Rows() != lu.n {
		return errors.New("lu sparse decompose: matrix dimension mismatch")
	}

	// 静态最小度重排序：按行宽升序排列，宽行（电源/地）最后消元。
	// 保持对称置换 U = P·A·Pᵀ，不改变数值解，仅改变消元顺序。
	lu.perm = lu.computeStaticPerm(matrix)

	// 数值阶段（骨架重放）：结构阶段已在上一步完成，这里只写值。
	if lu.skel != nil {
		err := lu.numericReplay(matrix)
		if err == nil {
			lu.staleStreak = 0
			return nil
		}
		if !errors.Is(err, errSkeletonStale) {
			return err
		}
		// 骨架与实际数值路径不符：作废骨架（U 已按骨架输出，动态路径会自行重置 U/L），
		// 并累计失效次数触发安全阀。
		lu.skel = nil
		lu.staleStreak++
		if lu.staleStreak >= luSparseStaleLimit {
			lu.skelOff = true
			lu.staleStreak = 0
		}
	}

	// 动态数值阶段：结构随消元生长；成功后固化骨架供后续复用。
	if err := lu.decomposeWithPerm(matrix); err == nil {
		lu.captureSkeleton(matrix)
		return nil
	} else if !strings.Contains(err.Error(), "singular") {
		return err
	}
	// 奇异回退：原顺序（数值路径不同，可能避免假奇异）
	lu.perm = identityPerm(lu.n)
	lu.skel = nil
	if err := lu.decomposeWithPerm(matrix); err != nil {
		return err
	}
	lu.captureSkeleton(matrix)
	return nil
}

// captureSkeleton 在动态分解成功后固化符号骨架（纯结构运算，不涉及数值）：
// 以刚记录的消元计划（lu.pivotID）做一次「符号消元」——按主元行逐步把槽位并集传播开，
// 得到每行的槽位上界与每步的消元行集。数值阶段据此只写值，不再重建行列结构。
//
// 槽位是「同一消元计划下不会发生数值消去」的符号上界，因此不会随数值变化而缺失；
// 唯一可能失效的是消元计划本身（主元行改变），由 numericReplay 逐步校验。
func (lu *luSparse[T]) captureSkeleton(matrix Matrix[T]) {
	if lu.n < luSparseMinDim || lu.skelOff || matrix == nil ||
		len(lu.perm) != lu.n || len(lu.pivotID) != lu.n {
		lu.skel = nil
		return
	}
	n := lu.n
	pinv := make([]int, n)
	for i, p := range lu.perm {
		pinv[p] = i
	}
	sk := newLuSkeleton[T](n)
	// 初始槽位：U = P·A·Pᵀ 的列集合（身份 i ↔ 初始位置 i ↔ 原矩阵行 perm[i]）。
	for i := 0; i < n; i++ {
		cols, vals := matrix.GetRow(lu.perm[i])
		if vals == nil || vals.Length() != len(cols) {
			lu.skel = nil
			return
		}
		row := make([]int, 0, len(cols))
		for _, c := range cols {
			row = append(row, pinv[c])
		}
		slices.Sort(row)
		sk.rowCols[i] = slices.Compact(row)
	}
	// colRows[j] = 含列 j 的行身份集合（无序；只用于「列 j 上有哪些行」，不做顺序依赖）。
	colRows := make([][]int, n)
	for i := 0; i < n; i++ {
		for _, j := range sk.rowCols[i] {
			colRows[j] = append(colRows[j], i)
		}
	}
	idAtPos := make([]int, n)
	posOfID := make([]int, n)
	for i := 0; i < n; i++ {
		idAtPos[i], posOfID[i] = i, i
	}
	for k := 0; k < n; k++ {
		p := lu.pivotID[k]
		if p < 0 || p >= n {
			lu.skel = nil
			return
		}
		// 主元搜索候选：交换前位置 > k 且列 k 有槽位的行（按位置升序）。
		sk.searchRows[k] = luSkeletonCandidates(colRows[k], posOfID, k)
		q := posOfID[p]
		idAtPos[k], idAtPos[q] = idAtPos[q], idAtPos[k]
		posOfID[idAtPos[k]], posOfID[idAtPos[q]] = k, q
		// 消元候选：交换后位置 > k 且列 k 有槽位的行（按位置升序）。
		sk.stepRows[k] = luSkeletonCandidates(colRows[k], posOfID, k)
		sk.pivotID[k] = p
		sk.pivotStep[p] = k
		pivotCols := sk.rowCols[p]
		from := firstAboveSorted(pivotCols, k)
		if from == len(pivotCols) {
			continue
		}
		for _, i := range sk.stepRows[k] {
			sk.rowCols[i] = luSkeletonUnionAbove(sk.rowCols[i], pivotCols, from, i, colRows)
		}
	}
	// 值槽按「符号消元结束后」的槽位上界一次性分配（槽位只增不减，此处即最终形状）。
	for i := 0; i < n; i++ {
		sk.rowVals[i] = make([]T, len(sk.rowCols[i]))
	}
	for k := 0; k < n; k++ {
		p := sk.pivotID[k]
		sk.pivotFrom[k] = firstAboveSorted(sk.rowCols[p], k)
		sk.pivotSlot[k] = slotOfSorted(sk.rowCols[p], k)
		rows := sk.stepRows[k]
		sk.stepSlots[k] = make([]int, len(rows))
		sk.stepStart[k] = make([]int, len(rows))
		for t, i := range rows {
			sk.stepSlots[k][t] = slotOfSorted(sk.rowCols[i], k)
			sk.stepStart[k][t] = firstAboveSorted(sk.rowCols[i], k)
		}
	}
	lu.skel = sk
}

// luSkeletonCandidates 取「列 k 上有槽位、且当前位置 > k」的行身份，按当前位置升序。
func luSkeletonCandidates(colRows []int, posOfID []int, k int) []int {
	rows := make([]int, 0, len(colRows))
	for _, i := range colRows {
		if posOfID[i] > k {
			rows = append(rows, i)
		}
	}
	slices.SortFunc(rows, func(a, b int) int { return posOfID[a] - posOfID[b] })
	return rows
}

// luSkeletonUnionAbove 把 src 中列号 > k 的槽位并入 dst（升序去重），并把新增列登记进
// colRows（供后续步骤取候选）。只做结构，不写值。
func luSkeletonUnionAbove(dst, src []int, from int, id int, colRows [][]int) []int {
	if from == len(src) {
		return dst
	}
	out := make([]int, 0, len(dst)+len(src)-from)
	a, b := 0, from
	for a < len(dst) || b < len(src) {
		switch {
		case b == len(src):
			out = append(out, dst[a])
			a++
		case a == len(dst) || dst[a] > src[b]:
			out = append(out, src[b])
			colRows[src[b]] = append(colRows[src[b]], id)
			b++
		case dst[a] < src[b]:
			out = append(out, dst[a])
			a++
		default:
			out = append(out, dst[a])
			a++
			b++
		}
	}
	return out
}

// swapEliminationRows 交换位置 k 与 pos 的消元状态：交换 L 的前 k 列（已算出的乘子）、
// 更新置换记录、同步行身份表。U 的行按身份索引，不需要搬运。
// 动态路径与骨架重放共用本函数，保证两条路径的 L 行序与置换序列逐字一致。
func (lu *luSparse[T]) swapEliminationRows(k, pos int, idAtPos []int) {
	if pos == k {
		return
	}
	// 对于稀疏矩阵，交换 U 的行，并更新置换矩阵。
	// L 必须只交换前 k 列（已计算出的乘子），与稠密 LU 一致；
	// 若整行交换 L，会把对角元素 1 移到次对角位置（如 L[maxRow][k]），
	// 在前向回代中被当作乘子读取，污染解向量。
	for j := 0; j < k; j++ {
		vk := lu.L.Get(k, j)
		vm := lu.L.Get(pos, j)
		lu.L.Set(k, j, vm)
		lu.L.Set(pos, j, vk)
	}
	lu.updatePermutation(k, pos)
	if idAtPos != nil {
		idAtPos[k], idAtPos[pos] = idAtPos[pos], idAtPos[k]
	}
}

// numericReplay 是数值阶段：按符号骨架重放消元，行列结构只读、只写值槽。
// 每一步都校验主元行是否仍与骨架记录一致；不一致（数值路径变了）→ 返回 errSkeletonStale。
// 与 decomposeWithPerm 的数值路径逐字对齐：同一个消元顺序、同一个因子表达式、
// 每个元素每步只被写一次 ⇒ 浮点累加顺序不变。
func (lu *luSparse[T]) numericReplay(matrix Matrix[T]) error {
	sk := lu.skel
	n := lu.n
	if sk == nil || sk.n != n || len(lu.perm) != n {
		return errSkeletonStale
	}
	pinv := make([]int, n)
	for i, p := range lu.perm {
		pinv[p] = i
	}
	// 初始值写入固定槽位：槽位是符号上界，初始结构必然被包含；只写值，不动结构。
	for i := 0; i < n; i++ {
		vals := sk.rowVals[i]
		for t := range vals {
			vals[t] = 0
		}
		cols, mv := matrix.GetRow(lu.perm[i])
		if mv == nil || mv.Length() != len(cols) {
			return errSkeletonStale
		}
		for idx, c := range cols {
			slot := slotOfSorted(sk.rowCols[i], pinv[c])
			if slot < 0 {
				return errSkeletonStale
			}
			vals[slot] = mv.Get(idx)
		}
	}
	// L 与置换初始化（与动态路径一致）。
	lu.L.Zero()
	for i := 0; i < n; i++ {
		lu.P[i] = i
		lu.pinverse[i] = i
		lu.L.Set(i, i, T(1))
	}
	idAtPos := make([]int, n)
	posOfID := make([]int, n)
	for i := 0; i < n; i++ {
		idAtPos[i], posOfID[i] = i, i
	}
	for k := 0; k < n; k++ {
		// --- 部分主元选择：先读 (k,k)，再按「交换前位置」升序扫描（与动态路径同序）---
		var prePivot T
		if s := slotOfSorted(sk.rowCols[idAtPos[k]], k); s >= 0 {
			prePivot = sk.rowVals[idAtPos[k]][s]
		}
		if !IsValidFloat64(Abs(prePivot)) {
			return fmt.Errorf("lu sparse decompose: pivot contains NaN/Inf at k=%d", k)
		}
		maxAbsVal := Abs(prePivot)
		winner := idAtPos[k]
		for _, i := range sk.searchRows[k] {
			if v := Abs(sk.rowVals[i][slotOfSorted(sk.rowCols[i], k)]); v > maxAbsVal {
				maxAbsVal = v
				winner = i
			}
		}
		if winner != sk.pivotID[k] {
			return errSkeletonStale // 主元行变了：骨架记录的消元计划不成立
		}
		if maxAbsVal < Epsilon {
			return fmt.Errorf("lu sparse decompose: matrix is singular or nearly singular (k=%d, maxAbs=%g)", k, maxAbsVal)
		}
		swapPos := posOfID[winner]
		if swapPos != k {
			lu.swapEliminationRows(k, swapPos, idAtPos)
			posOfID[idAtPos[k]], posOfID[idAtPos[swapPos]] = k, swapPos
		}
		// --- 消元：只写值槽 ---
		var pivotVal T
		if s := sk.pivotSlot[k]; s >= 0 {
			pivotVal = sk.rowVals[sk.pivotID[k]][s]
		}
		pivotVals := sk.rowVals[sk.pivotID[k]]
		pivotCols := sk.rowCols[sk.pivotID[k]]
		pivotFrom := sk.pivotFrom[k]
		rows := sk.stepRows[k]
		slots := sk.stepSlots[k]
		for t, i := range rows {
			valIK := sk.rowVals[i][slots[t]]
			if valIK == 0 { // 结构零（该列无此行元素），跳过
				continue
			}
			factor := valIK / pivotVal
			lu.L.Set(posOfID[i], k, factor)
			sk.rowVals[i][slots[t]] = 0
			if !luSkeletonAxpyAbove(sk.rowVals[i], sk.rowCols[i], sk.stepStart[k][t],
				pivotVals, pivotCols, pivotFrom, -factor) {
				return errSkeletonStale
			}
		}
	}
	// U 按最终位置输出：结构到此冻结，骨架模式下不维护列索引（只读路径不依赖它）。
	us := lu.U.(*rowSparseMatrix[T])
	for c := range us.colIndex {
		us.colIndex[c] = us.colIndex[c][:0]
	}
	// 输出时去掉已消去的次对角槽位（动态路径会把它们置零并删除）：U 的最终结构不含 j < pos，
	// 既省内存也让 SolveReuse 的回代扫描长度与动态路径一致。
	for pos := 0; pos < n; pos++ {
		id := idAtPos[pos]
		start := firstAboveSorted(sk.rowCols[id], sk.pivotStep[id]-1)
		us.colInd[pos] = sk.rowCols[id][start:]
		us.val[pos] = sk.rowVals[id][start:]
	}
	return nil
}

// decomposeWithPerm 按当前 lu.perm 构建 U 并执行消元（动态数值阶段：结构随消元生长）。
// 同时记录主元行身份序列 lu.pivotID，供 captureSkeleton 固化符号骨架。
func (lu *luSparse[T]) decomposeWithPerm(matrix Matrix[T]) error {
	pinv := make([]int, lu.n)
	for i, p := range lu.perm {
		pinv[p] = i
	}
	// 行身份表：identity[pos] = 初始（perm 后）行位置；随行交换同步，用来把「当前行」映射回固定身份。
	identity := make([]int, lu.n)
	for i := range identity {
		identity[i] = i
	}
	if cap(lu.pivotID) < lu.n {
		lu.pivotID = make([]int, lu.n)
	}
	lu.pivotID = lu.pivotID[:lu.n]

	// 自定义初始化：L 为单位阵，U = P·A·Pᵀ（行按 perm、列按 pinv 映射）
	lu.L.Zero()
	lu.U.Zero()
	for i := 0; i < lu.n; i++ {
		lu.P[i] = i
		lu.pinverse[i] = i
		lu.L.Set(i, i, T(1))
	}
	for i := 0; i < lu.n; i++ {
		cols, vals := matrix.GetRow(lu.perm[i])
		for idx, c := range cols {
			lu.U.Set(i, pinv[c], vals.Get(idx))
		}
	}

	for k := 0; k < lu.n; k++ {
		// --- 部分主元选择 ---
		// 沿列 k 的非零行集合搜索（列索引），避免行存储下遍历全部行
		// 线性扫描（CPU8 实测主元搜索占分解 40%+ CPU 的瓶颈）。
		us := lu.U.(*rowSparseMatrix[T])
		maxRow := k
		pivotVal := lu.U.Get(k, k)
		// 防止 NaN/Inf 主元绕过奇异矩阵检测，污染解向量
		if !IsValidFloat64(Abs(pivotVal)) {
			return fmt.Errorf("lu sparse decompose: pivot contains NaN/Inf at k=%d", k)
		}
		maxAbsVal := Abs(pivotVal)
		// 拷贝并按行号排序：保持与行序扫描一致的主元选择（平局时选最小行号），
		// 避免 colIndex 无序成员顺序改变数值路径（时序电路对舍入敏感）。
		pivotRows := append([]int(nil), us.ColumnRows(k)...)
		sort.Ints(pivotRows)
		for _, i := range pivotRows {
			if i <= k {
				continue
			}
			if v := Abs(lu.U.Get(i, k)); v > maxAbsVal {
				maxAbsVal = v
				maxRow = i
			}
		}

		if maxAbsVal < Epsilon {
			return fmt.Errorf("lu sparse decompose: matrix is singular or nearly singular (k=%d, maxAbs=%g)", k, maxAbsVal)
		}

		if maxRow != k {
			lu.U.SwapRows(k, maxRow)
			lu.swapEliminationRows(k, maxRow, identity)
		}
		lu.pivotID[k] = identity[k]

		// --- 消元过程 ---
		pivotVal = lu.U.Get(k, k)
		// 直接引用主元行内部切片（消元期间行 k 只读：SwapRows 在消元前，
		// 消元中 Set/SetAdd 仅修改行 i>k 与 L），避免 GetRow 每次拷贝分配。
		pivotCols := us.colInd[k]
		pivotVals := us.val[k]

		// 沿列 k 的非零行集合消元（列索引），避免行存储下遍历全部行
		// 线性扫描找列 k 的非零行（消元主循环另一处 O(n) 扫描）。
		// 注意：消元中 Set(i,k,0) 会从 colIndex[k] 删除成员，必须拷贝后遍历。
		colRows := append([]int(nil), us.ColumnRows(k)...)
		sort.Ints(colRows)
		for _, i := range colRows {
			if i <= k {
				continue
			}
			valIK := lu.U.Get(i, k)
			if valIK == 0 { // 结构零（该列无此行元素），跳过
				continue
			}

			factor := valIK / pivotVal
			lu.L.Set(i, k, factor)
			var zero T
			lu.U.Set(i, k, zero)

			// 仅更新主元行中的非零元素对应的列。
			// 不丢弃任何 fill-in：与稠密 LU 数值一致。若用绝对阈值丢弃（如 <Epsilon），
			// 会在近奇异矩阵（如二极管 Rs=0 用 1e9 近似短路）上把后续会成长的有效
			// pivot 提前置零，导致误判奇异。
			// SetAdd 一次扫描完成读-改-写（浮点 a-b ≡ a+(-b) 位级一致，行序不变），
			// 替代原 Get+Set 两次线性扫描——消元内层最大热点。
			for idx, j := range pivotCols {
				if j <= k {
					continue
				}
				us.SetAdd(i, j, -factor*pivotVals[idx])
			}
		}
	}
	return nil
}

// SolveReuse 使用稀疏 LU 分解结果求解 Ax=b。
func (lu *luSparse[T]) SolveReuse(b, x Vector[T]) error {
	if b.Length() != lu.n || x.Length() != lu.n {
		return errors.New("lu sparse solve: vector dimension mismatch")
	}

	// --- 前向替换: Ly = Pb ---
	// 利用 L 矩阵的稀疏性，只对非零元素进行计算
	// 行序经静态重排序（perm）与部分主元（P）两级置换：原矩阵行 = perm[P[i]]
	lu.Y.Zero()
	ls := lu.L.(*rowSparseMatrix[T])
	us := lu.U.(*rowSparseMatrix[T])
	for i := 0; i < lu.n; i++ {
		sum := b.Get(lu.perm[lu.P[i]])
		cols := ls.colInd[i]
		vals := ls.val[i]
		for idx, j := range cols {
			if j < i {
				sum -= vals[idx] * lu.Y.Get(j)
			}
		}
		lu.Y.Set(i, sum)
	}

	// --- 后向回代: Ux = y ---
	// 利用 U 矩阵的稀疏性
	x.Zero()
	for i := lu.n - 1; i >= 0; i-- {
		sum := lu.Y.Get(i)
		diag := lu.U.Get(i, i)

		if Abs(diag) < Epsilon {
			return errors.New("lu sparse solve: division by zero (U diagonal is zero)")
		}

		cols := us.colInd[i]
		vals := us.val[i]
		for idx, j := range cols {
			if j > i {
				sum -= vals[idx] * x.Get(j)
			}
		}
		x.Set(i, sum/diag)
	}

	// 静态重排序逆映射：x 按原矩阵列序输出（x[perm[i]] = x'[i]）
	if lu.perm != nil {
		for i := 0; i < lu.n; i++ {
			lu.Y.Set(lu.perm[i], x.Get(i))
		}
		for i := 0; i < lu.n; i++ {
			x.Set(i, lu.Y.Get(i))
		}
	}
	return nil
}
