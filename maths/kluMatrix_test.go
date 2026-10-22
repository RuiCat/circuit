package maths

import (
	"math/rand"
	"testing"
)

// 编译期断言:稀疏矩阵实现 Matrix[T] 接口。
var _ Matrix[float64] = (*kluMatrix[float64])(nil)

// fillRandomSparse 在 rows×cols 上按密度填入随机非零,返回稀疏与稠密两份。
func fillRandomSparse(t *testing.T, rows, cols int, density float64, seed int64) (Matrix[float64], Matrix[float64]) {
	t.Helper()
	sp := NewKLUMatrix[float64](rows, cols)
	de := NewDenseMatrix[float64](rows, cols)
	rng := rand.New(rand.NewSource(seed))
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			if rng.Float64() < density {
				v := rng.Float64()*4 - 2
				sp.Set(i, j, v)
				de.Set(i, j, v)
			}
		}
	}
	return sp, de
}

// assertSameMatrix 逐元素比较两个矩阵(含零),失败即 t.Fatalf。
func assertSameMatrix(t *testing.T, got, want Matrix[float64], name string) {
	t.Helper()
	if got.Rows() != want.Rows() || got.Cols() != want.Cols() {
		t.Fatalf("%s 维度不符: %dx%d vs %dx%d", name, got.Rows(), got.Cols(), want.Rows(), want.Cols())
	}
	for i := 0; i < want.Rows(); i++ {
		for j := 0; j < want.Cols(); j++ {
			if got.Get(i, j) != want.Get(i, j) {
				t.Fatalf("%s (%d,%d) = %v,期望 %v", name, i, j, got.Get(i, j), want.Get(i, j))
			}
		}
	}
}

// assertColumnIndex 校验列反向索引与元素一致:每列的槽位行集合 == 该列 Get 非零行集合,
// 且升序、无重复。
func assertColumnIndex(t *testing.T, m *kluMatrix[float64]) {
	t.Helper()
	for j := 0; j < m.Cols(); j++ {
		rows := m.columnRows(j)
		if len(rows) == 0 {
			continue
		}
		for k, r := range rows {
			if k > 0 && rows[k-1] >= r {
				t.Fatalf("列 %d 行索引未严格升序: %v", j, rows)
			}
			if m.Get(r, j) == 0 && !m.keepZeros {
				t.Fatalf("列 %d 行索引含零元素行 %d", j, r)
			}
		}
		want := 0
		for i := 0; i < m.Rows(); i++ {
			if m.hasSlot(i, j) {
				want++
			}
		}
		if len(rows) != want {
			t.Fatalf("列 %d 索引长度 %d 与槽位数 %d 不符", j, len(rows), want)
		}
	}
}

// TestSparseMatrixAgainstDense 断言基础读写、GetRow/ToDense/MVM/NonZeroCount
// 与稠密实现一致。
func TestSparseMatrixAgainstDense(t *testing.T) {
	sp, de := fillRandomSparse(t, 7, 5, 0.35, 7001)
	assertSameMatrix(t, sp, de, "初始")
	if sp.NonZeroCount() != de.NonZeroCount() {
		t.Fatalf("NonZeroCount = %d,期望 %d", sp.NonZeroCount(), de.NonZeroCount())
	}
	for i := 0; i < de.Rows(); i++ {
		cols, vals := sp.GetRow(i)
		wantCols, wantVals := de.GetRow(i)
		if len(cols) != len(wantCols) {
			t.Fatalf("行 %d 非零数 %d,期望 %d", i, len(cols), len(wantCols))
		}
		for k := range cols {
			if cols[k] != wantCols[k] || vals.Get(k) != wantVals.Get(k) {
				t.Fatalf("行 %d 第 %d 项 = (%d,%v),期望 (%d,%v)", i, k,
					cols[k], vals.Get(k), wantCols[k], wantVals.Get(k))
			}
		}
	}
	got := sp.ToDense().ToDense()
	want := de.ToDense().ToDense()
	for k := range want {
		if got[k] != want[k] {
			t.Fatalf("ToDense[%d] = %v,期望 %v", k, got[k], want[k])
		}
	}
	x := NewDenseVectorWithData([]float64{1, -2, 3, -4, 5})
	gm := sp.MatrixVectorMultiply(x).ToDense()
	wm := de.MatrixVectorMultiply(x).ToDense()
	for k := range wm {
		if gm[k] != wm[k] {
			t.Fatalf("MVM[%d] = %v,期望 %v", k, gm[k], wm[k])
		}
	}
}

// TestSparseMatrixSetDelete 断言稀疏语义:写零删除槽位(普通矩阵),
// 结构矩阵保留显式零槽位。
func TestSparseMatrixSetDelete(t *testing.T) {
	sp := NewKLUMatrix[float64](3, 3).(*kluMatrix[float64])
	sp.Set(1, 2, 4)
	if !sp.hasSlot(1, 2) || sp.Get(1, 2) != 4 {
		t.Fatal("Set 非零后应有槽位")
	}
	sp.Set(1, 2, 0)
	if sp.hasSlot(1, 2) {
		t.Fatal("普通稀疏矩阵写零应删除槽位")
	}
	sp.Increment(0, 0, 3)
	sp.Increment(0, 0, -3)
	if sp.hasSlot(0, 0) {
		t.Fatal("Increment 归零应删除槽位")
	}

	st := newStructuralMatrix[float64](3, 3)
	st.Set(1, 2, 4)
	st.Set(1, 2, 0)
	if !st.hasSlot(1, 2) || st.Get(1, 2) != 0 {
		t.Fatal("结构矩阵应保留显式零槽位")
	}
	st.deleteSlot(1, 2)
	if st.hasSlot(1, 2) {
		t.Fatal("deleteSlot 应删除槽位")
	}
}

// TestSparseMatrixSwapRows 断言换行后元素与列索引同步,且与稠密换行一致。
func TestSparseMatrixSwapRows(t *testing.T) {
	sp, de := fillRandomSparse(t, 6, 6, 0.4, 7002)
	sp.SwapRows(1, 4)
	de.SwapRows(1, 4)
	assertSameMatrix(t, sp, de, "换行后")
	assertColumnIndex(t, sp.(*kluMatrix[float64]))
	sp.SwapRows(2, 2) // 同行交换应为无操作
	assertSameMatrix(t, sp, de, "同行交换后")
}

// TestSparseMatrixCopyResize 断言 Copy/BuildFromDense/Resize 与稠密语义一致。
func TestSparseMatrixCopyResize(t *testing.T) {
	sp, de := fillRandomSparse(t, 5, 4, 0.5, 7003)
	dst := NewKLUMatrix[float64](5, 4)
	dstDense := NewDenseMatrix[float64](5, 4)
	sp.Copy(dst)
	de.Copy(dstDense)
	assertSameMatrix(t, dst, dstDense, "Copy")

	built := NewKLUMatrix[float64](3, 2)
	built.BuildFromDense([][]float64{{1, 0}, {0, 2}, {3, 0}})
	want := NewDenseMatrix[float64](3, 2)
	want.BuildFromDense([][]float64{{1, 0}, {0, 2}, {3, 0}})
	assertSameMatrix(t, built, want, "BuildFromDense")

	shrunk := NewKLUMatrix[float64](4, 3)
	shrunk.Set(0, 0, 1)
	shrunk.Set(3, 2, 5)
	shrunk.Resize(2, 2)
	if shrunk.Get(0, 0) != 1 {
		t.Fatal("Resize 应保留界内元素")
	}
	if shrunk.NonZeroCount() != 1 {
		t.Fatalf("Resize 缩后槽位 = %d,期望 1", shrunk.NonZeroCount())
	}
	shrunk.Resize(4, 3)
	if shrunk.Get(3, 2) != 0 {
		t.Fatal("Resize 收缩后再放大不应恢复旧值")
	}
	assertColumnIndex(t, shrunk.(*kluMatrix[float64]))
}

// TestSparseMatrixPanics 断言越界访问 panic(与稠密实现的容器惯例一致)。
func TestSparseMatrixPanics(t *testing.T) {
	sp := NewKLUMatrix[float64](2, 2)
	defer func() {
		if recover() == nil {
			t.Fatal("越界 Get 应 panic")
		}
	}()
	sp.Get(2, 0)
}

// TestSparseMatrixString 断言 String 与稠密同格式(逐元素空格分隔)。
func TestSparseMatrixString(t *testing.T) {
	sp, de := fillRandomSparse(t, 2, 2, 1, 7004)
	if sp.String() != de.String() {
		t.Fatalf("String 不一致:\n%q\n%q", sp.String(), de.String())
	}
}
