package maths

import (
	"math/rand"
	"testing"
)

func TestDenseVectorOperations(t *testing.T) {
	v1 := NewDenseVector(3)
	v1.Set(0, 1)
	v1.Set(1, 2)
	v1.Set(2, 3)

	if v1.Length() != 3 {
		t.Errorf("Expected length 3, got %d", v1.Length())
	}

	if v1.Get(1) != 2 {
		t.Errorf("Expected Get(1) to be 2, got %f", v1.Get(1))
	}

	v2 := NewDenseVector(3)
	v2.Set(0, 4)
	v2.Set(1, 5)
	v2.Set(2, 6)

	dot := v1.DotProduct(v2)
	expectedDot := 1.0*4.0 + 2.0*5.0 + 3.0*6.0
	if dot != expectedDot {
		t.Errorf("Expected dot product %f, got %f", expectedDot, dot)
	}

	v1.Add(v2)
	if v1.Get(0) != 5 || v1.Get(1) != 7 || v1.Get(2) != 9 {
		t.Errorf("Vector Add failed. Got [%f, %f, %f]", v1.Get(0), v1.Get(1), v1.Get(2))
	}

	v1.Scale(2)
	if v1.Get(0) != 10 || v1.Get(1) != 14 || v1.Get(2) != 18 {
		t.Errorf("Vector Scale failed. Got [%f, %f, %f]", v1.Get(0), v1.Get(1), v1.Get(2))
	}
}

func TestUpdateVector(t *testing.T) {
	base := NewDenseVector(4)
	base.Set(0, 1)
	base.Set(1, 2)
	base.Set(2, 3)
	base.Set(3, 4)

	uv := NewUpdateVectorPtr(base)

	// 1. Get should read from base vector initially
	if uv.Get(0) != 1 {
		t.Errorf("Initial Get(0) failed. Expected 1, got %f", uv.Get(0))
	}

	// 2. Set should write to cache, Get should read from cache
	uv.Set(0, 100)
	if uv.Get(0) != 100 {
		t.Errorf("Get after Set failed. Expected 100, got %f", uv.Get(0))
	}
	// Base vector should be unchanged
	if base.Get(0) != 1 {
		t.Errorf("Base vector was modified before Update. Expected 1, got %f", base.Get(0))
	}

	// 3. Update should write cache to base vector
	uv.Update()
	if base.Get(0) != 100 {
		t.Errorf("Base vector was not updated after Update. Expected 100, got %f", base.Get(0))
	}
	// After update, Get should still work
	if uv.Get(0) != 100 {
		t.Errorf("Get after Update failed. Expected 100, got %f", uv.Get(0))
	}

	// 4. Rollback should discard changes
	uv.Set(1, 200) // Change value
	if uv.Get(1) != 200 {
		t.Errorf("Get after Set for rollback test failed. Expected 200, got %f", uv.Get(1))
	}
	uv.Rollback()
	// After rollback, value should be the original from base
	if uv.Get(1) != 2 {
		t.Errorf("Get after Rollback failed. Expected 2, got %f", uv.Get(1))
	}
	// Base should not have changed
	if base.Get(1) != 2 {
		t.Errorf("Base vector was modified by a rolled-back change. Expected 2, got %f", base.Get(1))
	}

	// 5. Increment
	uv.Increment(2, 10)
	if uv.Get(2) != 13 {
		t.Errorf("Increment failed. Expected 13, got %f", uv.Get(2))
	}
	uv.Update()
	if base.Get(2) != 13 {
		t.Errorf("Base vector not updated after Increment and Update. Expected 13, got %f", base.Get(2))
	}
}

func BenchmarkDenseVectorSet(b *testing.B) {
	size := 1000
	v := NewDenseVector(size)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Cycle through indices to avoid cache effects of setting the same element
		index := i % size
		v.Set(index, rand.Float64())
	}
}

func BenchmarkUpdateVectorSet(b *testing.B) {
	size := 1000
	base := NewDenseVector(size)
	uv := NewUpdateVector(base)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index := i % size
		uv.Set(index, rand.Float64())
	}
}
