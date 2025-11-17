package maths

import (
	"circuit/utils"
	"fmt"
)

// denseMatrix 稠密矩阵实现
// 基于 MatrixDataManager 实现 Matrix 接口
type denseMatrix struct {
	*MatrixDataManager
}

// NewDenseMatrix 创建新的稠密矩阵
func NewDenseMatrix(rows, cols int) Matrix {
	return &denseMatrix{
		MatrixDataManager: NewMatrixDataManager(rows, cols),
	}
}

// BuildFromDense 从稠密矩阵构建矩阵
func (m *denseMatrix) BuildFromDense(dense [][]float64) {
	m.MatrixDataManager.BuildFromDense(dense)
}

// Clear 清空矩阵，重置为零矩阵
func (m *denseMatrix) Clear() {
	m.MatrixDataManager.Clear()
}

// Cols 返回矩阵列数
func (m *denseMatrix) Cols() int {
	return m.MatrixDataManager.Cols()
}

// Copy 将自身值复制到 a 矩阵
func (m *denseMatrix) Copy(a Matrix) {
	switch target := a.(type) {
	case *denseMatrix:
		// 直接复制矩阵数据管理器
		m.MatrixDataManager.DataManager.Copy(target.MatrixDataManager.DataManager)
		target.MatrixDataManager.rows = m.MatrixDataManager.rows
		target.MatrixDataManager.cols = m.MatrixDataManager.cols
	default:
		// 对于其他类型的矩阵实现，逐个元素复制
		for i := 0; i < m.Rows(); i++ {
			for j := 0; j < m.Cols(); j++ {
				value := m.Get(i, j)
				if value != 0 {
					a.Set(i, j, value)
				}
			}
		}
	}
}

// Get 获取指定位置的元素值
func (m *denseMatrix) Get(row int, col int) float64 {
	return m.MatrixDataManager.GetMatrix(row, col)
}

// GetRow 获取指定行的所有元素
func (m *denseMatrix) GetRow(row int) ([]int, Vector) {
	cols, values := m.MatrixDataManager.GetRow(row)
	return cols, NewDenseVectorWithData(values)
}

// Increment 增量设置矩阵元素（累加值）
func (m *denseMatrix) Increment(row int, col int, value float64) {
	m.MatrixDataManager.IncrementMatrix(row, col, value)
}

// IsSquare 检查矩阵是否为方阵
func (m *denseMatrix) IsSquare() bool {
	return m.MatrixDataManager.IsSquare()
}

// MatrixVectorMultiply 执行矩阵向量乘法
func (m *denseMatrix) MatrixVectorMultiply(x Vector) Vector {
	if x.Length() != m.Cols() {
		panic("vector dimension mismatch")
	}
	result := NewDenseVector(m.Rows())
	for i := 0; i < m.Rows(); i++ {
		sum := 0.0
		for j := 0; j < m.Cols(); j++ {
			sum += m.Get(i, j) * x.Get(j)
		}
		result.Set(i, sum)
	}
	return result
}

// NonZeroCount 返回非零元素数量
func (m *denseMatrix) NonZeroCount() int {
	return m.MatrixDataManager.NonZeroCount()
}

// Rows 返回矩阵行数
func (m *denseMatrix) Rows() int {
	return m.MatrixDataManager.Rows()
}

// Set 设置矩阵元素值
func (m *denseMatrix) Set(row int, col int, value float64) {
	m.MatrixDataManager.SetMatrix(row, col, value)
}

// String 返回矩阵的字符串表示
func (m *denseMatrix) String() string {
	return m.MatrixDataManager.String()
}

// ToDense 转换为稠密向量
func (m *denseMatrix) ToDense() Vector {
	return NewDenseVectorWithData(m.MatrixDataManager.ToDense())
}

// ==================== 更新矩阵实现 ====================

// updateMatrix 更新矩阵实现
// 基于 denseMatrix 实现 UpdateMatrix 接口，集成 Bitmap 接口
type updateMatrix struct {
	*denseMatrix
	bitmap    utils.Bitmap        // 位图管理
	cache     map[int][16]float64 // 缓存块
	blockSize int                 // 块大小
}

// NewUpdateMatrix 创建新的更新矩阵
func NewUpdateMatrix(base Matrix) UpdateMatrix {
	rows := base.Rows()
	cols := base.Cols()
	blockSize := 16
	return &updateMatrix{
		denseMatrix: &denseMatrix{
			MatrixDataManager: NewMatrixDataManager(rows, cols),
		},
		bitmap:    utils.NewBitmap(rows * cols),
		cache:     make(map[int][16]float64),
		blockSize: blockSize,
	}
}

// getBlockIndexAndPosition 计算给定行列对应的块索引和块内位置
func (um *updateMatrix) getBlockIndexAndPosition(row, col int) (int, int) {
	linearIndex := row*um.Cols() + col
	blockIndex := linearIndex / um.blockSize
	position := linearIndex % um.blockSize
	return blockIndex, position
}

// isBitSet 检查位图中指定位置的bit是否为1
func (um *updateMatrix) isBitSet(blockIndex, position int) bool {
	return um.bitmap.Get(utils.BitmapFlag(blockIndex*um.blockSize + position))
}

// setBit 设置位图中指定位置的bit为1
func (um *updateMatrix) setBit(blockIndex, position int) {
	um.bitmap.Set(utils.BitmapFlag(blockIndex*um.blockSize+position), true)
}

// clearBit 清除位图中指定位置的bit（设置为0）
func (um *updateMatrix) clearBit(blockIndex, position int) {
	um.bitmap.Set(utils.BitmapFlag(blockIndex*um.blockSize+position), false)
}

// Get 获取矩阵元素
// 先检查位图，如果位图为1则从cache中获取值，如果为0则从底层数据里面获取值
func (um *updateMatrix) Get(row, col int) float64 {
	if row < 0 || row >= um.Rows() || col < 0 || col >= um.Cols() {
		panic("index out of range")
	}
	blockIndex, position := um.getBlockIndexAndPosition(row, col)
	if um.isBitSet(blockIndex, position) {
		// 从缓存中获取值
		if block, exists := um.cache[blockIndex]; exists {
			return block[position]
		}
	}
	// 从底层矩阵获取值
	return um.denseMatrix.Get(row, col)
}

// Set 设置矩阵元素值
// 设置缓存值并且将位图设置为1
func (um *updateMatrix) Set(row, col int, value float64) {
	if row < 0 || row >= um.Rows() || col < 0 || col >= um.Cols() {
		panic("index out of range")
	}
	blockIndex, position := um.getBlockIndexAndPosition(row, col)
	// 获取或创建缓存块
	block, exists := um.cache[blockIndex]
	if !exists {
		// 初始化新的缓存块
		block = [16]float64{}
	}
	// 设置缓存值
	block[position] = value
	um.cache[blockIndex] = block
	// 设置位图标记
	um.setBit(blockIndex, position)
}

// Increment 增量设置矩阵元素（累加值）
func (um *updateMatrix) Increment(row, col int, value float64) {
	if row < 0 || row >= um.Rows() || col < 0 || col >= um.Cols() {
		panic("index out of range")
	}
	blockIndex, position := um.getBlockIndexAndPosition(row, col)
	if um.isBitSet(blockIndex, position) {
		// 在缓存中累加
		block := um.cache[blockIndex]
		block[position] += value
		um.cache[blockIndex] = block
	} else {
		// 不在缓存中，创建新的缓存项
		block, exists := um.cache[blockIndex]
		if !exists {
			block = [16]float64{}
		}
		block[position] = um.denseMatrix.Get(row, col) + value
		um.cache[blockIndex] = block
		um.setBit(blockIndex, position)
	}
}

// Update 更新操作
// 将位图为1的值写入底层以后将位图设置为0
func (um *updateMatrix) Update() {
	for blockIndex, block := range um.cache {
		// 遍历块中的16个位置
		for position := 0; position < um.blockSize; position++ {
			if um.isBitSet(blockIndex, position) {
				// 计算原始行列位置
				linearIndex := blockIndex*um.blockSize + position
				row := linearIndex / um.Cols()
				col := linearIndex % um.Cols()

				// 检查行列是否有效
				if row < um.Rows() && col < um.Cols() {
					// 将缓存值写入底层矩阵
					um.denseMatrix.Set(row, col, block[position])
					// 清除位图标记
					um.clearBit(blockIndex, position)
				}
			}
		}
	}
}

// Rollback 回溯操作
// 将位图标记置0，清空缓存
func (um *updateMatrix) Rollback() {
	// 重置位图
	for i := 0; i < um.Rows()*um.Cols(); i++ {
		blockIndex := i / um.blockSize
		position := i % um.blockSize
		um.clearBit(blockIndex, position)
	}
	// 清空缓存
	clear(um.cache)
}

// BuildFromDense 从稠密矩阵构建矩阵
func (um *updateMatrix) BuildFromDense(dense [][]float64) {
	um.denseMatrix.BuildFromDense(dense)
	um.Rollback()
}

// Clear 清空矩阵，重置为零矩阵
func (um *updateMatrix) Clear() {
	um.denseMatrix.Clear()
	um.Rollback()
}

// Copy 复制矩阵内容到另一个矩阵
func (um *updateMatrix) Copy(a Matrix) {
	switch target := a.(type) {
	case *updateMatrix:
		// 复制底层矩阵
		um.denseMatrix.Copy(target.denseMatrix)
		// 复制缓存状态
		target.bitmap = utils.NewBitmap(um.Rows() * um.Cols())
		target.cache = make(map[int][16]float64)
		for k, v := range um.cache {
			target.cache[k] = v
		}
		// 复制位图状态
		for i := 0; i < um.Rows()*um.Cols(); i++ {
			blockIndex := i / um.blockSize
			position := i % um.blockSize
			if um.isBitSet(blockIndex, position) {
				target.setBit(blockIndex, position)
			}
		}
	default:
		// 对于其他类型的矩阵，只复制当前可见的数据（底层+缓存）
		for i := 0; i < um.Rows(); i++ {
			for j := 0; j < um.Cols(); j++ {
				value := um.Get(i, j)
				if value != 0 {
					a.Set(i, j, value)
				}
			}
		}
	}
}

// GetRow 获取指定行的所有元素
func (um *updateMatrix) GetRow(row int) ([]int, Vector) {
	if row < 0 || row >= um.Rows() {
		panic("row index out of range")
	}
	// 获取底层矩阵的行数据
	baseCols, baseVector := um.denseMatrix.GetRow(row)
	baseValues := baseVector.ToDense()
	// 合并缓存中的修改
	resultCols := make([]int, 0, len(baseCols))
	resultValues := make([]float64, 0, len(baseValues))
	// 复制底层数据
	for i := range baseCols {
		resultCols = append(resultCols, baseCols[i])
		resultValues = append(resultValues, baseValues[i])
	}
	// 处理该行的缓存修改
	for col := 0; col < um.Cols(); col++ {
		blockIndex, position := um.getBlockIndexAndPosition(row, col)
		if um.isBitSet(blockIndex, position) {
			if block, exists := um.cache[blockIndex]; exists {
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
	return resultCols, NewDenseVectorWithData(resultValues)
}

// MatrixVectorMultiply 执行矩阵向量乘法
func (um *updateMatrix) MatrixVectorMultiply(x Vector) Vector {
	if x.Length() != um.Cols() {
		panic("vector dimension mismatch")
	}
	result := NewDenseVector(um.Rows())
	// 处理每一行
	for i := 0; i < um.Rows(); i++ {
		// 获取该行的所有元素（包括缓存）
		cols, values := um.GetRow(i)
		for j := range cols {
			result.Increment(i, values.Get(j)*x.Get(cols[j]))
		}
	}
	return result
}

// NonZeroCount 返回非零元素数量
func (um *updateMatrix) NonZeroCount() int {
	count := 0
	// 统计底层矩阵的非零元素
	for i := 0; i < um.Rows(); i++ {
		cols, _ := um.denseMatrix.GetRow(i)
		count += len(cols)
	}
	// 统计缓存中的修改（只统计不在底层矩阵中的新元素）
	for blockIndex, block := range um.cache {
		for position := 0; position < um.blockSize; position++ {
			if um.isBitSet(blockIndex, position) {
				linearIndex := blockIndex*um.blockSize + position
				row := linearIndex / um.Cols()
				col := linearIndex % um.Cols()
				if row < um.Rows() && col < um.Cols() {
					// 检查该位置是否在底层矩阵中
					baseValue := um.denseMatrix.Get(row, col)
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

// String 返回矩阵的字符串表示
func (um *updateMatrix) String() string {
	result := ""
	for i := 0; i < um.Rows(); i++ {
		for j := 0; j < um.Cols(); j++ {
			result += fmt.Sprintf("%8.4f ", um.Get(i, j))
		}
		result += "\n"
	}
	return result
}
