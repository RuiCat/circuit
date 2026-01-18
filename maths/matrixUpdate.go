package maths

import (
	"circuit/utils"
	"fmt"
)

// updateMatrix 为任矩阵实现提供了一个带缓存的装饰器，以优化频繁、临时的修改操作。
// 它的核心思想是：写操作首先进入一个临时缓存，而不是直接修改底层矩阵。
// 用户可以随后选择“提交”（Update）这些更改，将它们批量写入底层矩阵，
// 或者“回滚”（Rollback），直接丢弃缓存中的所有更改。
// 这种机制在需要进行探索性计算或需要撤销操作的场景下非常高效。
type updateMatrix[T Number] struct {
	Matrix[T]                     // Matrix 是底层的矩阵，存储着“已提交”的稳定数据。
	bitmap        utils.Bitmap    // bitmap 用于标记哪些矩阵元素在缓存中被修改过。每一位对应一个元素。
	cache         map[int][16]T   // cache 是一个分块缓存。key 是块索引，value 是一个固定大小的数组（块）。
	blockSize     int             // blockSize 定义了缓存块的大小，固定为16，这有助于利用CPU缓存行对齐。
	rowResultCols []int           // rowResultCols 是 GetRow 方法的列索引缓冲区，用于避免重复内存分配。
	rowResultVals []T             // rowResultVals 是 GetRow 方法的值缓冲区。
	rowResultVec  *denseVector[T] // rowResultVec 是 GetRow 方法返回的向量，重用此实例以减少GC压力。
}

// NewUpdateMatrix 基于一个现有的矩阵创建一个新的 updateMatrix。
// 它会深度复制基础矩阵的数据，确保两者在创建后完全独立。
func NewUpdateMatrix[T Number](base Matrix[T]) UpdateMatrix[T] {
	rows := base.Rows()
	cols := base.Cols()
	// 初始化底层的矩阵，并将基础矩阵的数据复制过来。
	dm := NewDenseMatrix[T](rows, cols)
	base.Copy(dm)
	return &updateMatrix[T]{
		Matrix:        dm,
		bitmap:        utils.NewBitmap(rows * cols), // 位图的大小等于矩阵元素的总数。
		cache:         make(map[int][16]T),
		blockSize:     16,
		rowResultCols: make([]int, 0, cols),
		rowResultVals: make([]T, 0, cols),
		rowResultVec:  NewDenseVector[T](0).(*denseVector[T]),
	}
}

// NewUpdateMatrixPtr 从一个矩阵指针创建一个新的 updateMatrix。
// 与 NewUpdateMatrix 不同，此函数不复制底层数据，而是直接使用传入的矩阵指针。
// 这意味着对 updateMatrix 的 `Update` 操作会直接修改原始矩阵。
func NewUpdateMatrixPtr[T Number](ptr Matrix[T]) UpdateMatrix[T] {
	cols := ptr.Cols()
	return &updateMatrix[T]{
		Matrix:        ptr,
		bitmap:        utils.NewBitmap(ptr.Rows() * ptr.Cols()),
		cache:         make(map[int][16]T),
		blockSize:     16,
		rowResultCols: make([]int, 0, cols),
		rowResultVals: make([]T, 0, cols),
		rowResultVec:  NewDenseVector[T](0).(*denseVector[T]),
	}
}

// getBlockIndexAndPosition 是一个内部辅助函数，用于将二维的（行，列）索引转换为
// 一维的缓存块索引和块内偏移量。
func (um *updateMatrix[T]) getBlockIndexAndPosition(row, col int) (int, int) {
	linearIndex := row*um.Cols() + col // 首先将二维索引转换为一维线性索引。
	return linearIndex / um.blockSize, linearIndex % um.blockSize
}

// isBitSet 检查给定（行，列）的元素是否在缓存中被修改过（即其在位图中的对应位是否被设置）。
func (um *updateMatrix[T]) isBitSet(row, col int) bool {
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	return um.bitmap.Get(utils.BitmapFlag(blockIdx*um.blockSize + pos))
}

// setBit 在位图中标记给定（行，列）的元素，表示其值已在缓存中更新。
func (um *updateMatrix[T]) setBit(row, col int) {
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	um.bitmap.Set(utils.BitmapFlag(blockIdx*um.blockSize+pos), true)
}

// clearBit 清除给定（行，列）元素在位图中的标记，通常在缓存被提交或回滚后调用。
func (um *updateMatrix[T]) clearBit(row, col int) {
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	um.bitmap.Set(utils.BitmapFlag(blockIdx*um.blockSize+pos), false)
}

// Get 获取指定位置的元素值。它实现了“读时合并”的逻辑：
// 1. 检查位图，判断该元素是否在缓存中。
// 2. 如果是，则从缓存中读取最新值。
// 3. 如果否，则从底层矩阵中读取稳定值。
func (um *updateMatrix[T]) Get(row, col int) T {
	if row < 0 || row >= um.Rows() || col < 0 || col >= um.Cols() {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, um.Rows(), um.Cols()))
	}
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	if um.isBitSet(row, col) {
		if block, exists := um.cache[blockIdx]; exists {
			return block[pos]
		}
	}
	return um.Matrix.Get(row, col)
}

// Set 设置指定位置的元素值。该操作只写缓存，不直接触及底层矩阵。
// 1. 计算元素的缓存块索引和块内位置。
// 2. 获取或创建对应的缓存块。
// 3. 将新值写入缓存块。
// 4. 在位图中标记该元素为“已修改”。
func (um *updateMatrix[T]) Set(row, col int, value T) {
	if row < 0 || row >= um.Rows() || col < 0 || col >= um.Cols() {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, um.Rows(), um.Cols()))
	}
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	block, exists := um.cache[blockIdx]
	if !exists {
		block = [16]T{}
	}
	block[pos] = value
	um.cache[blockIdx] = block
	um.setBit(row, col)
}

// Increment 增量更新指定位置的元素值。这是一个“读-改-写”操作，但被优化为只写缓存。
// 1. 如果元素已在缓存中，直接在缓存值上进行累加。
// 2. 如果元素不在缓存中，则先从底层矩阵读取原值，与增量相加后，将结果存入缓存，并标记该位置。
func (um *updateMatrix[T]) Increment(row, col int, value T) {
	if row < 0 || row >= um.Rows() || col < 0 || col >= um.Cols() {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, um.Rows(), um.Cols()))
	}
	blockIdx, pos := um.getBlockIndexAndPosition(row, col)
	if um.isBitSet(row, col) {
		block := um.cache[blockIdx]
		block[pos] += value
		um.cache[blockIdx] = block
	} else {
		block, exists := um.cache[blockIdx]
		if !exists {
			block = [16]T{}
		}
		block[pos] = um.Matrix.Get(row, col) + value
		um.cache[blockIdx] = block
		um.setBit(row, col)
	}
}

// Update 将缓存中的所有修改“提交”到底层矩阵。
// 这是一个批量操作，可以显著减少对底层矩阵的写操作次数。
// 遍历所有缓存块，并将其中被标记为已修改的元素写回底层矩阵，然后清除标记和缓存。
func (um *updateMatrix[T]) Update() {
	for blockIdx, block := range um.cache {
		for pos := 0; pos < um.blockSize; pos++ {
			linearIndex := blockIdx*um.blockSize + pos
			row := linearIndex / um.Cols()
			col := linearIndex % um.Cols()
			if row < um.Rows() && col < um.Cols() && um.isBitSet(row, col) {
				um.Matrix.Set(row, col, block[pos])
				um.clearBit(row, col)
			}
		}
	}
	clear(um.cache)
}

// Rollback 丢弃缓存中的所有修改，恢复到上一次 `Update` 之后的状态。
// 这个操作非常快速，因为它只清理缓存和位图，不涉及任何对底层矩阵的读写。
func (um *updateMatrix[T]) Rollback() {
	// 清空位图（所有标记置0）
	totalElements := um.Rows() * um.Cols()
	for i := 0; i < totalElements; i++ {
		um.bitmap.Set(utils.BitmapFlag(i), false)
	}
	// 清空缓存
	clear(um.cache)
}

// BuildFromDense 从一个二维切片重新构建矩阵。
// 此操作会完全覆盖底层矩阵的数据，并清空所有待处理的缓存更改。
func (um *updateMatrix[T]) BuildFromDense(dense [][]T) {
	um.Matrix.BuildFromDense(dense)
	um.Rollback()
}

// Zero 将整个矩阵（包括底层和缓存）清零。
func (um *updateMatrix[T]) Zero() {
	um.Matrix.Zero()
	um.Rollback()
}

// Copy 将当前矩阵的状态复制到另一个矩阵 `a`。
// - 如果目标 `a` 也是一个 `updateMatrix`，则执行一次完整的状态复制，包括底层矩阵、缓存和位图。
// - 否则，它将当前矩阵的“可见”状态（合并了底层和缓存的数据）复制到目标矩阵。
func (um *updateMatrix[T]) Copy(a Matrix[T]) {
	switch target := a.(type) {
	case *updateMatrix[T]:
		if target.Rows() != um.Rows() || target.Cols() != um.Cols() {
			panic(fmt.Sprintf("dimension mismatch: source %dx%d, target %dx%d", um.Rows(), um.Cols(), target.Rows(), target.Cols()))
		}
		um.Matrix.Copy(target.Matrix)
		target.cache = make(map[int][16]T)
		for k, v := range um.cache {
			target.cache[k] = v
		}
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
				var zero T
				if val != zero {
					target.Set(i, j, val)
				}
			}
		}
	}
}

// GetRow 获取指定行的非零元素（合并缓存+底层数据），利用缓冲区避免重复内存分配
func (um *updateMatrix[T]) GetRow(row int) ([]int, Vector[T]) {
	if row < 0 || row >= um.Rows() {
		panic(fmt.Sprintf("row index out of range: %d (rows: %d)", row, um.Rows()))
	}
	// 1. 清空并重用缓冲区
	um.rowResultCols = um.rowResultCols[:0]
	um.rowResultVals = um.rowResultVals[:0]

	// 2. 遍历该行的所有列，合并缓存和底层数据
	for j := 0; j < um.Cols(); j++ {
		var val T
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
	um.rowResultVec.dataManager.data = um.rowResultVals
	return um.rowResultCols, um.rowResultVec
}

// MatrixVectorMultiply 矩阵向量乘法（使用当前可见数据：缓存+底层）
func (um *updateMatrix[T]) MatrixVectorMultiply(x Vector[T]) Vector[T] {
	if x.Length() != um.Cols() {
		panic(fmt.Sprintf("vector dimension mismatch: x length=%d, matrix cols=%d", x.Length(), um.Cols()))
	}
	result := NewDenseVector[T](um.Rows())
	for i := 0; i < um.Rows(); i++ {
		cols, vals := um.GetRow(i) // 获取该行所有可见元素
		for jIdx, col := range cols {
			result.Increment(i, vals.Get(jIdx)*x.Get(col))
		}
	}
	return result
}

// NonZeroCount 统计非零元素数量（缓存+底层）
func (um *updateMatrix[T]) NonZeroCount() int {
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
				isBaseZero := Abs(baseVal) < epsilon
				isCachedZero := Abs(cachedVal) < epsilon
				if isBaseZero && !isCachedZero {
					count++
				} else if !isBaseZero && isCachedZero {
					count--
				}
			}
		}
	}
	return max(count, 0) // 避免负计数
}

// String 格式化输出矩阵（显示当前可见数据）
func (um *updateMatrix[T]) String() string {
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
func (um *updateMatrix[T]) Resize(rows, cols int) {
	if rows < 0 || cols < 0 {
		panic("invalid matrix dimensions: cannot be negative")
	}
	// 重置底层数据大小
	clear(um.cache)
	um.Matrix.Resize(rows, cols)
	um.bitmap = utils.NewBitmap(rows * cols)
}
