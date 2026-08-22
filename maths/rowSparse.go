package maths

import (
	"fmt"
)

// rowSparseMatrix 行切片稀疏矩阵：每行一个独立的 (列索引, 值) 切片，无序存储。
// 针对 LU 分解的高频 fill-in 插入/删除优化：
//   - Set 插入用 append（O(1) 摊还），删除用「末位交换 + 截断」（O(1)）
//   - SwapRows 交换两个切片引用（O(1)）
//   - Get / GetRow 线性扫描（O(该行非零数)，行短开销小）
//   - 列索引 colIndex（列 → 行集合）供主元搜索等按列访问：O(该列非零数)，
//     避免行存储下遍历全部行找一列（LU 部分主元搜索占 CPU 40%+ 的瓶颈）。
//
// 相比 sparseMatrix 的 CSR 单数组（insertElement/deleteElement 需 memmove 整个 colInd
// 并逐行更新 rowPtr，O(nnz)），消除了 LU 分解 fill-in 插入退化为 O(n^4) 的瓶颈。
// 专供 luSparse 内部使用。
type rowSparseMatrix[T Number] struct {
	rows, cols int
	colInd     [][]int // 每行的列索引
	val        [][]T   // 每行的值（与 colInd 对齐）
	colIndex   [][]int // 每列的行索引（与 colInd 双向同步；仅供按列访问）
}

// newRowSparseMatrix 创建一个 rows×cols 的全零行切片稀疏矩阵。
func newRowSparseMatrix[T Number](rows, cols int) *rowSparseMatrix[T] {
	if rows < 0 || cols < 0 {
		panic("invalid matrix dimensions: cannot be negative")
	}
	return &rowSparseMatrix[T]{
		rows:     rows,
		cols:     cols,
		colInd:   make([][]int, rows),
		val:      make([][]T, rows),
		colIndex: make([][]int, cols),
	}
}

// Base 返回矩阵自身。
func (m *rowSparseMatrix[T]) Base() Matrix[T] { return m }

// Rows 返回矩阵行数。
func (m *rowSparseMatrix[T]) Rows() int { return m.rows }

// Cols 返回矩阵列数。
func (m *rowSparseMatrix[T]) Cols() int { return m.cols }

// IsSquare 判断是否为方阵。
func (m *rowSparseMatrix[T]) IsSquare() bool { return m.rows == m.cols }

// String 返回矩阵的简要描述。
func (m *rowSparseMatrix[T]) String() string {
	return fmt.Sprintf("rowSparseMatrix(%dx%d)", m.rows, m.cols)
}

// Get 获取指定位置的元素值，不存在（结构零）则返回零值。
// 行内线性扫描（行短，~13 元素）。
func (m *rowSparseMatrix[T]) Get(row, col int) T {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, m.rows, m.cols))
	}
	for idx, c := range m.colInd[row] {
		if c == col {
			return m.val[row][idx]
		}
	}
	var zero T
	return zero
}

// Set 设置指定位置的元素值：非零且不存在则 append，存在则更新；值为零则删除。
// 同步维护列索引 colIndex（新增元素 → 列成员 append；删除 → 末位交换+截断）。
func (m *rowSparseMatrix[T]) Set(row, col int, value T) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, m.rows, m.cols))
	}
	var zero T
	for idx, c := range m.colInd[row] {
		if c == col {
			if value == zero {
				// 删除：末位交换 + 截断，O(1)
				last := len(m.colInd[row]) - 1
				m.colInd[row][idx] = m.colInd[row][last]
				m.val[row][idx] = m.val[row][last]
				m.colInd[row] = m.colInd[row][:last]
				m.val[row] = m.val[row][:last]
				// 列索引同步删除：末位交换 + 截断
				for ci, r := range m.colIndex[col] {
					if r == row {
						clast := len(m.colIndex[col]) - 1
						m.colIndex[col][ci] = m.colIndex[col][clast]
						m.colIndex[col] = m.colIndex[col][:clast]
						break
					}
				}
			} else {
				m.val[row][idx] = value
			}
			return
		}
	}
	if value != zero {
		m.colInd[row] = append(m.colInd[row], col)
		m.val[row] = append(m.val[row], value)
		m.colIndex[col] = append(m.colIndex[col], row)
	}
}

// ColumnRows 返回指定列的非零行集合（列索引，无序）。
// 供 LU 部分主元搜索等按列访问：避免行存储下遍历全部行线性扫描。
func (m *rowSparseMatrix[T]) ColumnRows(col int) []int {
	if col < 0 || col >= m.cols {
		panic(fmt.Sprintf("column index out of range: %d (cols: %d)", col, m.cols))
	}
	return m.colIndex[col]
}

// Increment 增量更新元素值。
func (m *rowSparseMatrix[T]) Increment(row, col int, value T) {
	m.Set(row, col, m.Get(row, col)+value)
}

// SetAdd 原位增量更新：v(row,col) += delta，一次线性扫描完成
// 「读旧值 + 写新值 + 必要时插入/删除 + 列索引同步」。
// 用于 LU 消元内层 U(i,j) -= factor*U(k,j)：替代 Set(Get(i,j)-x) 的两次扫描。
// 浮点等价：a - x ≡ a + (-x)（IEEE 754 位级一致），且插入仍为尾部 append
// （行序不变），数值路径与逐元素 Set/Get 完全一致。
// 语义同 Increment，但避免 Get+Set 两次遍历同一行。
func (m *rowSparseMatrix[T]) SetAdd(row, col int, delta T) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic(fmt.Sprintf("matrix index out of range: row=%d, col=%d (rows=%d, cols=%d)", row, col, m.rows, m.cols))
	}
	var zero T
	for idx, c := range m.colInd[row] {
		if c == col {
			v := m.val[row][idx] + delta
			if v == zero {
				// 归零：删除 + 列索引同步（同 Set 删除）
				last := len(m.colInd[row]) - 1
				m.colInd[row][idx] = m.colInd[row][last]
				m.val[row][idx] = m.val[row][last]
				m.colInd[row] = m.colInd[row][:last]
				m.val[row] = m.val[row][:last]
				for ci, r := range m.colIndex[col] {
					if r == row {
						clast := len(m.colIndex[col]) - 1
						m.colIndex[col][ci] = m.colIndex[col][clast]
						m.colIndex[col] = m.colIndex[col][:clast]
						break
					}
				}
			} else {
				m.val[row][idx] = v
			}
			return
		}
	}
	// 不存在：delta 非零则插入（尾部 append，行序不变）
	if delta != zero {
		m.colInd[row] = append(m.colInd[row], col)
		m.val[row] = append(m.val[row], delta)
		m.colIndex[col] = append(m.colIndex[col], row)
	}
}

// GetRow 返回该行非零元素（列索引 + 值向量）的独立拷贝。
func (m *rowSparseMatrix[T]) GetRow(row int) ([]int, Vector[T]) {
	if row < 0 || row >= m.rows {
		panic(fmt.Sprintf("row index out of range: %d (rows: %d)", row, m.rows))
	}
	cols := make([]int, len(m.colInd[row]))
	copy(cols, m.colInd[row])
	vals := make([]T, len(m.val[row]))
	copy(vals, m.val[row])
	return cols, NewDenseVectorWithData(vals)
}

// NonZeroCount 统计非零元素数量。
func (m *rowSparseMatrix[T]) NonZeroCount() int {
	c := 0
	for i := 0; i < m.rows; i++ {
		c += len(m.colInd[i])
	}
	return c
}

// SwapRows 交换两行（交换切片引用，O(1)），并同步列索引。
func (m *rowSparseMatrix[T]) SwapRows(r1, r2 int) {
	if r1 < 0 || r1 >= m.rows || r2 < 0 || r2 >= m.rows {
		panic(fmt.Sprintf("row index out of range: r1=%d, r2=%d (rows=%d)", r1, r2, m.rows))
	}
	if r1 == r2 {
		return
	}
	m.colInd[r1], m.colInd[r2] = m.colInd[r2], m.colInd[r1]
	m.val[r1], m.val[r2] = m.val[r2], m.val[r1]
	// 列索引同步：两行的所有列成员互换行号。
	// 遍历交换前的行集合（r1 旧 = r2 新，r2 旧 = r1 新），
	// 在对应列的成员里把 r1↔r2 替换。
	for _, c := range m.colInd[r1] {
		for ci, r := range m.colIndex[c] {
			if r == r2 {
				m.colIndex[c][ci] = r1
				break
			}
		}
	}
	for _, c := range m.colInd[r2] {
		for ci, r := range m.colIndex[c] {
			if r == r1 {
				m.colIndex[c][ci] = r2
				break
			}
		}
	}
}

// Zero 清空矩阵为全零。
func (m *rowSparseMatrix[T]) Zero() {
	for i := 0; i < m.rows; i++ {
		m.colInd[i] = m.colInd[i][:0]
		m.val[i] = m.val[i][:0]
	}
	for c := 0; c < m.cols; c++ {
		m.colIndex[c] = m.colIndex[c][:0]
	}
}

// Copy 复制自身数据到目标矩阵 a。
func (m *rowSparseMatrix[T]) Copy(a Matrix[T]) {
	if a.Rows() != m.rows || a.Cols() != m.cols {
		panic(fmt.Sprintf("dimension mismatch: %dx%d -> %dx%d", m.rows, m.cols, a.Rows(), a.Cols()))
	}
	for i := 0; i < m.rows; i++ {
		for idx := range m.colInd[i] {
			a.Set(i, m.colInd[i][idx], m.val[i][idx])
		}
	}
}

// Resize 改变矩阵维度并清空。
func (m *rowSparseMatrix[T]) Resize(rows, cols int) {
	if rows < 0 || cols < 0 {
		panic("invalid matrix dimensions: cannot be negative")
	}
	m.rows = rows
	m.cols = cols
	m.colInd = make([][]int, rows)
	m.val = make([][]T, rows)
	m.colIndex = make([][]int, cols)
}

// ToDense 转换为稠密向量（行优先展开）。
func (m *rowSparseMatrix[T]) ToDense() Vector[T] {
	dense := make([]T, m.rows*m.cols)
	for i := 0; i < m.rows; i++ {
		for idx, c := range m.colInd[i] {
			dense[i*m.cols+c] = m.val[i][idx]
		}
	}
	return NewDenseVectorWithData(dense)
}

// BuildFromDense 从稠密二维切片构建。
func (m *rowSparseMatrix[T]) BuildFromDense(dense [][]T) {
	if len(dense) == 0 {
		m.rows, m.cols = 0, 0
		m.colInd, m.val = nil, nil
		return
	}
	m.rows = len(dense)
	m.cols = len(dense[0])
	m.colInd = make([][]int, m.rows)
	m.val = make([][]T, m.rows)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			var zero T
			if dense[i][j] != zero {
				m.colInd[i] = append(m.colInd[i], j)
				m.val[i] = append(m.val[i], dense[i][j])
			}
		}
	}
}

// MatrixVectorMultiply 矩阵向量乘法 A·x。
func (m *rowSparseMatrix[T]) MatrixVectorMultiply(x Vector[T]) Vector[T] {
	if x.Length() != m.cols {
		panic(fmt.Sprintf("vector dimension mismatch: x length=%d, matrix cols=%d", x.Length(), m.cols))
	}
	result := NewDenseVector[T](m.rows)
	for i := 0; i < m.rows; i++ {
		var sum T
		for idx, c := range m.colInd[i] {
			sum += m.val[i][idx] * x.Get(c)
		}
		result.Set(i, sum)
	}
	return result
}
