package mat

import (
	"fmt"
	"math"
)

// LU 稀疏LU分解
type LU struct {
	n        int           // 矩阵维度
	L        *SparseMatrix // 下三角矩阵
	U        *SparseMatrix // 上三角矩阵（直接引用原始矩阵）
	P        []int         // 置换向量，P[i]表示第i行原始位置
	Pinverse []int         // 逆置换向量
}

// NewLU 创建稀疏LU分解，U矩阵直接引用原始矩阵
func NewLU(matrix *SparseMatrix) *LU {
	n := matrix.Rows()
	lu := &LU{
		n:        n,
		L:        NewSparseMatrix(n, n),
		U:        NewSparseMatrix(n, n), // 直接引用原始矩阵，避免复制
		P:        make([]int, n),
		Pinverse: make([]int, n),
	}
	return lu
}

// Decompose 执行稀疏LU分解（原地分解，直接修改U矩阵）
func (lu *LU) Decompose(matrix *SparseMatrix) error {
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
		// 寻找主元
		maxRow := k
		maxVal := math.Abs(lu.U.Get(lu.P[k], k))
		for i := k + 1; i < n; i++ {
			currentVal := math.Abs(lu.U.Get(lu.P[i], k))
			if currentVal > maxVal {
				maxVal = currentVal
				maxRow = i
			}
		}
		if maxVal < 1e-12 {
			return fmt.Errorf("matrix is singular or nearly singular")
		}
		// 交换行
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
			// 更新U矩阵（原地操作）
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
func (lu *LU) SolveReuse(b, x []float64) error {
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
