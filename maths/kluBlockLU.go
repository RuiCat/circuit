// 分块 LU:先按 BTF 把矩阵排成块上三角,再对每个对角块独立做稀疏 LU,
// 求解时按块逆序回代(块间只用到存下来的右上非零块)。实现与 LU[T] 一致。

package maths

import "errors"

// blockLU 是 BTF 分块后的 LU 求解器:不显式构造整体 L/U,而是
// 「每块一个 kluSparse 因子 + 块间右上非零块」,块间回代时逐块求解。
type blockLU[T Number] struct {
	n       int
	perm    []int       // 置换后第 i 行/列来自原矩阵 perm[i]
	starts  []int       // 各块起始位置(starts[b]..下一块/n)
	blockOf []int       // 每个置换后位置所属块号
	blocks  []LU[T]     // 各对角块的稀疏 LU
	sizes   []int       // 各块尺寸(结构变化时重建因子)
	offCols [][][]int   // offCols[b][块内行] = 该行在后续块里的列(置换后全局列号)
	offVals [][][]T     // 与 offCols 对齐的值
	subRhs  []Vector[T] // 每块的右端缓冲(复用)
	subX    []Vector[T] // 每块的解缓冲(复用)
	rhs     []T         // 置换后的整体右端/解
}

// NewBlockLU 创建 n 阶 BTF 分块 LU 求解器;n < 1 → error。
func NewBlockLU[T Number](n int) (LU[T], error) {
	if n < 1 {
		return nil, errors.New("solve: 分块 LU 维度必须为正")
	}
	return &blockLU[T]{n: n, rhs: make([]T, n)}, nil
}

// Decompose 先做 BTF,再逐块分解对角块并保存右上块;结构只有单块时
// 退化为一次普通稀疏 LU(仍满足同一接口)。
func (b *blockLU[T]) Decompose(matrix Matrix[T]) error {
	if !matrix.IsSquare() {
		return errors.New("solve: 分块 LU 分解要求方阵")
	}
	if matrix.Rows() != b.n {
		return errors.New("solve: 分块 LU 分解维度不符")
	}
	n := b.n
	perm, starts := computeBtfOrder(matrix, n)
	if perm == nil {
		perm = identityPerm(n)
		starts = []int{0}
	}
	b.perm, b.starts = perm, starts
	b.blockOf = make([]int, n)
	for bi := 0; bi < len(starts); bi++ {
		end := n
		if bi+1 < len(starts) {
			end = starts[bi+1]
		}
		for pos := starts[bi]; pos < end; pos++ {
			b.blockOf[pos] = bi
		}
	}
	inv := make([]int, n)
	for i, p := range perm {
		inv[p] = i
	}
	recreate := len(b.blocks) != len(starts)
	if !recreate {
		for bi := 0; bi < len(starts); bi++ {
			if b.blockEnd(bi)-starts[bi] != b.sizes[bi] {
				recreate = true
				break
			}
		}
	}
	if recreate {
		b.blocks = make([]LU[T], len(starts))
		b.sizes = make([]int, len(starts))
		b.offCols = make([][][]int, len(starts))
		b.offVals = make([][][]T, len(starts))
		b.subRhs = make([]Vector[T], len(starts))
		b.subX = make([]Vector[T], len(starts))
		for bi := 0; bi < len(starts); bi++ {
			size := b.blockEnd(bi) - starts[bi]
			lu, err := NewSparseLU[T](size)
			if err != nil {
				return err
			}
			b.blocks[bi] = lu
			b.sizes[bi] = size
			b.subRhs[bi] = NewDenseVector[T](size)
			b.subX[bi] = NewDenseVector[T](size)
		}
	}
	subs := make([]*kluMatrix[T], len(starts))
	for bi := range starts {
		size := b.blockEnd(bi) - starts[bi]
		subs[bi] = newSparseMatrix[T](size, size)
		b.offCols[bi] = make([][]int, size)
		b.offVals[bi] = make([][]T, size)
	}
	badOrder := false
	for pos := 0; pos < n; pos++ {
		row := perm[pos]
		bi := b.blockOf[pos]
		start := starts[bi]
		local := pos - start
		forEachNonzero(matrix, row, func(c0 int, v T) {
			if c0 < 0 || c0 >= n {
				return
			}
			c := inv[c0]
			cb := b.blockOf[c]
			switch {
			case cb == bi:
				subs[bi].Set(local, c-start, v)
			case cb > bi:
				b.offCols[bi][local] = append(b.offCols[bi][local], c)
				b.offVals[bi][local] = append(b.offVals[bi][local], v)
			default:
				badOrder = true // 块下三角出现非零:置换不再是块上三角
			}
		})
	}
	if badOrder {
		return errors.New("solve: 分块 LU 的置换未形成块上三角")
	}
	for bi := range starts {
		if err := b.blocks[bi].Decompose(subs[bi]); err != nil {
			return err
		}
	}
	return nil
}

// SolveReuse 求解 A·x=b:置换右端,块逆序「扣除右上块贡献 → 解对角块」,
// 最后把解映回原顺序。
func (b *blockLU[T]) SolveReuse(bv, x Vector[T]) error {
	if bv.Length() != b.n || x.Length() != b.n {
		return errors.New("solve: 分块 LU 求解向量维度不符")
	}
	if b.blocks == nil {
		return errors.New("solve: 分块 LU 尚未分解")
	}
	n := b.n
	for i := 0; i < n; i++ {
		b.rhs[i] = bv.Get(b.perm[i])
	}
	for bi := len(b.starts) - 1; bi >= 0; bi-- {
		start := b.starts[bi]
		end := b.blockEnd(bi)
		size := end - start
		for r := 0; r < size; r++ {
			s := b.rhs[start+r]
			cols, vals := b.offCols[bi][r], b.offVals[bi][r]
			for k, c := range cols {
				s -= vals[k] * b.rhs[c]
			}
			b.subRhs[bi].Set(r, s)
		}
		if err := b.blocks[bi].SolveReuse(b.subRhs[bi], b.subX[bi]); err != nil {
			return err
		}
		for r := 0; r < size; r++ {
			b.rhs[start+r] = b.subX[bi].Get(r)
		}
	}
	for i := 0; i < n; i++ {
		x.Set(b.perm[i], b.rhs[i])
	}
	return nil
}

// blockEnd 返回第 bi 块的结束位置(不含)。
func (b *blockLU[T]) blockEnd(bi int) int {
	if bi+1 < len(b.starts) {
		return b.starts[bi+1]
	}
	return b.n
}
