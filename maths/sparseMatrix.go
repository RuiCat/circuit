package maths

import (
	"fmt"
	"sort"
)

// sparseMatrix 稀疏矩阵数据结构
// 使用CSR (Compressed Sparse Row) 格式存储，基于DataManager管理底层数据
type sparseMatrix struct {
	rows, cols int
	rowPtr     []int        // 行指针数组
	colInd     []int        // 列索引数组
	values     *DataManager // 非零元素值数据管理器
}

// NewSparseMatrix 创建新的稀疏矩阵
func NewSparseMatrix(rows, cols int) Matrix {
	return &sparseMatrix{
		rows:   rows,
		cols:   cols,
		rowPtr: make([]int, rows+1), // 多一个元素用于存储结束位置
		colInd: make([]int, 0),
		values: NewDataManager(0), // 初始长度为0
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
			m.values.Set(pos, value)
		}
	} else if value != 0 {
		// 插入新元素
		m.insertElement(row, col, value, pos)
	}
}

// Increment 增量设置矩阵元素
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
			current := m.values.Get(pos)
			m.values.Set(pos, current+value)
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
		return m.values.Get(pos)
	}
	return 0
}

// deleteElement 删除指定位置的元素
func (m *sparseMatrix) deleteElement(row, pos int) {
	// 删除列索引
	m.colInd = append(m.colInd[:pos], m.colInd[pos+1:]...)
	// 使用原地删除函数删除值
	m.values.RemoveInPlace(pos, 1)
	// 更新后续行的指针
	for i := row + 1; i <= m.rows; i++ {
		m.rowPtr[i]--
	}
}

// insertElement 在指定位置插入元素
func (m *sparseMatrix) insertElement(row, col int, value float64, pos int) {
	// 扩展列索引数组
	m.colInd = append(m.colInd, 0)
	copy(m.colInd[pos+1:], m.colInd[pos:])
	m.colInd[pos] = col
	// 使用原地插入函数插入值
	m.values.InsertInPlace(pos, value)
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
	return m.values.Length()
}

// Copy 复制矩阵
func (m *sparseMatrix) Copy(a Matrix) {
	switch target := a.(type) {
	case *sparseMatrix:
		target.rows, target.cols = m.rows, m.cols
		if cap(target.rowPtr) < len(m.rowPtr) {
			target.rowPtr = make([]int, len(m.rowPtr))
		} else {
			target.rowPtr = target.rowPtr[:len(m.rowPtr)]
		}
		copy(target.rowPtr, m.rowPtr)
		if cap(target.colInd) < len(m.colInd) {
			target.colInd = make([]int, len(m.colInd))
		} else {
			target.colInd = target.colInd[:len(m.colInd)]
		}
		copy(target.colInd, m.colInd)
		// 复制DataManager
		target.values = NewDataManager(m.values.Length())
		m.values.Copy(target.values)
	default:
		// 对于其他类型的矩阵实现，逐个元素复制
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
	m.values.ZeroInPlace()
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
				// 使用原地追加函数添加值
				m.values.AppendInPlace(dense[i][j])
				count++
			}
		}
	}
	m.rowPtr[m.rows] = count
}

// GetRow 获取指定行的非零元素
func (m *sparseMatrix) GetRow(row int) ([]int, Vector) {
	if row < 0 || row >= m.rows {
		panic("row index out of range")
	}
	start := m.rowPtr[row]
	end := m.rowPtr[row+1]
	// 创建稠密向量来存储该行的值
	rowValues := make([]float64, m.cols)
	for i := start; i < end; i++ {
		col := m.colInd[i]
		rowValues[col] = m.values.Get(i)
	}
	return m.colInd[start:end], NewDenseVectorWithData(rowValues)
}

// MatrixVectorMultiply 矩阵向量乘法
func (m *sparseMatrix) MatrixVectorMultiply(x Vector) Vector {
	if x.Length() != m.cols {
		panic("vector dimension mismatch")
	}
	result := NewDenseVector(m.rows)
	for i := 0; i < m.rows; i++ {
		start := m.rowPtr[i]
		end := m.rowPtr[i+1]
		for j := start; j < end; j++ {
			result.Increment(i, m.values.Get(j)*x.Get(m.colInd[j]))
		}
	}
	return result
}

// Clear 将矩阵重置为零矩阵
func (m *sparseMatrix) Clear() {
	// 清空所有非零元素
	m.colInd = m.colInd[:0]
	m.values.ZeroInPlace()
	clear(m.rowPtr)
}

// ToDense 转换为稠密向量
func (m *sparseMatrix) ToDense() Vector {
	// 返回稠密格式的矩阵数据
	dense := make([]float64, m.Rows()*m.Cols())
	for i := 0; i < m.Rows(); i++ {
		for j := 0; j < m.Cols(); j++ {
			dense[i*m.Cols()+j] = m.Get(i, j)
		}
	}
	return NewDenseVectorWithData(dense)
}
