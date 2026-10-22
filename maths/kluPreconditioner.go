// 预条件子接口与雅可比对角预条件:为 Krylov 迭代法提供 M⁻¹ 的近似,
// 加速收敛但不改变解(判据仍用真实残差)。

package maths

import (
	"errors"
	"fmt"
)

// Preconditioner 是线性预条件子:Apply 返回向量 M⁻¹·r(新切片,不修改 r)。
// 实现必须保持长度不变;返回 error 表示分解失效(零主元)或结果非有限。
type Preconditioner interface {
	Apply(r []float64) ([]float64, error)
}

// diagonalPreconditioner 是按对角元缩放的雅可比预条件子 M = diag(A)。
type diagonalPreconditioner struct {
	invDiag []float64 // 1/A[i][i]
}

// NewDiagonalPreconditioner 用 A 的对角元构造雅可比预条件子 M = diag(A)。
// A 非方阵/维度为 0/对角元为零或非有限 → error。
func NewDiagonalPreconditioner(A Matrix[float64]) (Preconditioner, error) {
	if A == nil || !A.IsSquare() || A.Rows() == 0 {
		return nil, errors.New("solve: 对角预条件要求非空方阵")
	}
	n := A.Rows()
	inv := make([]float64, n)
	for i := 0; i < n; i++ {
		d := A.Get(i, i)
		if d == 0 || !IsValidFloat64(d) {
			return nil, fmt.Errorf("solve: 对角预条件下标 %d 的对角元为零或非有限", i)
		}
		inv[i] = 1 / d
	}
	return &diagonalPreconditioner{invDiag: inv}, nil
}

// Apply 返回 M⁻¹·r = diag(1/A[i][i])·r。
func (d *diagonalPreconditioner) Apply(r []float64) ([]float64, error) {
	out := make([]float64, len(r))
	if err := d.applyInto(out, r); err != nil {
		return nil, err
	}
	return out, nil
}

// applyInto 把 M⁻¹·r 写入 dst(就地路径,供 Krylov 迭代复用缓冲)。
func (d *diagonalPreconditioner) applyInto(dst, r []float64) error {
	if len(r) != len(d.invDiag) || len(dst) != len(r) {
		return errors.New("solve: 对角预条件维度不符")
	}
	for i := range r {
		dst[i] = d.invDiag[i] * r[i]
		if !IsValidFloat64(dst[i]) {
			return errors.New("solve: 对角预条件结果非有限")
		}
	}
	return nil
}

// preconditionerInto 是可选的就地预条件接口(包内实现均支持);外部实现
// 只需实现 Apply,调用方会退回分配路径。
type preconditionerInto interface {
	applyInto(dst, r []float64) error
}

// applyPreTo 优先把 M⁻¹·r 就地写入 dst(长度不符时重分配),以便 Krylov
// 迭代内层复用缓冲;pre 为 nil 时退化为复制。
func applyPreTo(dst []float64, pre Preconditioner, r []float64) ([]float64, error) {
	if len(dst) != len(r) {
		dst = make([]float64, len(r))
	}
	if pre == nil {
		copy(dst, r)
		return dst, nil
	}
	if p, ok := pre.(preconditionerInto); ok {
		if err := p.applyInto(dst, r); err != nil {
			return nil, err
		}
		return dst, nil
	}
	out, err := pre.Apply(r)
	if err != nil {
		return nil, err
	}
	if len(out) != len(r) {
		return nil, errors.New("solve: 预条件子改变了向量长度")
	}
	for _, v := range out {
		if !IsValidFloat64(v) {
			return nil, errors.New("solve: 预条件结果非有限")
		}
	}
	copy(dst, out)
	return dst, nil
}

// applyPre 返回 M⁻¹·r 的新切片(外部调用路径)。
func applyPre(pre Preconditioner, r []float64) ([]float64, error) {
	return applyPreTo(nil, pre, r)
}
