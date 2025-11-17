package maths

import (
	"fmt"
)

// defaultMatrixReducer 默认矩阵简化器实现
// 基于数学原理的矩阵简化，专注于识别线性相关行和常数行
type defaultMatrixReducer struct {
	n           int             // 矩阵大小
	rowInfo     []RowInfo       // 行信息
	rowMapping  []int           // 行映射：简化行 -> 原始行
	colMapping  []int           // 列映射：简化列 -> 原始列
	constantMap map[int]float64 // 常数项映射
}

// NewDefaultMatrixReducer 创建新的默认矩阵简化器
func NewDefaultMatrixReducer(n int) MatrixReducer {
	return &defaultMatrixReducer{
		n:           n,
		rowInfo:     make([]RowInfo, n),
		rowMapping:  make([]int, 0, n),
		colMapping:  make([]int, 0, n),
		constantMap: make(map[int]float64),
	}
}

// Simplify 简化矩阵
// 数学原理：通过识别线性相关行和常数行来减少矩阵规模
func (dmr *defaultMatrixReducer) Simplify(matrix Matrix, rightSide Vector) (Matrix, Vector, error) {
	// 初始化行信息
	for i := 0; i < dmr.n; i++ {
		dmr.rowInfo[i] = RowInfo{Type: ROW_NORMAL}
	}

	// 分析矩阵结构
	err := dmr.analyzeMatrixStructure(matrix, rightSide)
	if err != nil {
		return nil, nil, err
	}

	// 创建简化矩阵
	reducedMatrix := dmr.createReducedMatrix(matrix)

	// 创建简化右侧向量
	reducedRightSide := dmr.createReducedRightSide(matrix, rightSide)

	return reducedMatrix, reducedRightSide, nil
}

// analyzeMatrixStructure 分析矩阵结构
// 数学原理：识别可简化的行（只有一个非零元素的行）
func (dmr *defaultMatrixReducer) analyzeMatrixStructure(matrix Matrix, rightSide Vector) error {
	processedCols := make([]bool, dmr.n)

	// 分析每一行
	for i := 0; i < dmr.n; i++ {
		if dmr.isRowReducible(matrix, i, processedCols) {
			// 该行是常数行
			dmr.rowInfo[i].Type = ROW_CONST
			dmr.constantMap[i] = dmr.extractConstantValue(matrix, i, rightSide)
		} else {
			// 该行需要保留
			dmr.rowMapping = append(dmr.rowMapping, i)
			dmr.rowInfo[i].MapRow = len(dmr.rowMapping) - 1

			// 标记该行中的非零列为需要保留的列
			for j := 0; j < dmr.n; j++ {
				if matrix.Get(i, j) != 0 && !processedCols[j] {
					dmr.colMapping = append(dmr.colMapping, j)
					dmr.rowInfo[j].MapCol = len(dmr.colMapping) - 1
					processedCols[j] = true
				}
			}
		}
	}

	// 检查简化后的矩阵是否有效
	if len(dmr.rowMapping) == 0 {
		return fmt.Errorf("matrix reduces to zero size")
	}

	return nil
}

// isRowReducible 检查行是否可以被简化
// 数学原理：如果一行只有一个非零元素，则该行可以表示为常数
func (dmr *defaultMatrixReducer) isRowReducible(matrix Matrix, row int, processedCols []bool) bool {
	nonZeroCount := 0

	for j := 0; j < dmr.n; j++ {
		if matrix.Get(row, j) != 0 {
			nonZeroCount++
			// 如果该列已经被处理过，不能简化
			if processedCols[j] {
				return false
			}
		}
	}

	// 如果只有一个非零元素且该列未被处理，则可以简化
	return nonZeroCount == 1
}

// extractConstantValue 提取常数行的常数值
// 数学原理：对于只有一个非零元素的行，该行的解等于右侧向量值除以该非零元素
func (dmr *defaultMatrixReducer) extractConstantValue(matrix Matrix, row int, rightSide Vector) float64 {
	// 寻找该行中唯一的非零元素
	for j := 0; j < dmr.n; j++ {
		if matrix.Get(row, j) != 0 {
			// 使用右侧向量的值计算常数项
			// 数学原理：对于方程 a_ij * x_j = b_i，解为 x_j = b_i / a_ij
			return rightSide.Get(row) / matrix.Get(row, j)
		}
	}
	return 0.0
}

// createReducedMatrix 创建简化后的矩阵
func (dmr *defaultMatrixReducer) createReducedMatrix(matrix Matrix) Matrix {
	reducedSize := len(dmr.rowMapping)
	reducedMatrix := NewDenseMatrix(reducedSize, reducedSize)

	for i, origRow := range dmr.rowMapping {
		for j, origCol := range dmr.colMapping {
			reducedMatrix.Set(i, j, matrix.Get(origRow, origCol))
		}
	}

	return reducedMatrix
}

// createReducedRightSide 创建简化后的右侧向量
func (dmr *defaultMatrixReducer) createReducedRightSide(matrix Matrix, rightSide Vector) Vector {
	reducedSize := len(dmr.rowMapping)
	reducedRightSide := NewDenseVector(reducedSize)

	for i, origRow := range dmr.rowMapping {
		// 调整右侧向量，减去常数项的影响
		adjustedValue := rightSide.Get(origRow)
		for constRow, constValue := range dmr.constantMap {
			adjustedValue -= matrix.Get(origRow, constRow) * constValue
		}
		reducedRightSide.Set(i, adjustedValue)
	}

	return reducedRightSide
}

// ApplySolution 将简化系统的解映射回原始系统
func (dmr *defaultMatrixReducer) ApplySolution(simplifiedSolution Vector, originalSolution Vector) error {
	simplifiedArray := simplifiedSolution.ToDense()

	for j := 0; j < dmr.n; j++ {
		res := 0.0

		if dmr.rowInfo[j].Type == ROW_CONST {
			// 常数行使用预先计算的常数值
			res = dmr.constantMap[j]
		} else {
			// 非常数行使用简化系统的解
			res = simplifiedArray[dmr.rowInfo[j].MapCol]
		}

		// 将结果设置到原始解向量
		if j < originalSolution.Length() {
			originalSolution.Set(j, res)
		}
	}

	return nil
}

// GetSimplifiedSize 获取简化后的矩阵大小
func (dmr *defaultMatrixReducer) GetSimplifiedSize() int {
	return len(dmr.rowMapping)
}

// SetRowChanges 设置行的变化状态
func (dmr *defaultMatrixReducer) SetRowChanges(row int, lsChanges, rsChanges, dropRow bool) {
	if row >= 0 && row < dmr.n {
		dmr.rowInfo[row].LSChanges = lsChanges
		dmr.rowInfo[row].RSChanges = rsChanges
		dmr.rowInfo[row].DropRow = dropRow
	}
}

// GetRowInfo 获取行信息
func (dmr *defaultMatrixReducer) GetRowInfo(row int) RowInfo {
	if row >= 0 && row < dmr.n {
		return dmr.rowInfo[row]
	}
	return RowInfo{}
}
