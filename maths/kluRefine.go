// 迭代精化:在已分解的稀疏 LU 上做若干步 r = b − A·x、解 A·δ = r、x += δ,
// 用于压低病态系统的残差。需要 Decompose 时开启 StoreMatrix(保存 A 的稀疏副本)。

package maths

import "errors"

// Refiner 是支持迭代精化的 LU 求解器接口(由 NewSparseLUWithOptions 且
// StoreMatrix=true 的稀疏 LU 实现;普通 LU 不满足本接口)。
//
// 语义:先做一次 SolveReuse,再对 step 步精化;steps=0 等价于 SolveReuse。
// 精化只改善残差、不承诺把病态系统的解误差降到你期望的量级。
type Refiner[T Number] interface {
	LU[T]
	SolveRefined(b, x Vector[T], steps int) error
}

// SolveRefined 用保存的 A 做 steps 步迭代精化。未开启 StoreMatrix → error;
// steps < 0 或向量维度不符 → error。
func (lu *kluSparse[T]) SolveRefined(b, x Vector[T], steps int) error {
	if lu.stored == nil {
		return errors.New("solve: 迭代精化需要 NewSparseLUWithOptions 开启 StoreMatrix")
	}
	if steps < 0 {
		return errors.New("solve: 迭代精化的 steps 不能为负")
	}
	if b.Length() != lu.n || x.Length() != lu.n {
		return errors.New("solve: 迭代精化向量维度不符")
	}
	if err := lu.SolveReuse(b, x); err != nil {
		return err
	}
	if steps == 0 {
		return nil
	}
	r := NewDenseVector[T](lu.n)
	delta := NewDenseVector[T](lu.n)
	for s := 0; s < steps; s++ {
		ax := lu.stored.MatrixVectorMultiply(x)
		for i := 0; i < lu.n; i++ {
			r.Set(i, b.Get(i)-ax.Get(i))
		}
		delta.Zero()
		if err := lu.SolveReuse(r, delta); err != nil {
			return err
		}
		for i := 0; i < lu.n; i++ {
			nv := x.Get(i) + delta.Get(i)
			if !IsValidFloat64(Abs(nv)) {
				return errors.New("solve: 迭代精化结果非有限")
			}
			x.Set(i, nv)
		}
	}
	return nil
}

// storeMatrixCopy 把输入矩阵复制成稀疏矩阵(A 只保留非零元,O(nnz) 内存),
// 供迭代精化时复用;storeMatrix 关闭时返回 nil。
func (lu *kluSparse[T]) storeMatrixCopy(m Matrix[T]) Matrix[T] {
	if !lu.storeMatrix {
		return nil
	}
	n := m.Rows()
	out := newSparseMatrix[T](n, m.Cols())
	for i := 0; i < n; i++ {
		forEachNonzero(m, i, func(c int, v T) {
			if c >= 0 && c < m.Cols() {
				out.Set(i, c, v)
			}
		})
	}
	return out
}
