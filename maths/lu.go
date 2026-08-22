package maths

import (
	"errors"
	"fmt"
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

// luSparse 实现稀疏矩阵的 LU 分解。
type luSparse[T Number] struct {
	baseLU[T]
	perm    []int  // 静态最小度重排序：U 的第 i 行 = 原矩阵 perm[i] 行（宽行排最后，抑制 fill-in）
	permSig uint64 // 矩阵结构签名（每行非零数的 FNV hash）：结构不变时复用 perm，避免每步重复最小度排序
}

// computeStaticPerm 计算静态重排置换，按矩阵结构自适应选择：
//   - 度数均匀（局部带状/低连接，maxDeg ≲ 10×avgDeg）：保持自然顺序。
//     带状结构 fill-in 仅 ~1.5x；强行最小度会打乱带宽且被部分主元破坏，反而 fill-in 爆炸。
//   - 存在 hub 节点（如电源/时钟总线，maxDeg >> avgDeg）：最小度重排，
//     避免高连接密度矩阵（如 RTL CPU）的 fill-in 爆炸。
func (lu *luSparse[T]) computeStaticPerm(matrix Matrix[T]) []int {
	// 小矩阵（<500 节点）fill-in 温和，保持原顺序以维持既有数值路径
	// （消元顺序改变会引入不同舍入误差，破坏边沿敏感的时序电路行为）。
	if lu.n < 500 {
		return identityPerm(lu.n)
	}
	// 结构签名：每行非零数的 FNV hash（电路每步矩阵结构不变，值变化不影响签名）。
	sig := uint64(14695981039346656037)
	adj := make([][]int, lu.n)
	total := 0
	maxDeg := 0
	for i := 0; i < lu.n; i++ {
		cols, _ := matrix.GetRow(i)
		adj[i] = cols
		d := len(cols)
		sig = (sig ^ uint64(d)) * 1099511628211
		total += d
		if d > maxDeg {
			maxDeg = d
		}
	}
	// 结构未变：复用缓存的 perm，避免每步重复最小度排序（CPU8 实测占 22% CPU）。
	if sig == lu.permSig && lu.perm != nil {
		return lu.perm
	}
	var perm []int
	avgDeg := float64(total) / float64(lu.n)
	if float64(maxDeg) > 10*avgDeg {
		perm = MinimumDegreeOrder(lu.n, adj)
	} else {
		perm = identityPerm(lu.n)
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
// 先按静态最小度重排序（宽行排最后，抑制 fill-in）尝试；
// 若该顺序触发数值奇异（近奇异矩阵对消元顺序敏感），回退原顺序重试一次。
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
	if err := lu.decomposeWithPerm(matrix); err == nil {
		return nil
	} else if !strings.Contains(err.Error(), "singular") {
		return err
	}
	// 奇异回退：原顺序（数值路径不同，可能避免假奇异）
	lu.perm = make([]int, lu.n)
	for i := range lu.perm {
		lu.perm[i] = i
	}
	return lu.decomposeWithPerm(matrix)
}

// decomposeWithPerm 按当前 lu.perm 构建 U 并执行消元。
func (lu *luSparse[T]) decomposeWithPerm(matrix Matrix[T]) error {
	pinv := make([]int, lu.n)
	for i, p := range lu.perm {
		pinv[p] = i
	}

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
		maxRow := k
		pivotVal := lu.U.Get(k, k)
		// 防止 NaN/Inf 主元绕过奇异矩阵检测，污染解向量
		if !IsValidFloat64(Abs(pivotVal)) {
			return fmt.Errorf("lu sparse decompose: pivot contains NaN/Inf at k=%d", k)
		}
		maxAbsVal := Abs(pivotVal)
		for i := k + 1; i < lu.n; i++ {
			if v := Abs(lu.U.Get(i, k)); v > maxAbsVal {
				maxAbsVal = v
				maxRow = i
			}
		}

		if maxAbsVal < Epsilon {
			return fmt.Errorf("lu sparse decompose: matrix is singular or nearly singular (k=%d, maxAbs=%g)", k, maxAbsVal)
		}

		if maxRow != k {
			// 对于稀疏矩阵，交换 U 的行，并更新置换矩阵。
			// L 必须只交换前 k 列（已计算出的乘子），与稠密 LU 一致；
			// 若整行交换 L，会把对角元素 1 移到次对角位置（如 L[maxRow][k]），
			// 在前向回代中被当作乘子读取，污染解向量。
			lu.U.SwapRows(k, maxRow)
			for j := 0; j < k; j++ {
				vk := lu.L.Get(k, j)
				vm := lu.L.Get(maxRow, j)
				lu.L.Set(k, j, vm)
				lu.L.Set(maxRow, j, vk)
			}
			lu.updatePermutation(k, maxRow)
		}

		// --- 消元过程 ---
		pivotVal = lu.U.Get(k, k)
		// 获取主元所在行的非零元素，以减少不必要的计算
		pivotCols, pivotVals := lu.U.GetRow(k)

		for i := k + 1; i < lu.n; i++ {
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
			for idx, j := range pivotCols {
				if j <= k {
					continue
				}
				lu.U.Set(i, j, lu.U.Get(i, j)-factor*pivotVals.Get(idx))
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
	for i := 0; i < lu.n; i++ {
		sum := b.Get(lu.perm[lu.P[i]])
		cols, vals := lu.L.GetRow(i)
		for idx, j := range cols {
			if j < i {
				sum -= vals.Get(idx) * lu.Y.Get(j)
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

		cols, vals := lu.U.GetRow(i)
		for idx, j := range cols {
			if j > i {
				sum -= vals.Get(idx) * x.Get(j)
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
