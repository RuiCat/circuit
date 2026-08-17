package maths

import (
	"errors"
	"fmt"
)

const BlockThreshold = 32 // 当矩阵大小小于此值时，切换到基础的 LU 分解算法

// luBlock 使用递归的分块算法实现 LU 分解。
// 对于小于 BlockThreshold 的子块，采用带列选主元 (partial pivoting) 的高斯消去法；
// 主元选择信息通过 perm 置换向量供 SolveReuse 进行行交换。
type luBlock[T Number] struct {
	n    int
	A    Matrix[T] // 存储 L 和 U 组合的矩阵
	perm []int     // 行置换向量，perm[i] 表示置换后第 i 行在原始矩阵中的行号
}

// NewLUBlock 创建一个新的分块 LU 分解器。
func NewLUBlock[T Number](n int) (LU[T], error) {
	if n < 1 {
		return nil, errors.New("lu dimension must be positive")
	}
	perm := make([]int, n)
	for i := range perm {
		perm[i] = i
	}
	return &luBlock[T]{
		n:    n,
		A:    NewDenseMatrix[T](n, n),
		perm: perm,
	}, nil
}

// Decompose 执行分块 LU 分解。
// 它将输入矩阵复制到内部分解矩阵 A 中，并在 A 上就地执行操作。
func (lu *luBlock[T]) Decompose(matrix Matrix[T]) error {
	if !matrix.IsSquare() || matrix.Rows() != lu.n {
		return errors.New("lu block decompose: matrix dimension mismatch")
	}
	matrix.Copy(lu.A)
	return lu.decomposeRecursive(lu.A)
}

// decomposeRecursive 是分块 LU 分解的核心递归函数。
func (lu *luBlock[T]) decomposeRecursive(A Matrix[T]) error {
	n := A.Rows()
	if n <= BlockThreshold {
		return lu.baseCaseLU(A)
	}

	// 将矩阵 A 分为四个子块
	n1 := n / 2
	n2 := n - n1
	A11 := NewSubMatrix(A, 0, 0, n1, n1)
	A12 := NewSubMatrix(A, 0, n1, n1, n2)
	A21 := NewSubMatrix(A, n1, 0, n2, n1)
	A22 := NewSubMatrix(A, n1, n1, n2, n2)

	// 1. 递归分解 A11 -> L11, U11
	if err := lu.decomposeRecursive(A11); err != nil {
		return err
	}

	// 2. 求解 U12 和 L21
	// U12 = L11^-1 * A12  (就地更新 A12)
	solveLowerTriangular(A11, A12)
	// L21 = A21 * U11^-1  (就地更新 A21)
	if err := solveUpperTriangular(A21, A11); err != nil {
		return err
	}

	// 3. 计算舒尔补更新 A22
	// A22 = A22 - L21 * U12
	matrixMultiplySubtract(A22, A21, A12)

	// 4. 递归分解舒尔补 A22
	return lu.decomposeRecursive(A22)
}

// getGlobalRowOffset 计算子矩阵 A 在完整矩阵 lu.A 中的全局行偏移量。
// 通过递归遍历 subMatrix 的 baseMatrix 链来计算累计偏移。
func (lu *luBlock[T]) getGlobalRowOffset(A Matrix[T]) int {
	offset := 0
	current := A
	for {
		sm, ok := current.(*subMatrix[T])
		if !ok {
			break
		}
		offset += sm.rowOffset
		current = sm.baseMatrix
	}
	return offset
}

// baseCaseLU 对小于阈值的矩阵执行一个带部分主元选择的 LU 分解。
// 行置换信息会同步更新到 lu.perm 中，以支持 SolveReuse 的置换求解。
func (lu *luBlock[T]) baseCaseLU(A Matrix[T]) error {
	n := A.Rows()
	// 计算 A 在完整矩阵 lu.A 中的全局行偏移
	globalRowOffset := lu.getGlobalRowOffset(A)
	for k := 0; k < n; k++ {
		// 部分主元选择：在第 k 列搜索绝对值最大的元素
		maxRow := k
		maxAbs := Abs(A.Get(k, k))
		for i := k + 1; i < n; i++ {
			absVal := Abs(A.Get(i, k))
			if absVal > maxAbs {
				maxAbs = absVal
				maxRow = i
			}
		}
		// 防止 NaN/Inf 主元绕过奇异检测（NaN 与任何值比较均为 false）。
		if !IsValidFloat64(maxAbs) {
			return errors.New("matrix contains NaN/Inf pivot")
		}
		if maxAbs < Epsilon {
			return errors.New("matrix is singular or nearly singular")
		}
		if maxRow != k {
			A.SwapRows(maxRow, k)
			lu.perm[globalRowOffset+maxRow], lu.perm[globalRowOffset+k] =
				lu.perm[globalRowOffset+k], lu.perm[globalRowOffset+maxRow]
		}

		pivot := A.Get(k, k)
		for i := k + 1; i < n; i++ {
			factor := A.Get(i, k) / pivot
			A.Set(i, k, factor) // 在下三角部分存储 L 的因子
			for j := k + 1; j < n; j++ {
				val := A.Get(i, j) - factor*A.Get(k, j)
				A.Set(i, j, val)
			}
		}
	}
	return nil
}

// solveLowerTriangular 求解矩阵方程 L * X = B，其中 L 是单位下三角矩阵。
// L 存储在矩阵 A 的下三角部分（对角线为1）。
// 结果 X 会就地覆盖 B。
func solveLowerTriangular[T Number](L Matrix[T], B Matrix[T]) {
	m, n := L.Rows(), B.Cols()
	if L.Cols() != B.Rows() {
		panic("dimension mismatch for solveLowerTriangular")
	}

	for j := 0; j < n; j++ { // 对 B 的每一列
		for i := 0; i < m; i++ { // 求解 X(i, j)
			sum := B.Get(i, j)
			for k := 0; k < i; k++ {
				sum -= L.Get(i, k) * B.Get(k, j) // B(k,j) 已经是计算出的 X(k,j)
			}
			B.Set(i, j, sum)
		}
	}
}

// solveUpperTriangular 求解矩阵方程 X * U = B，其中 U 是上三角矩阵。
// U 存储在矩阵 A 的上三角部分（包含对角线）。
// 若对角线元素绝对值小于 Epsilon，返回奇异错误。
// 结果 X 会就地覆盖 B。
func solveUpperTriangular[T Number](B Matrix[T], U Matrix[T]) error {
	m, n := B.Rows(), U.Cols()
	if B.Cols() != U.Rows() {
		panic("dimension mismatch for solveUpperTriangular")
	}

	for i := 0; i < m; i++ { // 对 B 的每一行
		for j := 0; j < n; j++ { // 求解 X(i, j)
			sum := B.Get(i, j)
			for k := 0; k < j; k++ {
				sum -= B.Get(i, k) * U.Get(k, j) // B(i,k) 已经是计算出的 X(i,k)
			}
			diag := U.Get(j, j)
			if Abs(diag) < Epsilon {
				return fmt.Errorf("solveUpperTriangular: matrix is singular")
			}
			B.Set(i, j, sum/diag)
		}
	}
	return nil
}

// matrixMultiplySubtract 执行矩阵运算 C = C - A * B。
func matrixMultiplySubtract[T Number](C, A, B Matrix[T]) {
	rowsC, colsC := C.Rows(), C.Cols()
	rowsA, colsA := A.Rows(), A.Cols()
	rowsB, colsB := B.Rows(), B.Cols()

	if colsA != rowsB || rowsA != rowsC || colsB != colsC {
		panic("matrix dimension mismatch for multiply-subtract")
	}

	for i := 0; i < rowsA; i++ {
		for j := 0; j < colsB; j++ {
			var sum T
			for k := 0; k < colsA; k++ {
				sum += A.Get(i, k) * B.Get(k, j)
			}
			C.Increment(i, j, -sum)
		}
	}
}

// SolveReuse 使用分解后的 L/U 矩阵求解线性方程组 Ax=b。
// 首先根据分解过程中记录的行置换交换 b 的行，然后执行前向替换和后向回代。
func (lu *luBlock[T]) SolveReuse(b, x Vector[T]) error {
	if b.Length() != lu.n || x.Length() != lu.n {
		return errors.New("lu block solve: vector dimension mismatch")
	}

	y := NewDenseVector[T](lu.n)

	// 根据置换向量构建置换后的 Pb
	pb := NewDenseVector[T](lu.n)
	for i := 0; i < lu.n; i++ {
		pb.Set(i, b.Get(lu.perm[i]))
	}

	// 前向替换: Ly = Pb (L 的对角线为 1)
	for i := 0; i < lu.n; i++ {
		sum := pb.Get(i)
		for j := 0; j < i; j++ {
			sum -= lu.A.Get(i, j) * y.Get(j)
		}
		y.Set(i, sum)
	}

	// 后向回代: Ux = y
	for i := lu.n - 1; i >= 0; i-- {
		sum := y.Get(i)
		for j := i + 1; j < lu.n; j++ {
			sum -= lu.A.Get(i, j) * x.Get(j)
		}
		diag := lu.A.Get(i, i)
		if Abs(diag) < Epsilon {
			return errors.New("lu block solve: division by zero")
		}
		x.Set(i, sum/diag)
	}

	return nil
}
