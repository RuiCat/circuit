package mat

import (
	"fmt"
)

// denseMatrix 稠密矩阵数据结构
type denseMatrix struct {
	rows, cols int
	data       [][]float64 // 二维数组存储所有元素
}

// NewDenseMatrix 创建新的稠密矩阵
func NewDenseMatrix(rows, cols int) Matrix {
	// 初始化二维数组
	data := make([][]float64, rows)
	for i := range data {
		data[i] = make([]float64, cols)
	}

	return &denseMatrix{
		rows: rows,
		cols: cols,
		data: data,
	}
}

// Set 设置矩阵元素
func (m *denseMatrix) Set(row, col int, value float64) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic("index out of range")
	}
	m.data[row][col] = value
}

// Increment 增量设置矩阵元素（累加值）
func (m *denseMatrix) Increment(row, col int, value float64) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic("index out of range")
	}
	m.data[row][col] += value
}

// Get 获取矩阵元素
func (m *denseMatrix) Get(row, col int) float64 {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic("index out of range")
	}
	return m.data[row][col]
}

// Rows 返回行数
func (m *denseMatrix) Rows() int {
	return m.rows
}

// Cols 返回列数
func (m *denseMatrix) Cols() int {
	return m.cols
}

// String 字符串表示
func (m *denseMatrix) String() string {
	result := ""
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result += fmt.Sprintf("%8.4f ", m.data[i][j])
		}
		result += "\n"
	}
	return result
}

// NonZeroCount 返回非零元素数量
func (m *denseMatrix) NonZeroCount() int {
	count := 0
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			if m.data[i][j] != 0 {
				count++
			}
		}
	}
	return count
}

// Copy 复制矩阵
func (m *denseMatrix) Copy(a Matrix) {
	switch dm := a.(type) {
	case *denseMatrix:
		// 直接复制二维数组
		dm.rows, dm.cols = m.rows, m.cols
		dm.data = make([][]float64, m.rows)
		for i := range m.data {
			dm.data[i] = make([]float64, m.cols)
			copy(dm.data[i], m.data[i])
		}
	default:
		// 对于其他类型的矩阵实现，逐个元素复制
		for i := 0; i < m.rows; i++ {
			for j := 0; j < m.cols; j++ {
				value := m.data[i][j]
				if value != 0 {
					a.Set(i, j, value)
				}
			}
		}
	}
}

// IsSquare 检查是否为方阵
func (m *denseMatrix) IsSquare() bool {
	return m.rows == m.cols
}

// BuildFromDense 从稠密矩阵构建矩阵
func (m *denseMatrix) BuildFromDense(dense [][]float64) {
	if len(dense) != m.rows || (len(dense) > 0 && len(dense[0]) != m.cols) {
		panic("dimension mismatch")
	}

	// 直接复制数据
	for i := 0; i < m.rows; i++ {
		copy(m.data[i], dense[i])
	}
}

// GetRow 获取指定行的所有元素
func (m *denseMatrix) GetRow(row int) ([]int, []float64) {
	if row < 0 || row >= m.rows {
		panic("row index out of range")
	}

	// 对于稠密矩阵，返回所有列索引和对应的值
	cols := make([]int, m.cols)
	values := make([]float64, m.cols)
	for j := 0; j < m.cols; j++ {
		cols[j] = j
		values[j] = m.data[row][j]
	}
	return cols, values
}

// MatrixVectorMultiply 执行矩阵向量乘法
func (m *denseMatrix) MatrixVectorMultiply(x []float64) []float64 {
	if len(x) != m.cols {
		panic("vector dimension mismatch")
	}

	result := make([]float64, m.rows)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result[i] += m.data[i][j] * x[j]
		}
	}
	return result
}

// Clear 将矩阵重置为零矩阵
func (m *denseMatrix) Clear() {
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			m.data[i][j] = 0
		}
	}
}
