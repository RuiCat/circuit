package mat

import (
	"fmt"
	"sort"
)

// sparseMatrix 稀疏矩阵数据结构
type sparseMatrix struct {
	rows, cols int
	// 使用CSR (Compressed Sparse Row) 格式存储
	rowPtr []int     // 行指针数组
	colInd []int     // 列索引数组
	values []float64 // 非零元素值数组
}

// NewsparseMatrix 创建新的优化稀疏矩阵
func NewSparseMatrix(rows, cols int) Matrix {
	return &sparseMatrix{
		rows:   rows,
		cols:   cols,
		rowPtr: make([]int, rows+1), // 多一个元素用于存储结束位置
		colInd: make([]int, 0),
		values: make([]float64, 0),
	}
}

// Set 设置矩阵元素
func (m *sparseMatrix) Set(row, col int, value float64) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic("index out of range")
	}
	// 查找插入位置
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	// 二分查找列索引
	pos := sort.Search(end-start, func(i int) bool {
		return m.colInd[start+i] >= col
	}) + start
	if pos < end && m.colInd[pos] == col {
		// 元素已存在
		if value == 0 {
			// 删除元素
			m.deleteElement(row, pos)
		} else {
			// 更新元素
			m.values[pos] = value
		}
	} else if value != 0 {
		// 插入新元素
		m.insertElement(row, col, value, pos)
	}
}

// Increment 设置矩阵元素
func (m *sparseMatrix) Increment(row, col int, value float64) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic("index out of range")
	}
	// 查找插入位置
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	// 二分查找列索引
	pos := sort.Search(end-start, func(i int) bool {
		return m.colInd[start+i] >= col
	}) + start

	if pos < end && m.colInd[pos] == col {
		// 元素已存在
		if value == 0 {
			// 删除元素
			m.deleteElement(row, pos)
		} else {
			// 更新元素
			m.values[pos] += value
		}
	} else if value != 0 {
		// 插入新元素
		m.insertElement(row, col, value, pos)
	}
}

// Get 获取矩阵元素
func (m *sparseMatrix) Get(row, col int) float64 {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic("index out of range")
	}
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	// 二分查找
	pos := sort.Search(end-start, func(i int) bool {
		return m.colInd[start+i] >= col
	}) + start
	if pos < end && m.colInd[pos] == col {
		return m.values[pos]
	}
	return 0
}

// deleteElement 删除指定位置的元素
func (m *sparseMatrix) deleteElement(row, pos int) {
	// 删除元素
	m.colInd = append(m.colInd[:pos], m.colInd[pos+1:]...)
	m.values = append(m.values[:pos], m.values[pos+1:]...)
	// 更新后续行的指针
	for i := row + 1; i <= m.rows; i++ {
		m.rowPtr[i]--
	}
}

// insertElement 在指定位置插入元素
func (m *sparseMatrix) insertElement(row, col int, value float64, pos int) {
	// 扩展数组
	m.colInd = append(m.colInd, 0)
	m.values = append(m.values, 0)
	// 移动元素
	copy(m.colInd[pos+1:], m.colInd[pos:])
	copy(m.values[pos+1:], m.values[pos:])
	// 插入新元素
	m.colInd[pos] = col
	m.values[pos] = value
	// 更新后续行的指针
	for i := row + 1; i <= m.rows; i++ {
		m.rowPtr[i]++
	}
}

// Rows 返回行数
func (m *sparseMatrix) Rows() int {
	return m.rows
}

// Cols 返回列数
func (m *sparseMatrix) Cols() int {
	return m.cols
}

// String 字符串表示
func (m *sparseMatrix) String() string {
	result := ""
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result += fmt.Sprintf("%8.4f ", m.Get(i, j))
		}
		result += "\n"
	}
	return result
}

// NonZeroCount 返回非零元素数量
func (m *sparseMatrix) NonZeroCount() int {
	return len(m.values)
}

// Copy 复制矩阵
func (m *sparseMatrix) Copy(a Matrix) {
	switch sm := a.(type) {
	case *sparseMatrix:
		sm.rows, sm.cols = m.rows, m.cols
		if cap(sm.rowPtr) < len(m.rowPtr) {
			sm.rowPtr = make([]int, len(m.rowPtr))
		} else {
			sm.rowPtr = sm.rowPtr[:len(m.rowPtr)]
		}
		copy(sm.rowPtr, m.rowPtr)
		if cap(sm.colInd) < len(m.colInd) {
			sm.colInd = make([]int, len(m.colInd))
		} else {
			sm.colInd = sm.colInd[:len(m.colInd)]
		}
		copy(sm.colInd, m.colInd)
		if cap(sm.values) < len(m.values) {
			sm.values = make([]float64, len(m.values))
		} else {
			sm.values = sm.values[:len(m.values)]
		}
		copy(sm.values, m.values)
	default:
		// 对于其他类型的矩阵实现，逐个元素复制
		for i := 0; i < m.rows; i++ {
			for j := 0; j < m.cols; j++ {
				value := m.Get(i, j)
				if value != 0 {
					sm.Set(i, j, value)
				}
			}
		}
	}
}

// IsSquare 检查是否为方阵
func (m *sparseMatrix) IsSquare() bool {
	return m.rows == m.cols
}

// BuildFromDense 从稠密矩阵构建稀疏矩阵
func (m *sparseMatrix) BuildFromDense(dense [][]float64) {
	if len(dense) != m.rows || (len(dense) > 0 && len(dense[0]) != m.cols) {
		panic("dimension mismatch")
	}
	// 完全重置所有数组
	m.colInd = m.colInd[:0]
	m.values = m.values[:0]
	// 优化内存分配：只在必要时重新分配
	if cap(m.rowPtr) < m.rows+1 {
		m.rowPtr = make([]int, m.rows+1)
	} else {
		m.rowPtr = m.rowPtr[:m.rows+1]
	}
	// 构建CSR格式
	count := 0
	for i := 0; i < m.rows; i++ {
		m.rowPtr[i] = count
		for j := 0; j < m.cols; j++ {
			if dense[i][j] != 0 {
				m.colInd = append(m.colInd, j)
				m.values = append(m.values, dense[i][j])
				count++
			}
		}
	}
	m.rowPtr[m.rows] = count
}

// GetRow 获取指定行的非零元素
func (m *sparseMatrix) GetRow(row int) ([]int, []float64) {
	if row < 0 || row >= m.rows {
		panic("row index out of range")
	}
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	return m.colInd[start:end], m.values[start:end]
}

// MatrixVectorMultiply 矩阵向量乘法
func (m *sparseMatrix) MatrixVectorMultiply(x []float64) []float64 {
	if len(x) != m.cols {
		panic("vector dimension mismatch")
	}
	result := make([]float64, m.rows)
	for i := 0; i < m.rows; i++ {
		start := m.rowPtr[i]
		end := m.rowPtr[i+1]
		for j := start; j < end; j++ {
			result[i] += m.values[j] * x[m.colInd[j]]
		}
	}
	return result
}

// Clear 将矩阵重置为零矩阵
func (m *sparseMatrix) Clear() {
	// 清空所有非零元素
	m.colInd = m.colInd[:0]
	m.values = m.values[:0]
	// 重置行指针数组
	for i := 0; i <= m.rows; i++ {
		m.rowPtr[i] = 0
	}
}
