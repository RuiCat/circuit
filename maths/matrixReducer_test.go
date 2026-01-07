package maths

import (
	"math/rand"
	"testing"
)

// Helper to create a sparse matrix for testing
func createTestSparseMatrix(rows, cols int, density float64) *sparseMatrix {
	mat := NewSparseMatrix(rows, cols)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			if rand.Float64() < density {
				mat.Set(i, j, rand.Float64()*10)
			}
		}
	}
	return mat.(*sparseMatrix)
}

func TestSparseMatrixPruner_Correctness(t *testing.T) {
	// Create a matrix with some zero rows and cols
	mat := NewSparseMatrix(10, 10).(*sparseMatrix)
	mat.Set(1, 2, 3.0)
	mat.Set(1, 8, 9.0)
	mat.Set(5, 4, 6.0)
	mat.Set(8, 1, 2.0)
	mat.Set(8, 9, 10.0)

	pruner := NewSparseMatrixPruner(mat)

	// Test RemoveZeroRows
	prunedRowsMat := pruner.RemoveZeroRows()

	if prunedRowsMat.Rows() != 3 {
		t.Errorf("RemoveZeroRows: expected 3 rows, got %d", prunedRowsMat.Rows())
	}
	if prunedRowsMat.Cols() != 10 {
		t.Errorf("RemoveZeroRows: expected 10 cols, got %d", prunedRowsMat.Cols())
	}
	// Check a value
	if val := prunedRowsMat.Get(0, 2); val != 3.0 {
		t.Errorf("RemoveZeroRows: expected value 3.0 at (0, 2), got %f", val)
	}

	// Test RemoveZeroCols
	prunedColsMat := pruner.RemoveZeroCols()
	if prunedColsMat.Rows() != 10 {
		t.Errorf("RemoveZeroCols: expected 10 rows, got %d", prunedColsMat.Rows())
	}
	if prunedColsMat.Cols() != 5 {
		t.Errorf("RemoveZeroCols: expected 5 cols, got %d", prunedColsMat.Cols())
	}
	// Check a value
	if val := prunedColsMat.Get(5, 2); val != 6.0 { // 5,4 maps to 5,2
		t.Errorf("RemoveZeroCols: expected value 6.0 at (5, 2), got %f", val)
	}
}

func BenchmarkSparseMatrixPruner_RemoveZeroRows(b *testing.B) {
	mat := createTestSparseMatrix(1000, 1000, 0.01)
	// Add some guaranteed zero rows
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

func BenchmarkSparseMatrixPruner_RemoveZeroCols(b *testing.B) {
	mat := createTestSparseMatrix(1000, 1000, 0.01)
	// Add some guaranteed zero columns (less efficient to do, but ok for benchmark setup)
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
