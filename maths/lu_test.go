package maths

import (
	"math/rand"
	"testing"
)

// TestLuDenseSolve 函数验证了针对密集矩阵的 LU 分解和求解过程的正确性。
func TestLuDenseSolve(t *testing.T) {
	// 求解线性方程组 Ax = b
	// A = [[2, 3, 1],
	//      [1, 2, 3],
	//      [3, 1, 2]]
	// b = [9, 6, 8]
	// 预期解 x = [35/18, 29/18, 5/18] ≈ [1.94, 1.61, 0.28]

	// 定义矩阵 A
	a := NewDenseMatrix[float64](3, 3)
	a.Set(0, 0, 2)
	a.Set(0, 1, 3)
	a.Set(0, 2, 1)
	a.Set(1, 0, 1)
	a.Set(1, 1, 2)
	a.Set(1, 2, 3)
	a.Set(2, 0, 3)
	a.Set(2, 1, 1)
	a.Set(2, 2, 2)

	// 定义向量 b
	b := NewDenseVector[float64](3)
	b.Set(0, 9)
	b.Set(1, 6)
	b.Set(2, 8)

	// 创建 LU 分解器
	lu, err := NewLU[float64](3)
	if err != nil {
		t.Fatalf("NewLU failed: %v", err)
	}
	// 对矩阵 A 进行分解
	err = lu.Decompose(a)
	if err != nil {
		t.Fatalf("Decomposition failed: %v", err)
	}

	// 求解 x
	x := NewDenseVector[float64](3)
	lu.SolveReuse(b, x)

	// 验证结果
	expected := []float64{35.0 / 18.0, 29.0 / 18.0, 5.0 / 18.0}
	tolerance := 1e-9

	for i := 0; i < 3; i++ {
		if Abs(x.Get(i)-expected[i]) > tolerance {
			t.Errorf("Element x[%d] is incorrect. Got %f, expected %f", i, x.Get(i), expected[i])
		}
	}
}

// TestLuDenseSolveComplex 函数验证了针对复数密集矩阵的 LU 分解和求解过程的正确性。
func TestLuDenseSolveComplex(t *testing.T) {
	// 求解复数线性方程组 Ax = b
	// A = [[1+2i, 2+3i],
	//      [3+4i, 4+5i]]
	// b = [6+7i, 12+13i]
	// 预期解 x = [1+i, 2-i]

	// 定义复数矩阵 A
	a := NewDenseMatrix[complex128](2, 2)
	a.Set(0, 0, 1+2i)
	a.Set(0, 1, 2+3i)
	a.Set(1, 0, 3+4i)
	a.Set(1, 1, 4+5i)

	// 定义复数向量 b
	b := NewDenseVector[complex128](2)
	b.Set(0, 6+7i)
	b.Set(1, 12+13i)

	// 创建 LU 分解器
	lu, err := NewLU[complex128](2)
	if err != nil {
		t.Fatalf("NewLU failed for complex: %v", err)
	}
	// 对矩阵 A 进行分解
	err = lu.Decompose(a)
	if err != nil {
		t.Fatalf("Decomposition failed for complex: %v", err)
	}

	// 求解 x
	x := NewDenseVector[complex128](2)
	lu.SolveReuse(b, x)

	// 验证结果
	expected := []complex128{1 + 1i, 2 - 1i}
	tolerance := 1e-9

	for i := 0; i < 2; i++ {
		// 使用 abs 计算复数差的模
		if Abs(x.Get(i)-expected[i]) > tolerance {
			t.Errorf("Element x[%d] is incorrect. Got %v, expected %v", i, x.Get(i), expected[i])
		}
	}
}

// TestLuDenseSingular 函数验证 Decompose 方法能否正确识别奇异矩阵。
func TestLuDenseSingular(t *testing.T) {
	// A 是一个奇异矩阵（有一行全为零）
	// A = [[1, 2, 3],
	//      [4, 5, 6],
	//      [0, 0, 0]]
	a := NewDenseMatrix[float64](3, 3)
	a.Set(0, 0, 1)
	a.Set(0, 1, 2)
	a.Set(0, 2, 3)
	a.Set(1, 0, 4)
	a.Set(1, 1, 5)
	a.Set(1, 2, 6)
	a.Set(2, 0, 0)
	a.Set(2, 1, 0)
	a.Set(2, 2, 0)

	lu, err := NewLU[float64](3)
	if err != nil {
		t.Fatalf("NewLU failed: %v", err)
	}
	// 对奇异矩阵进行分解，预期会返回错误
	err = lu.Decompose(a)
	if err == nil {
		t.Fatalf("Decompose should have failed for a singular matrix but it did not")
	}
}

// BenchmarkLuDenseDecompose 测试对密集矩阵进行 LU 分解的性能。
func BenchmarkLuDenseDecompose(b *testing.B) {
	size := 100
	m := NewDenseMatrix[float64](size, size)
	// 填充随机数据以避免对零矩阵的特殊优化
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			m.Set(i, j, rand.Float64())
		}
	}
	lu, err := NewLU[float64](size)
	if err != nil {
		b.Fatalf("NewLU failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := lu.Decompose(m)
		if err != nil {
			b.Fatalf("Decomposition failed during benchmark: %v", err)
		}
	}
}

// BenchmarkLuDenseSolve 测试 LU 分解后求解步骤的性能。
func BenchmarkLuDenseSolve(b *testing.B) {
	size := 100
	m := NewDenseMatrix[float64](size, size)
	vecB := NewDenseVector[float64](size)
	vecX := NewDenseVector[float64](size)

	// 填充随机数据
	for i := 0; i < size; i++ {
		vecB.Set(i, rand.Float64())
		for j := 0; j < size; j++ {
			m.Set(i, j, rand.Float64())
		}
	}
	// 通过增加对角线元素的值来确保矩阵非奇异
	for i := 0; i < size; i++ {
		m.Set(i, i, m.Get(i, i)+1)
	}

	lu, err := NewLU[float64](size)
	if err != nil {
		b.Fatalf("NewLU failed: %v", err)
	}
	// 先进行分解
	err = lu.Decompose(m)
	if err != nil {
		b.Fatalf("Decomposition failed during setup: %v", err)
	}

	b.ResetTimer()
	// 重复执行求解过程
	for i := 0; i < b.N; i++ {
		lu.SolveReuse(vecB, vecX)
	}
}
