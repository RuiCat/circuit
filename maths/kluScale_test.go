// kluScale_test.go：KLU 稀疏分解的规模探针（m4 诊断，1e5 三对角）。
package maths

import "testing"

// TestKluScale 构造 1e5 三对角稀疏矩阵并分解，用于定位 1e5 OOM 是否在 KLU 分解内。
func TestKluScale(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	n := 100000
	A := NewSparseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		A.Set(i, i, 2)
		if i+1 < n {
			A.Set(i, i+1, -1)
			A.Set(i+1, i, -1)
		}
	}
	lu, err := NewLUSparse[float64](n)
	if err != nil {
		t.Fatalf("NewLUSparse: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	t.Logf("KLU 1e5 三对角分解完成")
}
