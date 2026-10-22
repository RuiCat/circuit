// 稀疏 LU:行切片存储 + 列反向索引,部分主元 Doolittle;结构不变时复用
// 消元计划与 L/U 槽位(见 luSymbolic.go),结构变化时重建。

package maths

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// planStaleLimit 是消元计划连续失效上限:达到即停用计划(避免"重放失败 +
// 动态重建"双开销),直到结构签名变化再重新启用。
const planStaleLimit = 3

// luPlan 是符号骨架的落地形态:它记录"同一结构下每步选中的主元行身份",
// 配合 L/U 槽位复用即可跳过静态重排与结构重建。
type luPlan struct {
	signature uint64
	pivotIDs  []int   // 第 k 步主元在 permuted 行序下的身份(= 当时 lu.P[k])
	aCols     [][]int // 每行 A 的非零列(permuted 坐标,升序)
	aPos      [][]int // 每行按输入遍历顺序,各非零元在 aCols 中的下标
}

// SparseLUOptions 是稀疏 LU 的可选参数(零值 = 默认行为)。
type SparseLUOptions struct {
	// PivotThreshold 是阈值主元的接受比 τ ∈ (0,1]:候选行按行号升序取第一个
	// |a| ≥ τ·列最大模 者作主元(τ=1 即经典部分主元;τ<1 偏向就地主元,
	// 可能减少 fill,但数值稳定性略降)。零值按 1 处理。
	PivotThreshold float64
	// UseBTF 为 true 时先按结构图的强连通分量做块三角化(块内最小度),
	// 单块结构仍退回原有的恒等/最小度择优。默认 false(行为不变)。
	UseBTF bool
	// DropTolerance > 0 时丢弃「小于 τ·|pivot|」的新增 fill(近似分解,
	// 换更低 fill/内存);默认 0 = 不丢、分解精确。开启后不固化消元计划
	// (结构随数值变化,重放前提不成立)。
	DropTolerance float64
	// StoreMatrix 为 true 时在 Decompose 里保存 A 的稀疏副本,供
	// SolveRefined 迭代精化使用(额外 O(nnz) 内存,默认 false)。
	StoreMatrix bool
}

// kluSparse 实现稀疏矩阵的 LU 分解(A = P·L·U,静态重排 + 部分主元)。
type kluSparse[T Number] struct {
	baseLU[T]
	scale       float64
	perm        []int
	permSig     uint64
	plan        *luPlan
	skelOff     bool
	staleStreak int
	scratchCols []int // 消元归并的行缓冲(跨行复用)
	scratchVals []T
	permTmp     []T     // SolveReuse 的逆置换缓冲
	pivotTau    float64 // 阈值主元接受比 τ ∈ (0,1]
	useBTF      bool    // 是否启用块三角化
	blocks      []int   // 块起始下标(nil = 未启用或单块)
	dropTol     float64 // fill 丢弃阈值(0 = 精确)
	storeMatrix bool    // 是否保存 A 的稀疏副本(迭代精化用)
	stored      Matrix[T]
}

// NewSparseLU 创建 n 阶稀疏 LU 求解器(默认部分主元);n < 1 → error
// (与 NewLU 同语义)。
func NewSparseLU[T Number](n int) (LU[T], error) {
	return NewSparseLUWithOptions[T](n, SparseLUOptions{})
}

// NewSparseLUWithOptions 创建带选项的稀疏 LU 求解器;n < 1 或
// PivotThreshold ∉ (0,1](零值按 1) → error。
func NewSparseLUWithOptions[T Number](n int, opt SparseLUOptions) (LU[T], error) {
	if n < 1 {
		return nil, errors.New("solve: 稀疏 LU 维度必须为正")
	}
	tau := opt.PivotThreshold
	if tau == 0 {
		tau = 1
	}
	if !(tau > 0 && tau <= 1) || math.IsNaN(tau) {
		return nil, errors.New("solve: 稀疏 LU 的 PivotThreshold 必须在 (0,1]")
	}
	drop := opt.DropTolerance
	if drop < 0 || math.IsNaN(drop) || math.IsInf(drop, 0) {
		return nil, errors.New("solve: 稀疏 LU 的 DropTolerance 必须是非负有限数")
	}
	lu := &kluSparse[T]{pivotTau: tau, useBTF: opt.UseBTF, dropTol: drop,
		storeMatrix: opt.StoreMatrix}
	lu.n = n
	lu.L = newStructuralMatrix[T](n, n)
	lu.U = newStructuralMatrix[T](n, n)
	lu.Y = NewDenseVector[T](n)
	lu.P = make([]int, n)
	lu.permTmp = make([]T, n)
	return lu, nil
}

// Decompose 对稀疏矩阵执行 LU 分解。结构签名不变且存在消元计划时走重放;
// 重放发现主元行与计划不符 → 作废计划并回退动态分解(连续 3 次失败后停用计划)。
func (lu *kluSparse[T]) Decompose(matrix Matrix[T]) error {
	start := time.Now()
	defer func() { recordKluDecompose(time.Since(start)) }()
	if !matrix.IsSquare() {
		return errors.New("solve: 稀疏 LU 分解要求方阵")
	}
	if matrix.Rows() != lu.n {
		return errors.New("solve: 稀疏 LU 分解维度不符")
	}
	lu.stored = lu.storeMatrixCopy(matrix)
	sig := structureSignature(matrix)
	if lu.perm == nil || sig != lu.permSig {
		lu.perm = lu.computeStaticPerm(matrix)
		lu.permSig = sig
	}
	if lu.dropTol > 0 {
		lu.skelOff = true // 丢 fill 后结构随数值变化,禁止计划重放
	}
	if lu.plan != nil && !lu.skelOff && lu.plan.signature == sig {
		err := lu.replayPlan(matrix)
		if err == nil {
			lu.staleStreak = 0
			return nil
		}
		if !errors.Is(err, errPlanStale) {
			return err
		}
		lu.plan = nil
		lu.staleStreak++
		if lu.staleStreak >= planStaleLimit {
			lu.skelOff = true
			lu.staleStreak = 0
		}
	}
	pivotIDs, err := lu.decomposeDynamic(matrix)
	if err != nil {
		return err
	}
	lu.capturePlan(sig, pivotIDs, matrix)
	return nil
}

// decomposeDynamic 是全量路径:重建 L/U 结构、逐列选主元并消元,返回逐步
// 主元行身份序列(供后续重放校验)。
func (lu *kluSparse[T]) decomposeDynamic(matrix Matrix[T]) ([]int, error) {
	if err := lu.resetStructures(matrix); err != nil {
		return nil, err
	}
	pivotIDs := make([]int, lu.n)
	for k := 0; k < lu.n; k++ {
		maxRow, err := lu.selectPivot(k)
		if err != nil {
			return nil, err
		}
		lu.applyPivot(k, maxRow)
		pivotIDs[k] = lu.P[k]
		lu.eliminateColumn(k)
	}
	return pivotIDs, nil
}

// resetStructures 清空并重建 L/U 结构:U = P·A·Pᵀ(行按 perm、列按逆 perm),
// L 为单位下三角;scale 记录 A 的最大模(奇异判据的量级基准)。
func (lu *kluSparse[T]) resetStructures(matrix Matrix[T]) error {
	us := lu.U.(*kluMatrix[T])
	ls := lu.L.(*kluMatrix[T])
	us.Zero()
	ls.Zero()
	lu.scale = 0
	inv := lu.inversePerm()
	for i := 0; i < lu.n; i++ {
		lu.P[i] = i
		ls.Set(i, i, one[T]())
	}
	for i := 0; i < lu.n; i++ {
		var loadErr error
		forEachNonzero(matrix, lu.perm[i], func(c int, v T) {
			if loadErr != nil {
				return
			}
			if !IsValidFloat64(Abs(v)) {
				loadErr = fmt.Errorf("solve: 稀疏 LU 输入含 NaN/Inf (行 %d 列 %d)", lu.perm[i], c)
				return
			}
			if a := Abs(v); a > lu.scale {
				lu.scale = a
			}
			us.Set(i, inv[c], v)
		})
		if loadErr != nil {
			return loadErr
		}
	}
	return nil
}

// forEachNonzero 遍历矩阵第 row 行的非零元:对包内两种存储直接读底层,
// 避免 GetRow 每行两次分配(分解热路径逐列重建时的主要开销)。
func forEachNonzero[T Number](m Matrix[T], row int, fn func(col int, v T)) {
	var zero T
	switch src := m.(type) {
	case *denseMatrix[T]:
		rowData := src.DataPtr()[row*src.cols : (row+1)*src.cols]
		for c, v := range rowData {
			if v != zero {
				fn(c, v)
			}
		}
	case *kluMatrix[T]:
		cols, vals := src.rowEntries(row)
		for k, c := range cols {
			if vals[k] != zero {
				fn(c, vals[k])
			}
		}
	default:
		cols, vals := m.GetRow(row)
		for k, c := range cols {
			fn(c, vals.Get(k))
		}
	}
}

// selectPivot 在列 k 的结构非零行里选模最大者(并列取最小行号,与稠密
// 扫描顺序一致 ⇒ 结果确定);整列过小 → 奇异错误。
func (lu *kluSparse[T]) selectPivot(k int) (int, error) {
	us := lu.U.(*kluMatrix[T])
	maxRow := k
	maxAbs := Abs(us.Get(k, k))
	if !IsValidFloat64(maxAbs) {
		return 0, fmt.Errorf("solve: 稀疏 LU 主元含 NaN/Inf (k=%d)", k)
	}
	for _, i := range us.columnRows(k) {
		if i <= k {
			continue
		}
		v := Abs(us.Get(i, k))
		if !IsValidFloat64(v) {
			return 0, fmt.Errorf("solve: 稀疏 LU 主元含 NaN/Inf (k=%d,row=%d)", k, i)
		}
		if v > maxAbs {
			maxAbs = v
			maxRow = i
		}
	}
	if maxAbs == 0 || maxAbs < Epsilon*lu.scale {
		return 0, errors.New("solve: 矩阵奇异或接近奇异")
	}
	if lu.pivotTau < 1 {
		// 阈值主元:按行号升序取第一个 |a| ≥ τ·最大模 者(升序 ⇒ 平局取最小行号);
		// 就地主元优先可减少行交换带来的 fill。
		need := lu.pivotTau * maxAbs
		for _, i := range us.columnRows(k) {
			if i < k {
				continue
			}
			if Abs(us.Get(i, k)) >= need {
				return i, nil
			}
		}
	}
	return maxRow, nil
}

// applyPivot 交换主元行:k↔maxRow(交换 U 行、L 前 k 列槽位与 P 分量)。
func (lu *kluSparse[T]) applyPivot(k, maxRow int) {
	if maxRow == k {
		return
	}
	us := lu.U.(*kluMatrix[T])
	ls := lu.L.(*kluMatrix[T])
	us.SwapRows(k, maxRow)
	for j := 0; j < k; j++ {
		hasK := ls.hasSlot(k, j)
		hasR := ls.hasSlot(maxRow, j)
		vK := ls.Get(k, j)
		vR := ls.Get(maxRow, j)
		ls.deleteSlot(k, j)
		ls.deleteSlot(maxRow, j)
		if hasR {
			ls.Set(k, j, vR)
		}
		if hasK {
			ls.Set(maxRow, j, vK)
		}
	}
	lu.P[k], lu.P[maxRow] = lu.P[maxRow], lu.P[k]
}

// eliminateColumn 用第 k 行消去列 k 的下三角:先删下三角槽位(结构由模式
// 决定、与数值无关),再按模式传播 fill(L 的因子为 0 时也占槽位)。
// eliminateColumn 用第 k 行消去列 k 的下三角。内层按有序行归并一次写完
// (不再逐元素二分查找):新行 = 旧行去掉列 k,再并上 −factor×主元行的列 >k 部分;
// 列索引只对「新增槽位」与「被删的列 k」做增量维护。
func (lu *kluSparse[T]) eliminateColumn(k int) {
	us := lu.U.(*kluMatrix[T])
	ls := lu.L.(*kluMatrix[T])
	pivot := us.Get(k, k)
	candidates := append([]int(nil), us.columnRows(k)...)
	pivotCols, pivotVals := us.rowEntries(k)
	start := 0
	for start < len(pivotCols) && pivotCols[start] <= k {
		start++
	}
	for _, i := range candidates {
		if i <= k {
			continue
		}
		limit := lu.dropTol * Abs(pivot)
		lu.eliminateRow(us, ls, i, k, pivot, limit, pivotCols[start:], pivotVals[start:])
	}
}

// eliminateRow 归并更新第 i 行(结构确定:旧行∪主元行,去掉列 k)。
func (lu *kluSparse[T]) eliminateRow(us *kluMatrix[T], ls *kluMatrix[T],
	i, k int, pivot T, dropLimit float64, pivotCols []int, pivotVals []T) {
	colsI, valsI := us.rowEntries(i)
	posK, ok := findCol(colsI, k)
	if !ok {
		return
	}
	factor := valsI[posK] / pivot
	ls.Set(i, k, factor)
	scratchCols := lu.scratchCols[:0]
	scratchVals := lu.scratchVals[:0]
	a, b := 0, 0
	for a < len(colsI) || b < len(pivotCols) {
		switch {
		case a < len(colsI) && colsI[a] == k:
			a++
		case a < len(colsI) && (b >= len(pivotCols) || colsI[a] < pivotCols[b]):
			scratchCols = append(scratchCols, colsI[a])
			scratchVals = append(scratchVals, valsI[a])
			a++
		case b < len(pivotCols) && (a >= len(colsI) || pivotCols[b] < colsI[a]):
			fill := -factor * pivotVals[b]
			if dropLimit > 0 && Abs(fill) < dropLimit {
				b++ // 丢弃过小 fill:不建槽位(近似分解)
				continue
			}
			scratchCols = append(scratchCols, pivotCols[b])
			scratchVals = append(scratchVals, fill)
			us.addColEntry(pivotCols[b], i)
			b++
		default:
			scratchCols = append(scratchCols, colsI[a])
			scratchVals = append(scratchVals, valsI[a]+(-factor)*pivotVals[b])
			a++
			b++
		}
	}
	us.removeColEntry(k, i)
	// 用「交换缓冲」代替逐行拷贝:把归并结果直接交给该行,旧行切片回收作
	// 下一行的 scratch(省掉每行 O(行长) 的 memmove,1 万节点上填充达 22 万项)。
	oldCols, oldVals := us.rowCols[i], us.rowVals[i]
	us.rowCols[i], us.rowVals[i] = scratchCols, scratchVals
	lu.scratchCols, lu.scratchVals = oldCols[:0], oldVals[:0]
}

// SolveReuse 用分解结果解 A·x = b:解 L·y = P·b 与 U·x = y,再按 perm 逆映射
// 回原列序。
func (lu *kluSparse[T]) SolveReuse(b, x Vector[T]) error {
	if b.Length() != lu.n || x.Length() != lu.n {
		return errors.New("solve: 稀疏 LU 求解向量维度不符")
	}
	ls := lu.L.(*kluMatrix[T])
	us := lu.U.(*kluMatrix[T])
	lu.Y.Zero()
	for i := 0; i < lu.n; i++ {
		sum := b.Get(lu.perm[lu.P[i]])
		cols, vals := ls.rowEntries(i)
		for idx, j := range cols {
			if j < i {
				sum -= vals[idx] * lu.Y.Get(j)
			}
		}
		lu.Y.Set(i, sum)
	}
	x.Zero()
	for i := lu.n - 1; i >= 0; i-- {
		sum := lu.Y.Get(i)
		cols, vals := us.rowEntries(i)
		for idx, j := range cols {
			if j > i {
				sum -= vals[idx] * x.Get(j)
			}
		}
		diag := Abs(us.Get(i, i))
		if diagTooSmall(diag, lu.scale) {
			return errors.New("solve: 稀疏 LU 求解除零(U 对角为零)")
		}
		x.Set(i, sum/us.Get(i, i))
	}
	for i := 0; i < lu.n; i++ {
		lu.permTmp[lu.perm[i]] = x.Get(i)
	}
	for i := 0; i < lu.n; i++ {
		x.Set(i, lu.permTmp[i])
	}
	return nil
}

// inversePerm 返回 perm 的逆置换(inv[perm[i]] = i)。
func (lu *kluSparse[T]) inversePerm() []int {
	inv := make([]int, lu.n)
	for i, p := range lu.perm {
		inv[p] = i
	}
	return inv
}
