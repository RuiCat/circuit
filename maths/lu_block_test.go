package maths

import (
	"fmt"
	"math"
	"testing"
)

// matrixEquals 比较两个矩阵是否在给定的容差范围内相等。
func matrixEquals[T Number](a, b Matrix[T], tol float64) bool {
	if a.Rows() != b.Rows() || a.Cols() != b.Cols() {
		return false
	}
	for i := 0; i < a.Rows(); i++ {
		for j := 0; j < a.Cols(); j++ {
			if Abs(a.Get(i, j)-b.Get(i, j)) > tol {
				fmt.Printf("Mismatch at (%d, %d): A=%v, B=%v", i, j, a.Get(i, j), b.Get(i, j))
				return false
			}
		}
	}
	return true
}

// vectorEquals 比较两个向量是否在给定的容差范围内相等。
func vectorEquals[T Number](a, b Vector[T], tol float64) bool {
	if a.Length() != b.Length() {
		return false
	}
	for i := 0; i < a.Length(); i++ {
		if Abs(a.Get(i)-b.Get(i)) > tol {
			fmt.Printf("Vector mismatch at index %d: a=%v, b=%v", i, a.Get(i), b.Get(i))
			return false
		}
	}
	return true
}

// getLUMatrices 从 luBlock.A 中提取 L 和 U 矩阵。
func getLUMatrices[T Number](luA Matrix[T]) (Matrix[T], Matrix[T]) {
	n := luA.Rows()
	L := NewDenseMatrix[T](n, n)
	U := NewDenseMatrix[T](n, n)
	for i := 0; i < n; i++ {
		L.Set(i, i, 1.0) // L 的对角线为 1
		for j := 0; j < n; j++ {
			if i > j {
				L.Set(i, j, luA.Get(i, j))
			} else {
				U.Set(i, j, luA.Get(i, j))
			}
		}
	}
	return L, U
}

// multiplyMatrices 计算两个矩阵的乘积。
func multiplyMatrices[T Number](a, b Matrix[T]) Matrix[T] {
	if a.Cols() != b.Rows() {
		panic("matrix dimensions are not compatible for multiplication")
	}

	result := NewDenseMatrix[T](a.Rows(), b.Cols())
	for i := 0; i < a.Rows(); i++ {
		for j := 0; j < b.Cols(); j++ {
			var sum T
			for k := 0; k < a.Cols(); k++ {
				sum += a.Get(i, k) * b.Get(k, j)
			}
			result.Set(i, j, sum)
		}
	}
	return result
}

// TestLUBlockDecomposition 验证分块 LU 分解的正确性：检查分解后 L*U 是否等于置换后的原矩阵 PA。
func TestLUBlockDecomposition(t *testing.T) {
	// 创建一个可逆矩阵
	A := NewDenseMatrix[float64](4, 4)
	A.BuildFromDense([][]float64{
		{2, 3, 1, 5},
		{6, 13, 5, 19},
		{2, 19, 10, 23},
		{4, 10, 11, 31},
	})

	// 创建分块 LU 分解器
	lu, err := NewLUBlock[float64](4)
	if err != nil {
		t.Fatalf("Failed to create LUBlock: %v", err)
	}

	// 执行分解
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("Decomposition failed: %v", err)
	}

	// 从分解结果中提取 L 和 U
	luBlockImpl := lu.(*luBlock[float64])
	luA := luBlockImpl.A
	L, U := getLUMatrices(luA)

	// 计算 L * U (这等于 PA，而非 A)
	reconstructedPA := multiplyMatrices(L, U)

	// 根据置换向量构造 PA（将 A 的行按 perm 重排）
	n := A.Rows()
	PA := NewDenseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		srcRow := luBlockImpl.perm[i]
		for j := 0; j < n; j++ {
			PA.Set(i, j, A.Get(srcRow, j))
		}
	}

	// 比较置换后的矩阵 PA 和重构的矩阵 L*U
	if !matrixEquals(PA, reconstructedPA, 1e-9) {
		t.Errorf("Permuted A (PA) and reconstructed L*U are not equal.\nPA:\n%vReconstructed L*U:\n%vL:\n%vU:\n%vPermutation: %v", PA, reconstructedPA, L, U, luBlockImpl.perm)
	}
}

// TestLUBlockSolve 验证分块 LU 分解后求解线性方程组的正确性：检查求解结果 Ax=b 的精度。
func TestLUBlockSolve(t *testing.T) {
	// 创建矩阵 A 和向量 b
	A := NewDenseMatrix[float64](3, 3)
	A.BuildFromDense([][]float64{
		{1, 2, 3},
		{4, 5, 6},
		{7, 8, 10},
	})

	b := NewDenseVector[float64](3)
	b.BuildFromDense([]float64{6, 15, 25})

	// 预期的解 x = [1, 1, 1]
	expectedX := NewDenseMatrix[float64](3, 1)
	expectedX.BuildFromDense([][]float64{{1}, {1}, {1}})

	// 创建并执行分解
	lu, err := NewLUBlock[float64](3)
	if err != nil {
		t.Fatalf("Failed to create LUBlock: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("Decomposition failed: %v", err)
	}

	// 求解 Ax = b
	x := NewDenseVector[float64](3)
	if err := lu.SolveReuse(b, x); err != nil {
		t.Fatalf("SolveReuse failed: %v", err)
	}

	// 比较计算出的解和预期的解
	if !vectorEquals(expectedX.ToDense(), x, 1e-9) {
		t.Errorf("Solver returned incorrect result.\nExpected: %v\nGot:      %v", expectedX, x)
	}
}

// TestLUBlockDecompositionLarge 验证分块 LU 分解对大型矩阵(超过 BlockThreshold)的正确性：
// 使用对角占优矩阵，检查分解后能否通过 L*U 重构出置换后的原矩阵。
func TestLUBlockDecompositionLarge(t *testing.T) {
	n := 50 // 大于 BlockThreshold
	A := NewDenseMatrix[float64](n, n)

	// 创建一个良态矩阵 (对角占优)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				A.Set(i, j, float64(2*n))
			} else {
				A.Set(i, j, float64(math.Sin(float64(i*j+1))))
			}
		}
	}

	lu, err := NewLUBlock[float64](n)
	if err != nil {
		t.Fatalf("Failed to create LUBlock: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("Decomposition failed for large matrix: %v", err)
	}

	luBlockImpl := lu.(*luBlock[float64])
	luA := luBlockImpl.A
	L, U := getLUMatrices(luA)
	reconstructedPA := multiplyMatrices(L, U)

	// 构造 PA 用于比较
	PA := NewDenseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		srcRow := luBlockImpl.perm[i]
		for j := 0; j < n; j++ {
			PA.Set(i, j, A.Get(srcRow, j))
		}
	}

	if !matrixEquals(PA, reconstructedPA, 1e-9) {
		t.Errorf("Large matrix decomposition failed to reconstruct permuted original matrix.")
	}
}
