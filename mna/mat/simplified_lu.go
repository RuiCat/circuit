package mat

import (
	"fmt"
)

// RowInfo 行信息，用于矩阵简化
// 类似于Java中的RowInfo类，跟踪每行的状态
type RowInfo struct {
	Type      int     // 行类型：ROW_NORMAL, ROW_CONST
	MapCol    int     // 列映射
	MapRow    int     // 行映射
	Value     float64 // 常数值
	LSChanges bool    // 左侧变化
	RSChanges bool    // 右侧变化
	DropRow   bool    // 删除行
}

const (
	ROW_NORMAL = 0 // 普通行
	ROW_CONST  = 1 // 常数行
)

// MatrixSimplifier 矩阵简化器接口
// 定义矩阵简化的核心操作，与电路模拟器解耦
type MatrixSimplifier interface {
	// Simplify 简化矩阵，返回简化后的矩阵和右侧向量
	Simplify(matrix Matrix, rightSide []float64) (Matrix, []float64, error)

	// ApplySolution 将简化系统的解映射回原始系统
	ApplySolution(simplifiedSolution []float64, originalSolution []float64) error

	// GetSimplifiedSize 获取简化后的矩阵大小
	GetSimplifiedSize() int

	// SetRowChanges 设置行的变化状态
	SetRowChanges(row int, lsChanges, rsChanges, dropRow bool)

	// GetRowInfo 获取行信息
	GetRowInfo(row int) RowInfo
}

// SimplifiedLU 简化的LU分解结构体
// 封装原有的lu结构体，添加矩阵简化功能，与电路模拟器解耦
type SimplifiedLU struct {
	lu               LU               // 原有的LU分解器
	n                int              // 矩阵维度
	circuitMatrix    Matrix           // 原始电路矩阵
	circuitRightSide []float64        // 原始右侧向量
	rowInfo          []RowInfo        // 行信息数组
	permute          []int            // 置换向量
	needsMap         bool             // 是否需要映射
	simplifier       MatrixSimplifier // 矩阵简化器

	// 预分配的内存，用于优化性能
	simplifiedB       []float64 // 预分配的简化b向量
	simplifiedX       []float64 // 预分配的简化x向量
	simplifiedBVec    Vector    // 预分配的简化b向量对象
	simplifiedXVec    Vector    // 预分配的简化x向量对象
	maxSimplifiedSize int       // 最大简化大小
}

// NewSimplifiedLU 创建简化的LU分解器
func NewSimplifiedLU(n int) *SimplifiedLU {
	// 预分配内存，假设简化后的大小最多为原始大小的80%
	maxSimplifiedSize := int(float64(n) * 0.8)
	if maxSimplifiedSize < 1 {
		maxSimplifiedSize = 1
	}

	return &SimplifiedLU{
		lu:       NewLU(n),
		n:        n,
		rowInfo:  make([]RowInfo, n),
		permute:  make([]int, n),
		needsMap: false,
		simplifier: &DefaultMatrixSimplifier{
			n:       n,
			rowInfo: make([]RowInfo, n),
		},
		// 预分配内存
		simplifiedB:       make([]float64, maxSimplifiedSize),
		simplifiedX:       make([]float64, maxSimplifiedSize),
		simplifiedBVec:    NewDenseVector(maxSimplifiedSize),
		simplifiedXVec:    NewDenseVector(maxSimplifiedSize),
		maxSimplifiedSize: maxSimplifiedSize,
	}
}

// DefaultMatrixSimplifier 默认矩阵简化器实现
type DefaultMatrixSimplifier struct {
	n       int
	rowInfo []RowInfo
}

// Simplify 简化矩阵
func (dms *DefaultMatrixSimplifier) Simplify(matrix Matrix, rightSide []float64) (Matrix, []float64, error) {
	matrixSize := dms.n

	// 初始化行信息
	for i := 0; i < matrixSize; i++ {
		dms.rowInfo[i] = RowInfo{Type: ROW_NORMAL}
	}

	// 执行矩阵简化
	for i := 0; i < matrixSize; i++ {
		qp := -1
		qv := 0.0
		re := dms.rowInfo[i]

		// 如果行有变化或被标记删除，跳过
		if re.LSChanges || re.DropRow || re.RSChanges {
			continue
		}

		rsadd := 0.0

		// 寻找可以移除的行
		for j := 0; j < matrixSize; j++ {
			q := matrix.Get(i, j)

			// 如果该列已经是常数，累加到右侧
			if dms.rowInfo[j].Type == ROW_CONST {
				rsadd -= dms.rowInfo[j].Value * q
				continue
			}

			// 忽略零元素
			if q == 0 {
				continue
			}

			// 跟踪第一个非零非常数元素
			if qp == -1 {
				qp = j
				qv = q
				continue
			}

			// 超过一个非零元素，放弃
			break
		}

		// 如果找到只有一个非零非常数元素的行
		if qp != -1 {
			// 将该元素标记为常数
			if dms.rowInfo[qp].Type != ROW_NORMAL {
				fmt.Printf("type already %d for %d!\n", dms.rowInfo[qp].Type, qp)
				continue
			}
			dms.rowInfo[qp].Type = ROW_CONST
			dms.rowInfo[qp].Value = (rightSide[i] + rsadd) / qv
			dms.rowInfo[i].DropRow = true
			// 重新开始简化过程
			i = -1
		}
	}

	// 计算新矩阵的大小
	nn := 0
	for i := 0; i < matrixSize; i++ {
		elt := dms.rowInfo[i]
		if elt.Type == ROW_NORMAL {
			elt.MapCol = nn
			nn++
			continue
		}
		if elt.Type == ROW_CONST {
			elt.MapCol = -1
		}
	}

	// 创建新的简化矩阵
	newsize := nn
	newmatx := NewSparseMatrix(newsize, newsize)
	newrs := make([]float64, newsize)

	ii := 0
	for i := 0; i < matrixSize; i++ {
		rri := dms.rowInfo[i]
		if rri.DropRow {
			rri.MapRow = -1
			continue
		}
		newrs[ii] = rightSide[i]
		rri.MapRow = ii

		for j := 0; j < matrixSize; j++ {
			ri := dms.rowInfo[j]
			if ri.Type == ROW_CONST {
				newrs[ii] -= ri.Value * matrix.Get(i, j)
			} else {
				current := newmatx.Get(ii, ri.MapCol)
				newmatx.Set(ii, ri.MapCol, current+matrix.Get(i, j))
			}
		}
		ii++
	}

	return newmatx, newrs, nil
}

// ApplySolution 将简化系统的解映射回原始系统
func (dms *DefaultMatrixSimplifier) ApplySolution(simplifiedSolution []float64, originalSolution []float64) error {
	for j := 0; j < dms.n; j++ {
		ri := dms.rowInfo[j]
		res := 0.0

		if ri.Type == ROW_CONST {
			res = ri.Value
		} else {
			res = simplifiedSolution[ri.MapCol]
		}

		// 将结果设置到原始解向量
		if j < len(originalSolution) {
			originalSolution[j] = res
		}
	}
	return nil
}

// GetSimplifiedSize 获取简化后的矩阵大小
func (dms *DefaultMatrixSimplifier) GetSimplifiedSize() int {
	count := 0
	for i := 0; i < dms.n; i++ {
		if dms.rowInfo[i].Type == ROW_NORMAL && !dms.rowInfo[i].DropRow {
			count++
		}
	}
	return count
}

// SetRowChanges 设置行的变化状态
func (dms *DefaultMatrixSimplifier) SetRowChanges(row int, lsChanges, rsChanges, dropRow bool) {
	if row >= 0 && row < dms.n {
		dms.rowInfo[row].LSChanges = lsChanges
		dms.rowInfo[row].RSChanges = rsChanges
		dms.rowInfo[row].DropRow = dropRow
	}
}

// GetRowInfo 获取行信息
func (dms *DefaultMatrixSimplifier) GetRowInfo(row int) RowInfo {
	if row >= 0 && row < dms.n {
		return dms.rowInfo[row]
	}
	return RowInfo{}
}

// Decompose 执行LU分解，可选择是否简化矩阵
func (slu *SimplifiedLU) Decompose(matrix Matrix, simplify bool) error {
	slu.circuitMatrix = matrix

	if simplify {
		// 创建临时的右侧向量用于简化
		tempRightSide := make([]float64, slu.n)
		simplifiedMatrix, simplifiedRightSide, err := slu.simplifier.Simplify(matrix, tempRightSide)
		if err != nil {
			return err
		}

		// 保存简化后的矩阵和右侧向量
		slu.circuitMatrix = simplifiedMatrix
		slu.circuitRightSide = simplifiedRightSide
		slu.needsMap = true

		// 对简化后的矩阵进行LU分解
		return slu.lu.Decompose(simplifiedMatrix)
	}

	// 不简化，直接分解
	slu.needsMap = false
	return slu.lu.Decompose(matrix)
}

// SolveReuse 求解线性方程组
func (slu *SimplifiedLU) SolveReuse(b, x Vector) error {
	if slu.needsMap {
		simplifiedSize := slu.simplifier.GetSimplifiedSize()

		// 检查是否需要重新分配内存
		if simplifiedSize > slu.maxSimplifiedSize {
			// 需要重新分配更大的内存
			slu.maxSimplifiedSize = simplifiedSize
			slu.simplifiedB = make([]float64, simplifiedSize)
			slu.simplifiedX = make([]float64, simplifiedSize)
			slu.simplifiedBVec = NewDenseVector(simplifiedSize)
			slu.simplifiedXVec = NewDenseVector(simplifiedSize)
		}

		// 使用预分配的内存
		simplifiedB := slu.simplifiedB[:simplifiedSize]
		simplifiedX := slu.simplifiedX[:simplifiedSize]

		// 清空预分配的内存
		clear(simplifiedB)
		clear(simplifiedX)

		// 将b映射到简化空间
		// 正确的映射逻辑：考虑行信息中的映射关系
		simplifiedIndex := 0
		for i := 0; i < slu.n; i++ {
			ri := slu.simplifier.GetRowInfo(i)
			// 只有普通行且未被删除的行才需要映射到简化空间
			if ri.Type == ROW_NORMAL && !ri.DropRow {
				if simplifiedIndex < len(simplifiedB) && i < b.Length() {
					simplifiedB[simplifiedIndex] = b.Get(i)
					simplifiedIndex++
				}
			}
		}
		// 检查映射是否完整
		if simplifiedIndex != len(simplifiedB) {
			return fmt.Errorf("b vector mapping incomplete: expected %d, got %d", len(simplifiedB), simplifiedIndex)
		}

		// 使用预分配的向量对象
		simplifiedBVec := slu.simplifiedBVec
		simplifiedXVec := slu.simplifiedXVec

		// 使用BuildFromDense方法设置数据
		simplifiedBVec.BuildFromDense(simplifiedB)
		simplifiedXVec.BuildFromDense(simplifiedX)

		// 在简化空间求解
		err := slu.lu.SolveReuse(simplifiedBVec, simplifiedXVec)
		if err != nil {
			return err
		}

		// 将解映射回原始空间
		originalX := make([]float64, slu.n)
		err = slu.simplifier.ApplySolution(simplifiedX, originalX)
		if err != nil {
			return err
		}

		// 设置结果
		for i := 0; i < len(originalX) && i < x.Length(); i++ {
			x.Set(i, originalX[i])
		}

		return nil
	}

	// 不简化，直接求解
	return slu.lu.SolveReuse(b, x)
}

// SolveReuseFloat 求解线性方程组（float数组版本）
func (slu *SimplifiedLU) SolveReuseFloat(b, x []float64) error {
	if slu.needsMap {
		simplifiedSize := slu.simplifier.GetSimplifiedSize()

		// 检查是否需要重新分配内存
		if simplifiedSize > slu.maxSimplifiedSize {
			// 需要重新分配更大的内存
			slu.maxSimplifiedSize = simplifiedSize
			slu.simplifiedB = make([]float64, simplifiedSize)
			slu.simplifiedX = make([]float64, simplifiedSize)
			slu.simplifiedBVec = NewDenseVector(simplifiedSize)
			slu.simplifiedXVec = NewDenseVector(simplifiedSize)
		}

		// 使用预分配的内存
		simplifiedB := slu.simplifiedB[:simplifiedSize]
		simplifiedX := slu.simplifiedX[:simplifiedSize]

		// 清空预分配的内存
		clear(simplifiedB)
		clear(simplifiedX)

		// 将b映射到简化空间
		// 正确的映射逻辑：考虑行信息中的映射关系
		simplifiedIndex := 0
		for i := 0; i < slu.n; i++ {
			ri := slu.simplifier.GetRowInfo(i)
			// 只有普通行且未被删除的行才需要映射到简化空间
			if ri.Type == ROW_NORMAL && !ri.DropRow {
				if simplifiedIndex < len(simplifiedB) && i < len(b) {
					simplifiedB[simplifiedIndex] = b[i]
					simplifiedIndex++
				}
			}
		}
		// 检查映射是否完整
		if simplifiedIndex != len(simplifiedB) {
			return fmt.Errorf("b vector mapping incomplete: expected %d, got %d", len(simplifiedB), simplifiedIndex)
		}

		// 在简化空间求解
		err := slu.lu.SolveReuseFloat(simplifiedB, simplifiedX)
		if err != nil {
			return err
		}

		// 将解映射回原始空间
		return slu.simplifier.ApplySolution(simplifiedX, x)
	}

	// 不简化，直接求解
	return slu.lu.SolveReuseFloat(b, x)
}

// SetRowChanges 设置行的变化状态
// 通过接口开放优化参数，允许外部设置行的变化状态
func (slu *SimplifiedLU) SetRowChanges(row int, lsChanges, rsChanges, dropRow bool) {
	slu.simplifier.SetRowChanges(row, lsChanges, rsChanges, dropRow)
}

// GetRowInfo 获取行信息
func (slu *SimplifiedLU) GetRowInfo(row int) RowInfo {
	return slu.simplifier.GetRowInfo(row)
}

// GetSimplifiedSize 获取简化后的矩阵大小
func (slu *SimplifiedLU) GetSimplifiedSize() int {
	if slu.needsMap {
		return slu.simplifier.GetSimplifiedSize()
	}
	return slu.n
}

// SetSimplifier 设置矩阵简化器
// 允许使用自定义的矩阵简化器实现
func (slu *SimplifiedLU) SetSimplifier(simplifier MatrixSimplifier) {
	slu.simplifier = simplifier
}
