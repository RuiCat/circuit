package mat

import (
	"fmt"
	"math"
)

// LU 稀疏LU分解接口
// 定义稀疏矩阵LU分解的基本操作，支持部分主元法
type LU interface {
	// Decompose 执行稀疏LU分解（原地分解，直接修改U矩阵）
	// 参数：
	//   matrix - 待分解的稀疏矩阵
	// 返回：
	//   error - 如果矩阵奇异或接近奇异则返回错误
	Decompose(matrix SparseMatrix) error
	// SolveReuse 解线性方程组 Ax = b，重用预分配的向量
	// 参数：
	//   b - 右侧向量
	//   x - 解向量（预分配，结果将存储在此）
	// 返回：
	//   error - 如果向量维度不匹配则返回错误
	SolveReuse(b []float64, x []float64) error
}

// lu 稀疏LU分解
// 实现LU分解的数据结构，使用部分主元法提高数值稳定性
type lu struct {
	n        int          // 矩阵维度
	L        SparseMatrix // 下三角矩阵，对角线元素为1
	U        SparseMatrix // 上三角矩阵，存储分解后的上三角部分
	P        []int        // 置换向量，P[i]表示第i行原始位置
	Pinverse []int        // 逆置换向量，用于快速查找置换关系
}

// NewLU 创建稀疏LU分解，U矩阵直接引用原始矩阵
// 参数：
//
//	n - 矩阵大小
//
// 返回：
//
//	LU - 初始化后的LU分解实例
func NewLU(n int) LU {
	lu := &lu{
		n:        n,
		L:        NewSparseMatrix(n, n),
		U:        NewSparseMatrix(n, n), // 直接引用原始矩阵，避免复制
		P:        make([]int, n),
		Pinverse: make([]int, n),
	}
	return lu
}

// Decompose 执行稀疏LU分解（原地分解，直接修改U矩阵）
// 使用部分主元法进行LU分解，提高数值稳定性
// 算法步骤：
// 1. 复制原始矩阵到U矩阵
// 2. 初始化置换向量
// 3. 对每个列进行部分主元选择
// 4. 执行高斯消元，更新L和U矩阵
// 参数：
//
//	matrix - 待分解的稀疏矩阵
//
// 返回：
//
//	error - 如果矩阵奇异或接近奇异则返回错误
func (lu *lu) Decompose(matrix SparseMatrix) error {
	n := lu.n
	// 复制矩阵到U
	matrix.Copy(lu.U)
	lu.L.Clear()
	// 初始化置换向量
	for i := 0; i < n; i++ {
		lu.P[i] = i
		lu.Pinverse[i] = i
	}
	// 部分主元法LU分解（原地操作）
	for k := 0; k < n; k++ {
		// 寻找主元：在当前列中选择绝对值最大的元素作为主元
		maxRow := k
		maxVal := math.Abs(lu.U.Get(lu.P[k], k))
		for i := k + 1; i < n; i++ {
			currentVal := math.Abs(lu.U.Get(lu.P[i], k))
			if currentVal > maxVal {
				maxVal = currentVal
				maxRow = i
			}
		}
		// 检查矩阵是否奇异
		if maxVal < 1e-16 {
			return fmt.Errorf("matrix is singular or nearly singular")
		}
		// 交换行：将主元所在行交换到当前位置
		if maxRow != k {
			lu.P[k], lu.P[maxRow] = lu.P[maxRow], lu.P[k]
			// 更新逆置换
			lu.Pinverse[lu.P[k]] = k
			lu.Pinverse[lu.P[maxRow]] = maxRow
		}
		// 设置L的对角线为1
		lu.L.Set(k, k, 1.0)
		pivotRow := lu.P[k]
		pivot := lu.U.Get(pivotRow, k)
		// 计算消元因子并更新矩阵
		for i := k + 1; i < n; i++ {
			row := lu.P[i]
			factor := lu.U.Get(row, k) / pivot
			lu.L.Set(i, k, factor)
			// 更新U矩阵（原地操作）：执行高斯消元
			for j := k; j < n; j++ {
				current := lu.U.Get(row, j)
				update := factor * lu.U.Get(pivotRow, j)
				lu.U.Set(row, j, current-update)
			}
		}
	}
	return nil
}

// SolveReuse 解线性方程组 Ax = b，重用预分配的向量
// 使用LU分解结果求解线性方程组，分为两个步骤：
// 1. 前向替换：求解 Ly = Pb
// 2. 后向替换：求解 Ux = y
// 参数：
//
//	b - 右侧向量
//	x - 解向量（预分配，结果将存储在此）
//
// 返回：
//
//	error - 如果向量维度不匹配则返回错误
func (lu *lu) SolveReuse(b, x []float64) error {
	if len(b) != lu.n || len(x) != lu.n {
		return fmt.Errorf("vector dimension mismatch")
	}
	// 应用置换: Pb = P * b
	pb := make([]float64, lu.n)
	for i := 0; i < lu.n; i++ {
		pb[i] = b[lu.P[i]]
	}
	// 前向替换: Ly = Pb
	y := make([]float64, lu.n)
	for i := 0; i < lu.n; i++ {
		sum := pb[i]
		for j := 0; j < i; j++ {
			sum -= lu.L.Get(i, j) * y[j]
		}
		y[i] = sum // L[i,i] = 1
	}
	// 后向替换: Ux = y
	for i := lu.n - 1; i >= 0; i-- {
		sum := y[i]
		uRow := lu.P[i]
		for j := i + 1; j < lu.n; j++ {
			sum -= lu.U.Get(uRow, j) * x[j]
		}
		x[i] = sum / lu.U.Get(uRow, i)
	}
	return nil
}
