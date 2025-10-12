package mat

import (
	"fmt"
	"math"
)

// LUDecomposition LU分解结果
type LUDecomposition struct {
	L *SparseMatrix // 下三角矩阵
	U *SparseMatrix // 上三角矩阵
	P *SparseMatrix // 置换矩阵（用于部分主元法）
}

// LUResult LU分解结果接口
type LUResult interface {
	Solve(b []float64) []float64
	Update(row, col int, delta float64) error
	GetL() *SparseMatrix
	GetU() *SparseMatrix
}

// IncrementalLU 支持增量更新的LU分解实现
type IncrementalLU struct {
	original *SparseMatrix // 原始矩阵
	lu       *LUDecomposition

	dirty bool // 标记是否需要重新计算LU分解
}

// Update 增量更新记录
type Update struct {
	Row   int
	Col   int
	Delta float64
}

// NewIncrementalLU 创建支持增量更新的LU分解
func NewIncrementalLU(matrix *SparseMatrix) (*IncrementalLU, error) {
	if !matrix.IsSquare() {
		return nil, fmt.Errorf("matrix must be square for LU decomposition")
	}
	lu := &LUDecomposition{}
	if err := lu.LUDecompose(matrix); err != nil {
		return nil, err
	}
	return &IncrementalLU{
		original: matrix,
		lu:       lu,
	}, nil
}

// LUDecompose 执行LU分解，支持对象复用
func (reuse *LUDecomposition) LUDecompose(matrix *SparseMatrix) error {
	if !matrix.IsSquare() {
		return fmt.Errorf("matrix must be square for LU decomposition")
	}
	n := matrix.Rows()

	// 复用现有对象或创建新对象
	if reuse != nil && reuse.L != nil && reuse.L.Rows() == n {
		// 重置矩阵为零 - 使用Clear方法
		reuse.L.Clear()
		reuse.U.Clear()
		reuse.P.Clear()
	} else {
		// 创建新矩阵对象
		reuse.L = NewSparseMatrix(n, n)
		reuse.U = NewSparseMatrix(n, n)
		reuse.P = NewSparseMatrix(n, n)
	}

	// 复制输入矩阵到U
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			reuse.U.Set(i, j, matrix.Get(i, j))
		}
	}

	// 初始化置换矩阵为单位矩阵
	for i := 0; i < n; i++ {
		reuse.P.Set(i, i, 1)
	}

	// 部分主元法LU分解
	for k := 0; k < n; k++ {
		// 寻找主元
		maxRow := k
		maxVal := math.Abs(reuse.U.Get(k, k))
		for i := k + 1; i < n; i++ {
			currentVal := math.Abs(reuse.U.Get(i, k))
			if currentVal > maxVal {
				maxVal = currentVal
				maxRow = i
			}
		}

		// 如果主元太小，矩阵可能奇异
		if maxVal < 1e-12 {
			return fmt.Errorf("matrix is singular or nearly singular")
		}

		// 交换行
		if maxRow != k {
			// 交换U的行
			for j := k; j < n; j++ {
				temp := reuse.U.Get(k, j)
				reuse.U.Set(k, j, reuse.U.Get(maxRow, j))
				reuse.U.Set(maxRow, j, temp)
			}
			// 交换L的行（只交换已计算的部分）
			for j := 0; j < k; j++ {
				temp := reuse.L.Get(k, j)
				reuse.L.Set(k, j, reuse.L.Get(maxRow, j))
				reuse.L.Set(maxRow, j, temp)
			}
			// 更新置换矩阵
			for j := 0; j < n; j++ {
				temp := reuse.P.Get(k, j)
				reuse.P.Set(k, j, reuse.P.Get(maxRow, j))
				reuse.P.Set(maxRow, j, temp)
			}
		}

		// 设置L的对角线为1
		reuse.L.Set(k, k, 1)

		// 计算U和L的元素
		for i := k + 1; i < n; i++ {
			factor := reuse.U.Get(i, k) / reuse.U.Get(k, k)
			reuse.L.Set(i, k, factor)

			for j := k; j < n; j++ {
				newVal := reuse.U.Get(i, j) - factor*reuse.U.Get(k, j)
				reuse.U.Set(i, j, newVal)
			}
		}
	}

	return nil
}

// RecomputeLU 手动重新计算LU分解
func (ilu *IncrementalLU) RecomputeLU() error {
	// 使用对象复用重新计算LU分解
	if err := ilu.lu.LUDecompose(ilu.original); err != nil {
		return err
	}
	ilu.dirty = false
	return nil
}

// Solve 解线性方程组 Ax = b
func (ilu *IncrementalLU) Solve(b []float64) ([]float64, error) {
	if len(b) != ilu.original.Rows() {
		return nil, fmt.Errorf("vector b length must match matrix dimension")
	}
	// 重新分解
	ilu.RecomputeLU()
	// 应用置换矩阵：Pb = P * b
	pb := make([]float64, len(b))
	for i := 0; i < len(b); i++ {
		sum := 0.0
		for j := 0; j < len(b); j++ {
			sum += ilu.lu.P.Get(i, j) * b[j]
		}
		pb[i] = sum
	}

	// 前向替换：Ly = Pb
	y := make([]float64, len(b))
	for i := 0; i < len(b); i++ {
		sum := pb[i]
		for j := 0; j < i; j++ {
			sum -= ilu.lu.L.Get(i, j) * y[j]
		}
		y[i] = sum / ilu.lu.L.Get(i, i)
	}

	// 后向替换：Ux = y
	x := make([]float64, len(b))
	for i := len(b) - 1; i >= 0; i-- {
		sum := y[i]
		for j := i + 1; j < len(b); j++ {
			sum -= ilu.lu.U.Get(i, j) * x[j]
		}
		x[i] = sum / ilu.lu.U.Get(i, i)
	}

	return x, nil
}

// GetL 获取L矩阵
func (ilu *IncrementalLU) GetL() *SparseMatrix {
	return ilu.lu.L
}

// GetU 获取U矩阵
func (ilu *IncrementalLU) GetU() *SparseMatrix {
	return ilu.lu.U
}

// GetP 获取置换矩阵
func (ilu *IncrementalLU) GetP() *SparseMatrix {
	return ilu.lu.P
}

// GetOriginal 获取原始矩阵（包含所有更新）
func (ilu *IncrementalLU) GetOriginal() *SparseMatrix {
	return ilu.original
}

// VerifyDecomposition 验证LU分解的正确性
func (ilu *IncrementalLU) VerifyDecomposition() error {
	n := ilu.original.Rows()

	// 计算 P * A
	PA := NewSparseMatrix(n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			sum := 0.0
			for k := 0; k < n; k++ {
				sum += ilu.lu.P.Get(i, k) * ilu.original.Get(k, j)
			}
			PA.Set(i, j, sum)
		}
	}

	// 计算 L * U
	LU := NewSparseMatrix(n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			sum := 0.0
			for k := 0; k < n; k++ {
				sum += ilu.lu.L.Get(i, k) * ilu.lu.U.Get(k, j)
			}
			LU.Set(i, j, sum)
		}
	}

	// 比较 P*A 和 L*U
	tolerance := 1e-10
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			diff := math.Abs(PA.Get(i, j) - LU.Get(i, j))
			if diff > tolerance {
				return fmt.Errorf("LU decomposition verification failed at (%d,%d): diff = %e", i, j, diff)
			}
		}
	}

	return nil
}
