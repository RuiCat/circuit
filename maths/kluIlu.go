// ILU(0) 预条件子:在 A 既有的非零结构内做不完全 LU 分解(L 与 U 都按行表
// 只存结构非零,内存 O(nnz)),用前代/回代把残差映射为 M⁻¹·r。

package maths

import (
	"errors"
	"fmt"
)

// ilu0 保存 ILU(0) 的 L/U 因子:严格下三角按行表存,对角+上三角按行表存。
type ilu0 struct {
	n      int
	colIdx [][]int     // 每列在 A 模式中的行号(升序)
	lCols  [][]int     // 每行严格下三角列
	lVals  [][]float64 // 与 lCols 对齐的乘子
	uCols  [][]int     // 每行对角+上三角列(升序,uCols[i][0] == i)
	uVals  [][]float64 // 与 uCols 对齐的值

	// 分解完成后摊平的行偏移数组(热路径避免 [][] 二级间接):
	lStart []int
	lCol   []int
	lVal   []float64
	uStart []int
	uCol   []int
	uVal   []float64
}

// NewILU0 用 A 的非零结构构造 ILU(0) 预条件子。
// A 非方阵/维度为 0/含 NaN/Inf → error;分解遇零主元 → error
// (此时应改用对角预条件或先做重排,零主元不静默跳过)。
func NewILU0(A Matrix[float64]) (Preconditioner, error) {
	if A == nil || !A.IsSquare() || A.Rows() == 0 {
		return nil, errors.New("solve: ILU(0) 要求非空方阵")
	}
	n := A.Rows()
	f := &ilu0{
		n:      n,
		colIdx: make([][]int, n),
		lCols:  make([][]int, n),
		lVals:  make([][]float64, n),
		uCols:  make([][]int, n),
		uVals:  make([][]float64, n),
	}
	pat := make([][]int, n)
	var loadErr error
	for i := 0; i < n; i++ {
		forEachNonzero(A, i, func(c int, v float64) {
			if loadErr != nil {
				return
			}
			if !IsValidFloat64(v) {
				loadErr = fmt.Errorf("solve: ILU(0) 输入含 NaN/Inf (行 %d 列 %d)", i, c)
				return
			}
			pat[i] = append(pat[i], c)
			if c < i {
				f.lCols[i] = append(f.lCols[i], c)
				f.lVals[i] = append(f.lVals[i], v)
			} else {
				f.uCols[i] = append(f.uCols[i], c)
				f.uVals[i] = append(f.uVals[i], v)
			}
		})
		if loadErr != nil {
			return nil, loadErr
		}
		// 对角始终占 uCols[i][0] 槽位(缺失时补 0,分解时按零主元报错)。
		if len(f.uCols[i]) == 0 || f.uCols[i][0] != i {
			posPat := 0
			for posPat < len(pat[i]) && pat[i][posPat] < i {
				posPat++
			}
			pat[i] = insertAt(pat[i], posPat, i)
			f.uCols[i] = append([]int{i}, f.uCols[i]...)
			f.uVals[i] = append([]float64{0}, f.uVals[i]...)
		}
		for _, c := range pat[i] {
			f.colIdx[c] = append(f.colIdx[c], i)
		}
	}
	if err := f.factorize(); err != nil {
		return nil, err
	}
	f.flatten()
	return f, nil
}

// flatten 把 L/U 的行表摊平成「行偏移 + 列号 + 值」三数组,并释放二维表。
func (f *ilu0) flatten() {
	f.lStart, f.lCol, f.lVal = flattenRowTable(f.lCols, f.lVals)
	f.uStart, f.uCol, f.uVal = flattenRowTable(f.uCols, f.uVals)
	f.lCols, f.lVals, f.uCols, f.uVals = nil, nil, nil, nil
}

// flattenRowTable 把行表摊平;返回 (n+1) 个行偏移与拼接后的列/值数组。
func flattenRowTable(cols [][]int, vals [][]float64) (start, col []int, val []float64) {
	n := len(cols)
	start = make([]int, n+1)
	total := 0
	for i := 0; i < n; i++ {
		start[i] = total
		total += len(cols[i])
	}
	start[n] = total
	col = make([]int, total)
	val = make([]float64, total)
	pos := 0
	for i := 0; i < n; i++ {
		copy(col[pos:], cols[i])
		copy(val[pos:], vals[i])
		pos += len(cols[i])
	}
	return start, col, val
}

// factorize 就地做不完全分解:消去第 k 列时只用 A 的既有模式(不产生 fill-in)。
func (f *ilu0) factorize() error {
	for k := 0; k < f.n; k++ {
		pivot := f.uVals[k][0]
		if pivot == 0 || !IsValidFloat64(pivot) {
			return fmt.Errorf("solve: ILU(0) 第 %d 个主元为零或非有限", k)
		}
		for _, i := range f.colIdx[k] {
			if i <= k {
				continue
			}
			lp, ok := findCol(f.lCols[i], k)
			if !ok {
				continue
			}
			mult := f.lVals[i][lp] / pivot
			f.lVals[i][lp] = mult
			for up, j := range f.uCols[i] {
				if j <= k {
					continue
				}
				kp, ok := findCol(f.uCols[k], j)
				if !ok {
					continue // ILU(0):模式外的 fill 一律丢弃
				}
				f.uVals[i][up] -= mult * f.uVals[k][kp]
			}
		}
	}
	return nil
}

// Apply 返回 M⁻¹·r(新切片);等价于 applyInto 到新缓冲。
func (f *ilu0) Apply(r []float64) ([]float64, error) {
	dst := make([]float64, f.n)
	if err := f.applyInto(dst, r); err != nil {
		return nil, err
	}
	return dst, nil
}

// applyInto 依次解 L·y = r 与 U·z = y,把 z = M⁻¹·r 写入 dst(可就地复用缓冲)。
func (f *ilu0) applyInto(dst, r []float64) error {
	if len(r) != f.n || len(dst) != f.n {
		return errors.New("solve: ILU(0) 预条件维度不符")
	}
	copy(dst, r)
	for i := 0; i < f.n; i++ {
		s := dst[i]
		for p := f.lStart[i]; p < f.lStart[i+1]; p++ {
			s -= f.lVal[p] * dst[f.lCol[p]]
		}
		dst[i] = s
	}
	for i := f.n - 1; i >= 0; i-- {
		s := dst[i]
		for p := f.uStart[i] + 1; p < f.uStart[i+1]; p++ {
			s -= f.uVal[p] * dst[f.uCol[p]]
		}
		d := f.uVal[f.uStart[i]]
		if d == 0 || !IsValidFloat64(d) {
			return fmt.Errorf("solve: ILU(0) 回代第 %d 个对角元为零或非有限", i)
		}
		dst[i] = s / d
		if !IsValidFloat64(dst[i]) {
			return errors.New("solve: ILU(0) 回代结果非有限")
		}
	}
	return nil
}
