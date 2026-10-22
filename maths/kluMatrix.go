// 行切片稀疏矩阵:每行按列号升序存非零元,并维护列→行的反向索引,
// 实现 solve 的 Matrix 接口,供稀疏 LU 的主元搜索与消元只扫结构非零。

package maths

import "fmt"

// kluMatrix 是行切片稀疏矩阵(零元素不占槽位;Set 写零即删槽)。
type kluMatrix[T Number] struct {
	rows      int
	cols      int
	rowCols   [][]int // 每行升序的非零列号
	rowVals   [][]T   // 与 rowCols 对齐的值
	colRows   [][]int // 每列升序的非零行号(反向索引)
	keepZeros bool    // true:显式零也占槽位(LU 的 L/U 用,结构才与数值无关)
}

func maxInt() int { return int(^uint(0) >> 1) }

// NewKLUMatrix 创建 rows×cols 的空稀疏矩阵(负维度/尺寸溢出 panic,
// 与 NewDenseMatrix 同语义);返回 Matrix 接口以便与稠密实现互换。
func NewKLUMatrix[T Number](rows, cols int) Matrix[T] {
	if rows < 0 || cols < 0 {
		panic(fmt.Sprintf("solve: NewKLUMatrix 负维度 rows=%d cols=%d", rows, cols))
	}
	if uint64(rows)*uint64(cols) > uint64(maxInt()) {
		panic(fmt.Sprintf("solve: NewKLUMatrix 尺寸溢出 rows=%d cols=%d", rows, cols))
	}
	return newSparseMatrix[T](rows, cols)
}

// newSparseMatrix 是内部构造器(不做参数检查,供包内使用)。
func newSparseMatrix[T Number](rows, cols int) *kluMatrix[T] {
	m := &kluMatrix[T]{rows: rows, cols: cols}
	m.rowCols = make([][]int, rows)
	m.rowVals = make([][]T, rows)
	m.colRows = make([][]int, cols)
	return m
}

// newStructuralMatrix 创建保留显式零槽位的稀疏矩阵(LU 的 L/U 用):
// 结构只由非零模式决定、不随数值抵消而收缩,消元计划才能跨数值复用。
func newStructuralMatrix[T Number](rows, cols int) *kluMatrix[T] {
	m := newSparseMatrix[T](rows, cols)
	m.keepZeros = true
	return m
}

// Base 返回底层(接口惯例)。
func (m *kluMatrix[T]) Base() Matrix[T] { return m }

// Rows / Cols / IsSquare 基础属性。
func (m *kluMatrix[T]) Rows() int      { return m.rows }
func (m *kluMatrix[T]) Cols() int      { return m.cols }
func (m *kluMatrix[T]) IsSquare() bool { return m.rows == m.cols }

// checkIndex 校验(row, col) 在界内,越界 panic(与稠密实现一致)。
func (m *kluMatrix[T]) checkIndex(row, col int) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		panic(fmt.Sprintf("solve: 稀疏矩阵下标越界 row=%d col=%d (rows=%d cols=%d)",
			row, col, m.rows, m.cols))
	}
}

// findCol 在升序列号里二分查找 col,返回下标与是否存在。
func findCol(cols []int, col int) (int, bool) {
	lo, hi := 0, len(cols)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if cols[mid] < col {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(cols) && cols[lo] == col {
		return lo, true
	}
	return lo, false
}

// insertAt 在升序切片的位置 pos 插入值(返回新切片)。
func insertAt[T any](items []T, pos int, v T) []T {
	items = append(items, v)
	copy(items[pos+1:], items[pos:])
	items[pos] = v
	return items
}

// Get 说明:无槽位返回零值(本结构不存显式零,零即无槽位)。

// Get 取元素(无槽位返回零值)。
func (m *kluMatrix[T]) Get(row, col int) T {
	m.checkIndex(row, col)
	var zero T
	pos, ok := findCol(m.rowCols[row], col)
	if !ok {
		return zero
	}
	return m.rowVals[row][pos]
}

// Set 写元素(写零即删除槽位)。
func (m *kluMatrix[T]) Set(row, col int, value T) {
	m.checkIndex(row, col)
	var zero T
	pos, ok := findCol(m.rowCols[row], col)
	if ok {
		if value == zero && !m.keepZeros {
			m.removeAt(row, pos)
			return
		}
		m.rowVals[row][pos] = value
		return
	}
	if value == zero && !m.keepZeros {
		return
	}
	m.rowCols[row] = insertAt(m.rowCols[row], pos, col)
	m.rowVals[row] = insertAt(m.rowVals[row], pos, value)
	m.colRows[col] = insertAt(m.colRows[col], m.findRowPos(col, row), row)
}

// Increment 累加元素(结果为零时删除槽位)。
func (m *kluMatrix[T]) Increment(row, col int, value T) {
	m.Set(row, col, m.Get(row, col)+value)
}

// deleteSlot 删除 (row, col) 的槽位(不存在则无操作);供 LU 消元用,
// 与 keepZeros 无关 —— 消元后的下三角槽位由结构决定,不随数值变化。
func (m *kluMatrix[T]) deleteSlot(row, col int) {
	m.checkIndex(row, col)
	if pos, ok := findCol(m.rowCols[row], col); ok {
		m.removeAt(row, pos)
	}
}

// hasSlot 判断 (row, col) 是否有槽位。
func (m *kluMatrix[T]) hasSlot(row, col int) bool {
	m.checkIndex(row, col)
	_, ok := findCol(m.rowCols[row], col)
	return ok
}

// removeAt 删除第 row 行 pos 位置上的槽位并同步列索引。
func (m *kluMatrix[T]) removeAt(row, pos int) {
	col := m.rowCols[row][pos]
	m.rowCols[row] = append(m.rowCols[row][:pos], m.rowCols[row][pos+1:]...)
	m.rowVals[row] = append(m.rowVals[row][:pos], m.rowVals[row][pos+1:]...)
	rows := m.colRows[col]
	if rp, ok := findCol(rows, row); ok {
		m.colRows[col] = append(rows[:rp], rows[rp+1:]...)
	}
}

// findRowPos 返回在列 row 的升序行号里 row 应插入的位置。
func (m *kluMatrix[T]) findRowPos(col, row int) int {
	pos, _ := findCol(m.colRows[col], row)
	return pos
}

// GetRow 返回第 row 行的非零“列索引 + 值向量”(均为独立副本,与稠密实现一致)。
func (m *kluMatrix[T]) GetRow(row int) ([]int, Vector[T]) {
	if row < 0 || row >= m.rows {
		panic(fmt.Sprintf("solve: 稀疏矩阵行下标越界 %d (rows=%d)", row, m.rows))
	}
	cols := make([]int, len(m.rowCols[row]))
	copy(cols, m.rowCols[row])
	vals := make([]T, len(m.rowVals[row]))
	copy(vals, m.rowVals[row])
	return cols, NewDenseVectorWithData(vals)
}

// ToDense 返回行优先展开的稠密向量(副本)。
func (m *kluMatrix[T]) ToDense() Vector[T] {
	out := make([]T, m.rows*m.cols)
	for i := 0; i < m.rows; i++ {
		base := i * m.cols
		for k, c := range m.rowCols[i] {
			out[base+c] = m.rowVals[i][k]
		}
	}
	return NewDenseVectorWithData(out)
}

// BuildFromDense 从二维稠密数据重建(超界截断,未给出的元素保持零)。
func (m *kluMatrix[T]) BuildFromDense(dense [][]T) {
	m.Zero()
	for r := 0; r < len(dense) && r < m.rows; r++ {
		for c := 0; c < len(dense[r]) && c < m.cols; c++ {
			m.Set(r, c, dense[r][c])
		}
	}
}

// Zero 清空全部槽位。
func (m *kluMatrix[T]) Zero() {
	for i := range m.rowCols {
		m.rowCols[i] = m.rowCols[i][:0]
		m.rowVals[i] = m.rowVals[i][:0]
	}
	for j := range m.colRows {
		m.colRows[j] = m.colRows[j][:0]
	}
}

// Copy 复制自身到 target(维度不符 panic;目标先清空)。
func (m *kluMatrix[T]) Copy(a Matrix[T]) {
	if a.Rows() != m.rows || a.Cols() != m.cols {
		panic(fmt.Sprintf("solve: 稀疏矩阵 Copy 维度不符 src=%dx%d dst=%dx%d",
			m.rows, m.cols, a.Rows(), a.Cols()))
	}
	if target, ok := a.(*kluMatrix[T]); ok {
		target.Zero()
		for i := 0; i < m.rows; i++ {
			target.rowCols[i] = append(target.rowCols[i], m.rowCols[i]...)
			target.rowVals[i] = append(target.rowVals[i], m.rowVals[i]...)
		}
		for j := 0; j < m.cols; j++ {
			target.colRows[j] = append(target.colRows[j], m.colRows[j]...)
		}
		return
	}
	a.Zero()
	for i := 0; i < m.rows; i++ {
		for k, c := range m.rowCols[i] {
			a.Set(i, c, m.rowVals[i][k])
		}
	}
}

// Resize 改维度:出界槽位丢弃,其余保留(收缩后再放大不恢复旧值)。
func (m *kluMatrix[T]) Resize(rows, cols int) {
	if rows < 0 || cols < 0 {
		panic("solve: 稀疏矩阵 Resize 负维度")
	}
	if uint64(rows)*uint64(cols) > uint64(maxInt()) {
		panic(fmt.Sprintf("solve: 稀疏矩阵 Resize 尺寸溢出 rows=%d cols=%d", rows, cols))
	}
	for i := 0; i < m.rows; i++ {
		keepCols := m.rowCols[i][:0]
		keepVals := m.rowVals[i][:0]
		for k, c := range m.rowCols[i] {
			if i < rows && c < cols {
				keepCols = append(keepCols, c)
				keepVals = append(keepVals, m.rowVals[i][k])
			}
		}
		m.rowCols[i] = keepCols
		m.rowVals[i] = keepVals
	}
	if rows > m.rows {
		for i := m.rows; i < rows; i++ {
			m.rowCols = append(m.rowCols, nil)
			m.rowVals = append(m.rowVals, nil)
		}
	} else {
		m.rowCols = m.rowCols[:rows]
		m.rowVals = m.rowVals[:rows]
	}
	m.rows = rows
	m.cols = cols
	m.rebuildColRows()
}

// rebuildColRows 由行切片重建列反向索引。
func (m *kluMatrix[T]) rebuildColRows() {
	m.colRows = make([][]int, m.cols)
	for i := 0; i < m.rows; i++ {
		for _, c := range m.rowCols[i] {
			m.colRows[c] = append(m.colRows[c], i)
		}
	}
}

// SwapRows 交换两行(行列索引同步维护;两行都有某列项时该列集合不变)。
func (m *kluMatrix[T]) SwapRows(row1, row2 int) {
	if row1 < 0 || row1 >= m.rows || row2 < 0 || row2 >= m.rows {
		panic(fmt.Sprintf("solve: 稀疏矩阵 SwapRows 行越界 row1=%d row2=%d rows=%d",
			row1, row2, m.rows))
	}
	if row1 == row2 {
		return
	}
	m.rowCols[row1], m.rowCols[row2] = m.rowCols[row2], m.rowCols[row1]
	m.rowVals[row1], m.rowVals[row2] = m.rowVals[row2], m.rowVals[row1]
	for _, c := range m.rowCols[row1] { // 原 row2 的列,行号 row2 → row1
		if _, both := findCol(m.rowCols[row2], c); !both {
			m.moveColEntry(c, row2, row1)
		}
	}
	for _, c := range m.rowCols[row2] { // 原 row1 的列,行号 row1 → row2
		if _, both := findCol(m.rowCols[row1], c); !both {
			m.moveColEntry(c, row1, row2)
		}
	}
}

// addColEntry 把行 row 加入列 col 的反向索引(已存在则忽略,保持升序)。
func (m *kluMatrix[T]) addColEntry(col, row int) {
	if _, ok := findCol(m.colRows[col], row); ok {
		return
	}
	pos, _ := findCol(m.colRows[col], row)
	m.colRows[col] = insertAt(m.colRows[col], pos, row)
}

// removeColEntry 把行 row 从列 col 的反向索引里删除(不存在则忽略)。
func (m *kluMatrix[T]) removeColEntry(col, row int) {
	if pos, ok := findCol(m.colRows[col], row); ok {
		m.colRows[col] = append(m.colRows[col][:pos], m.colRows[col][pos+1:]...)
	}
}

// moveColEntry 把列 col 的行号 oldRow 改成 newRow(保持升序)。
func (m *kluMatrix[T]) moveColEntry(col, oldRow, newRow int) {
	rows := m.colRows[col]
	pos, ok := findCol(rows, oldRow)
	if !ok {
		return
	}
	rows = append(rows[:pos], rows[pos+1:]...)
	pos, _ = findCol(rows, newRow)
	m.colRows[col] = insertAt(rows, pos, newRow)
}

// MatrixVectorMultiply 返回 A·x。
func (m *kluMatrix[T]) MatrixVectorMultiply(x Vector[T]) Vector[T] {
	if x.Length() != m.cols {
		panic(fmt.Sprintf("solve: 稀疏矩阵 MVM 维度不符 x=%d cols=%d", x.Length(), m.cols))
	}
	out := make([]T, m.rows)
	for i := 0; i < m.rows; i++ {
		var sum T
		for k, c := range m.rowCols[i] {
			sum += m.rowVals[i][k] * x.Get(c)
		}
		out[i] = sum
	}
	return NewDenseVectorWithData(out)
}

// NonZeroCount 非零元素个数(本结构不存显式零,即槽位数)。
func (m *kluMatrix[T]) NonZeroCount() int {
	total := 0
	for i := range m.rowCols {
		total += len(m.rowCols[i])
	}
	return total
}

// columnRows 返回第 col 列的结构非零行号(升序,内部切片,调用方不得改写)。
func (m *kluMatrix[T]) columnRows(col int) []int {
	if col < 0 || col >= m.cols {
		panic(fmt.Sprintf("solve: 稀疏矩阵列下标越界 %d (cols=%d)", col, m.cols))
	}
	return m.colRows[col]
}

// rowEntries 返回第 row 行的列号与值切片(内部引用,只读)。
func (m *kluMatrix[T]) rowEntries(row int) ([]int, []T) {
	if row < 0 || row >= m.rows {
		panic(fmt.Sprintf("solve: 稀疏矩阵行下标越界 %d (rows=%d)", row, m.rows))
	}
	return m.rowCols[row], m.rowVals[row]
}

// String 简要输出(与稠密实现同格式:行优先、空格分隔)。
func (m *kluMatrix[T]) String() string {
	s := ""
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			s += fmt.Sprintf("%v ", m.Get(i, j))
		}
	}
	return s
}
