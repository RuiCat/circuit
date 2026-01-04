package maths

import (
	"errors"
	"math"
)

// NewLU 创建稠密矩阵LU分解器（输入矩阵维度n）
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
type baseLU struct {
	n        int    // 矩阵维度（方阵）
	L        Matrix // 下三角矩阵（L[i][i]=1，严格下三角存储消元因子）
	U        Matrix // 上三角矩阵（存储消元后上三角元素）
	Y        Vector // 中间变量：存储前向替换结果Ly=Pb
	P        []int  // 置换向量：P[i] = 分解后第i行对应的原始矩阵行索引
	pinverse []int  // 逆置换向量：pinverse[i] = 原始第i行对应的分解后行索引
}

// Dim 获取矩阵维度
func (lu *baseLU) Dim() int {
	return lu.n
}

// selectPivot 部分主元选择（公共方法）
// 输入：当前主元列k，返回：最大主元行索引、当前列最大绝对值、是否奇异
func (lu *baseLU) selectPivot(k int) (int, float64, error) {
	maxRow := k
	maxAbsVal := math.Abs(lu.U.Get(lu.P[k], k))

	// 遍历当前列k下方所有行，找绝对值最大元素
	for i := k + 1; i < lu.n; i++ {
		currAbsVal := math.Abs(lu.U.Get(lu.P[i], k))
		if currAbsVal > maxAbsVal {
			maxAbsVal = currAbsVal
			maxRow = i
		}
	}

	// 相对阈值判断矩阵奇异性（适配不同量级矩阵，1e-12为工程常用安全阈值）
	threshold := math.Max(maxAbsVal*1e-12, 1e-12)
	if maxAbsVal < threshold {
		return 0, 0, errors.New("lu decomposition: matrix is singular or nearly singular")
	}
	return maxRow, maxAbsVal, nil
}

// swapPermutation 交换置换向量（公共方法，避免实际移动矩阵行）
func (lu *baseLU) swapPermutation(k, maxRow int) {
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
// 数学原理：通过行置换+逐列消元，将A拆解为 P(置换矩阵)、L(单位下三角)、U(上三角) 乘积
func (lu *luDense) Decompose(matrix Matrix) error {
	// 1. 输入合法性校验
	if !matrix.IsSquare() {
		return errors.New("lu dense decompose: input must be square matrix")
	}
	if matrix.Rows() != lu.n {
		return errors.New("lu dense decompose: matrix dimension mismatch")
	}

	// 2. 初始化矩阵（双重置0，避免垃圾值残留）
	lu.L.Zero()
	lu.U.Zero()
	matrix.Copy(lu.U) // 原始矩阵拷贝到U，后续消元更新U

	// 3. 初始化置换向量（初始为单位置换：分解后行i对应原始行i）
	for i := 0; i < lu.n; i++ {
		lu.P[i] = i
		lu.pinverse[i] = i
		lu.L.Set(i, i, 1.0) // L对角线固定为1（单位下三角矩阵特性）
	}

	// 4. 逐列执行高斯消元（按主元列k遍历）
	for k := 0; k < lu.n; k++ {
		// 步骤1：选择部分主元（当前列k下方绝对值最大元素）
		maxRow, _, err := lu.selectPivot(k)
		if err != nil {
			return err
		}

		// 步骤2：交换置换向量（实现主元行置顶，无实际矩阵行移动，高效）
		if maxRow != k {
			lu.swapPermutation(k, maxRow)
		}

		// 步骤3：计算L消元因子+更新U上三角矩阵
		pivotOrigRow := lu.P[k]               // 主元对应的原始矩阵行索引
		pivotVal := lu.U.Get(pivotOrigRow, k) // 主元值（U矩阵中该位置元素）

		// 遍历当前列k下方所有行（i > k），执行消元
		for i := k + 1; i < lu.n; i++ {
			currOrigRow := lu.P[i] // 当前处理行的原始矩阵行索引
			// 计算消元因子：L[i][k] = U[当前行][k] / 主元值（数学核心：消去U当前行k列元素）
			factor := lu.U.Get(currOrigRow, k) / pivotVal
			lu.L.Set(i, k, factor) // 存入L矩阵（严格下三角部分）

			// 消元更新U矩阵：U[当前行][j] -= 因子 * U[主元行][j]（j >= k，左侧已为0）
			// 稠密矩阵直接遍历列（无冗余，比GetRow高效）
			for j := k; j < lu.n; j++ {
				origVal := lu.U.Get(currOrigRow, j)
				newVal := origVal - factor*lu.U.Get(pivotOrigRow, j)
				lu.U.Set(currOrigRow, j, newVal)
			}
		}
	}
	return nil
}

// SolveReuse 利用分解结果求解Ax=b（重用预分配向量，无内存额外分配）
// 数学步骤：1. 前向替换求解Ly=Pb；2. 后向替换求解Ux=y
func (lu *luDense) SolveReuse(b, x Vector) error {
	// 1. 输入合法性校验
	if b.Length() != lu.n || x.Length() != lu.n {
		return errors.New("lu dense solve: vector dimension mismatch")
	}
	lu.Y.Zero()
	x.Zero()

	// 2. 前向替换：求解Ly=Pb（Pb为b按置换向量P重新排序）
	for i := 0; i < lu.n; i++ {
		// 初始值 = Pb[i] = b[原始行索引] = b[lu.P[i]]
		sum := b.Get(lu.P[i])
		// 累加L[i][j] * Y[j]（j < i，L严格下三角，j>=i无值）
		for j := 0; j < i; j++ {
			sum -= lu.L.Get(i, j) * lu.Y.Get(j)
		}
		lu.Y.Set(i, sum)
	}

	// 3. 后向替换：求解Ux=y（从最后一行反向计算）
	for i := lu.n - 1; i >= 0; i-- {
		sum := lu.Y.Get(i)
		uOrigRow := lu.P[i] // U矩阵行对应原始矩阵行索引

		// 累加U[行][j] * x[j]（j > i，U上三角，j<=i无值）
		for j := i + 1; j < lu.n; j++ {
			sum -= lu.U.Get(uOrigRow, j) * x.Get(j)
		}

		// 求解x[i] = sum / U[行][i]（U对角线为非零主元，加除零容错）
		diagVal := lu.U.Get(uOrigRow, i)
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
func (lu *luSparse) Decompose(matrix Matrix) error {
	// 1. 输入合法性校验
	if !matrix.IsSquare() {
		return errors.New("lu sparse decompose: input must be square matrix")
	}
	if matrix.Rows() != lu.n {
		return errors.New("lu sparse decompose: matrix dimension mismatch")
	}
	// 2. 初始化矩阵（双重置0，清除残留值）
	lu.L.Zero()
	lu.U.Zero()
	matrix.Copy(lu.U) // 原始稀疏矩阵拷贝到U

	// 3. 初始化置换向量（同稠密矩阵逻辑）
	for i := 0; i < lu.n; i++ {
		lu.P[i] = i
		lu.pinverse[i] = i
		lu.L.Set(i, i, 1.0)
	}

	// 4. 逐列执行高斯消元（稀疏优化核心：仅处理非零元素）
	for k := 0; k < lu.n; k++ {
		// 步骤1：选择部分主元（同稠密逻辑，仅遍历非零元素但已兼容）
		maxRow, _, err := lu.selectPivot(k)
		if err != nil {
			return err
		}

		// 步骤2：交换置换向量
		if maxRow != k {
			lu.swapPermutation(k, maxRow)
		}

		// 步骤3：计算消元因子+更新U（稀疏专属优化）
		pivotOrigRow := lu.P[k]
		pivotVal := lu.U.Get(pivotOrigRow, k)
		if math.Abs(pivotVal) < 1e-16 {
			return errors.New("lu sparse decompose: pivot is zero (singular matrix)")
		}

		// 遍历当前列k下方所有行，仅处理非零元素行
		for i := k + 1; i < lu.n; i++ {
			currOrigRow := lu.P[i]
			currVal := lu.U.Get(currOrigRow, k)

			// 稀疏优化1：当前元素为零则跳过，无消元需求
			if math.Abs(currVal) < 1e-16 {
				continue
			}

			// 计算消元因子，存入L矩阵
			factor := currVal / pivotVal
			lu.L.Set(i, k, factor)

			// 稀疏优化2：获取主元行非零列+值（拷贝值快照，避免后续U修改污染）
			pivotNonZeroCols, pivotNonZeroVals := lu.U.GetRow(pivotOrigRow)
			pivotValsCopy := make([]float64, pivotNonZeroVals.Length())
			for idx := range pivotValsCopy {
				pivotValsCopy[idx] = pivotNonZeroVals.Get(idx)
			}

			// 消元更新U当前行：仅遍历主元行非零列（j >= k）
			for idx, j := range pivotNonZeroCols {
				if j < k {
					continue // 主元列左侧已为零，无需更新
				}
				origVal := lu.U.Get(currOrigRow, j)
				newVal := origVal - factor*pivotValsCopy[idx]

				// 稀疏优化3：新值接近零则删除元素（维持稀疏性）
				if math.Abs(newVal) < 1e-16 {
					lu.U.Set(currOrigRow, j, 0.0)
				} else {
					lu.U.Set(currOrigRow, j, newVal)
				}
			}
		}
	}
	return nil
}

// SolveReuse 稀疏矩阵LU分解结果求解Ax=b（复用向量，稀疏优化）
func (lu *luSparse) SolveReuse(b, x Vector) error {
	// 1. 输入合法性校验
	if b.Length() != lu.n || x.Length() != lu.n {
		return errors.New("lu sparse solve: vector dimension mismatch")
	}
	lu.Y.Zero()
	x.Zero()

	// 2. 前向替换：Ly=Pb（稀疏L仅遍历非零列，减少计算）
	for i := 0; i < lu.n; i++ {
		sum := b.Get(lu.P[i])
		// 仅遍历L[i]的非零列（j < i）
		LNonZeroCols, LNonZeroVals := lu.L.GetRow(i)
		for idx, j := range LNonZeroCols {
			if j >= i {
				continue
			}
			sum -= LNonZeroVals.Get(idx) * lu.Y.Get(j)
		}
		lu.Y.Set(i, sum)
	}

	// 3. 后向替换：Ux=y（稀疏U仅遍历非零列，减少计算）
	for i := lu.n - 1; i >= 0; i-- {
		sum := lu.Y.Get(i)
		uOrigRow := lu.P[i]

		// 仅遍历U[uOrigRow]的非零列（j > i）
		UNonZeroCols, UNonZeroVals := lu.U.GetRow(uOrigRow)
		for idx, j := range UNonZeroCols {
			if j <= i {
				continue
			}
			sum -= UNonZeroVals.Get(idx) * x.Get(j)
		}

		// 除零容错（U对角线为主元，非零）
		diagVal := lu.U.Get(uOrigRow, i)
		if math.Abs(diagVal) < 1e-16 {
			return errors.New("lu sparse solve: division by zero (U diagonal is zero)")
		}
		x.Set(i, sum/diagVal)
	}
	return nil
}
