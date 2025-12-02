package maths

// 补充必要常量（浮点精度阈值）
const Epsilon = 1e-16

// DataManager 一维浮点数据管理器（底层存储核心）
type DataManager interface {
	// 基础属性方法
	Length() int    // 获取数据长度
	String() string // 返回数据的字符串表示

	// 数据访问方法
	Get(index int) float64              // 获取指定索引处的元素值
	Set(index int, value float64)       // 设置指定索引处的元素值
	Increment(index int, value float64) // 增量更新指定索引处的元素值

	// 数据操作和转换方法
	Data() []float64    // 返回数据的切片副本
	DataPtr() []float64 // 返回数据的切片引用（直接操作底层数据）

	// 数据修改方法
	Zero()                                       // 清空所有数据
	ZeroInPlace()                                // 原地将所有元素设置为零
	FillInPlace(value float64)                   // 原地填充所有元素为指定值
	AppendInPlace(values ...float64)             // 原地追加元素（不创建新对象）
	InsertInPlace(index int, values ...float64)  // 在指定位置原地插入元素
	RemoveInPlace(index int, count int)          // 从指定位置原地移除指定数量的元素
	ReplaceInPlace(index int, values ...float64) // 从指定位置原地替换元素

	// 大小调整方法
	Resize(length int)           // 调整数据长度（可能重新分配内存）
	ResizeInPlace(newLength int) // 原地调整数据长度（不重新分配内存）

	// 统计和复制方法
	NonZeroCount() int       // 统计非零元素数量
	Copy(target DataManager) // 复制数据到目标管理器
}

// 向量接口定义（统一方法名为Len()，符合Go语言惯例）
type Vector interface {
	// 基础属性方法
	Base() Vector   // 获取底层
	Length() int    // 获取向量长度
	String() string // 格式化字符串输出

	// 数据访问方法
	Get(index int) float64              // 获取指定索引元素值
	Set(index int, value float64)       // 设置指定索引元素值
	Increment(index int, value float64) // 增量更新元素（value累加）

	// 数据操作和转换方法
	ToDense() []float64             // 转换为稠密切片（[]float64）
	BuildFromDense(dense []float64) // 从稠密切片构建向量

	// 数据修改方法
	Zero()         // 清空向量为零向量
	Copy(a Vector) // 复制自身数据到目标向量a

	// 数学运算方法
	DotProduct(other Vector) float64 // 计算与另一个向量的点积
	Scale(scalar float64)            // 向量缩放（所有元素乘scalar）
	Add(other Vector)                // 向量加法（自身 += 另一个向量）

	// 统计方法
	NonZeroCount() int // 统计非零元素数量
}

// 可更新向量接口（支持缓存与回溯，继承Vector）
type UpdateVector interface {
	Vector
	Update()   // 缓存数据刷到底层存储
	Rollback() // 回溯操作（清空缓存，放弃修改）
}

// 矩阵接口定义（补充ToDense()实现说明）
type Matrix interface {
	// 基础属性方法
	Base() Matrix   // 获取底层
	Rows() int      // 获取矩阵行数
	Cols() int      // 获取矩阵列数
	String() string // 格式化字符串输出
	IsSquare() bool // 判断是否为方阵（行数=列数）

	// 数据访问方法
	Get(row, col int) float64              // 获取指定行列元素值
	Set(row, col int, value float64)       // 设置指定行列元素值
	Increment(row, col int, value float64) // 增量更新元素
	GetRow(row int) ([]int, Vector)        // 获取指定行非零元素（列索引+值向量）

	// 数据操作和转换方法
	ToDense() Vector                  // 转换为稠密向量（行优先展开）
	BuildFromDense(dense [][]float64) // 从稠密矩阵构建

	// 数据修改方法
	Zero()                 // 清空矩阵为零矩阵
	Copy(a Matrix)         // 复制自身数据到目标矩阵a
	Resize(rows, cols int) // 重置矩阵大小和数据（清空所有元素）

	// 数学运算方法
	MatrixVectorMultiply(x Vector) Vector // 矩阵向量乘法（返回A*x）

	// 统计方法
	NonZeroCount() int // 统计非零元素数量
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
	// 精简操作
	RemoveZeroRows() Matrix               // 移除全零行（返回精简后的矩阵，不修改原始矩阵）
	RemoveZeroCols() Matrix               // 移除全零列（返回精简后的矩阵，不修改原始矩阵）
	Compress(removeRowsFirst bool) Matrix // 组合精简（先移除零行，再移除零列，可指定顺序）

	// 映射查询
	GetRowMapping() map[int]int                 // 获取「精简后行索引 → 原始行索引」的映射
	GetColMapping() map[int]int                 // 获取「精简后列索引 → 原始列索引」的映射
	GetOriginalRowIndex(newRow int) (int, bool) // 根据精简后行索引，获取原始行索引
	GetOriginalColIndex(newCol int) (int, bool) // 根据精简后列索引，获取原始列索引

	// 缓存访问
	GetPrunedMatrix() Matrix // 获取缓存的精简后矩阵（避免重复计算）
}
