package maths

import (
	"math/rand"
	"sort"
	"testing"
)

// Helper to create a dense matrix for testing updateMatrix
func createTestDenseMatrix(rows, cols int, density float64) Matrix {
	mat := NewDenseMatrix(rows, cols)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			if rand.Float64() < density {
				mat.Set(i, j, rand.Float64()*10)
			}
		}
	}
	return mat
}

// A comparable row entry for easy sorting and comparison
type rowEntry struct {
	col int
	val float64
}

// Helper to get a sorted slice of row entries from GetRow result for correctness checking
func getSortedRow(mat UpdateMatrix, row int) []rowEntry {
	cols, vec := mat.GetRow(row)
	vals := vec.ToDense() // This allocates, which is fine for a correctness test

	// Use a map to handle potential duplicate column indices from the merge logic
	rowMap := make(map[int]float64)
	for i, c := range cols {
		if i < len(vals) {
			rowMap[c] = vals[i]
		}
	}

	// Convert map to slice for sorting, filtering out zero values
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

func compareRows(t *testing.T, caseName string, got, expected []rowEntry) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf(`%s: row length mismatch:
got: %v
exp: %v`, caseName, got, expected)
	}
	for i := range got {
		// Compare floats with a small tolerance
		if got[i].col != expected[i].col || (got[i].val-expected[i].val > 1e-9 || expected[i].val-got[i].val > 1e-9) {
			t.Fatalf(`%s: row content mismatch:
got: %v
exp: %v`, caseName, got, expected)
		}
	}
}

func TestUpdateMatrix_GetRow_Correctness(t *testing.T) {
	baseMat := NewDenseMatrix(10, 10)
	baseMat.Set(5, 3, 10.0)
	baseMat.Set(5, 7, 20.0)

	updateMat := NewUpdateMatrix(baseMat)

	// Case 1: Get row with no cache modifications
	expected1 := []rowEntry{{col: 3, val: 10.0}, {col: 7, val: 20.0}}
	got1 := getSortedRow(updateMat, 5)
	compareRows(t, "No cache", got1, expected1)

	// Case 2: Modify an existing element via cache
	updateMat.Set(5, 3, 15.0)
	expected2 := []rowEntry{{col: 3, val: 15.0}, {col: 7, val: 20.0}}
	got2 := getSortedRow(updateMat, 5)
	compareRows(t, "Modify existing", got2, expected2)

	// Case 3: Add a new element via cache
	updateMat.Set(5, 1, 5.0)
	expected3 := []rowEntry{{col: 1, val: 5.0}, {col: 3, val: 15.0}, {col: 7, val: 20.0}}
	got3 := getSortedRow(updateMat, 5)
	compareRows(t, "Add new", got3, expected3)

	// Case 4: Zero out an element that existed in base
	updateMat.Set(5, 7, 0.0)
	expected4 := []rowEntry{{col: 1, val: 5.0}, {col: 3, val: 15.0}}
	got4 := getSortedRow(updateMat, 5)
	compareRows(t, "Zero out", got4, expected4)
}

func BenchmarkUpdateMatrix_GetRow(b *testing.B) {
	baseMat := createTestDenseMatrix(100, 100, 0.1)
	updateMat := NewUpdateMatrix(baseMat)

	// Add some cached values
	for i := 0; i < 500; i++ { // More updates
		row := rand.Intn(100)
		col := rand.Intn(100)
		updateMat.Set(row, col, rand.Float64())
	}

	b.ResetTimer()
	b.ReportAllocs()

	var cols []int
	var vec Vector

	for i := 0; i < b.N; i++ {
		// Cycle through rows to avoid CPU cache effects on the data itself
		cols, vec = updateMat.GetRow(i % 100)
	}
	// Use the results to prevent the compiler from optimizing the call away
	_ = cols
	_ = vec
}
