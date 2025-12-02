package maths

import (
	"fmt"
	"sort"
)

// denseMatrix 稠密矩阵实现（基于MatrixDataManager，全量存储所有元素）
type denseMatrix struct {
	*MatrixDataManager // 嵌入矩阵数据管理器复用功能
}

// Base 获取底层
func (m *denseMatrix) Base() Matrix {
	return m
}

// NewDenseMatrix 创建指定维度的空稠密矩阵
func NewDenseMatrix(rows, cols int) Matrix {
	return &denseMatrix{
		MatrixDataManager: NewMatrixDataManager(rows, cols),
	}
}

// BuildFromDense 从稠密矩阵构建（覆盖原有数据）
func (m *denseMatrix) BuildFromDense(dense [][]float64) {
	m.MatrixDataManager.BuildFromDense(dense)
}

// Zero 清空矩阵为零矩阵
func (m *denseMatrix) Zero() {
	m.MatrixDataManager.Zero()
}

// Cols 返回矩阵列数
func (m *denseMatrix) Cols() int {
	return m.MatrixDataManager.Cols()
}

// Copy 复制自身数据到目标矩阵（支持稠密/稀疏等类型）
func (m *denseMatrix) Copy(a Matrix) {
	switch target := a.(type) {
	case *denseMatrix:
		// 同类型直接复制（高效）
		if target.Rows() != m.Rows() || target.Cols() != m.Cols() {
			panic(fmt.Sprintf("dimension mismatch: source %dx%d, target %dx%d", m.Rows(), m.Cols(), target.Rows(), target.Cols()))
		}
		m.MatrixDataManager.DataManager.Copy(target.MatrixDataManager.DataManager)
		target.MatrixDataManager.rows = m.MatrixDataManager.rows
		target.MatrixDataManager.cols = m.MatrixDataManager.cols
	default:
		// 异类型逐个元素复制（兼容稀疏矩阵）
		for i := 0; i < m.Rows(); i++ {
			for j := 0; j < m.Cols(); j++ {
				val := m.Get(i, j)
				if val != 0 { // 非零元素才复制（优化）
					target.Set(i, j, val)
				}
			}
		}
	}
}

// Get 获取指定行列元素值（越界panic）
func (m *denseMatrix) Get(row int, col int) float64 {
	return m.MatrixDataManager.GetMatrix(row, col)
}

// GetRow 获取指定行的非零元素（返回：列索引切片+值向量）
func (m *denseMatrix) GetRow(row int) ([]int, Vector) {
	cols, values := m.MatrixDataManager.GetRow(row)
	return cols, NewDenseVectorWithData(values)
}

// Increment 增量更新矩阵元素（value累加，越界panic）
func (m *denseMatrix) Increment(row int, col int, value float64) {
	m.MatrixDataManager.IncrementMatrix(row, col, value)
}

// IsSquare 判断是否为方阵
func (m *denseMatrix) IsSquare() bool {
	return m.MatrixDataManager.IsSquare()
}

// MatrixVectorMultiply 矩阵向量乘法（A*x，返回新向量）
func (m *denseMatrix) MatrixVectorMultiply(x Vector) Vector {
	if x.Length() != m.Cols() {
		panic(fmt.Sprintf("vector dimension mismatch: x length=%d, matrix cols=%d", x.Length(), m.Cols()))
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

// NonZeroCount 统计非零元素数量
func (m *denseMatrix) NonZeroCount() int {
	return m.MatrixDataManager.NonZeroCount()
}

// Rows 返回矩阵行数
func (m *denseMatrix) Rows() int {
	return m.MatrixDataManager.Rows()
}

// Set 设置指定行列元素值（越界panic）
func (m *denseMatrix) Set(row int, col int, value float64) {
	m.MatrixDataManager.SetMatrix(row, col, value)
}

// String 格式化输出矩阵
func (m *denseMatrix) String() string {
	return m.MatrixDataManager.String()
}

// ToDense 转换为稠密向量（行优先展开）
func (m *denseMatrix) ToDense() Vector {
	return NewDenseVectorWithData(m.MatrixDataManager.ToDense())
}

// Resize 重置矩阵大小和数据（清空所有元素）
func (m *denseMatrix) Resize(rows, cols int) {
	if rows < 0 || cols < 0 {
		panic("invalid matrix dimensions: cannot be negative")
	}
	// 重置底层数据大小
	m.MatrixDataManager.rows = rows
	m.MatrixDataManager.cols = cols
	m.MatrixDataManager.Resize(rows * cols)
}

// sparseMatrix 稀疏矩阵实现（CSR格式：Compressed Sparse Row）
// 核心优化：仅存储非零元素，大幅节省内存（适合非零元素占比<10%的矩阵）
type sparseMatrix struct {
	DataManager       // 非零元素值：与colInd一一对应
	rows, cols  int   // 矩阵维度
	rowPtr      []int // 行指针：rowPtr[i] = 第i行非零元素在colInd/values中的起始索引
	colInd      []int // 列索引：存储非零元素的列号
}

// Base 获取底层
func (m *sparseMatrix) Base() Matrix {
	return m
}

// NewSparseMatrix 创建指定维度的空稀疏矩阵
func NewSparseMatrix(rows, cols int) Matrix {
	if rows < 0 || cols < 0 {
		panic("invalid matrix dimensions: cannot be negative")
	}
	return &sparseMatrix{
		rows:        rows,
		cols:        cols,
		rowPtr:      make([]int, rows+1), // rowPtr[rows] = 非零元素总数
		colInd:      make([]int, 0),
		DataManager: NewDataManager(0),
	}
}

// Set 设置矩阵元素值（非零则插入/更新，零则删除）
func (m *sparseMatrix) Set(row, col int, value float64) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, m.rows, m.cols))
	}
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	// 二分查找列索引在当前行的位置
	pos := sort.Search(end-start, func(i int) bool {
		return m.colInd[start+i] >= col
	}) + start

	if pos < end && m.colInd[pos] == col {
		// 元素已存在：更新或删除
		if value < -1e-16 || value > 1e-16 { // 非零：更新
			m.DataManager.Set(pos, value)
		} else { // 零：删除
			m.deleteElement(row, pos)
		}
	} else if value < -1e-16 || value > 1e-16 {
		// 元素不存在且非零：插入
		m.insertElement(row, col, value, pos)
	}
}

// Increment 增量更新矩阵元素（非零则累加，零则插入）
func (m *sparseMatrix) Increment(row, col int, value float64) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, m.rows, m.cols))
	}
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	pos := sort.Search(end-start, func(i int) bool {
		return m.colInd[start+i] >= col
	}) + start

	if pos < end && m.colInd[pos] == col {
		// 元素已存在：累加
		current := m.DataManager.Get(pos)
		newVal := current + value
		if newVal < -1e-16 || newVal > 1e-16 { // 累加后非零：更新
			m.DataManager.Set(pos, newVal)
		} else { // 累加后零：删除
			m.deleteElement(row, pos)
		}
	} else if value < -1e-16 || value > 1e-16 {
		// 元素不存在且增量非零：插入
		m.insertElement(row, col, value, pos)
	}
}

// Get 获取矩阵元素值（非零返回值，零返回0）
func (m *sparseMatrix) Get(row, col int) float64 {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, m.rows, m.cols))
	}
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	pos := sort.Search(end-start, func(i int) bool {
		return m.colInd[start+i] >= col
	}) + start
	if pos < end && m.colInd[pos] == col {
		return m.DataManager.Get(pos)
	}
	return 0.0
}

// deleteElement 删除指定位置的非零元素（内部方法）
func (m *sparseMatrix) deleteElement(row, pos int) {
	// 删除列索引
	m.colInd = append(m.colInd[:pos], m.colInd[pos+1:]...)
	// 删除值
	m.DataManager.RemoveInPlace(pos, 1)
	// 更新后续行的指针（所有行号>row的行指针减1）
	for i := row + 1; i <= m.rows; i++ {
		m.rowPtr[i]--
	}
}

// insertElement 在指定位置插入非零元素（内部方法）
func (m *sparseMatrix) insertElement(row, col int, value float64, pos int) {
	// 插入列索引
	m.colInd = append(m.colInd, 0)
	copy(m.colInd[pos+1:], m.colInd[pos:])
	m.colInd[pos] = col
	// 插入值
	m.DataManager.InsertInPlace(pos, value)
	// 更新后续行的指针（所有行号>row的行指针加1）
	for i := row + 1; i <= m.rows; i++ {
		m.rowPtr[i]++
	}
}

// Rows 返回矩阵行数
func (m *sparseMatrix) Rows() int {
	return m.rows
}

// Cols 返回矩阵列数
func (m *sparseMatrix) Cols() int {
	return m.cols
}

// String 格式化输出矩阵（显示所有元素，零元素也显示）
func (m *sparseMatrix) String() string {
	result := ""
	for i := 0; i < m.rows; i++ {
		colPtr := m.rowPtr[i]
		for j := 0; j < m.cols; j++ {
			if colPtr < m.rowPtr[i+1] && m.colInd[colPtr] == j {
				result += fmt.Sprintf("%8.4f ", m.DataManager.Get(colPtr))
				colPtr++
			} else {
				result += fmt.Sprintf("%8.4f ", 0.0)
			}
		}
		result += "\n"
	}
	return result
}

// NonZeroCount 统计非零元素数量
func (m *sparseMatrix) NonZeroCount() int {
	return m.DataManager.Length()
}

// Copy 复制自身数据到目标矩阵（支持稀疏/稠密等类型）
func (m *sparseMatrix) Copy(a Matrix) {
	switch target := a.(type) {
	case *sparseMatrix:
		// 同类型复制（高效）
		if target.rows != m.rows || target.cols != m.cols {
			panic(fmt.Sprintf("dimension mismatch: source %dx%d, target %dx%d", m.rows, m.cols, target.rows, target.cols))
		}
		// 复制行指针
		copy(target.rowPtr, m.rowPtr)
		// 复制列索引
		target.colInd = make([]int, len(m.colInd))
		copy(target.colInd, m.colInd)
		// 复制值
		target.DataManager = NewDataManager(m.DataManager.Length())
		m.DataManager.Copy(target.DataManager)
	default:
		// 异类型复制（逐个非零元素复制）
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

// IsSquare 判断是否为方阵
func (m *sparseMatrix) IsSquare() bool {
	return m.rows == m.cols
}

// BuildFromDense 从稠密矩阵构建稀疏矩阵（仅保留非零元素）
func (m *sparseMatrix) BuildFromDense(dense [][]float64) {
	if len(dense) != m.rows || (len(dense) > 0 && len(dense[0]) != m.cols) {
		panic(fmt.Sprintf("dense matrix dimension mismatch: expected %dx%d, got %dx%d", m.rows, m.cols, len(dense), len(dense[0])))
	}
	// 重置所有数据
	m.colInd = m.colInd[:0]
	m.DataManager.Zero()
	clear(m.rowPtr)

	count := 0
	for i := 0; i < m.rows; i++ {
		m.rowPtr[i] = count
		for j := 0; j < m.cols; j++ {
			val := dense[i][j]
			if val < -1e-16 || val > 1e-16 { // 仅保留非零元素
				m.colInd = append(m.colInd, j)
				m.DataManager.AppendInPlace(val)
				count++
			}
		}
	}
	m.rowPtr[m.rows] = count
}

// GetRow 获取指定行的非零元素（返回：列索引切片+值向量）
func (m *sparseMatrix) GetRow(row int) ([]int, Vector) {
	if row < 0 || row >= m.rows {
		panic(fmt.Sprintf("row index out of range: %d (rows: %d)", row, m.rows))
	}
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	// 提取列索引和值
	cols := m.colInd[start:end]
	values := make([]float64, len(cols))
	for i := range cols {
		values[i] = m.DataManager.Get(start + i)
	}
	return cols, NewDenseVectorWithData(values)
}

// MatrixVectorMultiply 矩阵向量乘法（A*x，稀疏优化：仅遍历非零元素）
func (m *sparseMatrix) MatrixVectorMultiply(x Vector) Vector {
	if x.Length() != m.cols {
		panic(fmt.Sprintf("vector dimension mismatch: x length=%d, matrix cols=%d", x.Length(), m.cols))
	}
	result := NewDenseVector(m.rows)
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

// Zero 清空矩阵为零矩阵（释放非零元素内存）
func (m *sparseMatrix) Zero() {
	m.colInd = m.colInd[:0]
	m.DataManager.Zero()
	m.DataManager.ResizeInPlace(0) // 释放值切片内存
	clear(m.rowPtr)
}

// ToDense 转换为稠密向量（行优先展开）
func (m *sparseMatrix) ToDense() Vector {
	dense := make([]float64, m.rows*m.cols)
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

// Resize 重置矩阵大小和数据（清空所有元素）
func (m *sparseMatrix) Resize(rows, cols int) {
	if rows < 0 || cols < 0 {
		panic("invalid matrix dimensions: cannot be negative")
	}
	// 重置底层数据大小
	m.rows = rows
	m.cols = cols
	m.rowPtr = make([]int, rows+1)
	m.colInd = m.colInd[:0]
	m.DataManager.Resize(rows * cols)
}
