package mat

import "fmt"

// UpdateVector 更新向量接口
// 扩展SparseVector接口，提供基于uint16分块位图的缓存机制
type UpdateVector interface {
	SparseVector // 继承SparseVector接口的所有方法

	// Update 更新操作
	// 将位图为1的值写入底层以后将位图设置为0
	Update()

	// Rollback 回溯操作
	// 将位图标记置0，清空缓存
	Rollback()
}

// updateVector 更新向量实现
// 实现UpdateVector接口，提供基于uint16分块位图的缓存机制
type updateVector struct {
	base   SparseVector // 底层稀疏向量
	length int          // 向量长度

	// 位图缓存系统
	bitmap    []uint16            // 分块位图，每个uint16表示16个元素的缓存状态
	cache     map[int][16]float64 // 缓存块，key为块索引，value为16个float64值
	blockSize int                 // 块大小（固定为16）
}

// NewUpdateVector 创建新的更新向量
// 参数：
//
//	base - 底层稀疏向量
//
// 返回：
//
//	UpdateVector - 新的更新向量实例
func NewUpdateVector(base SparseVector) UpdateVector {
	length := base.Length()
	blockSize := 16

	// 计算需要的位图数量
	bitmapSize := (length + blockSize - 1) / blockSize

	return &updateVector{
		base:      base,
		length:    length,
		bitmap:    make([]uint16, bitmapSize),
		cache:     make(map[int][16]float64),
		blockSize: blockSize,
	}
}

// getBlockIndexAndPosition 计算给定索引对应的块索引和块内位置
func (v *updateVector) getBlockIndexAndPosition(index int) (int, int) {
	blockIndex := index / v.blockSize
	position := index % v.blockSize
	return blockIndex, position
}

// isBitSet 检查位图中指定位置的bit是否为1
func (v *updateVector) isBitSet(blockIndex, position int) bool {
	return (v.bitmap[blockIndex] & (1 << position)) != 0
}

// setBit 设置位图中指定位置的bit为1
func (v *updateVector) setBit(blockIndex, position int) {
	v.bitmap[blockIndex] |= (1 << position)
}

// clearBit 清除位图中指定位置的bit（设置为0）
func (v *updateVector) clearBit(blockIndex, position int) {
	v.bitmap[blockIndex] &^= (1 << position)
}

// clearAllBits 清除所有位图（设置为0）
func (v *updateVector) clearAllBits() {
	for i := range v.bitmap {
		v.bitmap[i] = 0
	}
}

// Get 获取向量元素
// 先检查位图，如果位图为1则从cache中获取值，如果为0则从底层数据里面获取值
func (v *updateVector) Get(index int) float64 {
	if index < 0 || index >= v.length {
		panic("index out of range")
	}

	blockIndex, position := v.getBlockIndexAndPosition(index)

	if v.isBitSet(blockIndex, position) {
		// 从缓存中获取值
		if block, exists := v.cache[blockIndex]; exists {
			return block[position]
		}
	}

	// 从底层向量获取值
	return v.base.Get(index)
}

// Set 设置向量元素值
// 设置map值并且将位图设置为1
func (v *updateVector) Set(index int, value float64) {
	if index < 0 || index >= v.length {
		panic("index out of range")
	}

	blockIndex, position := v.getBlockIndexAndPosition(index)

	// 获取或创建缓存块
	block, exists := v.cache[blockIndex]
	if !exists {
		// 初始化新的缓存块
		block = [16]float64{}
	}

	// 设置缓存值
	block[position] = value
	v.cache[blockIndex] = block

	// 设置位图标记
	v.setBit(blockIndex, position)
}

// Increment 增量设置向量元素（累加值）
func (v *updateVector) Increment(index int, value float64) {
	if index < 0 || index >= v.length {
		panic("index out of range")
	}
	blockIndex, position := v.getBlockIndexAndPosition(index)
	if v.isBitSet(blockIndex, position) {
		// 在缓存中累加
		block := v.cache[blockIndex]
		block[position] += value
		v.cache[blockIndex] = block
	} else {
		// 不在缓存中，创建新的缓存项
		block, exists := v.cache[blockIndex]
		if !exists {
			block = [16]float64{}
		}
		block[position] = v.base.Get(index) + value
		v.cache[blockIndex] = block
		v.setBit(blockIndex, position)
	}
}

// Update 更新操作
// 将位图为1的值写入底层以后将位图设置为0
func (v *updateVector) Update() {
	for blockIndex, block := range v.cache {
		// 遍历块中的16个位置
		for position := 0; position < v.blockSize; position++ {
			if v.isBitSet(blockIndex, position) {
				// 计算原始索引位置
				index := blockIndex*v.blockSize + position

				// 检查索引是否有效
				if index < v.length {
					// 将缓存值写入底层向量
					v.base.Set(index, block[position])
					// 清除位图标记
					v.clearBit(blockIndex, position)
				}
			}
		}
	}

	// 清空缓存
	v.cache = make(map[int][16]float64)
}

// Rollback 回溯操作
// 将位图标记置0，清空缓存
func (v *updateVector) Rollback() {
	v.clearAllBits()
	v.cache = make(map[int][16]float64)
}

// BuildFromDense 从稠密向量构建稀疏向量
func (v *updateVector) BuildFromDense(dense []float64) {
	v.base.BuildFromDense(dense)
	v.Rollback() // 清空缓存
}

// Clear 清空向量，重置为零向量
func (v *updateVector) Clear() {
	v.base.Clear()
	v.Rollback() // 清空缓存
}

// Length 返回向量长度
func (v *updateVector) Length() int {
	return v.length
}

// Copy 复制向量内容到另一个向量
func (v *updateVector) Copy(a SparseVector) {
	switch target := a.(type) {
	case *updateVector:
		// 复制底层向量
		v.base.Copy(target.base)
		// 复制缓存状态
		target.length = v.length
		target.bitmap = make([]uint16, len(v.bitmap))
		copy(target.bitmap, v.bitmap)
		target.cache = make(map[int][16]float64)
		for k, v := range v.cache {
			target.cache[k] = v
		}
	default:
		// 对于其他类型的稀疏向量，只复制当前可见的数据（底层+缓存）
		// 不调用Update()，保持缓存状态不变
		for i := 0; i < v.length; i++ {
			value := v.Get(i)
			if value != 0 {
				a.Set(i, value)
			}
		}
	}
}

// ToDense 转换为稠密向量
func (v *updateVector) ToDense() []float64 {
	dense := make([]float64, v.length)
	for i := 0; i < v.length; i++ {
		dense[i] = v.Get(i)
	}
	return dense
}

// DotProduct 计算与另一个向量的点积
func (v *updateVector) DotProduct(other SparseVector) float64 {
	if other.Length() != v.length {
		panic("vector dimension mismatch")
	}

	result := 0.0
	// 遍历所有元素，包括缓存中的修改
	for i := 0; i < v.length; i++ {
		result += v.Get(i) * other.Get(i)
	}
	return result
}

// Scale 向量缩放
func (v *updateVector) Scale(scalar float64) {
	// 遍历所有元素，包括缓存中的修改
	for i := 0; i < v.length; i++ {
		value := v.Get(i)
		if value != 0 {
			v.Set(i, value*scalar)
		}
	}
}

// Add 向量加法
func (v *updateVector) Add(other SparseVector) {
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

// NonZeroCount 返回非零元素数量
func (v *updateVector) NonZeroCount() int {
	count := 0

	// 使用一个集合来跟踪已经处理过的索引
	processed := make(map[int]bool)

	// 首先处理缓存中的修改
	for blockIndex, block := range v.cache {
		for position := 0; position < v.blockSize; position++ {
			if v.isBitSet(blockIndex, position) {
				index := blockIndex*v.blockSize + position
				if index < v.length {
					cachedValue := block[position]
					// 如果缓存值不为0，则计数
					if cachedValue != 0 {
						count++
					}
					// 标记该索引已处理
					processed[index] = true
				}
			}
		}
	}

	// 然后处理底层向量中未被缓存修改的元素
	for i := 0; i < v.length; i++ {
		if !processed[i] {
			baseValue := v.base.Get(i)
			if baseValue != 0 {
				count++
			}
		}
	}

	return count
}

// String 返回向量的字符串表示
func (v *updateVector) String() string {
	result := "["
	for i := 0; i < v.length; i++ {
		result += fmt.Sprintf("%8.4f ", v.Get(i))
	}
	result += "]"
	return result
}
