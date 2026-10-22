// kluTestHelpers_test.go 提供 klu 迁移测试的公共预言机（移植自算法导论 solve/tridiag_test.go）。
package maths

import (
	"math"
	"testing"
)

// relErrInf 返回 got 相对 want 的无穷范数相对误差。
func relErrInf(got, want []float64) float64 {
	if len(got) != len(want) {
		return math.Inf(1)
	}
	num, den := 0.0, 1.0
	for i := range want {
		num = math.Max(num, math.Abs(got[i]-want[i]))
		den = math.Max(den, math.Abs(want[i]))
	}
	return num / den
}

// gridMatrix 构造 m×m 五点 Poisson 矩阵（移植自 solve/luSparse_bench_test.go）。
func gridMatrix(m int) Matrix[float64] {
	n := m * m
	A := NewDenseMatrix[float64](n, n)
	for i := 0; i < m; i++ {
		for j := 0; j < m; j++ {
			k := i*m + j
			A.Set(k, k, 4)
			if j+1 < m {
				A.Set(k, k+1, -1)
				A.Set(k+1, k, -1)
			}
			if i+1 < m {
				A.Set(k, k+m, -1)
				A.Set(k+m, k, -1)
			}
		}
	}
	return A
}

// sparseGridMatrix 构造 m×m 五点 Poisson 的稀疏版。
func sparseGridMatrix(m int) Matrix[float64] {
	n := m * m
	A := NewKLUMatrix[float64](n, n)
	for i := 0; i < m; i++ {
		for j := 0; j < m; j++ {
			k := i*m + j
			A.Set(k, k, 4)
			if j+1 < m {
				A.Set(k, k+1, -1)
				A.Set(k+1, k, -1)
			}
			if i+1 < m {
				A.Set(k, k+m, -1)
				A.Set(k+m, k, -1)
			}
		}
	}
	return A
}

// grid3DMatrix 构造 m³ 节点七点差分矩阵。
func grid3DMatrix(m int) Matrix[float64] {
	n := m * m * m
	A := NewKLUMatrix[float64](n, n)
	idx := func(i, j, k int) int { return (i*m+j)*m + k }
	for i := 0; i < m; i++ {
		for j := 0; j < m; j++ {
			for k := 0; k < m; k++ {
				p := idx(i, j, k)
				A.Set(p, p, 6)
				if k+1 < m {
					A.Set(p, idx(i, j, k+1), -1)
					A.Set(idx(i, j, k+1), p, -1)
				}
				if j+1 < m {
					A.Set(p, idx(i, j+1, k), -1)
					A.Set(idx(i, j+1, k), p, -1)
				}
				if i+1 < m {
					A.Set(p, idx(i+1, j, k), -1)
					A.Set(idx(i+1, j, k), p, -1)
				}
			}
		}
	}
	return A
}

func triLU(t *testing.T, A Matrix[float64], rhs []float64) []float64 {
	t.Helper()
	lu, err := NewLU[float64](len(rhs))
	if err != nil {
		t.Fatalf("NewLU: %v", err)
	}
	if err := lu.Decompose(A); err != nil {
		t.Fatalf("LU Decompose: %v", err)
	}
	x := NewDenseVector[float64](len(rhs))
	if err := lu.SolveReuse(NewDenseVectorWithData(rhs), x); err != nil {
		t.Fatalf("LU SolveReuse: %v", err)
	}
	return x.ToDense()
}
