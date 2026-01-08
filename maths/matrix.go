package maths

import (
	"fmt"
	"sort"
)

// MatrixDataManager 为稠密矩阵提供底层数据管理。
// 它嵌入了 DataManager 来处理一维数据存储，并增加了行和列的概念。
type MatrixDataManager[T Number] struct {
	DataManager[T]
	rows, cols int // rows 存储矩阵的行数，cols 存储矩阵的列数。
}

// NewMatrixDataManager 创建并初始化一个新的 MatrixDataManager。
// 参数 rows 和 cols 指定了矩阵的维度。
func NewMatrixDataManager[T Number](rows, cols int) *MatrixDataManager[T] {
	return &MatrixDataManager[T]{
		DataManager: NewDataManager[T](rows * cols), // 初始化一维数据切片
		rows:        rows,
		cols:        cols,
	}
}

// Rows 返回矩阵的行数。
func (mdm *MatrixDataManager[T]) Rows() int {
	return mdm.rows
}

// Cols 返回矩阵的列数。
func (mdm *MatrixDataManager[T]) Cols() int {
	return mdm.cols
}

// IsSquare 检查矩阵是否为方阵（行数等于列数）。
func (mdm *MatrixDataManager[T]) IsSquare() bool {
	return mdm.rows == mdm.cols
}

// GetMatrix 获取指定行列位置的元素值。
// 它将二维索引（row, col）转换为一维索引。
func (mdm *MatrixDataManager[T]) GetMatrix(row, col int) T {
	return mdm.Get(row*mdm.cols + col)
}

// SetMatrix 设置指定行列位置的元素值。
// 它将二维索引（row, col）转换为一维索引。
func (mdm *MatrixDataManager[T]) SetMatrix(row, col int, value T) {
	mdm.Set(row*mdm.cols+col, value)
}

// IncrementMatrix 原子性地增加指定行列位置的元素值。
// 它将二维索引（row, col）转换为一维索引。
func (mdm *MatrixDataManager[T]) IncrementMatrix(row, col int, value T) {
	mdm.Increment(row*mdm.cols+col, value)
}

// BuildFromDense 从一个二维切片（稠密表示）构建矩阵数据。
func (mdm *MatrixDataManager[T]) BuildFromDense(dense [][]T) {
	for r, rowData := range dense {
		for c, val := range rowData {
			mdm.SetMatrix(r, c, val)
		}
	}
}

func (mdm *MatrixDataManager[T]) String() string {
	var s string
	for r := 0; r < mdm.rows; r++ {
		for c := 0; c < mdm.cols; c++ {
			s += fmt.Sprintf("%v ", mdm.GetMatrix(r, c))
		}
		s += ""
	}
	return s
}

// ToDense 返回矩阵数据的完整一维切片副本。
func (mdm *MatrixDataManager[T]) ToDense() []T {
	return mdm.DataCopy()
}

// denseMatrix 实现了 Matrix 接口，表示一个稠密矩阵。
// 它嵌入了 MatrixDataManager 来管理数据，并包含一些缓冲区以提高 GetRow 等操作的性能。
type denseMatrix[T Number] struct {
	*MatrixDataManager[T]
	rowColsBuf   []int           // GetRow 操作的列索引缓冲区
	rowValsBuf   []T             // GetRow 操作的非零值缓冲区
	rowResultVec *denseVector[T] // GetRow 操作返回的向量，避免重复分配内存
}

// Base 返回矩阵自身，用于满足某些接口要求。
func (m *denseMatrix[T]) Base() Matrix[T] {
	return m
}

// NewDenseMatrix 创建一个新的稠密矩阵。
func NewDenseMatrix[T Number](rows, cols int) Matrix[T] {
	return &denseMatrix[T]{
		MatrixDataManager: NewMatrixDataManager[T](rows, cols),
		rowColsBuf:        make([]int, 0, cols),                   // 初始化列索引缓冲区，容量为列数
		rowValsBuf:        make([]T, 0, cols),                     // 初始化值缓冲区，容量为列数
		rowResultVec:      NewDenseVector[T](0).(*denseVector[T]), // 初始化结果向量
	}
}

// BuildFromDense 使用二维切片的数据填充矩阵。
func (m *denseMatrix[T]) BuildFromDense(dense [][]T) {
	m.MatrixDataManager.BuildFromDense(dense)
}

// Zero 将矩阵中的所有元素设置为零。
func (m *denseMatrix[T]) Zero() {
	m.DataManager.Zero()
}

// Cols 返回矩阵的列数。
func (m *denseMatrix[T]) Cols() int {
	return m.MatrixDataManager.Cols()
}

// Copy 将当前矩阵的内容复制到目标矩阵 `a`。
// 它针对目标矩阵是稠密矩阵的情况进行了优化，否则会逐元素复制。
func (m *denseMatrix[T]) Copy(a Matrix[T]) {
	switch target := a.(type) {
	case *denseMatrix[T]: // 优化路径：如果目标也是稠密矩阵
		if target.Rows() != m.Rows() || target.Cols() != m.Cols() {
			panic(fmt.Sprintf("dimension mismatch: source %dx%d, target %dx%d", m.Rows(), m.Cols(), target.Rows(), target.Cols()))
		}
		m.DataManager.Copy(target.DataManager) // 直接复制底层数据
		target.rows = m.rows
		target.cols = m.cols
	default: // 通用路径：逐个复制非零元素
		for i := 0; i < m.Rows(); i++ {
			for j := 0; j < m.Cols(); j++ {
				val := m.Get(i, j)
				var zero T
				if val != zero {
					target.Set(i, j, val)
				}
			}
		}
	}
}

// Get 获取指定行列位置的元素值。
func (m *denseMatrix[T]) Get(row int, col int) T {
	return m.MatrixDataManager.GetMatrix(row, col)
}

// GetRow 返回指定行的非零元素的列索引和值。
// 为了提高性能，它重用内部缓冲区来存储结果。
func (m *denseMatrix[T]) GetRow(row int) ([]int, Vector[T]) {
	if row < 0 || row >= m.Rows() {
		panic(fmt.Sprintf("row index out of range: %d (rows: %d)", row, m.Rows()))
	}
	// 清空缓冲区
	m.rowColsBuf = m.rowColsBuf[:0]
	m.rowValsBuf = m.rowValsBuf[:0]

	start := row * m.cols
	end := start + m.cols
	rowData := m.DataPtr()[start:end] // 直接访问该行的数据切片

	// 遍历行数据，收集非零元素
	for col, val := range rowData {
		var zero T
		if val != zero {
			m.rowColsBuf = append(m.rowColsBuf, col)
			m.rowValsBuf = append(m.rowValsBuf, val)
		}
	}

	// 使用缓冲区的数据更新结果向量
	m.rowResultVec.dataManager = &dataManager[T]{
		data: m.rowValsBuf,
	}
	return m.rowColsBuf, m.rowResultVec
}

// Increment 增加指定行列位置的元素值。
func (m *denseMatrix[T]) Increment(row int, col int, value T) {
	m.MatrixDataManager.IncrementMatrix(row, col, value)
}

// IsSquare 检查矩阵是否为方阵。
func (m *denseMatrix[T]) IsSquare() bool {
	return m.MatrixDataManager.IsSquare()
}

func (m *denseMatrix[T]) MatrixVectorMultiply(x Vector[T]) Vector[T] {
	if x.Length() != m.Cols() {
		panic(fmt.Sprintf("vector dimension mismatch: x length=%d, matrix cols=%d", x.Length(), m.Cols()))
	}
	result := NewDenseVector[T](m.Rows())
	for i := 0; i < m.Rows(); i++ {
		var sum T
		for j := 0; j < m.Cols(); j++ {
			sum += m.Get(i, j) * x.Get(j)
		}
		result.Set(i, sum)
	}
	return result
}

// NonZeroCount 返回矩阵中非零元素的数量。
func (m *denseMatrix[T]) NonZeroCount() int {
	return m.DataManager.NonZeroCount()
}

// Rows 返回矩阵的行数。
func (m *denseMatrix[T]) Rows() int {
	return m.MatrixDataManager.Rows()
}

// Set 设置指定行列位置的元素值。
func (m *denseMatrix[T]) Set(row int, col int, value T) {
	m.MatrixDataManager.SetMatrix(row, col, value)
}

func (m *denseMatrix[T]) String() string {
	return m.MatrixDataManager.String()
}

func (m *denseMatrix[T]) ToDense() Vector[T] {
	return NewDenseVectorWithData(m.MatrixDataManager.ToDense())
}

// Resize 改变矩阵的维度。
// 这会重新分配底层数据存储，原数据可能会丢失。
func (m *denseMatrix[T]) Resize(rows, cols int) {
	if rows < 0 || cols < 0 {
		panic("invalid matrix dimensions: cannot be negative")
	}
	m.rows = rows
	m.cols = cols
	m.DataManager.Resize(rows * cols)
}

// SwapRows 交换矩阵的两行。
// 这是一个高效的操作，直接在底层数据切片上进行。
func (m *denseMatrix[T]) SwapRows(row1, row2 int) {
	if row1 < 0 || row1 >= m.rows || row2 < 0 || row2 >= m.rows {
		panic(fmt.Sprintf("row index out of range: row1=%d, row2=%d, rows=%d", row1, row2, m.rows))
	}
	if row1 == row2 {
		return
	}

	start1 := row1 * m.cols
	row1Data := m.DataPtr()[start1 : start1+m.cols] // 获取第一行的数据切片

	start2 := row2 * m.cols
	row2Data := m.DataPtr()[start2 : start2+m.cols] // 获取第二行的数据切片

	// 逐元素交换
	for i := 0; i < m.cols; i++ {
		row1Data[i], row2Data[i] = row2Data[i], row1Data[i]
	}
}

// sparseMatrix 实现了 Matrix 接口，使用压缩稀疏行（CSR, Compressed Sparse Row）格式。
// CSR 格式通过三个数组来存储稀疏矩阵：
//  1. DataManager: 存储所有非零元素的值。
//  2. colInd: 存储每个非零元素对应的列索引。
//  3. rowPtr: 长度为 rows+1，rowPtr[i] 表示第 i 行的第一个非零元素在 DataManager 和 colInd 中的起始索引。
//     第 i 行的非零元素范围是 [rowPtr[i], rowPtr[i+1])。
type sparseMatrix[T Number] struct {
	DataManager DataManager[T] // 存储非零元素的值
	rows, cols  int            // 矩阵的维度
	rowPtr      []int          // 行指针数组
	colInd      []int          // 列索引数组
}

// Base 返回矩阵自身。
func (m *sparseMatrix[T]) Base() Matrix[T] {
	return m
}

// NewSparseMatrix 创建一个新的、全零的稀疏矩阵。
func NewSparseMatrix[T Number](rows, cols int) Matrix[T] {
	if rows < 0 || cols < 0 {
		panic("invalid matrix dimensions: cannot be negative")
	}
	return &sparseMatrix[T]{
		rows:        rows,
		cols:        cols,
		rowPtr:      make([]int, rows+1),  // 初始化行指针，所有行都为空
		colInd:      make([]int, 0),       // 列索引为空
		DataManager: NewDataManager[T](0), // 值存储为空
	}
}

// Set 设置指定行列位置的元素值。
// 该方法会高效地处理元素的插入、更新和删除。
func (m *sparseMatrix[T]) Set(row, col int, value T) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, m.rows, m.cols))
	}
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	// 二分查找确定元素应该存在或插入的位置
	pos := sort.Search(end-start, func(i int) bool {
		return m.colInd[start+i] >= col
	}) + start

	var zero T
	isZero := value == zero

	if pos < end && m.colInd[pos] == col { // 元素已存在
		if !isZero { // 更新值
			m.DataManager.Set(pos, value)
		} else { // 删除元素（设置为零）
			m.deleteElement(row, pos)
		}
	} else if !isZero { // 元素不存在且新值非零，插入新元素
		m.insertElement(row, col, value, pos)
	}
}

// Increment 增加指定行列位置的元素值。
// 如果元素不存在，则会创建它。如果增加后的值为零，则会删除该元素。
func (m *sparseMatrix[T]) Increment(row, col int, value T) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, m.rows, m.cols))
	}
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	// 二分查找确定元素位置
	pos := sort.Search(end-start, func(i int) bool {
		return m.colInd[start+i] >= col
	}) + start

	if pos < end && m.colInd[pos] == col { // 元素已存在
		current := m.DataManager.Get(pos)
		newVal := current + value
		var zero T
		if newVal != zero { // 更新值
			m.DataManager.Set(pos, newVal)
		} else { // 删除元素
			m.deleteElement(row, pos)
		}
	} else { // 元素不存在
		var zero T
		if value != zero { // 插入新元素
			m.insertElement(row, col, value, pos)
		}
	}
}

// Get 获取指定行列位置的元素值。
// 如果元素不存在（即为零），则返回零值。
func (m *sparseMatrix[T]) Get(row, col int) T {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, m.rows, m.cols))
	}
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	// 二分查找元素
	pos := sort.Search(end-start, func(i int) bool {
		return m.colInd[start+i] >= col
	}) + start
	if pos < end && m.colInd[pos] == col { // 找到元素
		return m.DataManager.Get(pos)
	}
	var zero T // 未找到，返回零
	return zero
}

// deleteElement 从 CSR 存储中删除一个元素。
// 这是一个内部辅助函数，会更新 colInd、DataManager 和 rowPtr。
func (m *sparseMatrix[T]) deleteElement(row, pos int) {
	// 从 colInd 和 DataManager 中移除元素
	m.colInd = append(m.colInd[:pos], m.colInd[pos+1:]...)
	m.DataManager.RemoveInPlace(pos, 1)
	// 更新受影响的行指针
	for i := row + 1; i <= m.rows; i++ {
		m.rowPtr[i]--
	}
}

// insertElement 向 CSR 存储中插入一个新元素。
// 这是一个内部辅助函数，会更新 colInd、DataManager 和 rowPtr。
func (m *sparseMatrix[T]) insertElement(row, col int, value T, pos int) {
	// 为新元素腾出空间
	m.colInd = append(m.colInd, 0)
	copy(m.colInd[pos+1:], m.colInd[pos:])
	m.colInd[pos] = col                     // 插入列索引
	m.DataManager.InsertInPlace(pos, value) // 插入值
	// 更新受影响的行指针
	for i := row + 1; i <= m.rows; i++ {
		m.rowPtr[i]++
	}
}

// Rows 返回矩阵的行数。
func (m *sparseMatrix[T]) Rows() int {
	return m.rows
}

// Cols 返回矩阵的列数。
func (m *sparseMatrix[T]) Cols() int {
	return m.cols
}

func (m *sparseMatrix[T]) String() string {
	result := ""
	for i := 0; i < m.rows; i++ {
		colPtr := m.rowPtr[i]
		for j := 0; j < m.cols; j++ {
			if colPtr < m.rowPtr[i+1] && m.colInd[colPtr] == j {
				result += fmt.Sprintf("%v ", m.DataManager.Get(colPtr))
				colPtr++
			} else {
				var zero T
				result += fmt.Sprintf("%v ", zero)
			}
		}
		result += ""
	}
	return result
}

// NonZeroCount 返回矩阵中非零元素的数量。
func (m *sparseMatrix[T]) NonZeroCount() int {
	return m.DataManager.Length()
}

// Copy 将当前稀疏矩阵的内容复制到目标矩阵 `a`。
// 它为稀疏矩阵之间的复制提供了优化路径。
func (m *sparseMatrix[T]) Copy(a Matrix[T]) {
	switch target := a.(type) {
	case *sparseMatrix[T]: // 优化路径：如果目标也是稀疏矩阵
		if target.rows != m.rows || target.cols != m.cols {
			panic(fmt.Sprintf("dimension mismatch: source %dx%d, target %dx%d", m.rows, m.cols, target.rows, target.cols))
		}
		copy(target.rowPtr, m.rowPtr)
		target.colInd = make([]int, len(m.colInd))
		copy(target.colInd, m.colInd)
		target.DataManager = NewDataManager[T](m.DataManager.Length())
		m.DataManager.Copy(target.DataManager)
	default: // 通用路径：迭代非零元素并设置到目标矩阵
		for i := 0; i < m.rows; i++ {
			start := m.rowPtr[i]
			end := m.rowPtr[i+1]
			for j := start; j < end; j++ {
				col := m.colInd[j]
				val := m.DataManager.Get(j)
				target.Set(i, col, val)
			}
		}
	}
}

// IsSquare 检查矩阵是否为方阵。
func (m *sparseMatrix[T]) IsSquare() bool {
	return m.rows == m.cols
}

// BuildFromDense 从二维切片（稠密表示）构建稀疏矩阵。
// 这会完全重置矩阵的现有数据。
func (m *sparseMatrix[T]) BuildFromDense(dense [][]T) {
	if len(dense) != m.rows || (len(dense) > 0 && len(dense[0]) != m.cols) {
		panic(fmt.Sprintf("dense matrix dimension mismatch: expected %dx%d, got %dx%d", m.rows, m.cols, len(dense), len(dense[0])))
	}
	// 重置内部状态
	m.colInd = m.colInd[:0]
	m.DataManager.Zero()
	clear(m.rowPtr)

	count := 0
	for i := 0; i < m.rows; i++ {
		m.rowPtr[i] = count // 记录当前行的起始位置
		for j := 0; j < m.cols; j++ {
			val := dense[i][j]
			var zero T
			if val != zero { // 只存储非零元素
				m.colInd = append(m.colInd, j)
				m.DataManager.AppendInPlace(val)
				count++
			}
		}
	}
	m.rowPtr[m.rows] = count // 记录总的非零元素数量
}

// GetRow 返回指定行的列索引和值的向量。
// 返回的向量是一个新的稠密向量实例。
func (m *sparseMatrix[T]) GetRow(row int) ([]int, Vector[T]) {
	if row < 0 || row >= m.rows {
		panic(fmt.Sprintf("row index out of range: %d (rows: %d)", row, m.rows))
	}
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	cols := m.colInd[start:end]
	values := make([]T, len(cols))
	for i := range cols {
		values[i] = m.DataManager.Get(start + i)
	}
	return cols, NewDenseVectorWithData(values)
}

func (m *sparseMatrix[T]) MatrixVectorMultiply(x Vector[T]) Vector[T] {
	if x.Length() != m.cols {
		panic(fmt.Sprintf("vector dimension mismatch: x length=%d, matrix cols=%d", x.Length(), m.cols))
	}
	result := NewDenseVector[T](m.rows)
	for i := 0; i < m.rows; i++ {
		start := m.rowPtr[i]
		end := m.rowPtr[i+1]
		for j := start; j < end; j++ {
			col := m.colInd[j]
			val := m.DataManager.Get(j)
			result.Increment(i, val*x.Get(col))
		}
	}
	return result
}

// Zero 将矩阵重置为全零状态，并释放存储空间。
func (m *sparseMatrix[T]) Zero() {
	m.colInd = m.colInd[:0]
	m.DataManager.Zero()
	m.DataManager.ResizeInPlace(0)
	clear(m.rowPtr) // 重置行指针为全零
}

func (m *sparseMatrix[T]) ToDense() Vector[T] {
	dense := make([]T, m.rows*m.cols)
	for i := 0; i < m.rows; i++ {
		start := m.rowPtr[i]
		end := m.rowPtr[i+1]
		for j := start; j < end; j++ {
			col := m.colInd[j]
			idx := i*m.cols + col
			dense[idx] = m.DataManager.Get(j)
		}
	}
	return NewDenseVectorWithData(dense)
}

// Resize 改变矩阵的维度，并将其重置为全零状态。
func (m *sparseMatrix[T]) Resize(rows, cols int) {
	if rows < 0 || cols < 0 {
		panic("invalid matrix dimensions: cannot be negative")
	}
	m.rows = rows
	m.cols = cols
	m.rowPtr = make([]int, rows+1) // 重新分配行指针
	m.colInd = m.colInd[:0]        // 清空列索引
	m.DataManager.Resize(0)        // 清空值
}

// SwapRows 交换稀疏矩阵的两行。
// 这是一个复杂的操作，因为它需要移动 colInd 和 DataManager 中的数据块。
func (m *sparseMatrix[T]) SwapRows(row1, row2 int) {
	if row1 < 0 || row1 >= m.rows || row2 < 0 || row2 >= m.rows {
		panic(fmt.Sprintf("row index out of range: row1=%d, row2=%d, rows=%d", row1, row2, m.rows))
	}
	if row1 == row2 {
		return
	}

	// 保证 row1 < row2
	if row1 > row2 {
		row1, row2 = row2, row1
	}

	start1, end1 := m.rowPtr[row1], m.rowPtr[row1+1]
	start2, end2 := m.rowPtr[row2], m.rowPtr[row2+1]
	len1, len2 := end1-start1, end2-start2

	// 优化：如果两行非零元素数量相同，可以直接交换数据。
	if len1 == len2 {
		for i := 0; i < len1; i++ {
			// 交换列索引
			m.colInd[start1+i], m.colInd[start2+i] = m.colInd[start2+i], m.colInd[start1+i]
			// 交换值
			val1 := m.DataManager.Get(start1 + i)
			val2 := m.DataManager.Get(start2 + i)
			m.DataManager.Set(start1+i, val2)
			m.DataManager.Set(start2+i, val1)
		}
		return
	}

	// 如果长度不同，需要移动数据块。
	// 1. 备份 row1, row2 以及它们之间的数据。
	cols1 := make([]int, len1)
	copy(cols1, m.colInd[start1:end1])
	vals1 := m.DataManager.DataCopy()[start1:end1]

	cols2 := make([]int, len2)
	copy(cols2, m.colInd[start2:end2])
	vals2 := m.DataManager.DataCopy()[start2:end2]

	middleStart := end1
	middleEnd := start2
	lenMiddle := middleEnd - middleStart
	middleCols := make([]int, lenMiddle)
	copy(middleCols, m.colInd[middleStart:middleEnd])
	middleVals := m.DataManager.DataCopy()[middleStart:middleEnd]

	// 2. 将 row2 的数据移动到 row1 的位置。
	copy(m.colInd[start1:], cols2)
	m.DataManager.ReplaceInPlace(start1, vals2...)

	// 3. 将中间数据移动到新位置。
	newMiddleStart := start1 + len2
	copy(m.colInd[newMiddleStart:], middleCols)
	m.DataManager.ReplaceInPlace(newMiddleStart, middleVals...)

	// 4. 将 row1 的数据移动到新位置。
	newRow1Start := newMiddleStart + lenMiddle
	copy(m.colInd[newRow1Start:], cols1)
	m.DataManager.ReplaceInPlace(newRow1Start, vals1...)

	// 5. 更新 rowPtr。
	delta := len2 - len1
	for i := row1 + 1; i <= row2; i++ {
		m.rowPtr[i] += delta
	}
}
