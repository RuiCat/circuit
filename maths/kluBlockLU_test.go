package maths

import (
	"math"
	"testing"
)

// TestBlockLUAgainstDense 断言分块 LU 在 BTF 可分的矩阵上与稠密 LU 一致,
// 且确实按块分解并保存了右上块。
func TestBlockLUAgainstDense(t *testing.T) {
	blocks := [][]int{{0, 1, 2}, {3, 4, 5, 6}, {7, 8}}
	A := blockMatrix(blocks)
	n := A.Rows()
	xtrue := make([]float64, n)
	for i := range xtrue {
		xtrue[i] = 1 + 0.3*float64(i)
	}
	b := A.MatrixVectorMultiply(NewDenseVectorWithData(xtrue)).ToDense()

	lu, err := NewBlockLU[float64](n)
	if err != nil {
		t.Fatalf("NewBlockLU: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	bl := lu.(*blockLU[float64])
	if len(bl.blocks) != 3 {
		t.Fatalf("应按 3 块分解,实际 %d", len(bl.blocks))
	}
	hasOff := false
	for bi := range bl.offCols {
		for r := range bl.offCols[bi] {
			if len(bl.offCols[bi][r]) > 0 {
				hasOff = true
			}
		}
	}
	if !hasOff {
		t.Fatal("应保存右上块的非零项")
	}
	x := NewDenseVector[float64](n)
	if err := lu.SolveReuse(NewDenseVectorWithData(b), x); err != nil {
		t.Fatalf("SolveReuse: %v", err)
	}
	if e := relErrInf(x.ToDense(), triLU(t, A, b)); e > 1e-10 {
		t.Fatalf("分块 LU 解与稠密 LU 相对误差 %.3e > 1e-10", e)
	}
	if e := relErrInf(x.ToDense(), xtrue); e > 1e-10 {
		t.Fatalf("分块 LU 解与真解相对误差 %.3e > 1e-10", e)
	}
}

// TestBlockLUSingleSCC 断言单强连通分量(BTF 退回单块)时仍正确求解。
func TestBlockLUSingleSCC(t *testing.T) {
	const n = 6
	A := NewDenseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		A.Set(i, i, 4)
		A.Set(i, (i+1)%n, -1)
	}
	xtrue := make([]float64, n)
	for i := range xtrue {
		xtrue[i] = math.Sin(float64(i) * 0.4)
	}
	b := A.MatrixVectorMultiply(NewDenseVectorWithData(xtrue)).ToDense()
	lu, err := NewBlockLU[float64](n)
	if err != nil {
		t.Fatalf("NewBlockLU: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if got := len(lu.(*blockLU[float64]).blocks); got != 1 {
		t.Fatalf("单 SCC 应退化为 1 块,实际 %d", got)
	}
	x := NewDenseVector[float64](n)
	if err := lu.SolveReuse(NewDenseVectorWithData(b), x); err != nil {
		t.Fatalf("SolveReuse: %v", err)
	}
	if e := relErrInf(x.ToDense(), xtrue); e > 1e-10 {
		t.Fatalf("解与真解相对误差 %.3e > 1e-10", e)
	}
}

// TestBlockLURepeatedValues 断言同结构换值后重复 Decompose 仍正确(连续迭代)。
func TestBlockLURepeatedValues(t *testing.T) {
	blocks := [][]int{{0, 1}, {2, 3, 4}}
	A := blockMatrix(blocks)
	n := A.Rows()
	lu, err := NewBlockLU[float64](n)
	if err != nil {
		t.Fatalf("NewBlockLU: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("第一次 Decompose: %v", err)
	}
	A2 := scaledMatrix(A, 2)
	xtrue := make([]float64, n)
	for i := range xtrue {
		xtrue[i] = 1
	}
	b := A2.MatrixVectorMultiply(NewDenseVectorWithData(xtrue)).ToDense()
	if err := lu.Decompose(A2); err != nil {
		t.Fatalf("第二次 Decompose: %v", err)
	}
	x := NewDenseVector[float64](n)
	if err := lu.SolveReuse(NewDenseVectorWithData(b), x); err != nil {
		t.Fatalf("第二次 SolveReuse: %v", err)
	}
	if e := relErrInf(x.ToDense(), triLU(t, A2, b)); e > 1e-10 {
		t.Fatalf("换值后解与稠密 LU 相对误差 %.3e > 1e-10", e)
	}
}

// TestBlockLUErrors 覆盖构造/维度/未分解/向量维度错误。
func TestBlockLUErrors(t *testing.T) {
	if _, err := NewBlockLU[float64](0); err == nil {
		t.Fatal("维度 0 应返回 error")
	}
	lu, err := NewBlockLU[float64](3)
	if err != nil {
		t.Fatalf("NewBlockLU: %v", err)
	}
	if err := lu.SolveReuse(NewDenseVector[float64](3), NewDenseVector[float64](3)); err == nil {
		t.Fatal("未分解就求解应返回 error")
	}
	if err := lu.Decompose(NewDenseMatrix[float64](3, 2)); err == nil {
		t.Fatal("非方阵应返回 error")
	}
	if err := lu.Decompose(NewDenseMatrix[float64](4, 4)); err == nil {
		t.Fatal("维度不符应返回 error")
	}
	if err := lu.Decompose(tridiagonalMatrix(3)); err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if err := lu.SolveReuse(NewDenseVector[float64](2), NewDenseVector[float64](3)); err == nil {
		t.Fatal("b 维度不符应返回 error")
	}
	if err := lu.SolveReuse(NewDenseVector[float64](3), NewDenseVector[float64](2)); err == nil {
		t.Fatal("x 维度不符应返回 error")
	}
}
