package maths

import (
	"math"
	"math/rand"
	"testing"
)

// nonsymmetric2D 构造 m×m 五点对流-扩散型非对称矩阵(维数 n=m²):
// 东/北向与西/南向的随机耦合强度不同(0.8/0.4、0.7/0.6 附近),
// 故 A 非对称;对角 = 该行非对角模之和 + 0.5,严格对角占优 ⇒ 非奇异。
func nonsymmetric2D(t testing.TB, m int, seed int64) Matrix[float64] {
	t.Helper()
	n := m * m
	A := NewDenseMatrix[float64](n, n)
	rng := rand.New(rand.NewSource(seed))
	for i := 0; i < m; i++ {
		for j := 0; j < m; j++ {
			k := i*m + j
			if j+1 < m {
				A.Set(k, k+1, -(0.8 + 0.1*rng.Float64()))
				A.Set(k+1, k, -(0.4 + 0.1*rng.Float64()))
			}
			if i+1 < m {
				A.Set(k, k+m, -(0.7 + 0.1*rng.Float64()))
				A.Set(k+m, k, -(0.6 + 0.1*rng.Float64()))
			}
		}
	}
	for k := 0; k < n; k++ {
		s := 0.0
		for j := 0; j < n; j++ {
			if j != k {
				s += math.Abs(A.Get(k, j))
			}
		}
		A.Set(k, k, s+0.5)
	}
	return A
}

// nonsymmetricRHS 返回 b = A·xtrue(xtrue 已知),供迭代法对照 LU 的精确解。
func nonsymmetricRHS(A Matrix[float64], n int) []float64 {
	xtrue := make([]float64, n)
	for i := range xtrue {
		xtrue[i] = math.Cos(float64(i) * 0.41)
	}
	return A.MatrixVectorMultiply(NewDenseVectorWithData(xtrue)).ToDense()
}

// TestBiCGSTAB 在非对称对角占优矩阵上验证:无预条件与对角/ILU(0) 预条件
// 均收敛到 tol、解与 NewLU 相对误差 ≤1e-8;ILU(0) 的迭代次数严格少于无预条件;
// 并覆盖初值已收敛(Iters==0)、未收敛语义(Iters==maxIter)与非法输入。
func TestBiCGSTAB(t *testing.T) {
	const m, tol, maxIter = 6, 1e-10, 2000
	n := m * m
	for _, seed := range []int64{9101, 9102, 9103} {
		A := nonsymmetric2D(t, m, seed)
		b := nonsymmetricRHS(A, n)
		want := triLU(t, A, b)
		diagIters, iluIters := 0, 0

		plain, err := BiCGSTAB(A, b, make([]float64, n), nil, tol, maxIter)
		if err != nil {
			t.Fatalf("seed=%d BiCGSTAB: %v", seed, err)
		}
		if plain.Residual > tol {
			t.Fatalf("seed=%d 无预条件未收敛:Residual=%.3e Iters=%d", seed, plain.Residual, plain.Iters)
		}
		if e := relErrInf(plain.X, want); e > 1e-8 {
			t.Fatalf("seed=%d 无预条件解与 LU 相对误差 %.3e > 1e-8", seed, e)
		}

		diag, err := NewDiagonalPreconditioner(A)
		if err != nil {
			t.Fatalf("seed=%d 对角预条件: %v", seed, err)
		}
		ilu, err := NewILU0(A)
		if err != nil {
			t.Fatalf("seed=%d ILU(0): %v", seed, err)
		}
		for _, c := range []struct {
			name string
			pre  Preconditioner
		}{{"对角", diag}, {"ILU(0)", ilu}} {
			got, err := BiCGSTAB(A, b, make([]float64, n), c.pre, tol, maxIter)
			if err != nil {
				t.Fatalf("seed=%d %s 预条件: %v", seed, c.name, err)
			}
			if got.Residual > tol {
				t.Fatalf("seed=%d %s 未收敛:Residual=%.3e Iters=%d", seed, c.name, got.Residual, got.Iters)
			}
			if e := relErrInf(got.X, want); e > 1e-8 {
				t.Fatalf("seed=%d %s 解与 LU 相对误差 %.3e > 1e-8", seed, c.name, e)
			}
			if c.pre == diag {
				diagIters = got.Iters
			} else {
				iluIters = got.Iters
			}
		}
		if !(iluIters < plain.Iters) {
			t.Fatalf("seed=%d ILU(0) Iters=%d 未少于无预条件 Iters=%d", seed, iluIters, plain.Iters)
		}
		t.Logf("seed=%d Iters 无预条件=%d 对角=%d ILU(0)=%d", seed, plain.Iters, diagIters, iluIters)
	}

	A := nonsymmetric2D(t, m, 9101)
	b := nonsymmetricRHS(A, n)
	zero, err := BiCGSTAB(A, b, triLU(t, A, b), nil, tol, maxIter)
	if err != nil {
		t.Fatalf("初值已收敛不应返回 error: %v", err)
	}
	if zero.Iters != 0 || zero.Residual > tol {
		t.Fatalf("初值已收敛应 Iters==0:Iters=%d Residual=%.3e", zero.Iters, zero.Residual)
	}
	slow, err := BiCGSTAB(A, b, make([]float64, n), nil, 1e-15, 1)
	if err != nil {
		t.Fatalf("未收敛不应返回 error: %v", err)
	}
	if slow.Iters != 1 || slow.Residual <= 1e-15 {
		t.Fatalf("未收敛语义应为 Iters==maxIter:Iters=%d Residual=%.3e", slow.Iters, slow.Residual)
	}
	if _, err := BiCGSTAB(A, b, make([]float64, n-1), nil, tol, maxIter); err == nil {
		t.Fatal("x0 维度不符应返回 error")
	}
	if _, err := BiCGSTAB(A, b, make([]float64, n), nil, tol, 0); err == nil {
		t.Fatal("maxIter=0 应返回 error")
	}
	bad := append([]float64(nil), b...)
	bad[0] = math.NaN()
	if _, err := BiCGSTAB(A, bad, make([]float64, n), nil, tol, maxIter); err == nil {
		t.Fatal("b 含 NaN 应返回 error")
	}
}
