package mat

import "fmt"

// UpdateMatrix 更新矩阵接口
// 扩展SparseMatrix接口，提供基于uint16分块位图的缓存机制
type UpdateMatrix interface {
	SparseMatrix // 继承SparseMatrix接口的所有方法

	// Update 更新操作
	// 将位图为1的值写入底层以后将位图设置为0
	Update()

	// Rollback 回溯操作
	// 将位图标记置0，清空缓存
	Rollback()
}

// updateMatrix 更新矩阵实现
// 实现UpdateMatrix接口，提供基于uint16分块位图的缓存机制
type updateMatrix struct {
	base       SparseMatrix // 底层稀疏矩阵
	rows, cols int          // 矩阵维度

	// 位图缓存系统
	bitmap    []uint16            // 分块位图，每个uint16表示16个元素的缓存状态
	cache     map[int][16]float64 // 缓存块，key为块索引，value为16个float64值
	blockSize int                 // 块大小（固定为16）
}

// NewUpdateMatrix 创建新的更新矩阵
// 参数：
//
//	base - 底层稀疏矩阵
//
// 返回：
//
//	UpdateMatrix - 新的更新矩阵实例
func NewUpdateMatrix(base SparseMatrix) UpdateMatrix {
	rows := base.Rows()
	cols := base.Cols()
	blockSize := 16

	// 计算需要的位图数量
	bitmapSize := (rows*cols + blockSize - 1) / blockSize

	return &updateMatrix{
		base:      base,
		rows:      rows,
		cols:      cols,
		bitmap:    make([]uint16, bitmapSize),
		cache:     make(map[int][16]float64),
		blockSize: blockSize,
	}
}

// getBlockIndexAndPosition 计算给定行列对应的块索引和块内位置
func (m *updateMatrix) getBlockIndexAndPosition(row, col int) (int, int) {
	linearIndex := row*m.cols + col
	blockIndex := linearIndex / m.blockSize
	position := linearIndex % m.blockSize
	return blockIndex, position
}

// isBitSet 检查位图中指定位置的bit是否为1
func (m *updateMatrix) isBitSet(blockIndex, position int) bool {
	return (m.bitmap[blockIndex] & (1 << position)) != 0
}

// setBit 设置位图中指定位置的bit为1
func (m *updateMatrix) setBit(blockIndex, position int) {
	m.bitmap[blockIndex] |= (1 << position)
}

// clearBit 清除位图中指定位置的bit（设置为0）
func (m *updateMatrix) clearBit(blockIndex, position int) {
	m.bitmap[blockIndex] &^= (1 << position)
}

// clearAllBits 清除所有位图（设置为0）
func (m *updateMatrix) clearAllBits() {
	for i := range m.bitmap {
		m.bitmap[i] = 0
	}
}

// Get 获取矩阵元素
// 先检查位图，如果位图为1则从cache中获取值，如果为0则从底层数据里面获取值
func (m *updateMatrix) Get(row, col int) float64 {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic("index out of range")
	}

	blockIndex, position := m.getBlockIndexAndPosition(row, col)

	if m.isBitSet(blockIndex, position) {
		// 从缓存中获取值
		if block, exists := m.cache[blockIndex]; exists {
			return block[position]
		}
	}

	// 从底层矩阵获取值
	return m.base.Get(row, col)
}

// Set 设置矩阵元素值
// 设置map值并且将位图设置为1
func (m *updateMatrix) Set(row, col int, value float64) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic("index out of range")
	}

	blockIndex, position := m.getBlockIndexAndPosition(row, col)

	// 获取或创建缓存块
	block, exists := m.cache[blockIndex]
	if !exists {
		// 初始化新的缓存块
		block = [16]float64{}
	}

	// 设置缓存值
	block[position] = value
	m.cache[blockIndex] = block

	// 设置位图标记
	m.setBit(blockIndex, position)
}

// Increment 增量设置矩阵元素（累加值）
func (m *updateMatrix) Increment(row, col int, value float64) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic("index out of range")
	}
	blockIndex, position := m.getBlockIndexAndPosition(row, col)
	if m.isBitSet(blockIndex, position) {
		// 在缓存中累加
		block := m.cache[blockIndex]
		block[position] += value
		m.cache[blockIndex] = block
	} else {
		// 不在缓存中，创建新的缓存项
		block, exists := m.cache[blockIndex]
		if !exists {
			block = [16]float64{}
		}
		block[position] = m.base.Get(row, col) + value
		m.cache[blockIndex] = block
		m.setBit(blockIndex, position)
	}
}

// Update 更新操作
// 将位图为1的值写入底层以后将位图设置为0
func (m *updateMatrix) Update() {
	for blockIndex, block := range m.cache {
		// 遍历块中的16个位置
		for position := 0; position < m.blockSize; position++ {
			if m.isBitSet(blockIndex, position) {
				// 计算原始行列位置
				linearIndex := blockIndex*m.blockSize + position
				row := linearIndex / m.cols
				col := linearIndex % m.cols

				// 检查行列是否有效
				if row < m.rows && col < m.cols {
					// 将缓存值写入底层矩阵
					m.base.Set(row, col, block[position])
					// 清除位图标记
					m.clearBit(blockIndex, position)
				}
			}
		}
	}

	// 清空缓存
	m.cache = make(map[int][16]float64)
}

// Rollback 回溯操作
// 将位图标记置0，清空缓存
func (m *updateMatrix) Rollback() {
	m.clearAllBits()
	m.cache = make(map[int][16]float64)
}

// BuildFromDense 从稠密矩阵构建稀疏矩阵
func (m *updateMatrix) BuildFromDense(dense [][]float64) {
	m.base.BuildFromDense(dense)
	m.Rollback() // 清空缓存
}

// Clear 清空矩阵，重置为零矩阵
func (m *updateMatrix) Clear() {
	m.base.Clear()
	m.Rollback() // 清空缓存
}

// Cols 返回矩阵列数
func (m *updateMatrix) Cols() int {
	return m.cols
}

// Copy 复制矩阵内容到另一个矩阵
func (m *updateMatrix) Copy(a SparseMatrix) {
	switch target := a.(type) {
	case *updateMatrix:
		// 复制底层矩阵
		m.base.Copy(target.base)
		// 复制缓存状态
		target.rows, target.cols = m.rows, m.cols
		target.bitmap = make([]uint16, len(m.bitmap))
		copy(target.bitmap, m.bitmap)
		target.cache = make(map[int][16]float64)
		for k, v := range m.cache {
			target.cache[k] = v
		}
	default:
		// 对于其他类型的稀疏矩阵，只复制当前可见的数据（底层+缓存）
		// 不调用Update()，保持缓存状态不变
		for i := 0; i < m.rows; i++ {
			for j := 0; j < m.cols; j++ {
				value := m.Get(i, j)
				if value != 0 {
					a.Set(i, j, value)
				}
			}
		}
	}
}

// GetRow 获取指定行的所有非零元素（列索引和值）
func (m *updateMatrix) GetRow(row int) ([]int, []float64) {
	if row < 0 || row >= m.rows {
		panic("row index out of range")
	}

	// 获取底层矩阵的行数据
	baseCols, baseValues := m.base.GetRow(row)

	// 合并缓存中的修改
	resultCols := make([]int, 0, len(baseCols))
	resultValues := make([]float64, 0, len(baseValues))

	// 复制底层数据
	for i := range baseCols {
		resultCols = append(resultCols, baseCols[i])
		resultValues = append(resultValues, baseValues[i])
	}

	// 处理该行的缓存修改
	for col := 0; col < m.cols; col++ {
		blockIndex, position := m.getBlockIndexAndPosition(row, col)
		if m.isBitSet(blockIndex, position) {
			if block, exists := m.cache[blockIndex]; exists {
				cachedValue := block[position]
				// 查找是否已存在该列
				found := false
				for i, c := range resultCols {
					if c == col {
						resultValues[i] = cachedValue
						found = true
						break
					}
				}
				if !found && cachedValue != 0 {
					resultCols = append(resultCols, col)
					resultValues = append(resultValues, cachedValue)
				}
			}
		}
	}

	return resultCols, resultValues
}

// IsSquare 检查矩阵是否为方阵
func (m *updateMatrix) IsSquare() bool {
	return m.rows == m.cols
}

// MatrixVectorMultiply 执行矩阵向量乘法
func (m *updateMatrix) MatrixVectorMultiply(x []float64) []float64 {
	if len(x) != m.cols {
		panic("vector dimension mismatch")
	}

	result := make([]float64, m.rows)

	// 处理每一行
	for i := 0; i < m.rows; i++ {
		// 获取该行的所有元素（包括缓存）
		cols, values := m.GetRow(i)
		for j := range cols {
			result[i] += values[j] * x[cols[j]]
		}
	}

	return result
}

// NonZeroCount 返回非零元素数量
func (m *updateMatrix) NonZeroCount() int {
	count := 0

	// 统计底层矩阵的非零元素
	for i := 0; i < m.rows; i++ {
		cols, _ := m.base.GetRow(i)
		count += len(cols)
	}

	// 统计缓存中的修改（只统计不在底层矩阵中的新元素）
	for blockIndex, block := range m.cache {
		for position := 0; position < m.blockSize; position++ {
			if m.isBitSet(blockIndex, position) {
				linearIndex := blockIndex*m.blockSize + position
				row := linearIndex / m.cols
				col := linearIndex % m.cols
				if row < m.rows && col < m.cols {
					// 检查该位置是否在底层矩阵中
					baseValue := m.base.Get(row, col)
					cachedValue := block[position]
					// 如果底层为0但缓存不为0，或者缓存值与底层不同，则计数
					if (baseValue == 0 && cachedValue != 0) || (baseValue != cachedValue) {
						count++
					}
				}
			}
		}
	}

	return count
}

// Rows 返回矩阵行数
func (m *updateMatrix) Rows() int {
	return m.rows
}

// String 返回矩阵的字符串表示
func (m *updateMatrix) String() string {
	result := ""
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result += fmt.Sprintf("%8.4f ", m.Get(i, j))
		}
		result += "\n"
	}
	return result
}
