package maths

// 补充必要常量（浮点精度阈值）
const Epsilon = 1e-16

// 向量接口定义（统一方法名为Len()，符合Go语言惯例）
type Vector interface {
	Length() int                        // 获取向量长度
	Get(index int) float64              // 获取指定索引元素值
	Set(index int, value float64)       // 设置指定索引元素值
	Increment(index int, value float64) // 增量更新元素（value累加）
	Copy(a Vector)                      // 复制自身数据到目标向量a
	Clear()                             // 清空向量为零向量
	NonZeroCount() int                  // 统计非零元素数量
	String() string                     // 格式化字符串输出
	ToDense() []float64                 // 转换为稠密切片（[]float64）
	DotProduct(other Vector) float64    // 计算与另一个向量的点积
	Scale(scalar float64)               // 向量缩放（所有元素乘scalar）
	Add(other Vector)                   // 向量加法（自身 += 另一个向量）
	BuildFromDense(dense []float64)     // 从稠密切片构建向量
}

// 可更新向量接口（支持缓存与回溯，继承Vector）
type UpdateVector interface {
	Vector
	Update()   // 缓存数据刷到底层存储
	Rollback() // 回溯操作（清空缓存，放弃修改）
}

// 矩阵接口定义（补充ToDense()实现说明）
type Matrix interface {
	Rows() int                             // 获取矩阵行数
	Cols() int                             // 获取矩阵列数
	Get(row, col int) float64              // 获取指定行列元素值
	Set(row, col int, value float64)       // 设置指定行列元素值
	Increment(row, col int, value float64) // 增量更新元素
	Copy(a Matrix)                         // 复制自身数据到目标矩阵a
	Clear()                                // 清空矩阵为零矩阵
	NonZeroCount() int                     // 统计非零元素数量
	String() string                        // 格式化字符串输出
	IsSquare() bool                        // 判断是否为方阵（行数=列数）
	BuildFromDense(dense [][]float64)      // 从稠密矩阵构建
	GetRow(row int) ([]int, Vector)        // 获取指定行非零元素（列索引+值向量）
	MatrixVectorMultiply(x Vector) Vector  // 矩阵向量乘法（返回A*x）
	ToDense() Vector                       // 转换为稠密向量（行优先展开）
}

// 可更新矩阵接口（支持缓存与回溯，继承Matrix）
type UpdateMatrix interface {
	Matrix
	Update()   // 缓存数据刷到底层存储
	Rollback() // 回溯操作（清空缓存，放弃修改）
}

// LU分解接口（支持稠密/稀疏矩阵）
type LU interface {
	Decompose(matrix Matrix) error // 对输入方阵执行LU分解（A=PLU）
	SolveReuse(b, x Vector) error  // 重用向量求解Ax=b（利用LU分解结果）
}

// MatrixPruner 矩阵精简接口（专注denseMatrix的零行/零列移除与压缩）
type MatrixPruner interface {
	// RemoveZeroRows 移除全零行（返回精简后的矩阵，不修改原始矩阵）
	RemoveZeroRows() Matrix
	// RemoveZeroCols 移除全零列（返回精简后的矩阵，不修改原始矩阵）
	RemoveZeroCols() Matrix
	// Compress 组合精简（先移除零行，再移除零列，可指定顺序）
	Compress(removeRowsFirst bool) Matrix
	// GetRowMapping 获取「精简后行索引 → 原始行索引」的映射
	GetRowMapping() map[int]int
	// GetColMapping 获取「精简后列索引 → 原始列索引」的映射
	GetColMapping() map[int]int
	// GetOriginalRowIndex 根据精简后行索引，获取原始行索引
	GetOriginalRowIndex(newRow int) (int, bool)
	// GetOriginalColIndex 根据精简后列索引，获取原始列索引
	GetOriginalColIndex(newCol int) (int, bool)
	// GetPrunedMatrix 获取缓存的精简后矩阵（避免重复计算）
	GetPrunedMatrix() Matrix
}
