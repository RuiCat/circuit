package maths

import (
	"math/rand"
	"testing"
)

// TestDenseVectorOperations 函数测试密集向量 (denseVector) 的基本操作，
// 包括创建、设置/获取元素、点积、加法和标量乘法。
func TestDenseVectorOperations(t *testing.T) {
	// 创建并初始化一个长度为 3 的密集向量 v1
	v1 := NewDenseVector[float64](3)
	v1.Set(0, 1)
	v1.Set(1, 2)
	v1.Set(2, 3)

	// 测试 Length() 方法
	if v1.Length() != 3 {
		t.Errorf("Expected length 3, got %d", v1.Length())
	}

	// 测试 Get() 方法
	if v1.Get(1) != 2 {
		t.Errorf("Expected Get(1) to be 2, got %f", v1.Get(1))
	}

	// 创建另一个密集向量 v2 用于测试二元运算
	v2 := NewDenseVector[float64](3)
	v2.Set(0, 4)
	v2.Set(1, 5)
	v2.Set(2, 6)

	// 测试点积 (DotProduct)
	dot := v1.DotProduct(v2)
	expectedDot := 1.0*4.0 + 2.0*5.0 + 3.0*6.0
	if dot != expectedDot {
		t.Errorf("Expected dot product %f, got %f", expectedDot, dot)
	}

	// 测试向量加法 (Add)
	v1.Add(v2)
	if v1.Get(0) != 5 || v1.Get(1) != 7 || v1.Get(2) != 9 {
		t.Errorf("Vector Add failed. Got [%f, %f, %f]", v1.Get(0), v1.Get(1), v1.Get(2))
	}

	// 测试标量乘法 (Scale)
	v1.Scale(2)
	if v1.Get(0) != 10 || v1.Get(1) != 14 || v1.Get(2) != 18 {
		t.Errorf("Vector Scale failed. Got [%f, %f, %f]", v1.Get(0), v1.Get(1), v1.Get(2))
	}
}

// TestUpdateVector 函数测试 updateVector 的核心功能，
// 它作为一个装饰器，为底层向量提供了一个缓存层，
// 用于支持修改的提交 (Update) 和回滚 (Rollback)。
func TestUpdateVector(t *testing.T) {
	// 创建一个基础向量
	base := NewDenseVector[float64](4)
	base.Set(0, 1)
	base.Set(1, 2)
	base.Set(2, 3)
	base.Set(3, 4)

	// 使用基础向量创建一个 updateVector
	uv := NewUpdateVectorPtr(base)

	// 1. 测试初始读取：Get 操作应直接从基础向量读取数据
	if uv.Get(0) != 1 {
		t.Errorf("Initial Get(0) failed. Expected 1, got %f", uv.Get(0))
	}

	// 2. 测试写入缓存：Set 操作应将数据写入缓存，此时 Get 操作应从缓存中读取
	uv.Set(0, 100)
	if uv.Get(0) != 100 {
		t.Errorf("Get after Set failed. Expected 100, got %f", uv.Get(0))
	}
	// 基础向量此时应保持不变
	if base.Get(0) != 1 {
		t.Errorf("Base vector was modified before Update. Expected 1, got %f", base.Get(0))
	}

	// 3. 测试提交：Update 操作应将缓存中的修改写入基础向量
	uv.Update()
	if base.Get(0) != 100 {
		t.Errorf("Base vector was not updated after Update. Expected 100, got %f", base.Get(0))
	}
	// 提交后，Get 操作仍然有效
	if uv.Get(0) != 100 {
		t.Errorf("Get after Update failed. Expected 100, got %f", uv.Get(0))
	}

	// 4. 测试回滚：Rollback 操作应丢弃缓存中的修改
	uv.Set(1, 200) // 修改一个值
	if uv.Get(1) != 200 {
		t.Errorf("Get after Set for rollback test failed. Expected 200, got %f", uv.Get(1))
	}
	uv.Rollback()
	// 回滚后，值应恢复为基础向量的原始值
	if uv.Get(1) != 2 {
		t.Errorf("Get after Rollback failed. Expected 2, got %f", uv.Get(1))
	}
	// 基础向量不应被已回滚的修改所影响
	if base.Get(1) != 2 {
		t.Errorf("Base vector was modified by a rolled-back change. Expected 2, got %f", base.Get(1))
	}

	// 5. 测试增量修改 (Increment)
	uv.Increment(2, 10)
	if uv.Get(2) != 13 {
		t.Errorf("Increment failed. Expected 13, got %f", uv.Get(2))
	}
	uv.Update() // 提交增量修改
	if base.Get(2) != 13 {
		t.Errorf("Base vector not updated after Increment and Update. Expected 13, got %f", base.Get(2))
	}
}

// BenchmarkDenseVectorSet 测试密集向量 Set 操作的性能。
func BenchmarkDenseVectorSet(b *testing.B) {
	size := 1000
	v := NewDenseVector[float64](size)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 通过循环索引来避免因重复设置同一元素而产生的缓存效应
		index := i % size
		v.Set(index, rand.Float64())
	}
}

// BenchmarkUpdateVectorSet 测试 updateVector Set 操作的性能。
// 这项测试衡量的是将值写入缓存的开销。
func BenchmarkUpdateVectorSet(b *testing.B) {
	size := 1000
	base := NewDenseVector[float64](size)
	uv := NewUpdateVector(base)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index := i % size
		uv.Set(index, rand.Float64())
	}
}
