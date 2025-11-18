package maths

import (
	"errors"
	"math"
)

// lu 带部分主元的LU分解实现（A=PLU）
// 数学原理：将方阵A分解为置换矩阵P、下三角矩阵L（对角线=1）、上三角矩阵U的乘积
// 部分主元法：每步选择列中绝对值最大元素作为主元，提升数值稳定性，避免奇异矩阵报错
type lu struct {
	N        int    // 矩阵维度（方阵）
	L        Matrix // 下三角矩阵（L[i][i]=1）
	U        Matrix // 上三角矩阵
	Y        Vector // 中间变量：存储Ly=Pb的解
	P        []int  // 置换向量：P[i] = 第i行对应的原始矩阵行索引
	Pinverse []int  // 逆置换向量：Pinverse[i] = 原始第i行对应的分解后行索引
}

// Decompose 执行LU分解
func (lu *lu) Decompose(matrix Matrix) error {
	// 检查输入矩阵是否为方阵
	if !matrix.IsSquare() {
		return errors.New("lu decomposition: input matrix must be square")
	}
	n := lu.N
	// 检查矩阵维度与分解器匹配
	if matrix.Rows() != n {
		return errors.New("lu decomposition: matrix dimension mismatch with decomposer")
	}
	// 复制输入矩阵到U（L初始为单位下三角矩阵）
	matrix.Copy(lu.U)
	// 初始化置换向量（初始为单位置换：P[i]=i）
	for i := 0; i < n; i++ {
		lu.P[i] = i
		lu.Pinverse[i] = i
		lu.L.Set(i, i, 1.0) // L对角线置1（下三角矩阵特性）
	}
	// 高斯消元+部分主元（按列遍历主元）
	for k := 0; k < n; k++ {
		// 步骤1：选择主元（当前列k，行k~n-1中绝对值最大的元素）
		maxRow := k
		maxVal := math.Abs(lu.U.Get(lu.P[k], k))
		for i := k + 1; i < n; i++ {
			currentVal := math.Abs(lu.U.Get(lu.P[i], k))
			if currentVal > maxVal {
				maxVal = currentVal
				maxRow = i
			}
		}
		// 检查矩阵奇异性（主元接近零则无法分解）
		if maxVal < 1e-16 {
			return errors.New("lu decomposition: matrix is singular or nearly singular (pivot is zero)")
		}
		// 步骤2：交换置换向量中的行（实现主元行交换，避免实际移动矩阵行）
		if maxRow != k {
			lu.P[k], lu.P[maxRow] = lu.P[maxRow], lu.P[k]
			lu.Pinverse[lu.P[k]] = k
			lu.Pinverse[lu.P[maxRow]] = maxRow
		}
		// 步骤3：计算L的下三角元素（消元因子）并更新U
		pivotRow := lu.P[k]               // 主元行（原始矩阵行索引）
		pivotVal := lu.U.Get(pivotRow, k) // 主元值（U[pivotRow][k]）
		// 遍历当前列k下方的行（i > k）
		for i := k + 1; i < n; i++ {
			row := lu.P[i] // 当前处理行（原始矩阵行索引）
			// 计算消元因子：L[i][k] = U[row][k] / pivotVal
			factor := lu.U.Get(row, k) / pivotVal
			lu.L.Set(i, k, factor) // 存入L矩阵
			// 高斯消元：更新U矩阵当前行 → U[row][j] -= factor * U[pivotRow][j]（j >= k）
			// 稀疏优化：仅遍历U[pivotRow]的非零列（减少无效计算）
			pivotCols, _ := lu.U.GetRow(pivotRow)
			for _, j := range pivotCols {
				if j >= k { // 仅更新主元列及右侧元素（左侧已为零）
					origVal := lu.U.Get(row, j)
					newVal := origVal - factor*lu.U.Get(pivotRow, j)
					lu.U.Set(row, j, newVal)
				}
			}
		}
	}
	return nil
}

// SolveReuse 利用LU分解结果求解线性方程组Ax=b（重用预分配向量，提升效率）
// 数学步骤：1. 前向替换求解Ly=Pb；2. 后向替换求解Ux=y
func (lu *lu) SolveReuse(b, x Vector) error {
	// 检查向量维度与分解器匹配
	if b.Length() != lu.N || x.Length() != lu.N {
		return errors.New("lu solve: vector dimension mismatch with decomposer")
	}
	// 处理分解
	n := lu.N
	// 步骤1：前向替换求解Ly=Pb（P为置换矩阵，Pb即对b按P重新排序）
	for i := 0; i < n; i++ {
		// Pb的第i个元素 = b[P[i]]（置换后的b值）
		sum := b.Get(lu.P[i])
		// 遍历L的前i列（下三角部分，非零元素）
		LCols, _ := lu.L.GetRow(i)
		for _, j := range LCols {
			if j < i { // 仅下三角部分（j < i），L[i][j]非零
				sum -= lu.L.Get(i, j) * lu.Y.Get(j)
			}
		}
		lu.Y.Set(i, sum)
	}
	// 步骤2：后向替换求解Ux=y
	for i := n - 1; i >= 0; i-- {
		sum := lu.Y.Get(i)
		uRow := lu.P[i] // U的第i行对应原始矩阵行索引
		// 遍历U的i+1~n-1列（上三角部分，非零元素）
		UCols, _ := lu.U.GetRow(uRow)
		for _, j := range UCols {
			if j > i { // 仅上三角部分（j > i），U[uRow][j]非零
				sum -= lu.U.Get(uRow, j) * x.Get(j)
			}
		}
		// 对角线元素U[uRow][i]，求解x[i] = sum / U[uRow][i]
		x.Set(i, sum/lu.U.Get(uRow, i))
	}
	return nil
}

// luSparse 稀疏矩阵的LU分解实现（A=PLU）
// 优化：底层使用稀疏矩阵存储，仅处理非零元素，大幅减少内存使用和计算量
type luSparse struct {
	lu
}

// Decompose 执行稀疏矩阵LU分解
// 假设输入矩阵是稀疏矩阵，底层使用稀疏矩阵操作进行优化
func (lu *luSparse) Decompose(matrix Matrix) error {
	// 判断输入是否为稀疏矩阵
	if _, ok := matrix.(*sparseMatrix); !ok {
		return lu.lu.Decompose(matrix)
	}
	// 检查输入矩阵是否为方阵
	if !matrix.IsSquare() {
		return errors.New("lu decomposition: input matrix must be square")
	}
	n := lu.N
	// 检查矩阵维度与分解器匹配
	if matrix.Rows() != n {
		return errors.New("lu decomposition: matrix dimension mismatch with decomposer")
	}
	// 复制输入矩阵到U（L初始为单位下三角矩阵）
	matrix.Copy(lu.U)
	// 初始化置换向量（初始为单位置换：P[i]=i）
	for i := 0; i < n; i++ {
		lu.P[i] = i
		lu.Pinverse[i] = i
		lu.L.Set(i, i, 1.0) // L对角线置1（下三角矩阵特性）
	}
	// 高斯消元+部分主元（按列遍历主元）
	for k := 0; k < n; k++ {
		// 步骤1：选择主元（当前列k，行k~n-1中绝对值最大的元素）
		maxRow := k
		maxVal := 0.0
		// 遍历当前列k的所有行，寻找最大主元
		for i := k; i < n; i++ {
			currentVal := math.Abs(lu.U.Get(lu.P[i], k))
			if currentVal > maxVal {
				maxVal = currentVal
				maxRow = i
			}
		}
		// 检查矩阵奇异性（主元接近零则无法分解）
		if maxVal < 1e-16 {
			return errors.New("lu decomposition: matrix is singular or nearly singular (pivot is zero)")
		}
		// 步骤2：交换置换向量中的行（实现主元行交换，避免实际移动矩阵行）
		if maxRow != k {
			lu.P[k], lu.P[maxRow] = lu.P[maxRow], lu.P[k]
			lu.Pinverse[lu.P[k]] = k
			lu.Pinverse[lu.P[maxRow]] = maxRow
		}
		// 步骤3：计算L的下三角元素（消元因子）并更新U
		pivotRow := lu.P[k]               // 主元行（原始矩阵行索引）
		pivotVal := lu.U.Get(pivotRow, k) // 主元值（U[pivotRow][k]）
		// 遍历当前列k下方的行（i > k）
		for i := k + 1; i < n; i++ {
			row := lu.P[i] // 当前处理行（原始矩阵行索引）
			currentVal := lu.U.Get(row, k)
			// 如果当前元素为零，则跳过（稀疏优化）
			if math.Abs(currentVal) < 1e-16 {
				continue
			}
			// 计算消元因子：L[i][k] = U[row][k] / pivotVal
			factor := currentVal / pivotVal
			lu.L.Set(i, k, factor) // 存入L矩阵
			// 高斯消元：更新U矩阵当前行 → U[row][j] -= factor * U[pivotRow][j]（j >= k）
			// 稀疏优化：仅遍历U[pivotRow]的非零列（减少无效计算）
			pivotCols, pivotValues := lu.U.GetRow(pivotRow)
			for idx, j := range pivotCols {
				if j >= k { // 仅更新主元列及右侧元素（左侧已为零）
					origVal := lu.U.Get(row, j)
					newVal := origVal - factor*pivotValues.Get(idx)
					// 如果新值接近零，则删除该元素（稀疏优化）
					if math.Abs(newVal) < 1e-16 {
						lu.U.Set(row, j, 0.0)
					} else {
						lu.U.Set(row, j, newVal)
					}
				}
			}
		}
	}
	return nil
}

// NewLU 创建基于稠密矩阵LU分解器
func NewLU(n int) LU {
	if n < 1 {
		panic("matrix dimension must be positive")
	}
	return &lu{
		N:        n,
		L:        NewDenseMatrix(n, n),
		U:        NewDenseMatrix(n, n),
		Y:        NewDenseVector(n),
		P:        make([]int, n),
		Pinverse: make([]int, n),
	}
}

// NewLUSparse 创建基于稀疏矩阵LU分解器
func NewLUSparse(n int) LU {
	if n < 1 {
		panic("matrix dimension must be positive")
	}
	return &luSparse{
		lu: lu{
			N:        n,
			L:        NewSparseMatrix(n, n),
			U:        NewSparseMatrix(n, n),
			Y:        NewDenseVector(n),
			P:        make([]int, n),
			Pinverse: make([]int, n),
		},
	}
}
