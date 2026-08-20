package maths

import (
	"math"
	"math/rand"
	"testing"
)

// solveLU 用指定 LU 求解器 + 均衡化求解 A·x=b，返回解或错误。
func solveLU(t *testing.T, sparse bool, A Matrix[float64], b Vector[float64]) ([]float64, error) {
	t.Helper()
	n := A.Rows()
	var lu LU[float64]
	var err error
	var row, col []float64
	if sparse {
		lu, _ = NewLUSparse[float64](n)
		row, col, err = EquilibrateAndDecomposeSparse(lu, A)
	} else {
		lu, _ = NewLU[float64](n)
		row, col, err = EquilibrateAndDecompose(lu, A)
	}
	if err != nil {
		return nil, err
	}
	x := NewDenseVector[float64](n)
	if err := SolveEquilibrated(lu, b, x, row, col); err != nil {
		return nil, err
	}
	r := make([]float64, n)
	for i := 0; i < n; i++ {
		r[i] = x.Get(i)
	}
	return r, nil
}

// TestLUSparseNearSingular 回归：近奇异矩阵（二极管 Rs=0 用 1e9 近似短路）下，
// 稀疏 LU 不得因丢弃小 fill-in 而误判奇异。此前 row1 与 row3 近乎相消，
// 残差 ~1e-21 被绝对阈值丢弃后导致后续 pivot 全零 → 误判奇异。
func TestLUSparseNearSingular(t *testing.T) {
	n := 6
	A := NewSparseMatrix[float64](n, n)
	A.Set(0, 0, 0.001)
	A.Set(0, 1, -0.001)
	A.Set(0, 4, 1)
	A.Set(1, 0, -0.001)
	A.Set(1, 1, 1.00000000000101e9)
	A.Set(1, 2, -1e-5)
	A.Set(1, 3, -1e9)
	A.Set(2, 1, -1e-5)
	A.Set(2, 2, 1e-5)
	A.Set(2, 5, 1)
	A.Set(3, 1, -1e9)
	A.Set(3, 3, 1e9)
	A.Set(4, 0, 1)
	A.Set(5, 2, 1)

	b := NewDenseVector[float64](n)
	b.Set(4, 5) // V1 = 5V
	b.Set(5, 0) // V2 = 0V

	xd, errD := solveLU(t, false, A, b)
	xs, errS := solveLU(t, true, A, b)
	if errD != nil {
		t.Fatalf("稠密 LU 意外失败: %v", errD)
	}
	if errS != nil {
		t.Fatalf("稀疏 LU 误判奇异（回归）: %v", errS)
	}
	for i := 0; i < n; i++ {
		if math.Abs(xd[i]-xs[i]) > 1e-6*math.Max(1.0, math.Abs(xd[i])) {
			t.Fatalf("解不一致 i=%d: dense=%g sparse=%g", i, xd[i], xs[i])
		}
	}
}

// TestLUSparseMatchesDenseFuzz 随机稀疏矩阵对拍稀疏 LU 与稠密 LU 的解。
func TestLUSparseMatchesDenseFuzz(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	fails := 0
	for trial := 0; trial < 5000; trial++ {
		n := 2 + rng.Intn(9) // 2..10
		A := NewSparseMatrix[float64](n, n)
		for i := 0; i < n; i++ {
			A.Set(i, i, math.Pow(10, rng.Float64()*8-3)) // 1e-3..1e5
		}
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if i != j && rng.Float64() < 0.3 {
					A.Set(i, j, math.Pow(10, rng.Float64()*8-3)*(1-2*rng.Float64()))
				}
			}
		}
		b := NewDenseVector[float64](n)
		for i := 0; i < n; i++ {
			b.Set(i, rng.Float64()*2-1)
		}

		xd, errD := solveLU(t, false, A, b)
		xs, errS := solveLU(t, true, A, b)

		// 稠密判奇异则稀疏也应判奇异（反之亦然），不一致记为失败
		if (errD == nil) != (errS == nil) {
			fails++
			if fails <= 3 {
				t.Logf("trial %d n=%d: 奇异判定不一致 dense=%v sparse=%v", trial, n, errD, errS)
			}
			continue
		}
		if errD != nil {
			continue // 两者都奇异，一致
		}
		maxErr := 0.0
		for i := 0; i < n; i++ {
			e := math.Abs(xd[i]-xs[i]) / math.Max(1.0, math.Abs(xd[i]))
			if e > maxErr {
				maxErr = e
			}
		}
		if maxErr > 1e-9 {
			fails++
			if fails <= 3 {
				t.Logf("trial %d n=%d: 相对误差 %g", trial, n, maxErr)
			}
		}
	}
	if fails > 0 {
		t.Fatalf("稀疏 LU 与稠密 LU 不一致 %d/5000 例", fails)
	}
}
