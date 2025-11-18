package maths

import (
	"circuit/utils"
	"fmt"
)

// denseVector 稠密向量实现（基于DataManager，全量存储所有元素）
type denseVector struct {
	*DataManager // 嵌入DataManager复用功能
}

// NewDenseVector 创建指定长度的空稠密向量
func NewDenseVector(length int) Vector {
	return &denseVector{
		DataManager: NewDataManager(length),
	}
}

// NewDenseVectorWithData 从切片创建稠密向量
func NewDenseVectorWithData(data []float64) Vector {
	return &denseVector{
		DataManager: NewDataManagerWithData(data),
	}
}

// BuildFromDense 从稠密切片构建向量（覆盖原有数据）
func (v *denseVector) BuildFromDense(dense []float64) {
	if len(dense) != v.Length() {
		panic(fmt.Sprintf("dimension mismatch: len(dense)=%d, vector length=%d", len(dense), v.Length()))
	}
	v.ReplaceInPlace(0, dense...)
}

// Clear 清空向量为零向量
func (v *denseVector) Clear() {
	v.DataManager.Clear()
}

// Copy 复制自身数据到目标向量（支持稠密/其他向量类型）
func (v *denseVector) Copy(a Vector) {
	switch target := a.(type) {
	case *denseVector:
		// 同类型直接复制（高效）
		if target.Length() != v.Length() {
			panic(fmt.Sprintf("dimension mismatch: source length=%d, target length=%d", v.Length(), target.Length()))
		}
		v.DataManager.Copy(target.DataManager)
	default:
		// 异类型逐个元素复制（兼容稀疏等其他实现）
		for i := 0; i < v.Length(); i++ {
			val := v.Get(i)
			if val != 0 { // 非零元素才复制（优化）
				target.Set(i, val)
			}
		}
	}
}

// Get 获取指定索引元素值（越界panic）
func (v *denseVector) Get(index int) float64 {
	return v.DataManager.Get(index)
}

// Increment 增量更新元素（value累加，越界panic）
func (v *denseVector) Increment(index int, value float64) {
	v.DataManager.Increment(index, value)
}

// Length 返回向量长度
func (v *denseVector) Length() int {
	return v.DataManager.Length()
}

// NonZeroCount 统计非零元素数量
func (v *denseVector) NonZeroCount() int {
	return v.DataManager.NonZeroCount()
}

// Set 设置指定索引元素值（越界panic）
func (v *denseVector) Set(index int, value float64) {
	v.DataManager.Set(index, value)
}

// String 格式化输出向量
func (v *denseVector) String() string {
	return v.DataManager.String()
}

// ToDense 转换为稠密切片（返回拷贝）
func (v *denseVector) ToDense() []float64 {
	return v.DataManager.Data()
}

// DotProduct 计算与另一个向量的点积（维度不匹配panic）
func (v *denseVector) DotProduct(other Vector) float64 {
	if other.Length() != v.Length() {
		panic(fmt.Sprintf("dimension mismatch: this length=%d, other length=%d", v.Length(), other.Length()))
	}
	result := 0.0
	for i := 0; i < v.Length(); i++ {
		result += v.Get(i) * other.Get(i)
	}
	return result
}

// Scale 向量缩放（所有元素乘scalar）
func (v *denseVector) Scale(scalar float64) {
	for i := 0; i < v.Length(); i++ {
		v.Set(i, v.Get(i)*scalar)
	}
}

// Add 向量加法（自身 += 另一个向量，维度不匹配panic）
func (v *denseVector) Add(other Vector) {
	if other.Length() != v.Length() {
		panic(fmt.Sprintf("dimension mismatch: this length=%d, other length=%d", v.Length(), other.Length()))
	}
	for i := 0; i < v.Length(); i++ {
		v.Increment(i, other.Get(i))
	}
}

// updateVector 带缓存更新向量（基于稠密向量，支持缓存+回溯）
// 核心优化：频繁修改先写缓存，批量刷盘，支持回滚，提升性能
type updateVector struct {
	*denseVector              // 嵌入稠密向量（底层存储）
	bitmap       utils.Bitmap // 位图：标记哪些位置被缓存修改（1=缓存有效）
	cache        []float64    // 缓存：存储修改后的值（与底层向量同长度）
	blockSize    int          // 缓存块大小（固定16，对齐CPU缓存）
}

// NewUpdateVector 从基础向量创建更新向量（复制基础向量数据）
func NewUpdateVector(base Vector) UpdateVector {
	length := base.Length()
	// 初始化底层稠密向量并复制基础数据
	dv := NewDenseVector(length).(*denseVector)
	base.Copy(dv)
	return &updateVector{
		denseVector: dv,
		bitmap:      utils.NewBitmap(length), // 位图长度=向量长度
		cache:       make([]float64, length),
		blockSize:   16,
	}
}

// getBlockIndexAndPosition 计算索引对应的缓存块索引和块内位置
func (uv *updateVector) getBlockIndexAndPosition(index int) (int, int) {
	return index / uv.blockSize, index % uv.blockSize
}

// isBitSet 检查位图中指定索引是否被标记（缓存是否有效）
func (uv *updateVector) isBitSet(index int) bool {
	blockIdx, pos := uv.getBlockIndexAndPosition(index)
	return uv.bitmap.Get(utils.BitmapFlag(blockIdx*uv.blockSize + pos))
}

// setBit 标记位图中指定索引（缓存有效）
func (uv *updateVector) setBit(index int) {
	blockIdx, pos := uv.getBlockIndexAndPosition(index)
	uv.bitmap.Set(utils.BitmapFlag(blockIdx*uv.blockSize+pos), true)
}

// clearBit 清除位图中指定索引（缓存无效）
func (uv *updateVector) clearBit(index int) {
	blockIdx, pos := uv.getBlockIndexAndPosition(index)
	uv.bitmap.Set(utils.BitmapFlag(blockIdx*uv.blockSize+pos), false)
}

// Get 获取元素值（优先读缓存，缓存无效则读底层）
func (uv *updateVector) Get(index int) float64 {
	if index < 0 || index >= uv.Length() {
		panic(fmt.Sprintf("index out of range: %d (length: %d)", index, uv.Length()))
	}
	if uv.isBitSet(index) {
		return uv.cache[index] // 缓存有效：返回缓存值
	}
	return uv.denseVector.Get(index) // 缓存无效：返回底层值
}

// Set 设置元素值（写缓存+标记位图）
func (uv *updateVector) Set(index int, value float64) {
	if index < 0 || index >= uv.Length() {
		panic(fmt.Sprintf("index out of range: %d (length: %d)", index, uv.Length()))
	}
	uv.cache[index] = value // 写入缓存
	uv.setBit(index)        // 标记位图
}

// Increment 增量更新元素（缓存有效则累加，否则读底层后累加）
func (uv *updateVector) Increment(index int, value float64) {
	if index < 0 || index >= uv.Length() {
		panic(fmt.Sprintf("index out of range: %d (length: %d)", index, uv.Length()))
	}
	if uv.isBitSet(index) {
		uv.cache[index] += value // 缓存有效：直接累加
	} else {
		// 缓存无效：读底层值+累加，写入缓存
		uv.cache[index] = uv.denseVector.Get(index) + value
		uv.setBit(index)
	}
}

// Update 批量更新：将缓存中修改的值刷到底层，清空缓存标记
func (uv *updateVector) Update() {
	for i := 0; i < uv.Length(); i++ {
		if uv.isBitSet(i) {
			uv.denseVector.Set(i, uv.cache[i]) // 缓存值刷到底层
			uv.clearBit(i)                     // 清除位图标记
		}
	}
}

// Rollback 回溯：放弃缓存修改，清空缓存和位图
func (uv *updateVector) Rollback() {
	// 清空位图（所有标记置0）
	for i := 0; i < uv.Length(); i++ {
		uv.clearBit(i)
	}
	// 清空缓存（置0，避免脏数据）
	clear(uv.cache)
}

// BuildFromDense 从稠密切片构建（覆盖底层数据，清空缓存）
func (uv *updateVector) BuildFromDense(dense []float64) {
	uv.denseVector.BuildFromDense(dense)
	uv.Rollback() // 构建后清空缓存
}

// Clear 清空向量（底层+缓存均置0，清空位图）
func (uv *updateVector) Clear() {
	uv.denseVector.Clear()
	uv.Rollback()
}

// Copy 复制向量（底层数据+缓存状态+位图均复制）
func (uv *updateVector) Copy(a Vector) {
	switch target := a.(type) {
	case *updateVector:
		if target.Length() != uv.Length() {
			panic(fmt.Sprintf("dimension mismatch: source length=%d, target length=%d", uv.Length(), target.Length()))
		}
		// 复制底层数据
		uv.denseVector.Copy(target.denseVector)
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
			if val != 0 {
				target.Set(i, val)
			}
		}
	}
}

// String 格式化输出向量（显示当前可见数据）
func (uv *updateVector) String() string {
	result := "["
	for i := 0; i < uv.Length(); i++ {
		result += fmt.Sprintf("%8.4f ", uv.Get(i))
	}
	return result + "]"
}
