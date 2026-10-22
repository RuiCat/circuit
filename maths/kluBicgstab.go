// BiCGSTAB:稳定双共轭梯度法,解一般(非对称、非 SPD)线性系统 A·x = b,
// 适合 MNA 等非对称矩阵;收敛判据用递推残差判停,退出时回代一次真实残差。

package maths

import (
	"errors"
	"math"
)

// BiCGSTAB 用稳定双共轭梯度法解 A·x = b。pre 为右预条件子(nil = 不预条件),
// 即内部解 A·M⁻¹·y = b 再取 x = M⁻¹·y;tol 为真实残差 2-范数容差,maxIter 为
// 迭代上限。循环内用 ‖r‖ 递推判停(每步 O(nnz),不再每步扫一遍残差),
// 递推残差达标时才回代真实残差;若两者偏离(舍入)按真实残差重启。
// 收敛语义与 Jacobi/CG 一致:真实残差 ≤ tol 即停,未达标时 Iters == maxIter
// (不返回 error)。矩阵非有限、ρ 或 ⟨r̂,v⟩ 为零(真正的 BiCGSTAB 崩溃)→ error。
func BiCGSTAB(A Matrix[float64], b, x0 []float64, pre Preconditioner, tol float64, maxIter int) (IterResult, error) {
	if err := checkIter(A, b, x0, tol, maxIter); err != nil {
		return IterResult{}, err
	}
	n := len(b)
	x := make([]float64, n)
	copy(x, x0)
	r := make([]float64, n)    // 当前残差 r = b − A·x
	rhat := make([]float64, n) // 影子残差(重启用新一轮残差)
	v := make([]float64, n)    // v = A·M⁻¹·p
	p := make([]float64, n)    // 搜索方向
	s := make([]float64, n)
	t := make([]float64, n)
	phat := make([]float64, n)
	shat := make([]float64, n)
	matVec(A, x, r)
	for i := range r {
		r[i] = b[i] - r[i]
	}
	res := math.Sqrt(dot(r, r))
	copy(rhat, r)
	rho, alpha, omega := 1.0, 1.0, 1.0
	iters := 0
	for iters < maxIter {
		if res <= tol {
			trueRes, err := resid(A, b, x)
			if err != nil {
				return IterResult{}, err
			}
			if trueRes <= tol {
				return IterResult{X: x, Iters: iters, Residual: trueRes}, nil
			}
			// 递推残差与真实残差偏离:按真实残差重启(重置方向与影子残差)。
			matVec(A, x, r)
			for i := range r {
				r[i] = b[i] - r[i]
			}
			copy(rhat, r)
			for i := range v {
				v[i] = 0
			}
			for i := range p {
				p[i] = 0
			}
			rho, alpha, omega = 1, 1, 1
			res = math.Sqrt(dot(r, r))
			continue
		}
		rhoNew := dot(rhat, r)
		if rhoNew == 0 || !IsValidFloat64(rhoNew) {
			return IterResult{}, errors.New("solve: BiCGSTAB 崩溃(ρ = 0,残差与影子残差正交)")
		}
		beta := (rhoNew / rho) * (alpha / omega)
		for i := range p {
			p[i] = r[i] + beta*(p[i]-omega*v[i])
		}
		var err error
		if phat, err = applyPreTo(phat, pre, p); err != nil {
			return IterResult{}, err
		}
		matVec(A, phat, v)
		denom := dot(rhat, v)
		if denom == 0 || !IsValidFloat64(denom) {
			return IterResult{}, errors.New("solve: BiCGSTAB 崩溃(⟨r̂, v⟩ = 0)")
		}
		alpha = rhoNew / denom
		for i := range s {
			s[i] = r[i] - alpha*v[i]
		}
		for i := range x {
			x[i] += alpha * phat[i]
		}
		if shat, err = applyPreTo(shat, pre, s); err != nil {
			return IterResult{}, err
		}
		matVec(A, shat, t)
		tt := dot(t, t)
		if tt == 0 {
			// A·M⁻¹·s = 0:非奇异 A 下必有 s = 0(x 已由 alpha 步更新到解),
			// 奇异时 ω 无定义 —— 两种情况都终止本次迭代并报告真实残差。
			for i := range r {
				r[i] = s[i]
			}
			iters++
			res = math.Sqrt(dot(r, r))
			break
		}
		omega = dot(t, s) / tt
		if !IsValidFloat64(omega) {
			return IterResult{}, errors.New("solve: BiCGSTAB 的 ω 非有限")
		}
		for i := range x {
			x[i] += omega * shat[i]
		}
		for i := range r {
			r[i] = s[i] - omega*t[i]
		}
		rho = rhoNew
		iters++
		res = math.Sqrt(dot(r, r))
	}
	trueRes, err := resid(A, b, x)
	if err != nil {
		return IterResult{}, err
	}
	return IterResult{X: x, Iters: iters, Residual: trueRes}, nil
}
