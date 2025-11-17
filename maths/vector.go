package maths

import (
	"circuit/utils"
	"fmt"
)

// denseVector 稠密向量实现
// 基于 DataManager 实现 Vector 接口
type denseVector struct {
	*DataManager
}

// NewDenseVector 创建新的稠密向量
func NewDenseVector(length int) Vector {
	return &denseVector{
		DataManager: NewDataManager(length),
	}
}

// NewDenseVectorWithData 从现有数据创建稠密向量
func NewDenseVectorWithData(data []float64) Vector {
	return &denseVector{
		DataManager: NewDataManagerWithData(data),
	}
}

// BuildFromDense 从稠密向量构建向量
func (v *denseVector) BuildFromDense(dense []float64) {
	if len(dense) != v.Length() {
		panic("dimension mismatch")
	}
	for i := 0; i < v.Length(); i++ {
		v.Set(i, dense[i])
	}
}

// Clear 清空向量，重置为零向量
func (v *denseVector) Clear() {
	v.DataManager.Clear()
}

// Copy 将自身值复制到 a 向量
func (v *denseVector) Copy(a Vector) {
	switch target := a.(type) {
	case *denseVector:
		// 直接复制数据管理器
		v.DataManager.Copy(target.DataManager)
	default:
		// 对于其他类型的向量实现，逐个元素复制
		for i := 0; i < v.Length(); i++ {
			value := v.Get(i)
			if value != 0 {
				a.Set(i, value)
			}
		}
	}
}

// Get 获取指定位置的元素值
func (v *denseVector) Get(index int) float64 {
	return v.DataManager.Get(index)
}

// Increment 增量设置向量元素（累加值）
func (v *denseVector) Increment(index int, value float64) {
	v.DataManager.Increment(index, value)
}

// Length 返回向量长度
func (v *denseVector) Length() int {
	return v.DataManager.Length()
}

// NonZeroCount 返回非零元素数量
func (v *denseVector) NonZeroCount() int {
	return v.DataManager.NonZeroCount()
}

// Set 设置向量元素值
func (v *denseVector) Set(index int, value float64) {
	v.DataManager.Set(index, value)
}

// String 返回向量的字符串表示
func (v *denseVector) String() string {
	return v.DataManager.String()
}

// ToDense 转换为稠密向量
func (v *denseVector) ToDense() []float64 {
	return v.DataManager.Data()
}

// DotProduct 计算与另一个向量的点积
func (v *denseVector) DotProduct(other Vector) float64 {
	if other.Length() != v.Length() {
		panic("vector dimension mismatch")
	}
	result := 0.0
	for i := 0; i < v.Length(); i++ {
		result += v.Get(i) * other.Get(i)
	}
	return result
}

// Scale 向量缩放
func (v *denseVector) Scale(scalar float64) {
	for i := 0; i < v.Length(); i++ {
		v.Set(i, v.Get(i)*scalar)
	}
}

// Add 向量加法
func (v *denseVector) Add(other Vector) {
	if other.Length() != v.Length() {
		panic("vector dimension mismatch")
	}
	for i := 0; i < v.Length(); i++ {
		v.Increment(i, other.Get(i))
	}
}

// ==================== 更新向量实现 ====================

// updateVector 更新向量实现
// 基于 denseVector 实现 UpdateVector 接口，集成 Bitmap 接口
type updateVector struct {
	*denseVector
	bitmap utils.Bitmap // 位图管理
	cache  []float64    // 缓存数据
}

// NewUpdateVector 创建新的更新向量
func NewUpdateVector(base Vector) UpdateVector {
	length := base.Length()
	return &updateVector{
		denseVector: &denseVector{
			DataManager: NewDataManager(length),
		},
		bitmap: utils.NewBitmap(length),
		cache:  make([]float64, length),
	}
}

// getBlockIndexAndPosition 计算给定索引对应的块索引和块内位置
func (uv *updateVector) getBlockIndexAndPosition(index int) (int, int) {
	blockSize := 16
	blockIndex := index / blockSize
	position := index % blockSize
	return blockIndex, position
}

// isBitSet 检查位图中指定位置的bit是否为1
func (uv *updateVector) isBitSet(blockIndex, position int) bool {
	return uv.bitmap.Get(utils.BitmapFlag(blockIndex*16 + position))
}

// setBit 设置位图中指定位置的bit为1
func (uv *updateVector) setBit(blockIndex, position int) {
	uv.bitmap.Set(utils.BitmapFlag(blockIndex*16+position), true)
}

// clearBit 清除位图中指定位置的bit（设置为0）
func (uv *updateVector) clearBit(blockIndex, position int) {
	uv.bitmap.Set(utils.BitmapFlag(blockIndex*16+position), false)
}

// Get 获取向量元素
// 先检查位图，如果位图为1则从cache中获取值，如果为0则从底层数据里面获取值
func (uv *updateVector) Get(index int) float64 {
	if index < 0 || index >= uv.Length() {
		panic("index out of range")
	}
	blockIndex, position := uv.getBlockIndexAndPosition(index)
	if uv.isBitSet(blockIndex, position) {
		// 从缓存中获取值
		return uv.cache[index]
	}
	// 从底层向量获取值
	return uv.denseVector.Get(index)
}

// Set 设置向量元素值
// 设置缓存值并且将位图设置为1
func (uv *updateVector) Set(index int, value float64) {
	if index < 0 || index >= uv.Length() {
		panic("index out of range")
	}
	blockIndex, position := uv.getBlockIndexAndPosition(index)
	// 设置缓存值
	uv.cache[index] = value
	// 设置位图标记
	uv.setBit(blockIndex, position)
}

// Increment 增量设置向量元素（累加值）
func (uv *updateVector) Increment(index int, value float64) {
	if index < 0 || index >= uv.Length() {
		panic("index out of range")
	}
	blockIndex, position := uv.getBlockIndexAndPosition(index)
	if uv.isBitSet(blockIndex, position) {
		// 在缓存中累加
		uv.cache[index] += value
	} else {
		// 不在缓存中，创建新的缓存项
		uv.cache[index] = uv.denseVector.Get(index) + value
		uv.setBit(blockIndex, position)
	}
}

// Update 更新操作
// 将位图为1的值写入底层以后将位图设置为0
func (uv *updateVector) Update() {
	for i := 0; i < uv.Length(); i++ {
		blockIndex, position := uv.getBlockIndexAndPosition(i)
		if uv.isBitSet(blockIndex, position) {
			// 将缓存值写入底层向量
			uv.denseVector.Set(i, uv.cache[i])
			// 清除位图标记
			uv.clearBit(blockIndex, position)
		}
	}
}

// Rollback 回溯操作
// 将位图标记置0，清空缓存
func (uv *updateVector) Rollback() {
	// 重置位图
	for i := 0; i < uv.Length(); i++ {
		blockIndex, position := uv.getBlockIndexAndPosition(i)
		uv.clearBit(blockIndex, position)
	}
	// 清空缓存
	clear(uv.cache)
}

// BuildFromDense 从稠密向量构建向量
func (uv *updateVector) BuildFromDense(dense []float64) {
	uv.denseVector.BuildFromDense(dense)
	uv.Rollback()
}

// Clear 清空向量，重置为零向量
func (uv *updateVector) Clear() {
	uv.denseVector.Clear()
	uv.Rollback()
}

// Copy 复制向量内容到另一个向量
func (uv *updateVector) Copy(a Vector) {
	switch target := a.(type) {
	case *updateVector:
		// 复制底层向量
		uv.denseVector.Copy(target.denseVector)
		// 复制缓存状态
		target.bitmap = utils.NewBitmap(uv.Length())
		target.cache = make([]float64, uv.Length())
		copy(target.cache, uv.cache)
		// 复制位图状态
		for i := 0; i < uv.Length(); i++ {
			blockIndex, position := uv.getBlockIndexAndPosition(i)
			if uv.isBitSet(blockIndex, position) {
				target.setBit(blockIndex, position)
			}
		}
	default:
		// 对于其他类型的向量，只复制当前可见的数据（底层+缓存）
		for i := 0; i < uv.Length(); i++ {
			value := uv.Get(i)
			if value != 0 {
				a.Set(i, value)
			}
		}
	}
}

// String 返回向量的字符串表示
func (uv *updateVector) String() string {
	result := "["
	for i := 0; i < uv.Length(); i++ {
		result += fmt.Sprintf("%8.4f ", uv.Get(i))
	}
	result += "]"
	return result
}
