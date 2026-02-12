package maths

import "fmt"

// subMatrix 提供了对另一个矩阵的矩形子区域的视图。
// 它实现了 Matrix 接口，允许将子区域视为独立的矩阵，
// 而无需复制底层数据。
type subMatrix[T Number] struct {
	baseMatrix Matrix[T] // 原始矩阵
	rowOffset  int       // 在基础矩阵中的起始行索引
	colOffset  int       // 在基础矩阵中的起始列索引
	rows       int       // 子矩阵的行数
	cols       int       // 子矩阵的列数
}

// NewSubMatrix 创建一个新的子矩阵视图。
func NewSubMatrix[T Number](base Matrix[T], rowOffset, colOffset, rows, cols int) Matrix[T] {
	if base == nil {
		panic("base matrix cannot be nil")
	}
	if rowOffset < 0 || colOffset < 0 || rows < 0 || cols < 0 {
		panic("offsets and dimensions cannot be negative")
	}
	if rowOffset+rows > base.Rows() || colOffset+cols > base.Cols() {
		panic("sub-matrix dimensions exceed base matrix boundaries")
	}

	return &subMatrix[T]{
		baseMatrix: base,
		rowOffset:  rowOffset,
		colOffset:  colOffset,
		rows:       rows,
		cols:       cols,
	}
}

// checkBounds 检查给定的行和列索引是否在子矩阵的边界内。
func (m *subMatrix[T]) checkBounds(row, col int) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic(fmt.Sprintf("sub-matrix index out of range: (%d, %d) with size %dx%d", row, col, m.rows, m.cols))
	}
}

// Base 返回子矩阵所基于的原始矩阵。
func (m *subMatrix[T]) Base() Matrix[T] {
	return m.baseMatrix
}

// Rows 返回子矩阵的行数。
func (m *subMatrix[T]) Rows() int {
	return m.rows
}

// Cols 返回子矩阵的列数。
func (m *subMatrix[T]) Cols() int {
	return m.cols
}

func (m *subMatrix[T]) String() string {
	var s string
	for r := 0; r < m.rows; r++ {
		for c := 0; c < m.cols; c++ {
			s += fmt.Sprintf("%v ", m.Get(r, c))
		}
		s += ""
	}
	return s
}

// IsSquare 检查子矩阵是否为方阵。
func (m *subMatrix[T]) IsSquare() bool {
	return m.rows == m.cols
}

// Get 获取子矩阵中指定位置的元素值。
func (m *subMatrix[T]) Get(row, col int) T {
	m.checkBounds(row, col)
	return m.baseMatrix.Get(row+m.rowOffset, col+m.colOffset)
}

// Set 设置子矩阵中指定位置的元素值。
func (m *subMatrix[T]) Set(row, col int, value T) {
	m.checkBounds(row, col)
	m.baseMatrix.Set(row+m.rowOffset, col+m.colOffset, value)
}

// Increment 增加子矩阵中指定位置的元素值。
func (m *subMatrix[T]) Increment(row, col int, value T) {
	m.checkBounds(row, col)
	m.baseMatrix.Increment(row+m.rowOffset, col+m.colOffset, value)
}

// GetRow 从子矩阵中获取一行。
// 它会从基础矩阵获取整行，然后过滤并调整列索引以匹配子矩阵的边界。
func (m *subMatrix[T]) GetRow(row int) ([]int, Vector[T]) {
	if row < 0 || row >= m.rows {
		panic(fmt.Sprintf("row index out of range: %d (rows: %d)", row, m.rows))
	}

	baseCols, baseVals := m.baseMatrix.GetRow(row + m.rowOffset)

	var subCols []int
	var subVals []T

	// 遍历基础行的非零元素，只保留在子矩阵列范围内的部分
	for i, col := range baseCols {
		if col >= m.colOffset && col < m.colOffset+m.cols {
			subCols = append(subCols, col-m.colOffset) // 调整列索引
			subVals = append(subVals, baseVals.Get(i))
		}
	}

	// 假设 NewDenseVectorWithData 可用
	return subCols, NewDenseVectorWithData(subVals)
}

// ToDense 将子矩阵转换为一个稠密向量。
func (m *subMatrix[T]) ToDense() Vector[T] {
	dense := make([]T, m.rows*m.cols)
	for r := 0; r < m.rows; r++ {
		for c := 0; c < m.cols; c++ {
			dense[r*m.cols+c] = m.Get(r, c)
		}
	}
	return NewDenseVectorWithData(dense)
}

// BuildFromDense 用一个二维切片的数据填充子矩阵。
func (m *subMatrix[T]) BuildFromDense(dense [][]T) {
	if len(dense) != m.rows || (len(dense) > 0 && len(dense[0]) != m.cols) {
		panic("dense matrix dimension mismatch")
	}
	for r := range dense {
		for c := range dense[r] {
			m.Set(r, c, dense[r][c])
		}
	}
}

// Zero 将子矩阵视图区域内的所有元素设置为零。
func (m *subMatrix[T]) Zero() {
	var zero T
	for r := 0; r < m.rows; r++ {
		for c := 0; c < m.cols; c++ {
			m.Set(r, c, zero)
		}
	}
}

// Copy 将子矩阵的内容复制到目标矩阵 `a`。
func (m *subMatrix[T]) Copy(a Matrix[T]) {
	if a.Rows() != m.rows || a.Cols() != m.cols {
		panic("dimension mismatch for copy")
	}
	for r := 0; r < m.rows; r++ {
		// GetRow 效率可能不高，但更通用
		cols, vals := m.GetRow(r)
		for i, c := range cols {
			a.Set(r, c, vals.Get(i))
		}
	}
}

// Resize 在子矩阵视图上不支持，会引发 panic。
func (m *subMatrix[T]) Resize(rows, cols int) {
	panic("Resize is not supported on a sub-matrix view")
}

// SwapRows 交换子矩阵中的两行。
func (m *subMatrix[T]) SwapRows(row1, row2 int) {
	m.checkBounds(row1, 0)
	m.checkBounds(row2, 0)
	m.baseMatrix.SwapRows(row1+m.rowOffset, row2+m.rowOffset)
}

// MatrixVectorMultiply 计算子矩阵与向量的乘积。
func (m *subMatrix[T]) MatrixVectorMultiply(x Vector[T]) Vector[T] {
	if x.Length() != m.cols {
		panic(fmt.Sprintf("vector dimension mismatch: x length=%d, matrix cols=%d", x.Length(), m.cols))
	}
	result := NewDenseVector[T](m.rows)
	for i := 0; i < m.rows; i++ {
		var sum T
		for j := 0; j < m.cols; j++ {
			sum += m.Get(i, j) * x.Get(j)
		}
		result.Set(i, sum)
	}
	return result
}

// NonZeroCount 返回子矩阵中非零元素的数量。
func (m *subMatrix[T]) NonZeroCount() int {
	count := 0
	var zero T
	for r := 0; r < m.rows; r++ {
		for c := 0; c < m.cols; c++ {
			if m.Get(r, c) != zero {
				count++
			}
		}
	}
	return count
}
