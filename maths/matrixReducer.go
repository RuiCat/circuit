package maths

// denseMatrixPruner 实现了 MatrixPruner 接口，用于移除稠密矩阵中的全零行和全零列。
type denseMatrixPruner[T Number] struct {
	originalMatrix Matrix[T]   // originalMatrix 存储原始的稠密矩阵，在处理过程中保持不变。
	prunedMatrix   Matrix[T]   // prunedMatrix 缓存经过修剪（移除零行/列）后的结果矩阵。
	rowMapping     map[int]int // rowMapping 存储新矩阵的行索引到原始矩阵行索引的映射。
	colMapping     map[int]int // colMapping 存储新矩阵的列索引到原始矩阵列索引的映射。
	epsilon        float64     // epsilon 用于判断一个数值是否接近于零的阈值。
}

// NewMatrixPruner 创建并返回一个用于稠密矩阵的 MatrixPruner 实例。
func NewMatrixPruner[T Number](mat Matrix[T]) MatrixPruner[T] {
	return &denseMatrixPruner[T]{
		originalMatrix: mat,
		rowMapping:     make(map[int]int),
		colMapping:     make(map[int]int),
		epsilon:        Epsilon,
	}
}

// RemoveZeroRows 从原始矩阵中移除所有全零行，并返回一个新的、不含全零行的稠密矩阵。
// 它会更新内部的 prunedMatrix 和 rowMapping。
func (p *denseMatrixPruner[T]) RemoveZeroRows() Matrix[T] {
	if p.originalMatrix == nil {
		return nil
	}

	originalRows := p.originalMatrix.Rows()
	originalCols := p.originalMatrix.Cols()
	var nonZeroRows []int

	for row := 0; row < originalRows; row++ {
		isZeroRow := true
		for col := 0; col < originalCols; col++ {
			if Abs(p.originalMatrix.Get(row, col)) > p.epsilon {
				isZeroRow = false
				break
			}
		}
		if !isZeroRow {
			nonZeroRows = append(nonZeroRows, row)
		}
	}

	newRows := len(nonZeroRows)
	// 创建一个新的稠密矩阵来存储结果
	prunedMat := NewDenseMatrix[T](newRows, originalCols)
	p.rowMapping = make(map[int]int, newRows)

	// 填充新矩阵并记录行映射
	for newRow, originalRow := range nonZeroRows {
		p.rowMapping[newRow] = originalRow
		for col := 0; col < originalCols; col++ {
			prunedMat.Set(newRow, col, p.originalMatrix.Get(originalRow, col))
		}
	}

	p.prunedMatrix = prunedMat
	// 缓存结果并清空列映射（因为此操作只影响行）
	p.prunedMatrix = prunedMat
	p.colMapping = make(map[int]int)
	return prunedMat
}

// RemoveZeroCols 从原始矩阵中移除所有全零列，并返回一个新的、不含全零列的稠密矩阵。
// 它会更新内部的 prunedMatrix 和 colMapping。
func (p *denseMatrixPruner[T]) RemoveZeroCols() Matrix[T] {
	if p.originalMatrix == nil {
		return nil
	}

	originalRows := p.originalMatrix.Rows()
	originalCols := p.originalMatrix.Cols()
	var nonZeroCols []int

	for col := 0; col < originalCols; col++ {
		isZeroCol := true
		for row := 0; row < originalRows; row++ {
			if Abs(p.originalMatrix.Get(row, col)) > p.epsilon {
				isZeroCol = false
				break
			}
		}
		if !isZeroCol {
			nonZeroCols = append(nonZeroCols, col)
		}
	}

	newCols := len(nonZeroCols)
	prunedMat := NewDenseMatrix[T](originalRows, newCols)
	p.colMapping = make(map[int]int, newCols)

	for newCol, originalCol := range nonZeroCols {
		p.colMapping[newCol] = originalCol
		for row := 0; row < originalRows; row++ {
			prunedMat.Set(row, newCol, p.originalMatrix.Get(row, originalCol))
		}
	}

	p.prunedMatrix = prunedMat
	// 缓存结果并清空行映射
	p.prunedMatrix = prunedMat
	p.rowMapping = make(map[int]int)
	return prunedMat
}

// Compress 依次移除全零行和全零列（或反序），返回一个完全压缩的矩阵。
// removeRowsFirst 参数控制是先移除零行还是先移除零列。
func (p *denseMatrixPruner[T]) Compress(removeRowsFirst bool) Matrix[T] {
	if p.originalMatrix == nil {
		return nil
	}

	var tempMat Matrix[T]
	if removeRowsFirst {
		tempMat = p.RemoveZeroRows()
	} else {
		tempMat = p.RemoveZeroCols()
	}

	tempPruner := &denseMatrixPruner[T]{
		originalMatrix: tempMat,
		rowMapping:     make(map[int]int),
		colMapping:     make(map[int]int),
		epsilon:        Epsilon,
	}
	var finalMat Matrix[T]

	if removeRowsFirst {
		finalMat = tempPruner.RemoveZeroCols()
		p.colMapping = tempPruner.GetColMapping()
	} else {
		finalMat = tempPruner.RemoveZeroRows()
		p.rowMapping = tempPruner.GetRowMapping()
	}

	p.prunedMatrix = finalMat
	return finalMat
}

// GetRowMapping 返回新矩阵行索引到原始矩阵行索引的映射。
func (p *denseMatrixPruner[T]) GetRowMapping() map[int]int {
	return p.rowMapping
}

// GetColMapping 返回新矩阵列索引到原始矩阵列索引的映射。
func (p *denseMatrixPruner[T]) GetColMapping() map[int]int {
	return p.colMapping
}

// GetOriginalRowIndex 根据新矩阵的行索引查找其在原始矩阵中的对应行索引。
func (p *denseMatrixPruner[T]) GetOriginalRowIndex(newRow int) (int, bool) {
	originalRow, ok := p.rowMapping[newRow]
	return originalRow, ok
}

// GetOriginalColIndex 根据新矩阵的列索引查找其在原始矩阵中的对应列索引。
func (p *denseMatrixPruner[T]) GetOriginalColIndex(newCol int) (int, bool) {
	originalCol, ok := p.colMapping[newCol]
	return originalCol, ok
}

// GetPrunedMatrix 返回最近一次修剪操作后缓存的结果矩阵。
func (p *denseMatrixPruner[T]) GetPrunedMatrix() Matrix[T] {
	return p.prunedMatrix
}

// sparseMatrixPruner 实现了 MatrixPruner 接口，用于高效地移除稀疏矩阵中的全零行和全零列。
// 它利用稀疏矩阵的 CSR 结构，并使用缓冲区来最小化内存分配，提高性能。
type sparseMatrixPruner[T Number] struct {
	originalMatrix *sparseMatrix[T] // 原始稀疏矩阵。
	prunedMatrix   Matrix[T]        // 缓存的修剪后矩阵。
	rowMapping     map[int]int      // 新行 → 原始行 映射。
	colMapping     map[int]int      // 新列 → 原始列 映射。
	reverseColMap  map[int]int      // 原始列 → 新列 映射，用于加速列移除操作。
	epsilon        float64          // 零值判断阈值。

	// --- 性能优化缓冲区 ---
	colIndBuf            []int // 存储修剪后矩阵的列索引。
	valuesBuf            []T   // 存储修剪后矩阵的值。
	nonZeroRowIndicesBuf []int // 存储非零行的原始索引。
	nonZeroColIndicesBuf []int // 存储非零列的原始索引。
	newRowPtrBuf         []int // 存储修剪后矩阵的行指针。
	colNonZeroCountBuf   []int // 用于统计每列的非零元素数量。
}

// NewSparseMatrixPruner 创建并返回一个用于稀疏矩阵的 MatrixPruner 实例。
// 它会预先分配合理大小的缓冲区以提高后续操作的性能。
func NewSparseMatrixPruner[T Number](mat *sparseMatrix[T]) MatrixPruner[T] {
	nonZeroCount := mat.NonZeroCount()
	return &sparseMatrixPruner[T]{
		originalMatrix:       mat,
		rowMapping:           make(map[int]int),
		colMapping:           make(map[int]int),
		reverseColMap:        make(map[int]int),
		epsilon:              Epsilon,
		colIndBuf:            make([]int, 0, nonZeroCount),
		valuesBuf:            make([]T, 0, nonZeroCount),
		nonZeroRowIndicesBuf: make([]int, 0, mat.Rows()),
		nonZeroColIndicesBuf: make([]int, 0, mat.Cols()),
		newRowPtrBuf:         make([]int, 0, mat.Rows()+1),
		colNonZeroCountBuf:   make([]int, 0, mat.Cols()),
	}
}

// RemoveZeroRows 高效地从稀疏矩阵中移除全零行。
// 它通过检查 CSR 格式的 rowPtr 数组来快速识别空行，避免了逐元素扫描。
func (p *sparseMatrixPruner[T]) RemoveZeroRows() Matrix[T] {
	if p.originalMatrix == nil {
		return nil
	}

	origRows := p.originalMatrix.rows
	origCols := p.originalMatrix.cols
	nonZeroRowIndices := p.nonZeroRowIndicesBuf[:0]

	// 利用 rowPtr 快速找到所有非零行。如果 rowPtr[i] == rowPtr[i+1]，则第 i 行为空。
	for row := 0; row < origRows; row++ {
		if p.originalMatrix.rowPtr[row] != p.originalMatrix.rowPtr[row+1] {
			nonZeroRowIndices = append(nonZeroRowIndices, row)
		}
	}
	p.nonZeroRowIndicesBuf = nonZeroRowIndices

	newRows := len(nonZeroRowIndices)
	if cap(p.newRowPtrBuf) < newRows+1 {
		p.newRowPtrBuf = make([]int, newRows+1)
	} else {
		p.newRowPtrBuf = p.newRowPtrBuf[:newRows+1]
	}
	newRowPtr := p.newRowPtrBuf
	p.colIndBuf = p.colIndBuf[:0]
	p.valuesBuf = p.valuesBuf[:0]

	dataPtr := p.originalMatrix.DataManager.DataPtr()
	currentIdx := 0
	for k := range p.rowMapping {
		delete(p.rowMapping, k)
	}
	for newRow, origRow := range nonZeroRowIndices {
		p.rowMapping[newRow] = origRow
		newRowPtr[newRow] = currentIdx

		start := p.originalMatrix.rowPtr[origRow]
		end := p.originalMatrix.rowPtr[origRow+1]
		p.colIndBuf = append(p.colIndBuf, p.originalMatrix.colInd[start:end]...)
		p.valuesBuf = append(p.valuesBuf, dataPtr[start:end]...)

		currentIdx += end - start
	}
	newRowPtr[newRows] = currentIdx

	// 基于收集到的非零行数据构建新的稀疏矩阵
	prunedMat := &sparseMatrix[T]{
		rows:        newRows,
		cols:        origCols,
		rowPtr:      newRowPtr,
		colInd:      p.colIndBuf,
		DataManager: NewDataManagerWithData(p.valuesBuf),
	}

	p.prunedMatrix = prunedMat
	// 清理列映射信息，因为此操作只影响行
	for k := range p.colMapping {
		delete(p.colMapping, k)
	}
	for k := range p.reverseColMap {
		delete(p.reverseColMap, k)
	}
	return prunedMat
}

// RemoveZeroCols 高效地从稀疏矩阵中移除全零列。
// 它首先遍历所有非零元素来统计每列的非零计数，然后只重建包含非零元素的列。
func (p *sparseMatrixPruner[T]) RemoveZeroCols() Matrix[T] {
	if p.originalMatrix == nil {
		return nil
	}

	origRows := p.originalMatrix.rows
	origCols := p.originalMatrix.cols

	if cap(p.colNonZeroCountBuf) < origCols {
		p.colNonZeroCountBuf = make([]int, origCols)
	} else {
		p.colNonZeroCountBuf = p.colNonZeroCountBuf[:origCols]
		for i := range p.colNonZeroCountBuf {
			p.colNonZeroCountBuf[i] = 0
		}
	}
	colNonZeroCount := p.colNonZeroCountBuf
	for _, col := range p.originalMatrix.colInd {
		colNonZeroCount[col]++
	}

	nonZeroColIndices := p.nonZeroColIndicesBuf[:0]
	for k := range p.colMapping {
		delete(p.colMapping, k)
	}
	for k := range p.reverseColMap {
		delete(p.reverseColMap, k)
	}
	for newCol, origCol := range colNonZeroCount {
		if origCol > 0 {
			p.colMapping[len(nonZeroColIndices)] = newCol
			p.reverseColMap[newCol] = len(nonZeroColIndices)
			nonZeroColIndices = append(nonZeroColIndices, newCol)
		}
	}
	p.nonZeroColIndicesBuf = nonZeroColIndices
	newCols := len(nonZeroColIndices)
	if newCols == 0 {
		return NewSparseMatrix[T](origRows, 0)
	}

	if cap(p.newRowPtrBuf) < origRows+1 {
		p.newRowPtrBuf = make([]int, origRows+1)
	} else {
		p.newRowPtrBuf = p.newRowPtrBuf[:origRows+1]
	}
	newRowPtr := p.newRowPtrBuf
	dataPtr := p.originalMatrix.DataManager.DataPtr()
	copy(newRowPtr, p.originalMatrix.rowPtr)

	nonZeroCount := len(p.originalMatrix.colInd)
	if cap(p.colIndBuf) < nonZeroCount {
		p.colIndBuf = make([]int, nonZeroCount)
	} else {
		p.colIndBuf = p.colIndBuf[:nonZeroCount]
	}
	newColInd := p.colIndBuf

	if cap(p.valuesBuf) < nonZeroCount {
		p.valuesBuf = make([]T, nonZeroCount)
	} else {
		p.valuesBuf = p.valuesBuf[:nonZeroCount]
	}
	newValues := p.valuesBuf

	newIdx := 0
	for i, origCol := range p.originalMatrix.colInd {
		if newCol, ok := p.reverseColMap[origCol]; ok {
			newColInd[newIdx] = newCol
			newValues[newIdx] = dataPtr[i]
			newIdx++
		}
	}

	prunedMat := &sparseMatrix[T]{
		rows:        origRows,
		cols:        newCols,
		rowPtr:      newRowPtr,
		colInd:      newColInd[:newIdx],
		DataManager: NewDataManagerWithData(newValues[:newIdx]),
	}

	p.prunedMatrix = prunedMat
	// 清理行映射信息
	for k := range p.rowMapping {
		delete(p.rowMapping, k)
	}
	return prunedMat
}

// Compress 依次移除稀疏矩阵的全零行和全零列（或反序），返回一个完全压缩的矩阵。
// 它会链接两次修剪操作的映射关系，以得到最终的新旧索引映射。
func (p *sparseMatrixPruner[T]) Compress(removeRowsFirst bool) Matrix[T] {
	if p.originalMatrix == nil {
		return nil
	}

	var tempMat *sparseMatrix[T]
	if removeRowsFirst {
		tempMat = p.RemoveZeroRows().(*sparseMatrix[T])
	} else {
		tempMat = p.RemoveZeroCols().(*sparseMatrix[T])
	}

	tempPruner := NewSparseMatrixPruner(tempMat)
	var finalMat *sparseMatrix[T]
	if removeRowsFirst {
		finalMat = tempPruner.RemoveZeroCols().(*sparseMatrix[T])
		p.colMapping = make(map[int]int)
		for newCol, tempCol := range tempPruner.GetColMapping() {
			originalCol := tempCol
			p.colMapping[newCol] = originalCol
		}
	} else {
		finalMat = tempPruner.RemoveZeroRows().(*sparseMatrix[T])
		p.rowMapping = make(map[int]int)
		for newRow, tempRow := range tempPruner.GetRowMapping() {
			originalRow := tempRow
			p.rowMapping[newRow] = originalRow
		}
	}
	p.prunedMatrix = finalMat
	return finalMat
}

// GetRowMapping 返回新矩阵行索引到原始矩阵行索引的映射。
func (p *sparseMatrixPruner[T]) GetRowMapping() map[int]int {
	return p.rowMapping
}

// GetColMapping 返回新矩阵列索引到原始矩阵列索引的映射。
func (p *sparseMatrixPruner[T]) GetColMapping() map[int]int {
	return p.colMapping
}

// GetOriginalRowIndex 根据新矩阵的行索引查找其在原始矩阵中的对应行索引。
func (p *sparseMatrixPruner[T]) GetOriginalRowIndex(newRow int) (int, bool) {
	origRow, ok := p.rowMapping[newRow]
	return origRow, ok
}

// GetOriginalColIndex 根据新矩阵的列索引查找其在原始矩阵中的对应列索引。
func (p *sparseMatrixPruner[T]) GetOriginalColIndex(newCol int) (int, bool) {
	origCol, ok := p.colMapping[newCol]
	return origCol, ok
}

// GetPrunedMatrix 返回最近一次修剪操作后缓存的结果矩阵。
func (p *sparseMatrixPruner[T]) GetPrunedMatrix() Matrix[T] {
	return p.prunedMatrix
}
