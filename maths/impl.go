package maths

// Matrix 通用矩阵接口
// 定义矩阵的基本操作，支持稀疏和密集两种实现
type Matrix interface {
	// BuildFromDense 从稠密矩阵构建矩阵
	BuildFromDense(dense [][]float64)
	// Clear 清空矩阵，重置为零矩阵
	Clear()
	// Cols 返回矩阵列数
	Cols() int
	// Copy 将自身值复制到 a 矩阵
	Copy(a Matrix)
	// Get 获取指定位置的元素值
	Get(row int, col int) float64
	// GetRow 获取指定行的所有元素
	GetRow(row int) ([]int, Vector)
	// Increment 增量设置矩阵元素（累加值）
	Increment(row int, col int, value float64)
	// IsSquare 检查矩阵是否为方阵
	IsSquare() bool
	// MatrixVectorMultiply 执行矩阵向量乘法
	MatrixVectorMultiply(x Vector) Vector
	// NonZeroCount 返回非零元素数量
	NonZeroCount() int
	// Rows 返回矩阵行数
	Rows() int
	// Set 设置矩阵元素值
	Set(row int, col int, value float64)
	// String 返回矩阵的字符串表示
	String() string
	// ToDense 转换为稠密向量
	ToDense() Vector
}

// UpdateMatrix 更新矩阵接口
// 扩展Matrix接口，提供基于uint16分块位图的缓存机制
type UpdateMatrix interface {
	Matrix // 继承Matrix接口的所有方法
	// Update 更新操作
	// 将位图为1的值写入底层以后将位图设置为0
	Update()
	// Rollback 回溯操作
	// 将位图标记置0，清空缓存
	Rollback()
}

// Vector 通用向量接口
// 定义向量的基本操作
type Vector interface {
	// BuildFromDense 从稠密向量构建向量
	BuildFromDense(dense []float64)
	// Clear 清空向量，重置为零向量
	Clear()
	// Copy 将自身值复制到 a 向量
	Copy(a Vector)
	// Get 获取指定位置的元素值
	Get(index int) float64
	// Increment 增量设置向量元素（累加值）
	Increment(index int, value float64)
	// Length 返回向量长度
	Length() int
	// NonZeroCount 返回非零元素数量
	NonZeroCount() int
	// Set 设置向量元素值
	Set(index int, value float64)
	// String 返回向量的字符串表示
	String() string
	// ToDense 转换为稠密向量
	ToDense() []float64
	// DotProduct 计算与另一个向量的点积
	DotProduct(other Vector) float64
	// Scale 向量缩放
	Scale(scalar float64)
	// Add 向量加法
	Add(other Vector)
}

// UpdateVector 更新向量接口
// 扩展 Vector接口，提供基于uint16分块位图的缓存机制
type UpdateVector interface {
	Vector // 继承Vector接口的所有方法
	// Update 更新操作
	// 将位图为1的值写入底层以后将位图设置为0
	Update()
	// Rollback 回溯操作
	// 将位图标记置0，清空缓存
	Rollback()
}

// RowInfoType 行类型
type RowInfoType uint8

const (
	ROW_NORMAL RowInfoType = 0 // 普通行
	ROW_CONST  RowInfoType = 1 // 常数行
)

// RowInfo 行信息，用于矩阵简化
type RowInfo struct {
	Type      RowInfoType // 行类型：ROW_NORMAL, ROW_CONST
	MapCol    int         // 列映射
	MapRow    int         // 行映射
	Value     float64     // 常数值
	LSChanges bool        // 左侧变化
	RSChanges bool        // 右侧变化
	DropRow   bool        // 删除行
}

// MatrixReducer 矩阵简化
type MatrixReducer interface {
	// Simplify 简化矩阵，返回简化后的矩阵和右侧向量
	Simplify(matrix Matrix, rightSide Vector) (Matrix, Vector, error)
	// ApplySolution 将简化系统的解映射回原始系统
	ApplySolution(simplifiedSolution Vector, originalSolution Vector) error
	// GetSimplifiedSize 获取简化后的矩阵大小
	GetSimplifiedSize() int
	// SetRowChanges 设置行的变化状态
	SetRowChanges(row int, lsChanges, rsChanges, dropRow bool)
	// GetRowInfo 获取行信息
	GetRowInfo(row int) RowInfo
}

// LU 稀疏LU分解接口
// 定义稀疏矩阵LU分解的基本操作，支持部分主元法
type LU interface {
	// Decompose 执行稀疏LU分解（原地分解，直接修改U矩阵）
	// 参数：
	//   matrix - 待分解的稀疏矩阵
	// 返回：
	//   error - 如果矩阵奇异或接近奇异则返回错误
	Decompose(matrix Matrix) error
	// SolveReuse 解线性方程组 Ax = b，重用预分配的向量
	// 参数：
	//   b - 右侧向量
	//   x - 解向量（预分配，结果将存储在此）
	// 返回：
	//   error - 如果向量维度不匹配则返回错误
	SolveReuse(b, x Vector) error
}
