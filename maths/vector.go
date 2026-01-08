package maths

import (
	"circuit/utils"
	"fmt"
)

// denseVector 稠密向量实现（基于DataManager，全量存储所有元素）
type denseVector[T Number] struct {
	dataManager *dataManager[T] // 嵌入DataManager复用功能
}

// Base 获取底层
func (v *denseVector[T]) Base() Vector[T] {
	return v
}

// NewDenseVector 创建指定长度的空稠密向量
func NewDenseVector[T Number](length int) Vector[T] {
	return &denseVector[T]{
		dataManager: &dataManager[T]{
			data: make([]T, length),
		},
	}
}

// NewDenseVectorWithData 从切片创建稠密向量
func NewDenseVectorWithData[T Number](data []T) Vector[T] {
	return &denseVector[T]{
		dataManager: &dataManager[T]{
			data: data,
		},
	}
}

// BuildFromDense 从稠密切片构建向量（覆盖原有数据）
func (v *denseVector[T]) BuildFromDense(dense []T) {
	if len(dense) != v.Length() {
		panic(fmt.Sprintf("dimension mismatch: len(dense)=%d, vector length=%d", len(dense), v.Length()))
	}
	v.dataManager.ReplaceInPlace(0, dense...)
}

// Zero 清空向量为零向量
func (v *denseVector[T]) Zero() {
	v.dataManager.Zero()
}

// Copy 复制自身数据到目标向量（支持稠密/其他向量类型）
func (v *denseVector[T]) Copy(a Vector[T]) {
	switch target := a.(type) {
	case *denseVector[T]:
		// 同类型直接复制（高效）
		if target.Length() != v.Length() {
			panic(fmt.Sprintf("dimension mismatch: source length=%d, target length=%d", v.Length(), target.Length()))
		}
		v.dataManager.Copy(target.dataManager)
	default:
		// 异类型逐个元素复制（兼容稀疏等其他实现）
		for i := 0; i < v.Length(); i++ {
			val := v.Get(i)
			var zero T
			if val != zero { // 非零元素才复制（优化）
				target.Set(i, val)
			}
		}
	}
}

// Get 获取指定索引元素值（越界panic）
func (v *denseVector[T]) Get(index int) T {
	return v.dataManager.Get(index)
}

// Increment 增量更新元素（value累加，越界panic）
func (v *denseVector[T]) Increment(index int, value T) {
	v.dataManager.Increment(index, value)
}

// Length 返回向量长度
func (v *denseVector[T]) Length() int {
	return v.dataManager.Length()
}

// NonZeroCount 统计非零元素数量
func (v *denseVector[T]) NonZeroCount() int {
	return v.dataManager.NonZeroCount()
}

// Set 设置指定索引元素值（越界panic）
func (v *denseVector[T]) Set(index int, value T) {
	v.dataManager.Set(index, value)
}

// String 格式化输出向量
func (v *denseVector[T]) String() string {
	return v.dataManager.String()
}

// ToDense 转换为稠密切片（返回拷贝）
func (v *denseVector[T]) ToDense() []T {
	return v.dataManager.DataCopy()
}

// DotProduct 计算与另一个向量的点积（维度不匹配panic）
func (v *denseVector[T]) DotProduct(other Vector[T]) T {
	if other.Length() != v.Length() {
		panic(fmt.Sprintf("dimension mismatch: this length=%d, other length=%d", v.Length(), other.Length()))
	}
	var result T
	for i := 0; i < v.Length(); i++ {
		result += v.Get(i) * other.Get(i)
	}
	return result
}

// Scale 向量缩放（所有元素乘scalar）
func (v *denseVector[T]) Scale(scalar T) {
	for i := 0; i < v.Length(); i++ {
		v.Set(i, v.Get(i)*scalar)
	}
}

// Add 向量加法（自身 += 另一个向量，维度不匹配panic）
func (v *denseVector[T]) Add(other Vector[T]) {
	if other.Length() != v.Length() {
		panic(fmt.Sprintf("dimension mismatch: this length=%d, other length=%d", v.Length(), other.Length()))
	}
	for i := 0; i < v.Length(); i++ {
		v.Increment(i, other.Get(i))
	}
}

// MaxAbs 返回向量中绝对值最大的元素
func (v *denseVector[T]) MaxAbs() T {
	if v.Length() == 0 {
		var zero T
		return zero
	}
	maxVal := abs(v.Get(0))
	maxIdx := 0
	for i := 1; i < v.Length(); i++ {
		if val := abs(v.Get(i)); val > maxVal {
			maxVal = val
			maxIdx = i
		}
	}
	return v.Get(maxIdx)
}

// updateVector 带缓存更新向量（基于稠密向量，支持缓存+回溯）
// 核心优化：频繁修改先写缓存，批量刷盘，支持回滚，提升性能
type updateVector[T Number] struct {
	Vector[T]              // 嵌入稠密向量（底层存储）
	bitmap    utils.Bitmap // 位图：标记哪些位置被缓存修改（1=缓存有效）
	cache     []T          // 缓存：存储修改后的值（与底层向量同长度）
	blockSize int          // 缓存块大小（固定16，对齐CPU缓存）
}

// NewUpdateVector 从基础向量创建更新向量（复制基础向量数据）
func NewUpdateVector[T Number](base Vector[T]) UpdateVector[T] {
	length := base.Length()
	// 初始化底层稠密向量并复制基础数据
	dv := NewDenseVector[T](length)
	base.Copy(dv)
	return &updateVector[T]{
		Vector:    dv,
		bitmap:    utils.NewBitmap(length), // 位图长度=向量长度
		cache:     make([]T, length),
		blockSize: 16,
	}
}

// NewUpdateVectorPtr 从基础向量指针创建更新向量。
// 这种方式创建的 updateVector 会直接修改传入的底层向量 ptr。
func NewUpdateVectorPtr[T Number](ptr Vector[T]) UpdateVector[T] {
	length := ptr.Length()
	return &updateVector[T]{
		Vector:    ptr,
		bitmap:    utils.NewBitmap(length), // 位图长度=向量长度
		cache:     make([]T, length),
		blockSize: 16,
	}
}

// getBlockIndexAndPosition 将一维向量索引转换为缓存块索引和块内偏移量。
// 这种分块策略旨在提高CPU缓存的命中率。
func (uv *updateVector[T]) getBlockIndexAndPosition(index int) (int, int) {
	return index / uv.blockSize, index % uv.blockSize
}

// isBitSet 检查位图中指定索引是否被标记（缓存是否有效）
func (uv *updateVector[T]) isBitSet(index int) bool {
	blockIdx, pos := uv.getBlockIndexAndPosition(index)
	return uv.bitmap.Get(utils.BitmapFlag(blockIdx*uv.blockSize + pos))
}

// setBit 标记位图中指定索引（缓存有效）
func (uv *updateVector[T]) setBit(index int) {
	blockIdx, pos := uv.getBlockIndexAndPosition(index)
	uv.bitmap.Set(utils.BitmapFlag(blockIdx*uv.blockSize+pos), true)
}

// clearBit 清除位图中指定索引（缓存无效）
func (uv *updateVector[T]) clearBit(index int) {
	blockIdx, pos := uv.getBlockIndexAndPosition(index)
	uv.bitmap.Set(utils.BitmapFlag(blockIdx*uv.blockSize+pos), false)
}

// Get 获取元素值（优先读缓存，缓存无效则读底层）
func (uv *updateVector[T]) Get(index int) T {
	if index < 0 || index >= uv.Length() {
		panic(fmt.Sprintf("index out of range: %d (length: %d)", index, uv.Length()))
	}
	if uv.isBitSet(index) {
		return uv.cache[index] // 缓存有效：返回缓存值
	}
	return uv.Vector.Get(index) // 缓存无效：返回底层值
}

// Set 设置元素值（写缓存+标记位图）
func (uv *updateVector[T]) Set(index int, value T) {
	if index < 0 || index >= uv.Length() {
		panic(fmt.Sprintf("index out of range: %d (length: %d)", index, uv.Length()))
	}
	uv.cache[index] = value // 写入缓存
	uv.setBit(index)        // 标记位图
}

// Increment 增量更新元素（缓存有效则累加，否则读底层后累加）
func (uv *updateVector[T]) Increment(index int, value T) {
	if index < 0 || index >= uv.Length() {
		panic(fmt.Sprintf("index out of range: %d (length: %d)", index, uv.Length()))
	}
	if uv.isBitSet(index) {
		uv.cache[index] += value // 缓存有效：直接累加
	} else {
		// 缓存无效：读底层值+累加，写入缓存
		uv.cache[index] = uv.Vector.Get(index) + value
		uv.setBit(index)
	}
}

// Update 将缓存中的所有修改应用到底层的向量中。
// 遍历位图，只处理被标记为已修改的元素。
func (uv *updateVector[T]) Update() {
	for i := 0; i < uv.Length(); i++ {
		if uv.isBitSet(i) {
			uv.Vector.Set(i, uv.cache[i]) // 缓存值刷到底层
			uv.clearBit(i)                // 清除位图标记
		}
	}
}

// Rollback 丢弃所有在缓存中的修改，恢复到底层向量的状态。
// 它通过清除位图和缓存来实现，避免了昂贵的数据恢复操作。
func (uv *updateVector[T]) Rollback() {
	// 清空位图（所有标记置0）
	for i := 0; i < uv.Length(); i++ {
		uv.clearBit(i)
	}
	// 清空缓存（置0，避免脏数据）
	clear(uv.cache)
}

// BuildFromDense 从稠密切片构建（覆盖底层数据，清空缓存）
func (uv *updateVector[T]) BuildFromDense(dense []T) {
	uv.Vector.BuildFromDense(dense)
	uv.Rollback() // 构建后清空缓存
}

// Zero 清空向量（底层+缓存均置0，清空位图）
func (uv *updateVector[T]) Zero() {
	uv.Vector.Zero()
	uv.Rollback()
}

// Copy 复制向量（底层数据+缓存状态+位图均复制）
func (uv *updateVector[T]) Copy(a Vector[T]) {
	switch target := a.(type) {
	case *updateVector[T]:
		if target.Length() != uv.Length() {
			panic(fmt.Sprintf("dimension mismatch: source length=%d, target length=%d", uv.Length(), target.Length()))
		}
		// 复制底层数据
		uv.Vector.Copy(target.Vector)
		// 复制缓存
		copy(target.cache, uv.cache)
		// 复制位图
		for i := 0; i < uv.Length(); i++ {
			if uv.isBitSet(i) {
				target.setBit(i)
			} else {
				target.clearBit(i)
			}
		}
	default:
		// 复制当前可见数据（缓存+底层）
		for i := 0; i < uv.Length(); i++ {
			val := uv.Get(i)
			var zero T
			if val != zero {
				target.Set(i, val)
			}
		}
	}
}

// String 返回向量当前状态的字符串表示，包括未提交的缓存修改。
func (uv *updateVector[T]) String() string {
	result := "["
	for i := 0; i < uv.Length(); i++ {
		result += fmt.Sprintf("%v ", uv.Get(i))
	}
	return result + "]"
}
