package maths

import (
	"fmt"
	"math/cmplx"
)

// data 是一个可以容纳 float64 或 complex128 的泛型结构体。
// 它实现了 Data 接口。
type data[T Number] struct {
	value T
}

// NewData 创建一个新的 Data 实例。
func NewData[T Number](value T) Data[T] {
	return &data[T]{value: value}
}

// Add 将另一个 Data 加到当前的 Data 上。
func (d *data[T]) Add(other Data[T]) Data[T] {
	return &data[T]{value: d.value + other.Get()}
}

// Sub 从当前的 Data 中减去另一个 Data。
func (d *data[T]) Sub(other Data[T]) Data[T] {
	return &data[T]{value: d.value - other.Get()}
}

// Mul 将当前的 Data 乘以另一个 Data。
func (d *data[T]) Mul(other Data[T]) Data[T] {
	return &data[T]{value: d.value * other.Get()}
}

// Div 将当前的 Data 除以另一个 Data。
func (d *data[T]) Div(other Data[T]) Data[T] {
	return &data[T]{value: d.value / other.Get()}
}

// Inv 返回 Data 的倒数。
func (d *data[T]) Inv() Data[T] {
	switch v := any(d.value).(type) {
	case float32:
		return &data[T]{value: any(1.0 / v).(T)}
	case float64:
		return &data[T]{value: any(1.0 / v).(T)}
	case complex64:
		return &data[T]{value: any(1.0 / v).(T)}
	case complex128:
		return &data[T]{value: any(1.0 / v).(T)}
	default:
		panic("unsupported type for Inv")
	}
}

// Abs 返回 Data 的绝对值。
func (d *data[T]) Abs() float64 {
	switch v := any(d.value).(type) {
	case float32:
		if v < 0 {
			return float64(-v)
		}
		return float64(v)
	case float64:
		if v < 0 {
			return -v
		}
		return v
	case complex64:
		return cmplx.Abs(complex128(v))
	case complex128:
		return cmplx.Abs(v)
	default:
		panic("unsupported type for Abs")
	}
}

// Get 返回底层值。
func (d *data[T]) Get() T {
	return d.value
}

// Set 设置底层值。
func (d *data[T]) Set(value T) {
	d.value = value
}

// dataManager 提供了 DataManager 接口的通用实现。
type dataManager[T Number] struct {
	data []T
}

// NewDataManager 创建一个指定长度的新的 DataManager。
func NewDataManager[T Number](length int) DataManager[T] {
	return &dataManager[T]{
		data: make([]T, length),
	}
}

// NewDataManagerWithData 使用给定的数据切片创建一个新的 DataManager。
func NewDataManagerWithData[T Number](data []T) DataManager[T] {
	return &dataManager[T]{
		data: data,
	}
}

// Length 返回数据的长度。
func (dm *dataManager[T]) Length() int {
	return len(dm.data)
}

// String 返回数据的字符串表示形式。
func (dm *dataManager[T]) String() string {
	return fmt.Sprintf("%v", dm.data)
}

// Get 返回指定索引处的值。
func (dm *dataManager[T]) Get(index int) T {
	return dm.data[index]
}

// Set 设置指定索引处的值。
func (dm *dataManager[T]) Set(index int, value T) {
	dm.data[index] = value
}

// Increment 增加指定索引处的值。
func (dm *dataManager[T]) Increment(index int, value T) {
	dm.data[index] += value
}

// DataCopy 返回数据切片的副本。
func (dm *dataManager[T]) DataCopy() []T {
	cpy := make([]T, len(dm.data))
	copy(cpy, dm.data)
	return cpy
}

// DataPtr 返回指向数据切片的指针。
// 注意：直接修改返回的切片会影响原始数据。
func (dm *dataManager[T]) DataPtr() []T {
	return dm.data
}

// Zero 将所有元素设置为零。
func (dm *dataManager[T]) Zero() {
	var zero T
	for i := range dm.data {
		dm.data[i] = zero
	}
}

// ZeroInPlace 是 Zero 的别名，原地将所有元素设置为零。
func (dm *dataManager[T]) ZeroInPlace() {
	dm.Zero()
}

// FillInPlace 使用指定值填充整个数据。
func (dm *dataManager[T]) FillInPlace(value T) {
	for i := range dm.data {
		dm.data[i] = value
	}
}

// AppendInPlace 在末尾追加一个或多个值。
func (dm *dataManager[T]) AppendInPlace(values ...T) {
	dm.data = append(dm.data, values...)
}

// InsertInPlace 在指定索引处插入一个或多个值。
func (dm *dataManager[T]) InsertInPlace(index int, values ...T) {
	dm.data = append(dm.data[:index], append(values, dm.data[index:]...)...)
}

// RemoveInPlace 从指定索引处移除指定数量的元素。
func (dm *dataManager[T]) RemoveInPlace(index int, count int) {
	dm.data = append(dm.data[:index], dm.data[index+count:]...)
}

// ReplaceInPlace 从指定索引处开始替换元素。
func (dm *dataManager[T]) ReplaceInPlace(index int, values ...T) {
	copy(dm.data[index:], values)
}

// Resize 调整数据的大小。如果新长度大于容量，则会重新分配内存。
func (dm *dataManager[T]) Resize(length int) {
	if length > cap(dm.data) {
		newData := make([]T, length)
		copy(newData, dm.data)
		dm.data = newData
	} else {
		dm.data = dm.data[:length]
	}
}

// ResizeInPlace 是 Resize 的别名，原地调整数据大小。
func (dm *dataManager[T]) ResizeInPlace(newLength int) {
	dm.Resize(newLength)
}

// NonZeroCount 计算非零元素的数量。
func (dm *dataManager[T]) NonZeroCount() int {
	count := 0
	var zero T
	for _, v := range dm.data {
		if v != zero {
			count++
		}
	}
	return count
}

// Copy 将数据复制到另一个 DataManager。
// 如果目标是 `*dataManager[T]` 类型，则使用高效的 `copy` 函数。
// 否则，逐个元素进行复制。
func (dm *dataManager[T]) Copy(target DataManager[T]) {
	if dm.Length() != target.Length() {
		panic("dataManager.Copy: length mismatch")
	}
	if targetDm, ok := target.(*dataManager[T]); ok {
		copy(targetDm.data, dm.data)
	} else {
		for i := 0; i < dm.Length(); i++ {
			target.Set(i, dm.Get(i))
		}
	}
}
