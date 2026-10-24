package maths

import (
	"circuit/utils"
	"fmt"
	"sort"
	"time"
)

// updateMatrix 为任矩阵实现提供了一个带缓存的装饰器，以优化频繁、临时的修改操作。
// 它的核心思想是：写操作首先进入一个临时缓存，而不是直接修改底层矩阵。
// 用户可以随后选择“提交”（Update）这些更改，将它们批量写入底层矩阵，
// 或者“回滚”（Rollback），直接丢弃缓存中的所有更改。
// 这种机制在需要进行探索性计算或需要撤销操作的场景下非常高效。
type updateMatrix[T Number] struct {
	Matrix[T]                      // Matrix 是底层的矩阵，存储着“已提交”的稳定数据。
	bitmap    utils.Bitmap         // bitmap 用于标记哪些矩阵元素在缓存中被修改过。每一位对应一个元素，下标 = 行优先线性下标 row*Cols+col。
	cache     map[int]T            // cache 是线性下标 → 缓存值的稀疏缓存（只存被改过的元素）。
	rowSorter rowPairSorter[T]     // rowSorter 是 GetRow 原地排序用的成对视图（复用字段，避免每次排序为接口装箱分配）。
	dirtyRows map[int]map[int]bool // 每行被缓存修改的列集合，加速 GetRow 的稀疏合并。
}

// rowPairSorter 把「列索引 + 值」两个切片当作可排序的成对视图：成对交换，
// 排序不需要旧实现那样的 idx/colsTmp/valsTmp 三份临时切片。
type rowPairSorter[T Number] struct {
	cols []int
	vals []T
}

// Len 返回待排序元素个数。
func (s *rowPairSorter[T]) Len() int { return len(s.cols) }

// Less 按列索引升序比较。
func (s *rowPairSorter[T]) Less(i, j int) bool { return s.cols[i] < s.cols[j] }

// Swap 成对交换列索引与对应值，保持两者对齐。
func (s *rowPairSorter[T]) Swap(i, j int) {
	s.cols[i], s.cols[j] = s.cols[j], s.cols[i]
	s.vals[i], s.vals[j] = s.vals[j], s.vals[i]
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
		Matrix:    dm,
		bitmap:    utils.NewBitmap(rows * cols), // 位图的大小等于矩阵元素的总数。
		cache:     make(map[int]T),
		dirtyRows: make(map[int]map[int]bool),
	}
}

// NewUpdateMatrixPtr 从一个矩阵指针创建一个新的 updateMatrix。
// 与 NewUpdateMatrix 不同，此函数不复制底层数据，而是直接使用传入的矩阵指针。
// 这意味着对 updateMatrix 的 `Update` 操作会直接修改原始矩阵。
func NewUpdateMatrixPtr[T Number](ptr Matrix[T]) UpdateMatrix[T] {
	return &updateMatrix[T]{
		Matrix:    ptr,
		bitmap:    utils.NewBitmap(ptr.Rows() * ptr.Cols()),
		cache:     make(map[int]T),
		dirtyRows: make(map[int]map[int]bool),
	}
}

// linearIndex 把（行，列）转换为行优先线性下标；该下标同时是缓存键与位图位下标
// （位图长度 = Rows×Cols），因此不再需要旧实现里的「块索引/块内偏移」两次整数除法。
func (um *updateMatrix[T]) linearIndex(row, col int) int {
	return row*um.Cols() + col
}

// isBitSet 检查给定（行，列）的元素是否在缓存中被修改过（即其在位图中的对应位是否被设置）。
func (um *updateMatrix[T]) isBitSet(row, col int) bool {
	return um.bitmap.Get(utils.BitmapFlag(um.linearIndex(row, col)))
}

// setBit 在位图中标记给定（行，列）的元素，表示其值已在缓存中更新。
func (um *updateMatrix[T]) setBit(row, col int) {
	um.bitmap.Set(utils.BitmapFlag(um.linearIndex(row, col)), true)
}

// clearBit 清除给定（行，列）元素在位图中的标记，通常在缓存被提交或回滚后调用。
func (um *updateMatrix[T]) clearBit(row, col int) {
	um.bitmap.Set(utils.BitmapFlag(um.linearIndex(row, col)), false)
}

// resetDirtyRows 清空每行的脏列集合内容，但保留（复用）内层 map：
// 提交/回滚只丢弃标记，不必为同一批行反复分配新 map。
func (um *updateMatrix[T]) resetDirtyRows() {
	for row := range um.dirtyRows {
		clear(um.dirtyRows[row])
	}
}

// Get 获取指定位置的元素值。它实现了“读时合并”的逻辑：
// 1. 检查位图，判断该元素是否在缓存中。
// 2. 如果是，则从缓存中读取最新值。
// 3. 如果否，则从底层矩阵中读取稳定值。
func (um *updateMatrix[T]) Get(row, col int) T {
	recordMatrixGet()
	rows, cols := um.Rows(), um.Cols()
	if row < 0 || row >= rows || col < 0 || col >= cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, rows, cols))
	}
	index := row*cols + col
	if um.bitmap.Get(utils.BitmapFlag(index)) {
		if value, exists := um.cache[index]; exists {
			return value
		}
	}
	return um.Matrix.Get(row, col)
}

// Set 设置指定位置的元素值。该操作只写缓存，不直接触及底层矩阵。
// 1. 计算元素的线性下标（缓存键 = 位图位下标）。
// 2. 把新值写入缓存。
// 3. 在位图中标记该元素为“已修改”，并登记到该行的脏列集合。
func (um *updateMatrix[T]) Set(row, col int, value T) {
	recordMatrixSet()
	rows, cols := um.Rows(), um.Cols()
	if row < 0 || row >= rows || col < 0 || col >= cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, rows, cols))
	}
	index := row*cols + col
	um.cache[index] = value
	um.bitmap.Set(utils.BitmapFlag(index), true)
	if um.dirtyRows[row] == nil {
		um.dirtyRows[row] = make(map[int]bool)
	}
	um.dirtyRows[row][col] = true
}

// Increment 增量更新指定位置的元素值。这是一个“读-改-写”操作，但被优化为只写缓存。
// 1. 如果元素已在缓存中，直接在缓存值上进行累加。
// 2. 如果元素不在缓存中，则先从底层矩阵读取原值，与增量相加后，将结果存入缓存，并标记该位置。
func (um *updateMatrix[T]) Increment(row, col int, value T) {
	recordMatrixInc()
	rows, cols := um.Rows(), um.Cols()
	if row < 0 || row >= rows || col < 0 || col >= cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, rows, cols))
	}
	index := row*cols + col
	if cached, exists := um.cache[index]; exists {
		um.cache[index] = cached + value
		return
	}
	um.cache[index] = um.Matrix.Get(row, col) + value
	um.bitmap.Set(utils.BitmapFlag(index), true)
	if um.dirtyRows[row] == nil {
		um.dirtyRows[row] = make(map[int]bool)
	}
	um.dirtyRows[row][col] = true
}

// Update 将缓存中的所有修改“提交”到底层矩阵。
// 这是一个批量操作，可以显著减少对底层矩阵的写操作次数。
// 缓存条目一一对应「被修改过的元素」（写缓存与标记位图始终成对发生），
// 故直接遍历缓存提交，随后整体清位图与缓存。
func (um *updateMatrix[T]) Update() {
	cols := um.Cols()
	if sm, ok := um.Matrix.(*sparseMatrix[T]); ok {
		// 稀疏底层：批量归并重建，避免逐元素 InsertInPlace 搬移（1e5 时累计百 GB）。
		sm.updateBatch(um.cache)
	} else {
		for index, value := range um.cache {
			if !um.bitmap.Get(utils.BitmapFlag(index)) {
				continue
			}
			um.Matrix.Set(index/cols, index%cols, value)
		}
	}
	um.bitmap.ClearAll()
	clear(um.cache)
	um.resetDirtyRows()
}

// Rollback 丢弃缓存中的所有修改，恢复到上一次 `Update` 之后的状态。
// 这个操作非常快速，因为它只清理缓存和位图，不涉及任何对底层矩阵的读写。
func (um *updateMatrix[T]) Rollback() {
	// 整体清零位图（O(字数)），而非逐位 Set(false)（O(元素数)，2250²=506 万次）
	um.bitmap.ClearAll()
	// 清空缓存
	clear(um.cache)
	// 复用内层脏列 map（只清内容），避免每次回滚都为脏行重新分配 map
	um.resetDirtyRows()
}

// BuildFromDense 从一个二维切片重新构建矩阵。
// 此操作会完全覆盖底层矩阵的数据，并清空所有待处理的缓存更改。
func (um *updateMatrix[T]) BuildFromDense(dense [][]T) {
	clear(um.cache)
	um.Matrix.BuildFromDense(dense)
	// 直接重建位图而非 Rollback，防止旧维度位图与新矩阵不匹配
	um.bitmap = utils.NewBitmap(um.Rows() * um.Cols())

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
		// 缓存是「线性下标 → 值」的稀疏 map：逐键复制即可，值类型 T 无指针别名问题
		target.cache = make(map[int]T, len(um.cache))
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

// GetRow 获取指定行的非零元素列索引副本和值副本，合并缓存与底层数据。
// 返回的切片和向量均为独立分配，与内部缓冲区无共享，修改不会影响矩阵状态。
func (um *updateMatrix[T]) GetRow(row int) ([]int, Vector[T]) {
	start := time.Now()
	defer func() { recordGetRow(time.Since(start)) }()
	rows := um.Rows()
	if row < 0 || row >= rows {
		panic(fmt.Sprintf("row index out of range: %d (rows: %d)", row, rows))
	}
	cols := um.Cols()

	// 稀疏合并：只遍历底层行非零 + 缓存脏列，避免 O(Cols) 全列扫描。
	// 电路 MNA 矩阵每行非零 ~10 个（Cols 可达数千），全列扫描会令
	// LU 分解/残差计算退化为 O(n³)（GetRow 占 CPU8 仿真实测 72%）。
	baseCols, baseVals := um.Matrix.GetRow(row)
	dirty := um.dirtyRows[row]
	// 结果直接写进新分配的切片：容量按「底层非零 + 脏列」上界一次分配，
	// 省掉旧实现的持久缓冲 + 二次拷贝（旧路径每次调用 5 次分配，其中 3 次只用于排序）。
	rowStart := row * cols
	resultCols := make([]int, 0, len(baseCols)+len(dirty))
	resultVals := make([]T, 0, len(baseCols)+len(dirty))
	// 底层行（dirty 覆盖同名列）
	for idx, j := range baseCols {
		if dirty != nil && dirty[j] {
			if val, exists := um.cache[rowStart+j]; exists {
				if val != 0 {
					resultCols = append(resultCols, j)
					resultVals = append(resultVals, val)
				}
			}
			continue
		}
		v := baseVals.Get(idx)
		if v != 0 {
			resultCols = append(resultCols, j)
			resultVals = append(resultVals, v)
		}
	}
	// 缓存中不在底层的新列
	for j := range dirty {
		// 已在底层行处理过（baseCols 有序，二分查找判存在）
		if len(baseCols) > 0 {
			pos := sort.SearchInts(baseCols, j)
			if pos < len(baseCols) && baseCols[pos] == j {
				continue
			}
		}
		if val, exists := um.cache[rowStart+j]; exists {
			if val != 0 {
				resultCols = append(resultCols, j)
				resultVals = append(resultVals, val)
			}
		}
	}
	// 按列号升序排序（base 行有序 + 脏列无序）：成对原地排序，零额外分配
	if len(resultCols) > 1 {
		um.rowSorter.cols = resultCols
		um.rowSorter.vals = resultVals
		sort.Sort(&um.rowSorter)
		um.rowSorter.cols = nil
		um.rowSorter.vals = nil
	}

	return resultCols, &denseVector[T]{
		dataManager: &dataManager[T]{
			data: resultVals,
		},
	}
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
	// 统计缓存中新增/修改的非零元素（缓存键即线性下标，条目与已标记元素一一对应）
	cols := um.Cols()
	for linearIndex, cachedVal := range um.cache {
		if !um.bitmap.Get(utils.BitmapFlag(linearIndex)) {
			continue
		}
		baseVal := um.Matrix.Get(linearIndex/cols, linearIndex%cols)
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
