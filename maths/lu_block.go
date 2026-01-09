package maths

import "errors"

const BlockThreshold = 32 // 当矩阵大小小于此值时，切换到基础的 LU 分解算法

// luBlock 使用递归的分块算法实现 LU 分解。
// 注意：此实现不使用主元选择，因此对于某些矩阵可能存在数值不稳定性。
type luBlock[T Number] struct {
	n int
	A Matrix[T] // 存储 L 和 U 组合的矩阵
}

// NewLUBlock 创建一个新的分块 LU 分解器。
func NewLUBlock[T Number](n int) (LU[T], error) {
	if n < 1 {
		return nil, errors.New("lu dimension must be positive")
	}
	return &luBlock[T]{
		n: n,
		A: NewDenseMatrix[T](n, n),
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
	solveUpperTriangular(A21, A11)

	// 3. 计算舒尔补更新 A22
	// A22 = A22 - L21 * U12
	matrixMultiplySubtract(A22, A21, A12)

	// 4. 递归分解舒尔补 A22
	return lu.decomposeRecursive(A22)
}

// baseCaseLU 对小于阈值的矩阵执行一个标准的非主元选择 LU 分解。
func (lu *luBlock[T]) baseCaseLU(A Matrix[T]) error {
	n := A.Rows()
	for k := 0; k < n; k++ {
		pivot := A.Get(k, k)
		if abs(pivot) < Epsilon {
			return errors.New("matrix is singular or nearly singular")
		}
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
// U 存储在矩阵 A 的上三角部分。
// 结果 X 会就地覆盖 B。
func solveUpperTriangular[T Number](B Matrix[T], U Matrix[T]) {
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
			if abs(diag) < Epsilon {
				panic("solveUpperTriangular: matrix is singular")
			}
			B.Set(i, j, sum/diag)
		}
	}
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
// 由于没有主元选择，过程相对简单：前向替换解 Ly=b，然后后向回代解 Ux=y。
func (lu *luBlock[T]) SolveReuse(b, x Vector[T]) error {
	if b.Length() != lu.n || x.Length() != lu.n {
		return errors.New("lu block solve: vector dimension mismatch")
	}

	y := NewDenseVector[T](lu.n)

	// 前向替换: Ly = b (L 的对角线为 1)
	for i := 0; i < lu.n; i++ {
		sum := b.Get(i)
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
		if abs(diag) < Epsilon {
			return errors.New("lu block solve: division by zero")
		}
		x.Set(i, sum/diag)
	}

	return nil
}
