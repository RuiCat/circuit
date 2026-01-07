package maths

import (
	"circuit/utils"
	"fmt"
)

// updateMatrix 带缓存更新矩阵（基于稠密矩阵，支持缓存+回溯）
// 核心优化：频繁修改先写缓存，批量刷盘，支持回滚，提升性能
type updateMatrix struct {
	Matrix                            // 嵌入稠密矩阵（底层存储）
	bitmap        utils.Bitmap        // 位图：标记缓存修改位置（1=缓存有效）
	cache         map[int][16]float64 // 缓存：块存储（key=块索引，value=16元素块）
	blockSize     int                 // 块大小（固定16，对齐CPU缓存）
	rowResultCols []int               // GetRow 缓冲区：列索引
	rowResultVals []float64           // GetRow 缓冲区：值
	rowResultVec  *denseVector        // GetRow 缓冲区：返回的向量
}

// NewUpdateMatrix 从基础矩阵创建更新矩阵（复制基础矩阵数据）
func NewUpdateMatrix(base Matrix) UpdateMatrix {
	rows := base.Rows()
	cols := base.Cols()
	// 初始化底层稠密矩阵并复制基础数据
	dm := NewDenseMatrix(rows, cols)
	base.Copy(dm)
	return &updateMatrix{
		Matrix:        dm,
		bitmap:        utils.NewBitmap(rows * cols), // 位图长度=矩阵元素总数
		cache:         make(map[int][16]float64),
		blockSize:     16,
		rowResultCols: make([]int, 0, cols),
		rowResultVals: make([]float64, 0, cols),
		rowResultVec:  NewDenseVector(0).(*denseVector),
	}
}

// NewUpdateMatrixPtr 从基础矩阵指针创建更新矩阵
func NewUpdateMatrixPtr(ptr Matrix) UpdateMatrix {
	cols := ptr.Cols()
	return &updateMatrix{
		Matrix:        ptr,
		bitmap:        utils.NewBitmap(ptr.Rows() * ptr.Cols()), // 位图长度=矩阵元素总数
		cache:         make(map[int][16]float64),
		blockSize:     16,
		rowResultCols: make([]int, 0, cols),
		rowResultVals: make([]float64, 0, cols),
		rowResultVec:  NewDenseVector(0).(*denseVector),
	}
}

// getBlockIndexAndPosition 计算行列对应的缓存块索引和块内位置
func (um *updateMatrix) getBlockIndexAndPosition(row, col int) (int, int) {
	linearIndex := row*um.Cols() + col // 行列转一维索引
	return linearIndex / um.blockSize, linearIndex % um.blockSize
}

// isBitSet 检查位图中指定行列是否被标记（缓存是否有效）
func (um *updateMatrix) isBitSet(row, col int) bool {
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	return um.bitmap.Get(utils.BitmapFlag(blockIdx*um.blockSize + pos))
}

// setBit 标记位图中指定行列（缓存有效）
func (um *updateMatrix) setBit(row, col int) {
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	um.bitmap.Set(utils.BitmapFlag(blockIdx*um.blockSize+pos), true)
}

// clearBit 清除位图中指定行列（缓存无效）
func (um *updateMatrix) clearBit(row, col int) {
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	um.bitmap.Set(utils.BitmapFlag(blockIdx*um.blockSize+pos), false)
}

// Get 获取矩阵元素值（优先读缓存，缓存无效则读底层）
func (um *updateMatrix) Get(row, col int) float64 {
	if row < 0 || row >= um.Rows() || col < 0 || col >= um.Cols() {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, um.Rows(), um.Cols()))
	}
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	if um.isBitSet(row, col) {
		if block, exists := um.cache[blockIdx]; exists {
			return block[pos] // 缓存有效：返回缓存值
		}
	}
	return um.Matrix.Get(row, col) // 缓存无效：返回底层值
}

// Set 设置矩阵元素值（写缓存+标记位图）
func (um *updateMatrix) Set(row, col int, value float64) {
	if row < 0 || row >= um.Rows() || col < 0 || col >= um.Cols() {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, um.Rows(), um.Cols()))
	}
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	// 获取或创建缓存块
	block, exists := um.cache[blockIdx]
	if !exists {
		block = [16]float64{} // 初始化新块（默认0）
	}
	block[pos] = value // 写入缓存块
	um.cache[blockIdx] = block
	um.setBit(row, col) // 标记位图
}

// Increment 增量更新矩阵元素（缓存有效则累加，否则读底层后累加）
func (um *updateMatrix) Increment(row, col int, value float64) {
	if row < 0 || row >= um.Rows() || col < 0 || col >= um.Cols() {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, um.Rows(), um.Cols()))
	}
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	if um.isBitSet(row, col) {
		// 缓存有效：直接累加
		block := um.cache[blockIdx]
		block[pos] += value
		um.cache[blockIdx] = block
	} else {
		// 缓存无效：读底层值+累加，写入缓存
		block, exists := um.cache[blockIdx]
		if !exists {
			block = [16]float64{}
		}
		block[pos] = um.Matrix.Get(row, col) + value
		um.cache[blockIdx] = block
		um.setBit(row, col)
	}
}

// Update 批量更新：将缓存中修改的值刷到底层，清空缓存标记
func (um *updateMatrix) Update() {
	for blockIdx, block := range um.cache {
		for pos := 0; pos < um.blockSize; pos++ {
			// 块内位置转矩阵行列
			linearIndex := blockIdx*um.blockSize + pos
			row := linearIndex / um.Cols()
			col := linearIndex % um.Cols()
			// 检查行列有效性（避免越界）
			if row < um.Rows() && col < um.Cols() && um.isBitSet(row, col) {
				um.Matrix.Set(row, col, block[pos]) // 缓存刷底层
				um.clearBit(row, col)               // 清除标记
			}
		}
	}
	// 清空缓存（释放内存）
	clear(um.cache)
}

// Rollback 回溯：放弃缓存修改，清空缓存和位图
func (um *updateMatrix) Rollback() {
	// 清空位图（所有标记置0）
	totalElements := um.Rows() * um.Cols()
	for i := 0; i < totalElements; i++ {
		um.bitmap.Set(utils.BitmapFlag(i), false)
	}
	// 清空缓存
	clear(um.cache)
}

// BuildFromDense 从稠密矩阵构建（覆盖底层数据，清空缓存）
func (um *updateMatrix) BuildFromDense(dense [][]float64) {
	um.Matrix.BuildFromDense(dense)
	um.Rollback()
}

// Zero 清空矩阵（底层+缓存均置0，清空位图）
func (um *updateMatrix) Zero() {
	um.Matrix.Zero()
	um.Rollback()
}

// Copy 复制矩阵（底层数据+缓存状态+位图均复制）
func (um *updateMatrix) Copy(a Matrix) {
	switch target := a.(type) {
	case *updateMatrix:
		if target.Rows() != um.Rows() || target.Cols() != um.Cols() {
			panic(fmt.Sprintf("dimension mismatch: source %dx%d, target %dx%d", um.Rows(), um.Cols(), target.Rows(), target.Cols()))
		}
		// 复制底层矩阵
		um.Matrix.Copy(target.Matrix)
		// 复制缓存
		target.cache = make(map[int][16]float64)
		for k, v := range um.cache {
			target.cache[k] = v
		}
		// 复制位图
		totalElements := um.Rows() * um.Cols()
		for i := 0; i < totalElements; i++ {
			row := i / um.Cols()
			col := i % um.Cols()
			if um.isBitSet(row, col) {
				target.setBit(row, col)
			} else {
				target.clearBit(row, col)
			}
		}
	default:
		// 复制当前可见数据（缓存+底层）
		for i := 0; i < um.Rows(); i++ {
			for j := 0; j < um.Cols(); j++ {
				val := um.Get(i, j)
				if val != 0 {
					target.Set(i, j, val)
				}
			}
		}
	}
}

// GetRow 获取指定行的非零元素（合并缓存+底层数据），利用缓冲区避免重复内存分配
func (um *updateMatrix) GetRow(row int) ([]int, Vector) {
	if row < 0 || row >= um.Rows() {
		panic(fmt.Sprintf("row index out of range: %d (rows: %d)", row, um.Rows()))
	}
	// 1. 清空并重用缓冲区
	um.rowResultCols = um.rowResultCols[:0]
	um.rowResultVals = um.rowResultVals[:0]

	// 2. 遍历该行的所有列，合并缓存和底层数据
	for j := 0; j < um.Cols(); j++ {
		var val float64
		// 优先从缓存读取
		if um.isBitSet(row, j) {
			blockIdx, pos := um.getBlockIndexAndPosition(row, j)
			// A block should exist if the bit is set
			val = um.cache[blockIdx][pos]
		} else {
			// 缓存未命中，从底层矩阵读取
			val = um.Matrix.Get(row, j)
		}

		// 仅添加非零元素到结果中
		if val != 0 {
			um.rowResultCols = append(um.rowResultCols, j)
			um.rowResultVals = append(um.rowResultVals, val)
		}
	}

	// 3. 直接返回内部缓冲区的切片和向量，以最大化性能
	// 调用方不应修改返回的切片或向量
	um.rowResultVec.DataManager.Data = um.rowResultVals
	um.rowResultVec.DataManager.Len = len(um.rowResultVals)
	return um.rowResultCols, um.rowResultVec
}

// MatrixVectorMultiply 矩阵向量乘法（使用当前可见数据：缓存+底层）
func (um *updateMatrix) MatrixVectorMultiply(x Vector) Vector {
	if x.Length() != um.Cols() {
		panic(fmt.Sprintf("vector dimension mismatch: x length=%d, matrix cols=%d", x.Length(), um.Cols()))
	}
	result := NewDenseVector(um.Rows())
	for i := 0; i < um.Rows(); i++ {
		cols, vals := um.GetRow(i) // 获取该行所有可见元素
		for jIdx, col := range cols {
			result.Increment(i, vals.Get(jIdx)*x.Get(col))
		}
	}
	return result
}

// NonZeroCount 统计非零元素数量（缓存+底层）
func (um *updateMatrix) NonZeroCount() int {
	count := 0
	epsilon := 1e-16
	// 统计底层非零元素
	for i := 0; i < um.Rows(); i++ {
		cols, _ := um.Matrix.GetRow(i)
		count += len(cols)
	}
	// 统计缓存中新增/修改的非零元素
	for blockIdx, block := range um.cache {
		for pos := 0; pos < um.blockSize; pos++ {
			linearIndex := blockIdx*um.blockSize + pos
			row := linearIndex / um.Cols()
			col := linearIndex % um.Cols()
			if row < um.Rows() && col < um.Cols() && um.isBitSet(row, col) {
				baseVal := um.Matrix.Get(row, col)
				cachedVal := block[pos]
				// 情况1：底层为零，缓存非零 → 新增非零
				// 情况2：底层非零，缓存修改后仍非零 → 不重复统计
				// 情况3：底层非零，缓存修改为零 → 减少统计
				if (baseVal > -epsilon && baseVal < epsilon) && (cachedVal < -epsilon || cachedVal > epsilon) {
					count++
				} else if !(baseVal > -epsilon && baseVal < epsilon) && (cachedVal > -epsilon && cachedVal < epsilon) {
					count--
				}
			}
		}
	}
	return max(count, 0) // 避免负计数
}

// String 格式化输出矩阵（显示当前可见数据）
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

// Resize 重置矩阵大小和数据（清空所有元素）
func (um *updateMatrix) Resize(rows, cols int) {
	if rows < 0 || cols < 0 {
		panic("invalid matrix dimensions: cannot be negative")
	}
	// 重置底层数据大小
	clear(um.cache)
	um.Matrix.Resize(rows, cols)
	um.bitmap = utils.NewBitmap(rows * cols)
}
