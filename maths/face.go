package maths

import (
	"math"
	"math/cmplx"
)

// 补充必要常量（浮点精度阈值）
const Epsilon = 1e-16

// abs 是一个泛型函数，返回任何支持的 Number 类型的绝对值。
func abs[T Number](v T) float64 {
	// 通过类型断言检查具体类型
	switch x := any(v).(type) {
	case float32:
		return math.Abs(float64(x))
	case float64:
		return math.Abs(x)
	case complex64:
		return cmplx.Abs(complex128(x))
	case complex128:
		return cmplx.Abs(x)
	}
	return 0
}

// Number 是一个约束，允许任何浮点或复数类型
type Number interface {
	~float32 | ~float64 | ~complex64 | ~complex128
}

// Data 接口定义了对单个数据元素的操作
type Data[T Number] interface {
	Add(other Data[T]) Data[T]
	Sub(other Data[T]) Data[T]
	Mul(other Data[T]) Data[T]
	Div(other Data[T]) Data[T]
	Inv() Data[T]
	Abs() float64
	Get() T
	Set(value T)
}

// DataManager 一维数据管理器（底层存储核心）
type DataManager[T Number] interface {
	// 基础属性方法
	Length() int    // 获取数据长度
	String() string // 返回数据的字符串表示

	// 数据访问方法
	Get(index int) T              // 获取指定索引处的元素值
	Set(index int, value T)       // 设置指定索引处的元素值
	Increment(index int, value T) // 增量更新指定索引处的元素值

	// 数据操作和转换方法
	DataCopy() []T // 返回数据的切片副本
	DataPtr() []T  // 返回数据的切片引用（直接操作底层数据）

	// 数据修改方法
	Zero()                                 // 清空所有数据
	ZeroInPlace()                          // 原地将所有元素设置为零
	FillInPlace(value T)                   // 原地填充所有元素为指定值
	AppendInPlace(values ...T)             // 原地追加元素（不创建新对象）
	InsertInPlace(index int, values ...T)  // 在指定位置原地插入元素
	RemoveInPlace(index int, count int)    // 从指定位置原地移除指定数量的元素
	ReplaceInPlace(index int, values ...T) // 从指定位置原地替换元素

	// 大小调整方法
	Resize(length int)           // 调整数据长度（可能重新分配内存）
	ResizeInPlace(newLength int) // 原地调整数据长度（不重新分配内存）

	// 统计和复制方法
	NonZeroCount() int          // 统计非零元素数量
	Copy(target DataManager[T]) // 复制数据到目标管理器
}

// 向量接口定义
type Vector[T Number] interface {
	// 基础属性方法
	Base() Vector[T] // 获取底层
	Length() int     // 获取向量长度
	String() string  // 格式化字符串输出

	// 数据访问方法
	Get(index int) T              // 获取指定索引元素值
	Set(index int, value T)       // 设置指定索引元素值
	Increment(index int, value T) // 增量更新元素（value累加）

	// 数据操作和转换方法
	ToDense() []T             // 转换为稠密切片
	BuildFromDense(dense []T) // 从稠密切片构建向量

	// 数据修改方法
	Zero()            // 清空向量为零向量
	Copy(a Vector[T]) // 复制自身数据到目标向量a

	// 数学运算方法
	DotProduct(other Vector[T]) T // 计算与另一个向量的点积
	Scale(scalar T)               // 向量缩放（所有元素乘scalar）
	Add(other Vector[T])          // 向量加法（自身 += 另一个向量）

	// 统计方法
	NonZeroCount() int // 统计非零元素数量
	MaxAbs() T         // 获取向量中绝对值最大的元素
}

// 可更新向量接口（支持缓存与回溯）
type UpdateVector[T Number] interface {
	Vector[T]
	Update()   // 缓存数据刷到底层存储
	Rollback() // 回溯操作（清空缓存，放弃修改）
}

// 矩阵接口定义
type Matrix[T Number] interface {
	// 基础属性方法
	Base() Matrix[T] // 获取底层
	Rows() int       // 获取矩阵行数
	Cols() int       // 获取矩阵列数
	String() string  // 格式化字符串输出
	IsSquare() bool  // 判断是否为方阵（行数=列数）

	// 数据访问方法
	Get(row, col int) T                // 获取指定行列元素值
	Set(row, col int, value T)         // 设置指定行列元素值
	Increment(row, col int, value T)   // 增量更新元素
	GetRow(row int) ([]int, Vector[T]) // 获取指定行非零元素（列索引+值向量）

	// 数据操作和转换方法
	ToDense() Vector[T]         // 转换为稠密向量（行优先展开）
	BuildFromDense(dense [][]T) // 从稠密矩阵构建

	// 数据修改方法
	Zero()                   // 清空矩阵为零矩阵
	Copy(a Matrix[T])        // 复制自身数据到目标矩阵a
	Resize(rows, cols int)   // 重置矩阵大小和数据（清空所有元素）
	SwapRows(row1, row2 int) // 交换两行

	// 数学运算方法
	MatrixVectorMultiply(x Vector[T]) Vector[T] // 矩阵向量乘法（返回A*x）

	// 统计方法
	NonZeroCount() int // 统计非零元素数量
}

// 可更新矩阵接口（支持缓存与回溯）
type UpdateMatrix[T Number] interface {
	Matrix[T]
	Update()   // 缓存数据刷到底层存储
	Rollback() // 回溯操作（清空缓存，放弃修改）
}

// LU 接口定义了 LU 分解和求解线性方程组的操作。
type LU[T Number] interface {
	Decompose(matrix Matrix[T]) error // 对输入方阵执行LU分解（A=PLU）
	SolveReuse(b, x Vector[T]) error  // 重用向量求解Ax=b（利用LU分解结果）
}

// MatrixPruner 矩阵精简接口（专注denseMatrix的零行/零列移除与压缩）
type MatrixPruner[T Number] interface {
	// 精简操作
	RemoveZeroRows() Matrix[T]               // 移除全零行（返回精简后的矩阵，不修改原始矩阵）
	RemoveZeroCols() Matrix[T]               // 移除全零列（返回精简后的矩阵，不修改原始矩阵）
	Compress(removeRowsFirst bool) Matrix[T] // 组合精简（先移除零行，再移除零列，可指定顺序）

	// 映射查询
	GetRowMapping() map[int]int                 // 获取「精简后行索引 → 原始行索引」的映射
	GetColMapping() map[int]int                 // 获取「精简后列索引 → 原始列索引」的映射
	GetOriginalRowIndex(newRow int) (int, bool) // 根据精简后行索引，获取原始行索引
	GetOriginalColIndex(newCol int) (int, bool) // 根据精简后列索引，获取原始列索引

	// 缓存访问
	GetPrunedMatrix() Matrix[T] // 获取缓存的精简后矩阵（避免重复计算）
}
