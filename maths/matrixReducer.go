package maths

import (
	"fmt"
	"math"
)

type denseMatrixPruner struct {
	originalMatrix Matrix      // 原始稠密矩阵（只读，不修改）
	prunedMatrix   Matrix      // 精简后的矩阵（缓存）
	rowMapping     map[int]int // 新行索引 → 原始行索引
	colMapping     map[int]int // 新列索引 → 原始列索引
	epsilon        float64     // 零值判断阈值（默认复用全局Epsilon）
}

// RemoveZeroRows 移除全零行（核心逻辑：筛选非零行，记录索引映射）
func (p *denseMatrixPruner) RemoveZeroRows() Matrix {
	if p.originalMatrix == nil {
		return nil
	}

	originalRows := p.originalMatrix.Rows()
	originalCols := p.originalMatrix.Cols()
	var nonZeroRows []int // 存储非零行的原始索引

	// 1. 筛选非零行（判断行内所有元素是否全零）
	for row := 0; row < originalRows; row++ {
		isZeroRow := true
		for col := 0; col < originalCols; col++ {
			if math.Abs(p.originalMatrix.Get(row, col)) > p.epsilon {
				isZeroRow = false
				break
			}
		}
		if !isZeroRow {
			nonZeroRows = append(nonZeroRows, row)
		}
	}

	// 2. 构建精简矩阵和行映射
	newRows := len(nonZeroRows)
	prunedMat := NewDenseMatrix(newRows, originalCols)
	p.rowMapping = make(map[int]int, newRows)

	for newRow, originalRow := range nonZeroRows {
		p.rowMapping[newRow] = originalRow // 新行 → 原始行
		// 复制非零行数据
		for col := 0; col < originalCols; col++ {
			prunedMat.Set(newRow, col, p.originalMatrix.Get(originalRow, col))
		}
	}

	// 3. 缓存精简矩阵（列映射不变，清空列映射缓存）
	p.prunedMatrix = prunedMat
	p.colMapping = make(map[int]int)
	return prunedMat
}

// RemoveZeroCols 移除全零列（逻辑与行类似，遍历列判断）
func (p *denseMatrixPruner) RemoveZeroCols() Matrix {
	if p.originalMatrix == nil {
		return nil
	}

	originalRows := p.originalMatrix.Rows()
	originalCols := p.originalMatrix.Cols()
	var nonZeroCols []int // 存储非零列的原始索引

	// 1. 筛选非零列
	for col := 0; col < originalCols; col++ {
		isZeroCol := true
		for row := 0; row < originalRows; row++ {
			if math.Abs(p.originalMatrix.Get(row, col)) > p.epsilon {
				isZeroCol = false
				break
			}
		}
		if !isZeroCol {
			nonZeroCols = append(nonZeroCols, col)
		}
	}

	// 2. 构建精简矩阵和列映射
	newCols := len(nonZeroCols)
	prunedMat := NewDenseMatrix(originalRows, newCols)
	p.colMapping = make(map[int]int, newCols)

	for newCol, originalCol := range nonZeroCols {
		p.colMapping[newCol] = originalCol // 新列 → 原始列
		// 复制非零列数据
		for row := 0; row < originalRows; row++ {
			prunedMat.Set(row, newCol, p.originalMatrix.Get(row, originalCol))
		}
	}

	// 3. 缓存精简矩阵（行映射不变，清空行映射缓存）
	p.prunedMatrix = prunedMat
	p.rowMapping = make(map[int]int)
	return prunedMat
}

// Compress 组合精简（先移除零行/零列，再移除另一维度的零元素）
func (p *denseMatrixPruner) Compress(removeRowsFirst bool) Matrix {
	if p.originalMatrix == nil {
		return nil
	}

	var tempMat Matrix
	// 第一步：按指定顺序移除第一个维度的零元素
	if removeRowsFirst {
		tempMat = p.RemoveZeroRows() // 先删零行（列索引不变）
	} else {
		tempMat = p.RemoveZeroCols() // 先删零列（行索引不变）
	}

	// 第二步：基于第一步结果，移除第二个维度的零元素
	tempPruner := &denseMatrixPruner{
		originalMatrix: tempMat,
		rowMapping:     make(map[int]int),
		colMapping:     make(map[int]int),
		epsilon:        Epsilon,
	}
	var finalMat Matrix

	if removeRowsFirst {
		// 先删零行 → 再删零列
		finalMat = tempPruner.RemoveZeroCols()
		// 关键修复：tempMat的列索引 = 原始矩阵的列索引，直接复用tempPruner的列映射
		p.colMapping = tempPruner.GetColMapping() // 新列 → 原始列（正确映射）
	} else {
		// 先删零列 → 再删零行
		finalMat = tempPruner.RemoveZeroRows()
		// 复用tempPruner的行映射（tempMat的行索引 = 原始矩阵的行索引）
		p.rowMapping = tempPruner.GetRowMapping()
	}

	// 缓存最终精简矩阵
	p.prunedMatrix = finalMat
	return finalMat
}
func (p *denseMatrixPruner) GetRowMapping() map[int]int {
	return p.rowMapping
}

func (p *denseMatrixPruner) GetColMapping() map[int]int {
	return p.colMapping
}

func (p *denseMatrixPruner) GetOriginalRowIndex(newRow int) (int, bool) {
	originalRow, ok := p.rowMapping[newRow]
	return originalRow, ok
}

func (p *denseMatrixPruner) GetOriginalColIndex(newCol int) (int, bool) {
	originalCol, ok := p.colMapping[newCol]
	return originalCol, ok
}

func (p *denseMatrixPruner) GetPrunedMatrix() Matrix {
	return p.prunedMatrix
}

// SetEpsilon 自定义零值判断阈值（默认1e-16）
func (p *denseMatrixPruner) SetEpsilon(epsilon float64) {
	if epsilon > 0 {
		p.epsilon = epsilon
	}
}

func NewMatrixPruner(mat Matrix) MatrixPruner {
	return &denseMatrixPruner{
		originalMatrix: mat,
		rowMapping:     make(map[int]int),
		colMapping:     make(map[int]int),
		epsilon:        Epsilon,
	}
}

// sparseMatrixPruner 稀疏矩阵精简器（实现 MatrixPruner 接口）
type sparseMatrixPruner struct {
	originalMatrix *sparseMatrix // 原始稀疏矩阵（只读）
	prunedMatrix   Matrix        // 精简后的矩阵（缓存）
	rowMapping     map[int]int   // 精简后行索引 → 原始行索引
	colMapping     map[int]int   // 精简后列索引 → 原始列索引
	reverseColMap  map[int]int   // 原始列索引 → 精简后列索引（内部辅助）
	epsilon        float64       // 零值判断阈值（复用全局 Epsilon）
}

// NewSparseMatrixPruner 创建稀疏矩阵精简器
func NewSparseMatrixPruner(mat *sparseMatrix) MatrixPruner {
	return &sparseMatrixPruner{
		originalMatrix: mat,
		rowMapping:     make(map[int]int),
		colMapping:     make(map[int]int),
		reverseColMap:  make(map[int]int),
		epsilon:        Epsilon,
	}
}

// ------------------------------ MatrixPruner 接口实现 ------------------------------
// RemoveZeroRows 移除全零行（利用 rowPtr 高效判断，O(rows + nonZeroCount) 时间）
func (p *sparseMatrixPruner) RemoveZeroRows() Matrix {
	if p.originalMatrix == nil {
		return nil
	}

	origRows := p.originalMatrix.rows
	origCols := p.originalMatrix.cols
	var nonZeroRowIndices []int // 存储非零行的原始索引

	// 1. 筛选非零行（核心：通过 rowPtr 判断，无需遍历列）
	for row := 0; row < origRows; row++ {
		if p.originalMatrix.rowPtr[row] != p.originalMatrix.rowPtr[row+1] {
			nonZeroRowIndices = append(nonZeroRowIndices, row)
		}
	}

	// 2. 构建新的 CSR 结构
	newRows := len(nonZeroRowIndices)
	newRowPtr := make([]int, newRows+1)
	newColInd := make([]int, 0, p.originalMatrix.NonZeroCount())
	newValues := make([]float64, 0, p.originalMatrix.NonZeroCount())

	// 3. 复制非零行数据，更新 rowPtr
	dataPtr := p.originalMatrix.DataManager.DataPtr()
	currentIdx := 0
	p.rowMapping = make(map[int]int, newRows)
	for newRow, origRow := range nonZeroRowIndices {
		p.rowMapping[newRow] = origRow // 记录行映射
		newRowPtr[newRow] = currentIdx

		// 提取当前行的非零元素（利用 rowPtr 定位范围）
		start := p.originalMatrix.rowPtr[origRow]
		end := p.originalMatrix.rowPtr[origRow+1]
		newColInd = append(newColInd, p.originalMatrix.colInd[start:end]...)
		newValues = append(newValues, dataPtr[start:end]...)

		currentIdx += end - start
	}
	newRowPtr[newRows] = currentIdx

	// 4. 构建精简后的稀疏矩阵
	prunedMat := &sparseMatrix{
		rows:        newRows,
		cols:        origCols,
		rowPtr:      newRowPtr,
		colInd:      newColInd,
		DataManager: NewDataManagerWithData(newValues),
	}

	// 缓存结果，清空列映射（列未变化）
	p.prunedMatrix = prunedMat
	p.colMapping = make(map[int]int)
	p.reverseColMap = make(map[int]int)
	return prunedMat
}

// RemoveZeroCols 移除全零列（先统计列非零数，再筛选非零列）
func (p *sparseMatrixPruner) RemoveZeroCols() Matrix {
	if p.originalMatrix == nil {
		return nil
	}

	origRows := p.originalMatrix.rows
	origCols := p.originalMatrix.cols

	// 1. 统计每列的非零元素个数（O(nonZeroCount) 时间，高效）
	colNonZeroCount := make([]int, origCols)
	for _, col := range p.originalMatrix.colInd {
		colNonZeroCount[col]++
	}

	// 2. 筛选非零列，构建列映射
	var nonZeroColIndices []int // 原始非零列索引
	p.colMapping = make(map[int]int)
	p.reverseColMap = make(map[int]int)
	for newCol, origCol := range colNonZeroCount {
		if origCol > 0 {
			p.colMapping[len(nonZeroColIndices)] = newCol    // 新列→原始列
			p.reverseColMap[newCol] = len(nonZeroColIndices) // 原始列→新列
			nonZeroColIndices = append(nonZeroColIndices, newCol)
		}
	}
	newCols := len(nonZeroColIndices)
	if newCols == 0 {
		return NewSparseMatrix(origRows, 0) // 所有列都是零列
	}

	// 3. 构建新的 CSR 结构（更新 colInd 为新列索引）
	newRowPtr := make([]int, origRows+1)
	dataPtr := p.originalMatrix.DataManager.DataPtr()
	copy(newRowPtr, p.originalMatrix.rowPtr) // rowPtr 长度不变，值直接复用
	newColInd := make([]int, len(p.originalMatrix.colInd))
	newValues := make([]float64, len(dataPtr))

	// 4. 映射原始列索引到新列索引，复制非零元素
	for i, origCol := range p.originalMatrix.colInd {
		newCol, ok := p.reverseColMap[origCol]
		if !ok {
			panic(fmt.Sprintf("invalid column index: %d (should be non-zero column)", origCol))
		}
		newColInd[i] = newCol
		newValues[i] = dataPtr[i]
	}

	// 5. 构建精简后的稀疏矩阵
	prunedMat := &sparseMatrix{
		rows:        origRows,
		cols:        newCols,
		rowPtr:      newRowPtr,
		colInd:      newColInd,
		DataManager: NewDataManagerWithData(newValues),
	}

	// 缓存结果，清空行映射（行未变化）
	p.prunedMatrix = prunedMat
	p.rowMapping = make(map[int]int)
	return prunedMat
}

// Compress 组合精简（先移除零行/零列，再移除另一维度零元素，保持稀疏特性）
func (p *sparseMatrixPruner) Compress(removeRowsFirst bool) Matrix {
	if p.originalMatrix == nil {
		return nil
	}

	var tempMat *sparseMatrix
	// 第一步：按顺序移除第一个维度的零元素
	if removeRowsFirst {
		// 先删零行
		tempMat = p.RemoveZeroRows().(*sparseMatrix)
	} else {
		// 先删零列
		tempMat = p.RemoveZeroCols().(*sparseMatrix)
	}

	// 第二步：基于临时结果，移除第二个维度的零元素
	tempPruner := NewSparseMatrixPruner(tempMat)
	var finalMat *sparseMatrix
	if removeRowsFirst {
		// 再删零列（基于无零行的矩阵）
		finalMat = tempPruner.RemoveZeroCols().(*sparseMatrix)
		// 合并列映射：新列 → 临时列 → 原始列
		p.colMapping = make(map[int]int)
		for newCol, tempCol := range tempPruner.GetColMapping() {
			// 临时列是第一步删零行后的列索引（与原始列索引一致）
			originalCol := tempCol
			p.colMapping[newCol] = originalCol
		}
	} else {
		// 再删零行（基于无零列的矩阵）
		finalMat = tempPruner.RemoveZeroRows().(*sparseMatrix)
		// 合并行映射：新行 → 临时行 → 原始行
		p.rowMapping = make(map[int]int)
		for newRow, tempRow := range tempPruner.GetRowMapping() {
			// 临时行是第一步删零列后的行索引（与原始行索引一致）
			originalRow := tempRow
			p.rowMapping[newRow] = originalRow
		}
	}
	// 缓存最终结果
	p.prunedMatrix = finalMat
	return finalMat
}

// ------------------------------ 索引映射查询方法 ------------------------------
func (p *sparseMatrixPruner) GetRowMapping() map[int]int {
	return p.rowMapping
}

func (p *sparseMatrixPruner) GetColMapping() map[int]int {
	return p.colMapping
}

func (p *sparseMatrixPruner) GetOriginalRowIndex(newRow int) (int, bool) {
	origRow, ok := p.rowMapping[newRow]
	return origRow, ok
}

func (p *sparseMatrixPruner) GetOriginalColIndex(newCol int) (int, bool) {
	origCol, ok := p.colMapping[newCol]
	return origCol, ok
}

func (p *sparseMatrixPruner) GetPrunedMatrix() Matrix {
	return p.prunedMatrix
}

// SetEpsilon 自定义零值判断阈值（兼容接口）
func (p *sparseMatrixPruner) SetEpsilon(epsilon float64) {
	if epsilon > 0 {
		p.epsilon = epsilon
	}
}
