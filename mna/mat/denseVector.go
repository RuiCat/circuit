package mat

import (
	"fmt"
)

// denseVector 稠密向量数据结构
type denseVector struct {
	length int
	data   []float64 // 一维数组存储所有元素
}

// NewDenseVector 创建新的稠密向量
func NewDenseVector(length int) Vector {
	return &denseVector{
		length: length,
		data:   make([]float64, length),
	}
}

// Set 设置向量元素
func (v *denseVector) Set(index int, value float64) {
	if index < 0 || index >= v.length {
		panic("index out of range")
	}
	v.data[index] = value
}

// Increment 增量设置向量元素（累加值）
func (v *denseVector) Increment(index int, value float64) {
	if index < 0 || index >= v.length {
		panic("index out of range")
	}
	v.data[index] += value
}

// Get 获取向量元素
func (v *denseVector) Get(index int) float64 {
	if index < 0 || index >= v.length {
		panic("index out of range")
	}
	return v.data[index]
}

// Length 返回向量长度
func (v *denseVector) Length() int {
	return v.length
}

// String 字符串表示
func (v *denseVector) String() string {
	result := "["
	for i := 0; i < v.length; i++ {
		result += fmt.Sprintf("%8.4f ", v.data[i])
	}
	result += "]"
	return result
}

// NonZeroCount 返回非零元素数量
func (v *denseVector) NonZeroCount() int {
	count := 0
	for i := 0; i < v.length; i++ {
		if v.data[i] != 0 {
			count++
		}
	}
	return count
}

// Copy 复制向量
func (v *denseVector) Copy(a Vector) {
	switch dv := a.(type) {
	case *denseVector:
		// 直接复制一维数组
		dv.length = v.length
		dv.data = make([]float64, v.length)
		copy(dv.data, v.data)
	default:
		// 对于其他类型的向量实现，逐个元素复制
		for i := 0; i < v.length; i++ {
			value := v.data[i]
			if value != 0 {
				a.Set(i, value)
			}
		}
	}
}

// BuildFromDense 从稠密向量构建向量
func (v *denseVector) BuildFromDense(dense []float64) {
	if len(dense) != v.length {
		panic("dimension mismatch")
	}
	copy(v.data, dense)
}

// ToDense 转换为稠密向量
func (v *denseVector) ToDense() []float64 {
	result := make([]float64, v.length)
	copy(result, v.data)
	return result
}

// DotProduct 计算与另一个向量的点积
func (v *denseVector) DotProduct(other Vector) float64 {
	if other.Length() != v.length {
		panic("vector dimension mismatch")
	}
	result := 0.0
	for i := 0; i < v.length; i++ {
		result += v.data[i] * other.Get(i)
	}
	return result
}

// Scale 向量缩放
func (v *denseVector) Scale(scalar float64) {
	for i := 0; i < v.length; i++ {
		v.data[i] *= scalar
	}
}

// Add 向量加法
func (v *denseVector) Add(other Vector) {
	if other.Length() != v.length {
		panic("vector dimension mismatch")
	}
	for i := 0; i < v.length; i++ {
		v.data[i] += other.Get(i)
	}
}

// Clear 将向量重置为零向量
func (v *denseVector) Clear() {
	for i := 0; i < v.length; i++ {
		v.data[i] = 0
	}
}
