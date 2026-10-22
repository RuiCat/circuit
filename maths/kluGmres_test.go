package maths

import (
	"math"
	"testing"
)

// TestGMRES 在非对称对角占优矩阵上验证重启式 GMRES:不同重启维数与
// ILU(0) 预条件下均收敛到 tol、解与 NewLU 相对误差 ≤1e-8;并覆盖
// 初值已收敛(Iters==0)、未收敛语义(Iters==maxIter)与非法输入。
func TestGMRES(t *testing.T) {
	const m, tol, maxIter = 6, 1e-10, 2000
	n := m * m
	for _, seed := range []int64{9201, 9202, 9203} {
		A := nonsymmetric2D(t, m, seed)
		b := nonsymmetricRHS(A, n)
		want := triLU(t, A, b)
		ilu, err := NewILU0(A)
		if err != nil {
			t.Fatalf("seed=%d ILU(0): %v", seed, err)
		}
		for _, c := range []struct {
			name    string
			restart int
			pre     Preconditioner
		}{
			{"GMRES(20)", 20, nil},
			{"GMRES(5)", 5, nil},
			{"GMRES(20)+ILU(0)", 20, ilu},
		} {
			got, err := GMRES(A, b, make([]float64, n), c.pre, c.restart, tol, maxIter)
			if err != nil {
				t.Fatalf("seed=%d %s: %v", seed, c.name, err)
			}
			if got.Residual > tol {
				t.Fatalf("seed=%d %s 未收敛:Residual=%.3e Iters=%d", seed, c.name, got.Residual, got.Iters)
			}
			if e := relErrInf(got.X, want); e > 1e-8 {
				t.Fatalf("seed=%d %s 解与 LU 相对误差 %.3e > 1e-8", seed, c.name, e)
			}
			t.Logf("seed=%d %s: Iters=%d Residual=%.3e", seed, c.name, got.Iters, got.Residual)
		}
	}

	A := nonsymmetric2D(t, m, 9201)
	b := nonsymmetricRHS(A, n)
	zero, err := GMRES(A, b, triLU(t, A, b), nil, 10, tol, maxIter)
	if err != nil {
		t.Fatalf("初值已收敛不应返回 error: %v", err)
	}
	if zero.Iters != 0 || zero.Residual > tol {
		t.Fatalf("初值已收敛应 Iters==0:Iters=%d Residual=%.3e", zero.Iters, zero.Residual)
	}
	slow, err := GMRES(A, b, make([]float64, n), nil, 4, 1e-15, 2)
	if err != nil {
		t.Fatalf("未收敛不应返回 error: %v", err)
	}
	if slow.Iters != 2 || slow.Residual <= 1e-15 {
		t.Fatalf("未收敛语义应为 Iters==maxIter:Iters=%d Residual=%.3e", slow.Iters, slow.Residual)
	}
	if _, err := GMRES(A, b, make([]float64, n), nil, 0, tol, maxIter); err == nil {
		t.Fatal("restart=0 应返回 error")
	}
	if _, err := GMRES(A, b, make([]float64, n-1), nil, 10, tol, maxIter); err == nil {
		t.Fatal("x0 维度不符应返回 error")
	}
	bad := append([]float64(nil), b...)
	bad[1] = math.Inf(-1)
	if _, err := GMRES(A, bad, make([]float64, n), nil, 10, tol, maxIter); err == nil {
		t.Fatal("b 含 Inf 应返回 error")
	}
}
