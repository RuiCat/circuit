package maths

import (
	"math/rand"
	"testing"
)

// createTestSparseMatrix 是一个辅助函数，用于创建一个用于测试的稀疏矩阵
func createTestSparseMatrix(rows, cols int, density float64) *sparseMatrix[float64] {
	mat := NewSparseMatrix[float64](rows, cols)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			if rand.Float64() < density {
				mat.Set(i, j, rand.Float64()*10)
			}
		}
	}
	return mat.(*sparseMatrix[float64])
}

// TestSparseMatrixPruner_Correctness 测试稀疏矩阵修剪器（pruner）的正确性。
// 它验证了移除零行和零列的功能是否按预期工作。
func TestSparseMatrixPruner_Correctness(t *testing.T) {
	// 创建一个包含一些零行和零列的矩阵
	mat := NewSparseMatrix[float64](10, 10).(*sparseMatrix[float64])
	mat.Set(1, 2, 3.0)
	mat.Set(1, 8, 9.0)
	mat.Set(5, 4, 6.0)
	mat.Set(8, 1, 2.0)
	mat.Set(8, 9, 10.0)

	pruner := NewSparseMatrixPruner(mat)

	// 测试 RemoveZeroRows
	prunedRowsMat := pruner.RemoveZeroRows()

	if prunedRowsMat.Rows() != 3 {
		t.Errorf("RemoveZeroRows: expected 3 rows, got %d", prunedRowsMat.Rows())
	}
	if prunedRowsMat.Cols() != 10 {
		t.Errorf("RemoveZeroRows: expected 10 cols, got %d", prunedRowsMat.Cols())
	}
	// 检查一个值
	if val := prunedRowsMat.Get(0, 2); val != 3.0 {
		t.Errorf("RemoveZeroRows: expected value 3.0 at (0, 2), got %f", val)
	}

	// 测试 RemoveZeroCols
	prunedColsMat := pruner.RemoveZeroCols()
	if prunedColsMat.Rows() != 10 {
		t.Errorf("RemoveZeroCols: expected 10 rows, got %d", prunedColsMat.Rows())
	}
	if prunedColsMat.Cols() != 5 {
		t.Errorf("RemoveZeroCols: expected 5 cols, got %d", prunedColsMat.Cols())
	}
	// 检查一个值
	if val := prunedColsMat.Get(5, 2); val != 6.0 { // 原始的 (5,4) 映射到新的 (5,2)
		t.Errorf("RemoveZeroCols: expected value 6.0 at (5, 2), got %f", val)
	}
}

// BenchmarkSparseMatrixPruner_RemoveZeroRows 测试移除零行的性能。
func BenchmarkSparseMatrixPruner_RemoveZeroRows(b *testing.B) {
	mat := createTestSparseMatrix(1000, 1000, 0.01)
	// 添加一些确保为空的零行
	for i := 0; i < 100; i++ {
		rowToClear := rand.Intn(1000)
		start := mat.rowPtr[rowToClear]
		end := mat.rowPtr[rowToClear+1]
		for j := start; j < end; j++ {
			mat.DataManager.DataPtr()[j] = 0
		}
	}

	pruner := NewSparseMatrixPruner(mat)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pruner.RemoveZeroRows()
	}
}

// BenchmarkSparseMatrixPruner_RemoveZeroCols 测试移除零列的性能。
func BenchmarkSparseMatrixPruner_RemoveZeroCols(b *testing.B) {
	mat := createTestSparseMatrix(1000, 1000, 0.01)
	// 添加一些确保为空的零列（这样做效率较低，但对于基准测试设置来说可以接受）
	colsToClear := make(map[int]bool)
	for i := 0; i < 100; i++ {
		colsToClear[rand.Intn(1000)] = true
	}
	for i, col := range mat.colInd {
		if colsToClear[col] {
			mat.DataManager.DataPtr()[i] = 0
		}
	}

	pruner := NewSparseMatrixPruner(mat)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pruner.RemoveZeroCols()
	}
}
