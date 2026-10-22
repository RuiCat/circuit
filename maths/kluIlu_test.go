package maths

import (
	"math"
	"runtime"
	"testing"
)

// tridiagonalMatrix 构造 n 阶对称三对角矩阵:对角 4、次对角 −1(严格对角占优)。
// 三对角结构下 ILU(0) 不丢 fill-in,故分解是精确的(M = A)。
func tridiagonalMatrix(n int) Matrix[float64] {
	A := NewDenseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		A.Set(i, i, 4)
		if i+1 < n {
			A.Set(i, i+1, -1)
			A.Set(i+1, i, -1)
		}
	}
	return A
}

// TestILU0ExactOnTridiagonal 断言三对角矩阵上的 ILU(0) 是精确分解:
// Apply 与 NewLU 解一致(相对误差 ≤1e-12),且作为预条件时 BiCGSTAB 一步收敛。
func TestILU0ExactOnTridiagonal(t *testing.T) {
	const n = 24
	A := tridiagonalMatrix(n)
	pre, err := NewILU0(A)
	if err != nil {
		t.Fatalf("NewILU0: %v", err)
	}
	r := make([]float64, n)
	for i := range r {
		r[i] = math.Sin(float64(i) * 0.73)
	}
	got, err := pre.Apply(r)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := triLU(t, A, r)
	if e := relErrInf(got, want); e > 1e-12 {
		t.Fatalf("ILU(0) 在三对角上应精确:M⁻¹r 与 LU 解相对误差 %.3e > 1e-12", e)
	}

	b := nonsymmetricRHS(tridiagonalMatrix(n), n)
	res, err := BiCGSTAB(A, b, make([]float64, n), pre, 1e-12, 10)
	if err != nil {
		t.Fatalf("BiCGSTAB+ILU(0): %v", err)
	}
	if res.Iters != 1 {
		t.Fatalf("精确预条件应在 1 步收敛:Iters=%d Residual=%.3e", res.Iters, res.Residual)
	}
	if e := relErrInf(res.X, triLU(t, A, b)); e > 1e-10 {
		t.Fatalf("解与 LU 相对误差 %.3e > 1e-10", e)
	}
}

// TestNewILU0Errors 覆盖 ILU(0) 的构造与使用错误:非方阵、空矩阵、
// 零主元、输入含 NaN、Apply 维度不符。
func TestNewILU0Errors(t *testing.T) {
	if _, err := NewILU0(NewDenseMatrix[float64](3, 2)); err == nil {
		t.Fatal("非方阵应返回 error")
	}
	if _, err := NewILU0(NewDenseMatrix[float64](0, 0)); err == nil {
		t.Fatal("空矩阵应返回 error")
	}
	if _, err := NewILU0(NewDenseMatrix[float64](3, 3)); err == nil {
		t.Fatal("零主元应返回 error")
	}
	nan := NewDenseMatrix[float64](2, 2)
	nan.Set(0, 0, math.NaN())
	if _, err := NewILU0(nan); err == nil {
		t.Fatal("含 NaN 应返回 error")
	}
	pre, err := NewILU0(tridiagonalMatrix(4))
	if err != nil {
		t.Fatalf("NewILU0: %v", err)
	}
	if _, err := pre.Apply(make([]float64, 3)); err == nil {
		t.Fatal("Apply 维度不符应返回 error")
	}
}

// TestDiagonalPreconditioner 断言对角预条件等于逐元素乘 1/A[i][i],
// 并覆盖非方阵、零对角元、维度不符与 applyPre 的恒等/长度校验。
func TestDiagonalPreconditioner(t *testing.T) {
	A := tridiagonalMatrix(5)
	pre, err := NewDiagonalPreconditioner(A)
	if err != nil {
		t.Fatalf("NewDiagonalPreconditioner: %v", err)
	}
	r := []float64{1, -2, 3, -4, 5}
	got, err := pre.Apply(r)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for i := range r {
		if want := r[i] / A.Get(i, i); got[i] != want {
			t.Fatalf("Apply[%d] = %v,期望 %v", i, got[i], want)
		}
	}
	if _, err := NewDiagonalPreconditioner(NewDenseMatrix[float64](2, 3)); err == nil {
		t.Fatal("非方阵应返回 error")
	}
	if _, err := NewDiagonalPreconditioner(NewDenseMatrix[float64](2, 2)); err == nil {
		t.Fatal("零对角元应返回 error")
	}
	if _, err := pre.Apply(r[:4]); err == nil {
		t.Fatal("Apply 维度不符应返回 error")
	}

	ident, err := applyPre(nil, r)
	if err != nil {
		t.Fatalf("applyPre(nil): %v", err)
	}
	ident[0] = 99
	if r[0] != 1 {
		t.Fatal("applyPre(nil) 应返回副本,不应别名输入")
	}
	small, err := NewDiagonalPreconditioner(tridiagonalMatrix(4))
	if err != nil {
		t.Fatalf("NewDiagonalPreconditioner: %v", err)
	}
	if _, err := applyPre(small, r); err == nil {
		t.Fatal("预条件子长度不符应返回 error")
	}
}

// TestILU0MemoryLinear 断言 ILU(0) 的存储随 nnz 线性、不再 n×n:
// 100×100 网格(n=10000,nnz≈5n)的建表分配应远小于 n²/10 字节
// (稠密 n×n 存储 + bool 模式约 9n² ≈ 900MB,是此前的规模化阻塞)。
func TestILU0MemoryLinear(t *testing.T) {
	const m = 100
	n := m * m
	A := sparseGridMatrix(m)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if _, err := NewILU0(A); err != nil {
		t.Fatalf("NewILU0: %v", err)
	}
	runtime.ReadMemStats(&after)
	delta := after.TotalAlloc - before.TotalAlloc
	if limit := uint64(n * n / 10); delta > limit {
		t.Fatalf("ILU(0) 建表分配 %d 字节,超过 n²/10=%d(疑似退回稠密存储)", delta, limit)
	}
	t.Logf("n=%d nnz=%d,ILU(0) 分配 %d 字节(稠密下界约 %d 字节)", n, A.NonZeroCount(), delta, 9*n*n)
}
