// 消元计划的固化与重放:结构签名不变时按捕获的 A 模式重建 L/U 槽位,
// 再按计划校验主元行并消元;主元行与计划不符即判计划失效,交回动态路径。

package maths

import (
	"errors"
	"fmt"
	"sort"
)

// errPlanStale 表示消元计划记录的主元行序列与实际数值选择不符(计划作废),
// 或输入结构与捕获时的模式不符(签名碰撞的兜底)。
var errPlanStale = errors.New("solve: 稀疏 LU 消元计划失效")

// capturePlan 固化本次动态分解的主元行身份序列与 A 的行模式
// (permuted 坐标下的升序列号 + 每个输入非零元的槽位下标);
// skelOff(安全阀关闭)期间不再固化。
func (lu *kluSparse[T]) capturePlan(sig uint64, pivotIDs []int, matrix Matrix[T]) {
	if lu.skelOff {
		lu.plan = nil
		return
	}
	inv := lu.inversePerm()
	plan := &luPlan{
		signature: sig,
		pivotIDs:  append([]int(nil), pivotIDs...),
		aCols:     make([][]int, lu.n),
		aPos:      make([][]int, lu.n),
	}
	for i := 0; i < lu.n; i++ {
		orig := make([]int, 0, 8)
		forEachNonzero(matrix, lu.perm[i], func(c int, v T) {
			orig = append(orig, inv[c])
		})
		sorted := append([]int(nil), orig...)
		sort.Ints(sorted)
		plan.aCols[i] = sorted
		plan.aPos[i] = make([]int, len(orig))
		for k, m := range orig {
			plan.aPos[i][k] = sort.SearchInts(sorted, m)
		}
	}
	lu.plan = plan
}

// replayPlan 按计划重建结构并重放消元:先按捕获的 A 模式建 L/U 槽位,
// 再逐步校验主元行身份并消元。主元行与计划不符 / 模式不符 → errPlanStale。
func (lu *kluSparse[T]) replayPlan(matrix Matrix[T]) error {
	if err := lu.rebuildFromPlan(matrix); err != nil {
		return err
	}
	for k := 0; k < lu.n; k++ {
		maxRow, err := lu.selectPivot(k)
		if err != nil {
			return err
		}
		if lu.P[maxRow] != lu.plan.pivotIDs[k] {
			return errPlanStale
		}
		lu.applyPivot(k, maxRow)
		lu.eliminateColumn(k)
	}
	return nil
}

// rebuildFromPlan 用捕获的 A 模式重建 U(仅 A 的非零槽位)与 L(仅对角 1),
// 装入 A 的新值并重建列索引;槽位与动态路径的初始结构逐格一致。
func (lu *kluSparse[T]) rebuildFromPlan(matrix Matrix[T]) error {
	us := lu.U.(*kluMatrix[T])
	ls := lu.L.(*kluMatrix[T])
	us.Zero()
	ls.Zero()
	lu.scale = 0
	for i := 0; i < lu.n; i++ {
		lu.P[i] = i
		ls.Set(i, i, one[T]())
		cols := lu.plan.aCols[i]
		us.rowCols[i] = append(us.rowCols[i][:0], cols...)
		if cap(us.rowVals[i]) >= len(cols) {
			us.rowVals[i] = us.rowVals[i][:len(cols)]
			clear(us.rowVals[i])
		} else {
			us.rowVals[i] = make([]T, len(cols))
		}
	}
	for i := 0; i < lu.n; i++ {
		pos := 0
		var loadErr error
		forEachNonzero(matrix, lu.perm[i], func(c int, v T) {
			if loadErr != nil {
				return
			}
			if pos >= len(lu.plan.aPos[i]) {
				loadErr = errPlanStale
				return
			}
			if !IsValidFloat64(Abs(v)) {
				loadErr = fmt.Errorf("solve: 稀疏 LU 输入含 NaN/Inf (行 %d 列 %d)", lu.perm[i], c)
				return
			}
			if a := Abs(v); a > lu.scale {
				lu.scale = a
			}
			us.rowVals[i][lu.plan.aPos[i][pos]] = v
			pos++
		})
		if loadErr != nil {
			return loadErr
		}
		if pos != len(lu.plan.aPos[i]) {
			return errPlanStale
		}
	}
	us.rebuildColRows()
	return nil
}
