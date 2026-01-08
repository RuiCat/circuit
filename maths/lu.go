package maths

import (
	"errors"
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
			L:        NewSparseMatrix[T](n, n),
			U:        NewSparseMatrix[T](n, n),
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
		maxAbsVal := abs(lu.U.Get(k, k))
		for i := k + 1; i < lu.n; i++ {
			if v := abs(lu.U.Get(i, k)); v > maxAbsVal {
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
		pivotVal := lu.U.Get(k, k)
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
		if abs(diagVal) < Epsilon {
			return errors.New("lu dense solve: division by zero (U diagonal is zero)")
		}
		x.Set(i, sum/diagVal)
	}

	return nil
}

// luSparse 实现稀疏矩阵的 LU 分解。
type luSparse[T Number] struct {
	baseLU[T]
}

// Decompose 对稀疏矩阵执行 LU 分解。
func (lu *luSparse[T]) Decompose(matrix Matrix[T]) error {
	if !matrix.IsSquare() {
		return errors.New("lu sparse decompose: input must be square matrix")
	}
	if matrix.Rows() != lu.n {
		return errors.New("lu sparse decompose: matrix dimension mismatch")
	}

	lu.init(matrix)

	for k := 0; k < lu.n; k++ {
		// --- 部分主元选择 ---
		maxRow := k
		maxAbsVal := abs(lu.U.Get(k, k))
		for i := k + 1; i < lu.n; i++ {
			if v := abs(lu.U.Get(i, k)); v > maxAbsVal {
				maxAbsVal = v
				maxRow = i
			}
		}

		if maxAbsVal < Epsilon {
			return errors.New("lu sparse decompose: matrix is singular or nearly singular")
		}

		if maxRow != k {
			// 对于稀疏矩阵，交换 L 和 U 的行，并更新置换矩阵
			lu.U.SwapRows(k, maxRow)
			lu.L.SwapRows(k, maxRow)
			lu.updatePermutation(k, maxRow)
		}

		// --- 消元过程 ---
		pivotVal := lu.U.Get(k, k)
		// 获取主元所在行的非零元素，以减少不必要的计算
		pivotCols, pivotVals := lu.U.GetRow(k)

		for i := k + 1; i < lu.n; i++ {
			valIK := lu.U.Get(i, k)
			if abs(valIK) < Epsilon { // 如果 (i,k) 元素已为零，则跳过该行
				continue
			}

			factor := valIK / pivotVal
			lu.L.Set(i, k, factor)
			var zero T
			lu.U.Set(i, k, zero)

			// 仅更新主元行中的非零元素对应的列
			for idx, j := range pivotCols {
				if j <= k {
					continue
				}
				updatedVal := lu.U.Get(i, j) - factor*pivotVals.Get(idx)
				// 维持稀疏性：如果更新后的值接近于零，则视其为零
				if abs(updatedVal) < Epsilon {
					lu.U.Set(i, j, zero)
				} else {
					lu.U.Set(i, j, updatedVal)
				}
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
	lu.Y.Zero()
	for i := 0; i < lu.n; i++ {
		sum := b.Get(lu.P[i])
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

		if abs(diag) < Epsilon {
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
	return nil
}

//
