package maths

import (
	"math/rand"
	"sort"
	"testing"
)

// createTestDenseMatrix 是一个辅助函数，用于创建一个用于测试 updateMatrix 的密集矩阵
func createTestDenseMatrix(rows, cols int, density float64) Matrix[float64] {
	mat := NewDenseMatrix[float64](rows, cols)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			if rand.Float64() < density {
				mat.Set(i, j, rand.Float64()*10)
			}
		}
	}
	return mat
}

// rowEntry 是一个可比较的行条目，便于排序和比较
type rowEntry struct {
	col int
	val float64
}

// getSortedRow 是一个辅助函数，用于从 GetRow 的结果中获取一个已排序的行条目切片，以便进行正确性检查
func getSortedRow(mat UpdateMatrix[float64], row int) []rowEntry {
	cols, vec := mat.GetRow(row)
	vals := vec.ToDense() // 这里会发生内存分配，但对于正确性测试来说是可以接受的

	// 使用 map 来处理合并逻辑中可能出现的重复列索引
	rowMap := make(map[int]float64)
	for i, c := range cols {
		if i < len(vals) {
			rowMap[c] = vals[i]
		}
	}

	// 将 map 转换为切片以便排序，并过滤掉零值
	entries := make([]rowEntry, 0, len(rowMap))
	for c, v := range rowMap {
		if v != 0 {
			entries = append(entries, rowEntry{col: c, val: v})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].col < entries[j].col
	})
	return entries
}

// compareRows 是一个辅助函数，用于比较两个行条目切片是否相等
func compareRows(t *testing.T, caseName string, got, expected []rowEntry) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf(`%s: row length mismatch:
got: %v
exp: %v`, caseName, got, expected)
	}
	for i := range got {
		// 使用一个小的容差来比较浮点数
		if got[i].col != expected[i].col || (got[i].val-expected[i].val > 1e-9 || expected[i].val-got[i].val > 1e-9) {
			t.Fatalf(`%s: row content mismatch:
got: %v
exp: %v`, caseName, got, expected)
		}
	}
}

// TestUpdateMatrix_GetRow_Correctness 测试 updateMatrix 的 GetRow 方法的正确性，
// 验证它是否能正确地将基础矩阵的行与缓存中的修改合并。
func TestUpdateMatrix_GetRow_Correctness(t *testing.T) {
	baseMat := NewDenseMatrix[float64](10, 10)
	baseMat.Set(5, 3, 10.0)
	baseMat.Set(5, 7, 20.0)

	updateMat := NewUpdateMatrix(baseMat)

	// 情况 1: 获取没有任何缓存修改的行
	expected1 := []rowEntry{{col: 3, val: 10.0}, {col: 7, val: 20.0}}
	got1 := getSortedRow(updateMat, 5)
	compareRows(t, "No cache", got1, expected1)

	// 情况 2: 通过缓存修改一个现有元素
	updateMat.Set(5, 3, 15.0)
	expected2 := []rowEntry{{col: 3, val: 15.0}, {col: 7, val: 20.0}}
	got2 := getSortedRow(updateMat, 5)
	compareRows(t, "Modify existing", got2, expected2)

	// 情况 3: 通过缓存添加一个新元素
	updateMat.Set(5, 1, 5.0)
	expected3 := []rowEntry{{col: 1, val: 5.0}, {col: 3, val: 15.0}, {col: 7, val: 20.0}}
	got3 := getSortedRow(updateMat, 5)
	compareRows(t, "Add new", got3, expected3)

	// 情况 4: 将一个存在于基础矩阵中的元素置零
	updateMat.Set(5, 7, 0.0)
	expected4 := []rowEntry{{col: 1, val: 5.0}, {col: 3, val: 15.0}}
	got4 := getSortedRow(updateMat, 5)
	compareRows(t, "Zero out", got4, expected4)
}

// BenchmarkUpdateMatrix_GetRow 测试 updateMatrix 的 GetRow 方法的性能。
func BenchmarkUpdateMatrix_GetRow(b *testing.B) {
	baseMat := createTestDenseMatrix(100, 100, 0.1)
	updateMat := NewUpdateMatrix(baseMat)

	// 添加一些缓存值
	for i := 0; i < 500; i++ { // 更多的更新
		row := rand.Intn(100)
		col := rand.Intn(100)
		updateMat.Set(row, col, rand.Float64())
	}

	b.ResetTimer()
	b.ReportAllocs()

	var cols []int
	var vec Vector[float64]

	for i := 0; i < b.N; i++ {
		// 循环遍历行以避免 CPU 缓存对数据本身的影响
		cols, vec = updateMat.GetRow(i % 100)
	}
	// 使用结果以防止编译器将调用优化掉
	_ = cols
	_ = vec
}
