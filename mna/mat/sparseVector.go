package mat

import (
	"fmt"
	"sort"
)

// sparseVector 稀疏向量数据结构
type sparseVector struct {
	length int
	// 使用索引-值对存储
	indices []int
	values  []float64
}

// NewSparseVector 创建新的稀疏向量
func NewSparseVector(length int) Vector {
	return &sparseVector{
		length:  length,
		indices: make([]int, 0),
		values:  make([]float64, 0),
	}
}

// Set 设置向量元素
func (v *sparseVector) Set(index int, value float64) {
	if index < 0 || index >= v.length {
		panic("index out of range")
	}
	// 二分查找插入位置
	pos := sort.Search(len(v.indices), func(i int) bool {
		return v.indices[i] >= index
	})
	if pos < len(v.indices) && v.indices[pos] == index {
		// 元素已存在
		if value == 0 {
			// 删除元素
			v.deleteElement(pos)
		} else {
			// 更新元素
			v.values[pos] = value
		}
	} else if value != 0 {
		// 插入新元素
		v.insertElement(index, value, pos)
	}
}

// Increment 增量设置向量元素
func (v *sparseVector) Increment(index int, value float64) {
	if index < 0 || index >= v.length {
		panic("index out of range")
	}
	// 二分查找插入位置
	pos := sort.Search(len(v.indices), func(i int) bool {
		return v.indices[i] >= index
	})
	if pos < len(v.indices) && v.indices[pos] == index {
		// 元素已存在
		if value == 0 {
			// 删除元素
			v.deleteElement(pos)
		} else {
			// 更新元素
			v.values[pos] += value
		}
	} else if value != 0 {
		// 插入新元素
		v.insertElement(index, value, pos)
	}
}

// Get 获取向量元素
func (v *sparseVector) Get(index int) float64 {
	if index < 0 || index >= v.length {
		panic("index out of range")
	}
	// 二分查找
	pos := sort.Search(len(v.indices), func(i int) bool {
		return v.indices[i] >= index
	})
	if pos < len(v.indices) && v.indices[pos] == index {
		return v.values[pos]
	}
	return 0
}

// deleteElement 删除指定位置的元素
func (v *sparseVector) deleteElement(pos int) {
	// 删除元素
	v.indices = append(v.indices[:pos], v.indices[pos+1:]...)
	v.values = append(v.values[:pos], v.values[pos+1:]...)
}

// insertElement 在指定位置插入元素
func (v *sparseVector) insertElement(index int, value float64, pos int) {
	// 扩展数组
	v.indices = append(v.indices, 0)
	v.values = append(v.values, 0)
	// 移动元素
	copy(v.indices[pos+1:], v.indices[pos:])
	copy(v.values[pos+1:], v.values[pos:])
	// 插入新元素
	v.indices[pos] = index
	v.values[pos] = value
}

// Length 返回向量长度
func (v *sparseVector) Length() int {
	return v.length
}

// String 字符串表示
func (v *sparseVector) String() string {
	result := "["
	for i := 0; i < v.length; i++ {
		result += fmt.Sprintf("%8.4f ", v.Get(i))
	}
	result += "]"
	return result
}

// NonZeroCount 返回非零元素数量
func (v *sparseVector) NonZeroCount() int {
	return len(v.values)
}

// Copy 复制向量
func (v *sparseVector) Copy(sv Vector) {
	switch a := sv.(type) {
	case *sparseVector:
		a.length = v.length
		if cap(a.indices) < len(v.indices) {
			a.indices = make([]int, len(v.indices))
		} else {
			a.indices = a.indices[:len(v.indices)]
		}
		copy(a.indices, v.indices)
		if cap(a.values) < len(v.values) {
			a.values = make([]float64, len(v.values))
		} else {
			a.values = a.values[:len(v.values)]
		}
		copy(a.values, v.values)
	default:
		// 对于其他类型的稀疏向量实现，逐个元素复制
		for i := 0; i < v.length; i++ {
			value := v.Get(i)
			if value != 0 {
				a.Set(i, value)
			}
		}
	}
}

// BuildFromDense 从稠密向量构建稀疏向量
func (v *sparseVector) BuildFromDense(dense []float64) {
	if len(dense) != v.length {
		panic("dimension mismatch")
	}
	// 完全重置所有数组
	v.indices = v.indices[:0]
	v.values = v.values[:0]
	// 构建稀疏格式
	for i := 0; i < v.length; i++ {
		if dense[i] != 0 {
			v.indices = append(v.indices, i)
			v.values = append(v.values, dense[i])
		}
	}
}

// ToDense 转换为稠密向量
func (v *sparseVector) ToDense() []float64 {
	dense := make([]float64, v.length)
	for i := 0; i < len(v.indices); i++ {
		dense[v.indices[i]] = v.values[i]
	}
	return dense
}

// DotProduct 计算与另一个向量的点积
func (v *sparseVector) DotProduct(other Vector) float64 {
	if other.Length() != v.length {
		panic("vector dimension mismatch")
	}
	result := 0.0
	// 遍历当前向量的非零元素
	for i := 0; i < len(v.indices); i++ {
		index := v.indices[i]
		result += v.values[i] * other.Get(index)
	}
	return result
}

// Scale 向量缩放
func (v *sparseVector) Scale(scalar float64) {
	for i := 0; i < len(v.values); i++ {
		v.values[i] *= scalar
	}
}

// Add 向量加法
func (v *sparseVector) Add(other Vector) {
	if other.Length() != v.length {
		panic("vector dimension mismatch")
	}
	// 遍历另一个向量的所有元素
	for i := 0; i < other.Length(); i++ {
		value := other.Get(i)
		if value != 0 {
			v.Increment(i, value)
		}
	}
}

// Clear 将向量重置为零向量
func (v *sparseVector) Clear() {
	// 清空所有非零元素
	v.indices = make([]int, 0)
	v.values = make([]float64, 0)
}
