package maths

import (
	"math"
	"testing"
)

// hilbertMatrix 构造 n 阶 Hilbert 矩阵(病态 SPD,用于检验迭代精化)。
func hilbertMatrix(n int) Matrix[float64] {
	A := NewDenseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			A.Set(i, j, 1/float64(i+j+1))
		}
	}
	return A
}

// residualInf 返回 ‖b − A·x‖∞。
func residualInf(A Matrix[float64], b []float64, x []float64) float64 {
	ax := A.MatrixVectorMultiply(NewDenseVectorWithData(x)).ToDense()
	res := 0.0
	for i := range b {
		if d := math.Abs(b[i] - ax[i]); d > res {
			res = d
		}
	}
	return res
}

// TestSparseLURefine 断言迭代精化:未开 StoreMatrix → error;steps=0 与
// SolveReuse 逐位一致;Hilbert 病态系统上精化后的残差显著低于未精化;
// 连续两次 Decompose 后精化用的是最近一次矩阵(不残留旧副本)。
func TestSparseLURefine(t *testing.T) {
	const n = 8
	A := hilbertMatrix(n)
	xtrue := make([]float64, n)
	for i := range xtrue {
		xtrue[i] = 1 + 0.25*float64(i)
	}
	b := A.MatrixVectorMultiply(NewDenseVectorWithData(xtrue)).ToDense()

	plainLU, err := NewSparseLU[float64](n)
	if err != nil {
		t.Fatalf("NewSparseLU: %v", err)
	}
	if err := plainLU.Decompose(A); err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	ref, ok := plainLU.(Refiner[float64])
	if !ok {
		t.Fatal("稀疏 LU 应实现 Refiner 接口")
	}
	if err := ref.SolveRefined(NewDenseVectorWithData(b), NewDenseVector[float64](n), 1); err == nil {
		t.Fatal("未开启 StoreMatrix 时 SolveRefined 应返回 error")
	}

	lu, err := NewSparseLUWithOptions[float64](n, SparseLUOptions{StoreMatrix: true})
	if err != nil {
		t.Fatalf("NewSparseLUWithOptions: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("Decompose(StoreMatrix): %v", err)
	}
	ref2 := lu.(Refiner[float64])

	xPlain := NewDenseVector[float64](n)
	if err := lu.SolveReuse(NewDenseVectorWithData(b), xPlain); err != nil {
		t.Fatalf("SolveReuse: %v", err)
	}
	xZero := NewDenseVector[float64](n)
	if err := ref2.SolveRefined(NewDenseVectorWithData(b), xZero, 0); err != nil {
		t.Fatalf("SolveRefined(0): %v", err)
	}
	for i := 0; i < n; i++ {
		if xZero.ToDense()[i] != xPlain.ToDense()[i] {
			t.Fatalf("steps=0 应与 SolveReuse 逐位一致(第 %d 位不同)", i)
		}
	}

	xRef := NewDenseVector[float64](n)
	if err := ref2.SolveRefined(NewDenseVectorWithData(b), xRef, 3); err != nil {
		t.Fatalf("SolveRefined(3): %v", err)
	}
	rPlain := residualInf(A, b, xPlain.ToDense())
	rRef := residualInf(A, b, xRef.ToDense())
	ePlain := relErrInf(xPlain.ToDense(), xtrue)
	eRef := relErrInf(xRef.ToDense(), xtrue)
	t.Logf("Hilbert8: plain 残差=%.3e 误差=%.3e; refined 残差=%.3e 误差=%.3e", rPlain, ePlain, rRef, eRef)
	if !(rRef <= rPlain) {
		t.Fatalf("精化残差不应变差:plain=%.3e refined=%.3e", rPlain, rRef)
	}
	if !(eRef < ePlain) {
		t.Fatalf("精化应降低解误差:plain=%.3e refined=%.3e", ePlain, eRef)
	}
	if eRef > 1e-6 {
		t.Fatalf("精化解与真解相对误差 %.3e > 1e-6(已接近 ε·cond 量级)", eRef)
	}

	if err := ref2.SolveRefined(NewDenseVectorWithData(b), NewDenseVector[float64](n), -1); err == nil {
		t.Fatal("steps<0 应返回 error")
	}
	if err := ref2.SolveRefined(NewDenseVector[float64](n-1), NewDenseVector[float64](n), 1); err == nil {
		t.Fatal("b 维度不符应返回 error")
	}

	// 第二次 Decompose 换一套数值:精化必须基于最近一次保存的矩阵。
	A2 := NewDenseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			v := A.Get(i, j)
			if i == j {
				v += 1
			}
			A2.Set(i, j, v)
		}
	}
	if err := lu.Decompose(A2); err != nil {
		t.Fatalf("第二次 Decompose: %v", err)
	}
	b2 := A2.MatrixVectorMultiply(NewDenseVectorWithData(xtrue)).ToDense()
	x2 := NewDenseVector[float64](n)
	if err := ref2.SolveRefined(NewDenseVectorWithData(b2), x2, 2); err != nil {
		t.Fatalf("第二次 SolveRefined: %v", err)
	}
	if e := relErrInf(x2.ToDense(), triLU(t, A2, b2)); e > 1e-8 {
		t.Fatalf("第二次精化解与稠密 LU 相对误差 %.3e > 1e-8(可能用了旧矩阵)", e)
	}
}
