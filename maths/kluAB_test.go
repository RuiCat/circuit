// kluAB_test.go 对比旧 NewLUSparse 与迁移后的 NewSparseLU（klu）：解一致性与耗时。
package maths

import (
	"testing"
)

// abMatrix 构造对角占优的非对称稀疏矩阵，保证两条 LU 路径都可解。
func abMatrix(n int) Matrix[float64] {
	A := NewKLUMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		A.Set(i, i, 2*float64(n))
		if i+1 < n {
			A.Set(i, i+1, -1)
			A.Set(i+1, i, -1)
		}
		if i+7 < n {
			A.Set(i, i+7, 0.5)
		}
		if i-3 >= 0 {
			A.Set(i, i-3, -0.25)
		}
	}
	return A
}

// abSolve 用给定 LU 构造函数解 A·x=b。
func abSolve(t *testing.T, newLU func(int) (LU[float64], error), A Matrix[float64], b []float64) []float64 {
	t.Helper()
	lu, err := newLU(len(b))
	if err != nil {
		t.Fatalf("构造 LU 失败: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("Decompose 失败: %v", err)
	}
	x := NewDenseVector[float64](len(b))
	if err := lu.SolveReuse(NewDenseVectorWithData(b), x); err != nil {
		t.Fatalf("SolveReuse 失败: %v", err)
	}
	return x.ToDense()
}

// TestKLUABAgainstLegacy 断言旧稀疏 LU 与 klu 在同类矩阵上解一致、残差同量级。
func TestKLUABAgainstLegacy(t *testing.T) {
	cases := []struct {
		name string
		A    Matrix[float64]
	}{
		{"poisson2d40", sparseGridMatrix(40)},
		{"poisson3d10", grid3DMatrix(10)},
		{"nonsym500", abMatrix(500)},
	}
	for _, c := range cases {
		n := c.A.Rows()
		b := make([]float64, n)
		for i := range b {
			b[i] = 1 + float64(i%7)
		}
		oldX := abSolve(t, func(k int) (LU[float64], error) { return NewLUSparse[float64](k) }, c.A, b)
		newX := abSolve(t, func(k int) (LU[float64], error) { return NewSparseLU[float64](k) }, c.A, b)
		if rel := relErrInf(newX, oldX); rel > 1e-10 {
			t.Errorf("%s: klu 与旧 LU 解相对误差 %.3g > 1e-10", c.name, rel)
		}
		t.Logf("%s: n=%d relErr=%.3g", c.name, n, relErrInf(newX, oldX))
	}
}

// mnaLikeMatrix 构造近似 MNA 的稀疏结构：对角 + 相邻耦合 + 跨节点局部耦合，
// 元素值非对称（模拟导纳与电压源混合），用于电路类 1e4 基准。
func mnaLikeMatrix(n int) Matrix[float64] {
	A := NewKLUMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		A.Set(i, i, 1e-2)
		if i+1 < n {
			A.Set(i, i+1, -1e-4)
			A.Set(i+1, i, -1e-4)
		}
		if i+10 < n {
			A.Set(i, i+10, -1e-5)
			A.Set(i+10, i, -1e-5)
		}
	}
	return A
}

// BenchmarkKLUDecompose1e4MNA 测 klu 在 1e4 节点电路类稀疏结构上的首次分解。
func BenchmarkKLUDecompose1e4MNA(b *testing.B) {
	A := mnaLikeMatrix(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lu, err := NewSparseLU[float64](A.Rows())
		if err != nil {
			b.Fatal(err)
		}
		if err := lu.Decompose(A); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkKLUBTFDecompose1e4MNA 测 BTF 开启时同一矩阵的分解耗时。
func BenchmarkKLUBTFDecompose1e4MNA(b *testing.B) {
	A := mnaLikeMatrix(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lu, err := NewSparseLUWithOptions[float64](A.Rows(), SparseLUOptions{UseBTF: true})
		if err != nil {
			b.Fatal(err)
		}
		if err := lu.Decompose(A); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkKrylovILU01e4MNA 测 ILU(0) 预条件 + BiCGSTAB 在 1e4 节点上的求解耗时。
func BenchmarkKrylovILU01e4MNA(b *testing.B) {
	A := mnaLikeMatrix(10000)
	n := A.Rows()
	pre, err := NewILU0(A)
	if err != nil {
		b.Fatal(err)
	}
	rhs := make([]float64, n)
	for i := range rhs {
		rhs[i] = 1
	}
	x0 := make([]float64, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := BiCGSTAB(A, rhs, x0, pre, 1e-8, 2000); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLegacyDecompose1e4 测旧 NewLUSparse 在 1e4 节点上的首次分解（对照用）。
func BenchmarkLegacyDecompose1e4(b *testing.B) {
	A := sparseGridMatrix(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lu, err := NewLUSparse[float64](A.Rows())
		if err != nil {
			b.Fatal(err)
		}
		if err := lu.Decompose(A); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkKLUDecompose1e4 测 klu 在 1e4 节点五点网格上的首次分解。
func BenchmarkKLUDecompose1e4(b *testing.B) {
	A := sparseGridMatrix(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lu, err := NewSparseLU[float64](A.Rows())
		if err != nil {
			b.Fatal(err)
		}
		if err := lu.Decompose(A); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkKLUSolveReuse1e4 测 klu 在 1e4 节点上的重复求解（结构复用）。
func BenchmarkKLUSolveReuse1e4(b *testing.B) {
	A := sparseGridMatrix(100)
	n := A.Rows()
	lu, err := NewSparseLU[float64](n)
	if err != nil {
		b.Fatal(err)
	}
	if err := lu.Decompose(A); err != nil {
		b.Fatal(err)
	}
	rhs := NewDenseVector[float64](n)
	for i := 0; i < n; i++ {
		rhs.Set(i, 1)
	}
	x := NewDenseVector[float64](n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := lu.SolveReuse(rhs, x); err != nil {
			b.Fatal(err)
		}
	}
}
