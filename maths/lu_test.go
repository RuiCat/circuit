package maths

import (
	"math/rand"
	"testing"
)

// TestLuDenseSolve verifies the correctness of the LU decomposition and solve for dense matrices.
func TestLuDenseSolve(t *testing.T) {
	// System Ax = b
	// A = [[2, 3, 1],
	//      [1, 2, 3],
	//      [3, 1, 2]]
	// b = [9, 6, 8]
	// Expected x = [2.5, 1, 0.5] (approximately)

	a := NewDenseMatrix(3, 3)
	a.Set(0, 0, 2)
	a.Set(0, 1, 3)
	a.Set(0, 2, 1)
	a.Set(1, 0, 1)
	a.Set(1, 1, 2)
	a.Set(1, 2, 3)
	a.Set(2, 0, 3)
	a.Set(2, 1, 1)
	a.Set(2, 2, 2)

	b := NewDenseVector(3)
	b.Set(0, 9)
	b.Set(1, 6)
	b.Set(2, 8)

	lu, err := NewLU(3)
	if err != nil {
		t.Fatalf("NewLU failed: %v", err)
	}
	err = lu.Decompose(a)
	if err != nil {
		t.Fatalf("Decomposition failed: %v", err)
	}

	x := NewDenseVector(3)
	lu.SolveReuse(b, x)

	expected := []float64{35.0 / 18.0, 29.0 / 18.0, 5.0 / 18.0}
	tolerance := 1e-9

	for i := 0; i < 3; i++ {
		if abs(x.Get(i)-expected[i]) > tolerance {
			t.Errorf("Element x[%d] is incorrect. Got %f, expected %f", i, x.Get(i), expected[i])
		}
	}
}

// TestLuDenseSingular verifies that Decompose correctly identifies a singular matrix.
func TestLuDenseSingular(t *testing.T) {
	// A is a singular matrix
	// A = [[1, 2, 3],
	//      [4, 5, 6],
	//      [0, 0, 0]]
	a := NewDenseMatrix(3, 3)
	a.Set(0, 0, 1)
	a.Set(0, 1, 2)
	a.Set(0, 2, 3)
	a.Set(1, 0, 4)
	a.Set(1, 1, 5)
	a.Set(1, 2, 6)
	a.Set(2, 0, 0)
	a.Set(2, 1, 0)
	a.Set(2, 2, 0)

	lu, err := NewLU(3)
	if err != nil {
		t.Fatalf("NewLU failed: %v", err)
	}
	err = lu.Decompose(a)
	if err == nil {
		t.Fatalf("Decompose should have failed for a singular matrix but it did not")
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// BenchmarkLuDenseDecompose benchmarks the LU decomposition for a dense matrix.
func BenchmarkLuDenseDecompose(b *testing.B) {
	size := 100
	m := NewDenseMatrix(size, size)
	// Fill with random data to prevent any optimization for zero matrices
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			m.Set(i, j, rand.Float64())
		}
	}
	lu, err := NewLU(size)
	if err != nil {
		b.Fatalf("NewLU failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := lu.Decompose(m)
		if err != nil {
			b.Fatalf("Decomposition failed during benchmark: %v", err)
		}
	}
}

// BenchmarkLuDenseSolve benchmarks the solve step after LU decomposition for a dense matrix.
func BenchmarkLuDenseSolve(b *testing.B) {
	size := 100
	m := NewDenseMatrix(size, size)
	vecB := NewDenseVector(size)
	vecX := NewDenseVector(size)

	for i := 0; i < size; i++ {
		vecB.Set(i, rand.Float64())
		for j := 0; j < size; j++ {
			m.Set(i, j, rand.Float64())
		}
	}
	// Add a value to the diagonal to make it non-singular
	for i := 0; i < size; i++ {
		m.Set(i, i, m.Get(i, i)+1)
	}

	lu, err := NewLU(size)
	if err != nil {
		b.Fatalf("NewLU failed: %v", err)
	}
	err = lu.Decompose(m)
	if err != nil {
		b.Fatalf("Decomposition failed during setup: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lu.SolveReuse(vecB, vecX)
	}
}
