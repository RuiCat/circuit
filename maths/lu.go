package maths

import (
	"errors"
	"math"
)

// NewLU 创建稠密矩阵LU分解器（输入矩阵维度n）
// 参数:
//
//	n - 矩阵维度（必须为正整数）
//
// 返回:
//
//	LU接口实例，错误信息
func NewLU(n int) (LU, error) {
	if n < 1 {
		return nil, errors.New("lu dimension must be positive")
	}
	return &luDense{
		baseLU: baseLU{
			n:        n,
			L:        NewDenseMatrix(n, n),
			U:        NewDenseMatrix(n, n),
			Y:        NewDenseVector(n),
			P:        make([]int, n),
			pinverse: make([]int, n),
		},
	}, nil
}

// NewLUSparse 创建稀疏矩阵LU分解器（输入矩阵维度n）
// 参数:
//
//	n - 矩阵维度（必须为正整数）
//
// 返回:
//
//	LU接口实例，错误信息
func NewLUSparse(n int) (LU, error) {
	if n < 1 {
		return nil, errors.New("lu sparse dimension must be positive")
	}
	return &luSparse{
		baseLU: baseLU{
			n:        n,
			L:        NewSparseMatrix(n, n),
			U:        NewSparseMatrix(n, n),
			Y:        NewDenseVector(n), // 中间向量用稠密更高效（访问速度优先）
			P:        make([]int, n),
			pinverse: make([]int, n),
		},
	}, nil
}

// baseLU 公共LU分解结构体（存储共用字段）
// 实现PA = LU分解，其中：
//
//	P - 置换矩阵（用向量表示）
//	L - 单位下三角矩阵（对角线为1）
//	U - 上三角矩阵
type baseLU struct {
	n        int    // 矩阵维度（方阵n×n）
	L        Matrix // 下三角矩阵L（L[i][i]=1，严格下三角存储消元因子）
	U        Matrix // 上三角矩阵U（存储消元后上三角元素）
	Y        Vector // 中间变量：存储前向替换结果Ly=Pb
	P        []int  // 置换向量：P[i] = 分解后第i行对应的原始矩阵行索引
	pinverse []int  // 逆置换向量：pinverse[i] = 原始第i行对应的分解后行索引
}

// Dim 获取矩阵维度
// 返回:
//
//	矩阵维度n
func (lu *baseLU) Dim() int {
	return lu.n
}

// init 初始化置换向量和L矩阵的对角线
// 参数:
//
//	matrix - 输入矩阵A（将被拷贝到U矩阵）
//
// 功能:
//  1. 清零L和U矩阵
//  2. 将输入矩阵A拷贝到U矩阵
//  3. 初始化置换向量P和pinverse为单位置换
//  4. 设置L矩阵对角线为1（单位下三角矩阵特性）
func (lu *baseLU) init(matrix Matrix) {
	lu.L.Zero()
	lu.U.Zero()
	matrix.Copy(lu.U) // 将A拷贝到U，后续在U上进行原位消元
	for i := 0; i < lu.n; i++ {
		lu.P[i] = i         // 初始置换：分解后行i对应原始行i
		lu.pinverse[i] = i  // 初始逆置换：原始行i对应分解后行i
		lu.L.Set(i, i, 1.0) // L对角线固定为1（单位下三角矩阵特性）
	}
}

// updatePermutation 更新置换向量（交换并同步更新逆置换）
// 参数:
//
//	k, maxRow - 要交换的分解后行索引
//
// 功能:
//  1. 交换P向量中的第k和第maxRow个元素
//  2. 更新pinverse向量以保持P和pinverse互为逆关系
func (lu *baseLU) updatePermutation(k, maxRow int) {
	// 交换P向量（分解后行对应的原始行索引）
	lu.P[k], lu.P[maxRow] = lu.P[maxRow], lu.P[k]
	// 同步更新逆置换向量（原始行对应的分解后行索引）
	lu.pinverse[lu.P[k]] = k
	lu.pinverse[lu.P[maxRow]] = maxRow
}

// luDense 稠密矩阵LU分解实现（A=PLU，带部分主元）
type luDense struct {
	baseLU // 嵌入公共结构体，复用字段与方法
}

// Decompose 执行稠密矩阵LU分解（核心逻辑：高斯消元+部分主元）
// 参数:
//
//	matrix - 输入矩阵A（必须为方阵）
//
// 返回:
//
//	错误信息（如果矩阵奇异或维度不匹配）
//
// 算法步骤:
//  1. 初始化：拷贝A到U，初始化P、pinverse和L
//  2. 对每一列k（0到n-1）:
//     a. 部分主元选择：在U的当前列k中找[k, n-1]行的最大值
//     b. 行交换：交换U的行，交换L的前k-1列，更新置换向量
//     c. 高斯消元：计算消元因子存入L，更新U矩阵
//
// 数学原理:
//
//	通过行置换+逐列消元，将A拆解为 P(置换矩阵)、L(单位下三角)、U(上三角) 乘积
func (lu *luDense) Decompose(matrix Matrix) error {
	// 1. 输入合法性校验
	if !matrix.IsSquare() {
		return errors.New("lu dense decompose: input must be square matrix")
	}
	if matrix.Rows() != lu.n {
		return errors.New("lu dense decompose: matrix dimension mismatch")
	}

	// 2. 初始化
	lu.init(matrix)

	// 3. 逐列执行高斯消元（按主元列k遍历）
	for k := 0; k < lu.n; k++ {
		// 步骤1：部分主元选择 - 在U的当前列k中找[k, n-1]行的最大值
		maxRow := k
		maxAbsVal := math.Abs(lu.U.Get(k, k))
		for i := k + 1; i < lu.n; i++ {
			if v := math.Abs(lu.U.Get(i, k)); v > maxAbsVal {
				maxAbsVal = v
				maxRow = i
			}
		}

		// 检查矩阵是否奇异（主元接近零）
		if maxAbsVal < 1e-16 {
			return errors.New("lu dense decompose: matrix is singular or nearly singular")
		}

		// 步骤2：行交换（如果找到的主元不在当前行）
		if maxRow != k {
			// 交换U矩阵的整行
			lu.U.SwapRows(k, maxRow)
			// 交换L矩阵的前k-1列（只交换已填充的消元因子）
			for j := 0; j < k; j++ {
				val1 := lu.L.Get(k, j)
				val2 := lu.L.Get(maxRow, j)
				lu.L.Set(k, j, val2)
				lu.L.Set(maxRow, j, val1)
			}
			// 更新置换向量
			lu.updatePermutation(k, maxRow)
		}

		// 步骤3：高斯消元
		pivotVal := lu.U.Get(k, k) // 主元值
		for i := k + 1; i < lu.n; i++ {
			// 计算消元因子：L[i][k] = U[i][k] / 主元值
			factor := lu.U.Get(i, k) / pivotVal
			lu.L.Set(i, k, factor) // 存入L矩阵（严格下三角部分）
			lu.U.Set(i, k, 0.0)    // 显式置零（数值稳定性）

			// 消元更新U矩阵：U[i][j] -= 因子 * U[k][j]（j >= k+1）
			for j := k + 1; j < lu.n; j++ {
				newVal := lu.U.Get(i, j) - factor*lu.U.Get(k, j)
				lu.U.Set(i, j, newVal)
			}
		}
	}
	return nil
}

// SolveReuse 利用分解结果求解Ax=b（重用预分配向量，无内存额外分配）
// 参数:
//
//	b - 右侧向量b
//	x - 输出向量x（用于存储解）
//
// 返回:
//
//	错误信息（如果向量维度不匹配或矩阵奇异）
//
// 数学步骤:
//  1. 前向替换：求解Ly = Pb（Pb为b按置换向量P重新排序）
//  2. 后向替换：求解Ux = y
//
// 注意:
//
//	解x已经是原始顺序，无需额外重新排序
func (lu *luDense) SolveReuse(b, x Vector) error {
	// 1. 输入合法性校验
	if b.Length() != lu.n || x.Length() != lu.n {
		return errors.New("lu dense solve: vector dimension mismatch")
	}

	// 2. 前向替换：求解Ly = Pb
	lu.Y.Zero() // 清零中间向量
	for i := 0; i < lu.n; i++ {
		// 初始值 = Pb[i] = b[原始行索引] = b[lu.P[i]]
		sum := b.Get(lu.P[i])
		// 累加L[i][j] * Y[j]（j < i，L严格下三角）
		for j := 0; j < i; j++ {
			sum -= lu.L.Get(i, j) * lu.Y.Get(j)
		}
		lu.Y.Set(i, sum)
	}

	// 3. 后向替换：求解Ux = y
	x.Zero() // 清零输出向量
	for i := lu.n - 1; i >= 0; i-- {
		sum := lu.Y.Get(i)
		// 累加U[i][j] * x[j]（j > i，U上三角）
		for j := i + 1; j < lu.n; j++ {
			sum -= lu.U.Get(i, j) * x.Get(j)
		}
		// 求解x[i] = sum / U[i][i]（U对角线为非零主元）
		diagVal := lu.U.Get(i, i)
		if math.Abs(diagVal) < 1e-16 {
			return errors.New("lu dense solve: division by zero (U diagonal is zero)")
		}
		x.Set(i, sum/diagVal)
	}

	return nil
}

// luSparse 稀疏矩阵LU分解实现（A=PLU，带部分主元+稀疏优化）
type luSparse struct {
	baseLU // 嵌入公共结构体，复用字段与方法
}

// Decompose 执行稀疏矩阵LU分解（核心：保留非零元素，减少计算/内存开销）
// 参数:
//
//	matrix - 输入稀疏矩阵A（必须为方阵）
//
// 返回:
//
//	错误信息（如果矩阵奇异或维度不匹配）
//
// 稀疏优化:
//  1. 仅处理非零元素，跳过零元素计算
//  2. 使用GetRow获取非零列索引，减少内层循环次数
//  3. 新值接近零时删除元素，维持矩阵稀疏性
func (lu *luSparse) Decompose(matrix Matrix) error {
	// 1. 输入合法性校验
	if !matrix.IsSquare() {
		return errors.New("lu sparse decompose: input must be square matrix")
	}
	if matrix.Rows() != lu.n {
		return errors.New("lu sparse decompose: matrix dimension mismatch")
	}

	// 2. 初始化
	lu.init(matrix)

	// 3. 逐列执行高斯消元（稀疏优化版本）
	for k := 0; k < lu.n; k++ {
		// 步骤1：部分主元选择
		maxRow := k
		maxAbsVal := math.Abs(lu.U.Get(k, k))
		for i := k + 1; i < lu.n; i++ {
			if v := math.Abs(lu.U.Get(i, k)); v > maxAbsVal {
				maxAbsVal = v
				maxRow = i
			}
		}

		// 检查矩阵是否奇异
		if maxAbsVal < 1e-16 {
			return errors.New("lu sparse decompose: matrix is singular or nearly singular")
		}

		// 步骤2：行交换
		if maxRow != k {
			// 交换U和L矩阵的行（使用高效的接口方法）
			lu.U.SwapRows(k, maxRow)
			lu.L.SwapRows(k, maxRow) // 对于L，完全交换是安全的，因为j>=k的列是零
			// 更新置换向量
			lu.updatePermutation(k, maxRow)
		}

		// 步骤3：稀疏消元
		pivotVal := lu.U.Get(k, k) // 主元值
		// 获取主元行的非零列索引和值（稀疏优化：加速内层循环）
		pivotCols, pivotVals := lu.U.GetRow(k)

		// 遍历当前列k下方所有行
		for i := k + 1; i < lu.n; i++ {
			valIK := lu.U.Get(i, k)
			// 稀疏优化1：当前元素为零则跳过，无消元需求
			if math.Abs(valIK) < 1e-16 {
				continue
			}

			// 计算消元因子
			factor := valIK / pivotVal
			lu.L.Set(i, k, factor) // 存入L矩阵
			lu.U.Set(i, k, 0.0)    // 显式置零

			// 稀疏优化2：只更新主元行中存在的非零列
			for idx, j := range pivotCols {
				if j <= k {
					continue // 主元列左侧已为零，无需更新
				}
				updatedVal := lu.U.Get(i, j) - factor*pivotVals.Get(idx)
				// 稀疏优化3：新值接近零则删除元素（维持稀疏性）
				if math.Abs(updatedVal) < 1e-16 {
					lu.U.Set(i, j, 0.0)
				} else {
					lu.U.Set(i, j, updatedVal)
				}
			}
		}
	}
	return nil
}

// SolveReuse 稀疏矩阵LU分解结果求解Ax=b（复用向量，稀疏优化）
// 参数:
//
//	b - 右侧向量b
//	x - 输出向量x（用于存储解）
//
// 返回:
//
//	错误信息（如果向量维度不匹配或矩阵奇异）
//
// 稀疏优化:
//  1. 使用GetRow获取非零列索引，减少内层循环次数
//  2. 仅遍历非零元素进行计算
func (lu *luSparse) SolveReuse(b, x Vector) error {
	// 1. 输入合法性校验
	if b.Length() != lu.n || x.Length() != lu.n {
		return errors.New("lu sparse solve: vector dimension mismatch")
	}

	// 2. 前向替换：求解Ly = Pb（稀疏优化版本）
	lu.Y.Zero()
	for i := 0; i < lu.n; i++ {
		sum := b.Get(lu.P[i]) // 按置换向量取b的元素
		// 稀疏优化：仅遍历L[i]的非零列（j < i）
		cols, vals := lu.L.GetRow(i)
		for idx, j := range cols {
			if j < i { // 只累加严格下三角部分
				sum -= vals.Get(idx) * lu.Y.Get(j)
			}
		}
		lu.Y.Set(i, sum)
	}

	// 3. 后向替换：求解Ux = y（稀疏优化版本）
	x.Zero()
	for i := lu.n - 1; i >= 0; i-- {
		sum := lu.Y.Get(i)
		diag := lu.U.Get(i, i) // 显式获取对角线元素，提高可读性

		// 检查对角线元素是否为零
		if math.Abs(diag) < 1e-16 {
			return errors.New("lu sparse solve: division by zero (U diagonal is zero)")
		}

		// 稀疏优化：仅遍历U[i]的非零列（j > i）
		cols, vals := lu.U.GetRow(i)
		for idx, j := range cols {
			if j > i {
				// 累加上三角部分
				sum -= vals.Get(idx) * x.Get(j)
			}
		}
		// 求解x[i] = sum / diag
		x.Set(i, sum/diag)
	}
	return nil
}
