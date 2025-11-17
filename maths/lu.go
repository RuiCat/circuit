package maths

import (
	"errors"
	"math"
)

// luDecomposition LU分解实现
// 基于部分主元法的稀疏LU分解，专注于数学实现
type luDecomposition struct {
	n        int    // 矩阵维度
	L        Matrix // 下三角矩阵，对角线元素为1
	U        Matrix // 上三角矩阵，存储分解后的上三角部分
	Y        Vector // 求解时中间使用变量
	P        []int  // 置换向量，P[i]表示第i行原始位置
	Pinverse []int  // 逆置换向量，用于快速查找置换关系
}

// NewLUDecomposition 创建稀疏LU分解器
func NewLUDecomposition(n int) LU {
	return &luDecomposition{
		n:        n,
		L:        NewDenseMatrix(n, n),
		U:        NewDenseMatrix(n, n),
		Y:        NewDenseVector(n),
		P:        make([]int, n),
		Pinverse: make([]int, n),
	}
}

// Decompose 执行稀疏LU分解（原地分解，直接修改U矩阵）
// 使用部分主元法进行LU分解，提高数值稳定性
func (lu *luDecomposition) Decompose(matrix Matrix) error {
	if !matrix.IsSquare() {
		return errors.New("matrix must be square for LU decomposition")
	}

	n := lu.n

	// 复制矩阵到U
	matrix.Copy(lu.U)

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
			return errors.New("matrix is singular or nearly singular")
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
func (lu *luDecomposition) SolveReuse(b, x Vector) error {
	if b.Length() != lu.n || x.Length() != lu.n {
		return errors.New("vector dimension mismatch")
	}

	// 求解 Ly = Pb
	for i := 0; i < lu.n; i++ {
		sum := b.Get(lu.P[i])
		for j := 0; j < i; j++ {
			sum -= lu.L.Get(i, j) * lu.Y.Get(j)
		}
		lu.Y.Set(i, sum)
	}

	// 求解 Ux = y
	for i := lu.n - 1; i >= 0; i-- {
		sum := lu.Y.Get(i)
		uRow := lu.P[i]
		for j := i + 1; j < lu.n; j++ {
			sum -= lu.U.Get(uRow, j) * x.Get(j)
		}
		x.Set(i, sum/lu.U.Get(uRow, i))
	}

	return nil
}
