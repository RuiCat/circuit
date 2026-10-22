// GMRES(m):重启式广义最小残差法,解一般(非对称)线性系统 A·x = b,
// 用 Arnoldi 正交化 + Givens 旋转在 Krylov 子空间上最小化残差。

package maths

import (
	"errors"
	"math"
)

// GMRES 用重启式广义最小残差法解 A·x = b。pre 为左预条件子(nil = 不预条件,
// 即对 M⁻¹A 做 Arnoldi);restart 为每轮 Krylov 子空间维数(≥1),maxIter 为
// 总 Arnoldi 步数上限。收敛判据与其它迭代法一致:真实残差 ≤ tol 即停,
// 未达标时 Iters == maxIter(不返回 error);restart < 1 或上三角对角为零 → error。
func GMRES(A Matrix[float64], b, x0 []float64, pre Preconditioner, restart int, tol float64, maxIter int) (IterResult, error) {
	if err := checkIter(A, b, x0, tol, maxIter); err != nil {
		return IterResult{}, err
	}
	if restart < 1 {
		return IterResult{}, errors.New("solve: GMRES 要求 restart ≥ 1")
	}
	n := len(b)
	x := make([]float64, n)
	copy(x, x0)
	res, err := resid(A, b, x)
	if err != nil {
		return IterResult{}, err
	}
	iters := 0
	for res > tol && iters < maxIter {
		limit := restart
		if iters+limit > maxIter {
			limit = maxIter - iters
		}
		if x, res, iters, err = gmresCycle(A, b, x, pre, limit, tol, iters); err != nil {
			return IterResult{}, err
		}
	}
	return IterResult{X: x, Iters: iters, Residual: res}, nil
}

// gmresCycle 在 limit 维 Krylov 子空间上做一次 Arnoldi + Givens 迭代,
// 每步取当前子空间的最小残差解并回代真实残差;返回最新的 x、真实残差与
// 累计的 Arnoldi 步数(未收敛时把最后一次迭代解作为下一轮的初值)。
func gmresCycle(A Matrix[float64], b, x []float64, pre Preconditioner, limit int, tol float64, iters int) ([]float64, float64, int, error) {
	n := len(b)
	ax := make([]float64, n)
	matVec(A, x, ax)
	rb := make([]float64, n)
	for i := range rb {
		rb[i] = b[i] - ax[i]
	}
	r0, err := applyPre(pre, rb)
	if err != nil {
		return nil, 0, 0, err
	}
	beta := math.Sqrt(dot(r0, r0))
	res, err := resid(A, b, x)
	if err != nil {
		return nil, 0, 0, err
	}
	if beta == 0 {
		return x, res, iters, nil
	}
	m := limit
	base := make([]float64, n) // 本轮的初值:Krylov 基由它对应的残差生成
	copy(base, x)
	V := make([][]float64, m+1) // Krylov 基 V[0..m]
	for i := range V {
		V[i] = make([]float64, n)
	}
	for i := range V[0] {
		V[0][i] = r0[i] / beta
	}
	H := make([]float64, (m+1)*m) // H[i*m+j] = h_{i,j}(行优先的 (m+1)×m)
	g := make([]float64, m+1)     // 旋转后的右端项
	cs := make([]float64, m)
	sn := make([]float64, m)
	g[0] = beta
	w := make([]float64, n)
	wPre := make([]float64, n)
	lastJ := 0
	for j := 0; j < m; j++ {
		lastJ = j
		matVec(A, V[j], w)
		if w, err = applyPreTo(wPre, pre, w); err != nil {
			return nil, 0, 0, err
		}
		for i := 0; i <= j; i++ {
			hij := dot(w, V[i])
			H[i*m+j] = hij
			for k := range w {
				w[k] -= hij * V[i][k]
			}
		}
		hNext := math.Sqrt(dot(w, w))
		H[(j+1)*m+j] = hNext
		for i := 0; i < j; i++ {
			aVal, bVal := H[i*m+j], H[(i+1)*m+j]
			H[i*m+j] = cs[i]*aVal + sn[i]*bVal
			H[(i+1)*m+j] = -sn[i]*aVal + cs[i]*bVal
		}
		aVal, bVal := H[j*m+j], H[(j+1)*m+j]
		rot := math.Hypot(aVal, bVal)
		if rot == 0 {
			// 该列已无分量:用单位旋转避免除零,后续由三角回代兜底。
			cs[j], sn[j] = 1, 0
		} else {
			cs[j], sn[j] = aVal/rot, bVal/rot
		}
		H[j*m+j] = rot
		H[(j+1)*m+j] = 0
		g[j+1] = -sn[j] * g[j]
		g[j] = cs[j] * g[j]
		iters++
		// 收敛用 Givens 残差估计 |g_{j+1}| 判停(每步 O(1));只有估计达标时
		// 才回代真实残差校验,避免每步一次 O(n²) 的 resid 扫描。
		if math.Abs(g[j+1]) <= tol {
			next, err := gmresSolution(V, H, g, m, j, base)
			if err != nil {
				return nil, 0, 0, err
			}
			res, err = resid(A, b, next)
			if err != nil {
				return nil, 0, 0, err
			}
			if res <= tol {
				return next, res, iters, nil
			}
			x = next // 估计与真实残差偏离(舍入):保留该解,本轮继续
		}
		if hNext == 0 {
			// 不变子空间:本子空间内的最小残差解已取尽,交由下一轮重启。
			break
		}
		for i := range V[j+1] {
			V[j+1][i] = w[i] / hNext
		}
	}
	// 本轮结束:取最后一列子空间的最小残差解并回代真实残差。
	last, err := gmresSolution(V, H, g, m, lastJ, base)
	if err != nil {
		return nil, 0, 0, err
	}
	res, err = resid(A, b, last)
	if err != nil {
		return nil, 0, 0, err
	}
	return last, res, iters, nil
}

// gmresSolution 解上三角系统 R·y = g(维数 j+1),返回 x₀ + Σ yₖ·Vₖ。
// R 是对角线非零的上三角矩阵;对角为零说明分解崩溃 → error。
func gmresSolution(V [][]float64, H, g []float64, m, j int, x0 []float64) ([]float64, error) {
	y := make([]float64, j+1)
	for k := j; k >= 0; k-- {
		s := g[k]
		for i := k + 1; i <= j; i++ {
			s -= H[k*m+i] * y[i]
		}
		d := H[k*m+k]
		if d == 0 {
			return nil, errors.New("solve: GMRES 上三角对角为零(分解崩溃)")
		}
		y[k] = s / d
	}
	out := make([]float64, len(x0))
	copy(out, x0)
	for k := 0; k <= j; k++ {
		yK := y[k]
		for i := range out {
			out[i] += yK * V[k][i]
		}
	}
	return out, nil
}
