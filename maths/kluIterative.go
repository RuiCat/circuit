package maths

import (
	"errors"
	"math"
)

// IterResult 是迭代法结果:解 X、实际迭代次数 Iters 与最终残差 2-范数
// Residual(= ‖b − A·X‖₂)。
//
// 字段名/类型冻结(C13):physics 的 PoissonSOR 直接暴露本类型,改名即破坏其接口。
// 语义:未收敛**不**返回 error,而以 Iters == maxIter 表达(此时 Residual > tol);
// 初值已满足容差时 Iters == 0。
type IterResult struct {
	X        []float64
	Iters    int
	Residual float64
}

// OptimalOmega 返回维数 n 的 Poisson 型矩阵的 SOR 最优松弛因子 2/(1+sin(π/n)),
// 用于 SOR(ω=OptimalOmega(n))(n ≥ 1)。
func OptimalOmega(n int) float64 { return 2 / (1 + math.Sin(math.Pi/float64(n))) }

// checkIter 校验迭代法公共输入:方阵、维度一致、tol 为正有限、maxIter ≥ 1、
// 矩阵(按非零元)与向量元素全部有限。任一不满足 → error(不 panic)。
func checkIter(A Matrix[float64], b, x0 []float64, tol float64, maxIter int) error {
	if A == nil || !A.IsSquare() || A.Rows() != len(b) || len(x0) != len(b) {
		return errors.New("solve: 迭代法维度不符")
	}
	if !IsValidFloat64(tol) || tol <= 0 || maxIter < 1 {
		return errors.New("solve: 迭代法要求 tol 为正有限且 maxIter ≥ 1")
	}
	for i := range b {
		if !IsValidFloat64(b[i]) || !IsValidFloat64(x0[i]) {
			return errors.New("solve: 迭代法输入含 NaN/Inf")
		}
	}
	for i := 0; i < A.Rows(); i++ {
		bad := false
		forEachNonzero(A, i, func(_ int, v float64) {
			if !IsValidFloat64(v) {
				bad = true
			}
		})
		if bad {
			return errors.New("solve: 迭代法矩阵含 NaN/Inf")
		}
	}
	return nil
}

// resid 返回 ‖b − A·x‖₂;任一残差分量非有限 → error。
func resid(A Matrix[float64], b, x []float64) (float64, error) {
	sum := 0.0
	for i := 0; i < len(b); i++ {
		ax := 0.0
		forEachNonzero(A, i, func(j int, v float64) {
			ax += v * x[j]
		})
		d := b[i] - ax
		if !IsValidFloat64(d) {
			return 0, errors.New("solve: 迭代残差非有限")
		}
		sum += d * d
	}
	return math.Sqrt(sum), nil
}

// sweep 做一轮逐行扫描,结果写入 xn(不修改 x):gs=false 全用旧解(加权 Jacobi),
// gs=true 对 j<i 用本次已更新的 xn[j](Gauss-Seidel / SOR);omega 为松弛因子。
func sweep(A Matrix[float64], b, x, xn []float64, omega float64, gs bool) error {
	for i := range b {
		d := A.Get(i, i)
		if d == 0 || !IsValidFloat64(d) {
			return errors.New("solve: 迭代法对角元为零或非有限")
		}
		sum := 0.0
		for j := range b {
			if j == i {
				continue
			}
			v := x[j]
			if gs && j < i {
				v = xn[j]
			}
			sum += A.Get(i, j) * v
		}
		xn[i] = (1-omega)*x[i] + omega*(b[i]-sum)/d
	}
	return nil
}

// iterate 是 Jacobi/GaussSeidel/SOR 共用骨架:每轮扫描后按真实残差判收敛,
// 收敛即提前停止;maxIter 轮后仍未达标则 Iters == maxIter 且 Residual > tol。
func iterate(A Matrix[float64], b, x0 []float64, tol float64, maxIter int,
	omega float64, gs bool) (IterResult, error) {
	if err := checkIter(A, b, x0, tol, maxIter); err != nil {
		return IterResult{}, err
	}
	x := make([]float64, len(b))
	copy(x, x0)
	res, err := resid(A, b, x)
	if err != nil {
		return IterResult{}, err
	}
	xn := make([]float64, len(b))
	iters := 0
	for res > tol && iters < maxIter {
		if err := sweep(A, b, x, xn, omega, gs); err != nil {
			return IterResult{}, err
		}
		copy(x, xn)
		iters++
		if res, err = resid(A, b, x); err != nil {
			return IterResult{}, err
		}
	}
	return IterResult{X: x, Iters: iters, Residual: res}, nil
}

// Jacobi 用 Jacobi 迭代解 A·x = b(x0 为初值,不得为 nil 之外的短切片)。
func Jacobi(A Matrix[float64], b, x0 []float64, tol float64, maxIter int) (IterResult, error) {
	return iterate(A, b, x0, tol, maxIter, 1, false)
}

// GaussSeidel 用 Gauss-Seidel 迭代解 A·x = b(即 SOR 取 omega=1)。
func GaussSeidel(A Matrix[float64], b, x0 []float64, tol float64, maxIter int) (IterResult, error) {
	return iterate(A, b, x0, tol, maxIter, 1, true)
}

// SOR 用逐次超松弛迭代解 A·x = b,omega 为松弛因子(0 < omega < 2 时收敛);
// 仅拒绝非有限的 omega,越界 omega 表现为不收敛而非 error。
func SOR(A Matrix[float64], b, x0 []float64, omega, tol float64, maxIter int) (IterResult, error) {
	if !IsValidFloat64(omega) {
		return IterResult{}, errors.New("solve: SOR 的 omega 非有限")
	}
	return iterate(A, b, x0, tol, maxIter, omega, true)
}

// matVec 计算 y = A·x(维度由调用方保证)。稠密走底层行切片、稀疏只扫非零元,
// 不再按 A.Get(i,j) 做 O(n²) 全表扫描(1 万节点稀疏阵上这是 100 倍级差距)。
func matVec(A Matrix[float64], x, y []float64) {
	if sp, ok := A.(*kluMatrix[float64]); ok {
		for i := 0; i < sp.rows; i++ {
			cols, vals := sp.rowCols[i], sp.rowVals[i]
			s := 0.0
			for k, j := range cols {
				s += vals[k] * x[j]
			}
			y[i] = s
		}
		return
	}
	if dm, ok := A.(*denseMatrix[float64]); ok {
		for i := 0; i < dm.rows; i++ {
			row := dm.DataPtr()[i*dm.cols : (i+1)*dm.cols]
			s := 0.0
			for j, v := range row {
				s += v * x[j]
			}
			y[i] = s
		}
		return
	}
	for i := range y {
		s := 0.0
		forEachNonzero(A, i, func(j int, v float64) {
			s += v * x[j]
		})
		y[i] = s
	}
}

// dot 返回向量内积。
func dot(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

// CG 用共轭梯度法解对称正定系统 A·x = b(非 SPD 不保证收敛)。
// 收敛判据与 IterResult 语义同 iterate:真实残差 ≤ tol 即停,未达标时
// Iters == maxIter;方向导数非正(矩阵非正定)→ error。
func CG(A Matrix[float64], b, x0 []float64, tol float64, maxIter int) (IterResult, error) {
	if err := checkIter(A, b, x0, tol, maxIter); err != nil {
		return IterResult{}, err
	}
	n := len(b)
	x := make([]float64, n)
	copy(x, x0)
	r := make([]float64, n) // r = b − A·x
	p := make([]float64, n) // 搜索方向
	ap := make([]float64, n)
	matVec(A, x, ap)
	for i := range r {
		r[i] = b[i] - ap[i]
	}
	copy(p, r)
	rs := dot(r, r)
	res := math.Sqrt(rs)
	iters := 0
	var err error
	for res > tol && iters < maxIter {
		iters++
		matVec(A, p, ap)
		pap := dot(p, ap)
		if !(pap > 0) {
			return IterResult{}, errors.New("solve: CG 要求对称正定矩阵(方向导数非正)")
		}
		alpha := rs / pap
		for i := range x {
			x[i] += alpha * p[i]
		}
		for i := range r {
			r[i] -= alpha * ap[i]
		}
		if res, err = resid(A, b, x); err != nil {
			return IterResult{}, err
		}
		rsn := dot(r, r)
		beta := rsn / rs
		rs = rsn
		for i := range p {
			p[i] = r[i] + beta*p[i]
		}
	}
	return IterResult{X: x, Iters: iters, Residual: res}, nil
}
