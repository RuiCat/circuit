package mat

// Matrix 通用矩阵接口
// 定义矩阵的基本操作，支持稀疏和密集两种实现
type Matrix interface {
	// BuildFromDense 从稠密矩阵构建矩阵
	BuildFromDense(dense [][]float64)
	// Clear 清空矩阵，重置为零矩阵
	Clear()
	// Cols 返回矩阵列数
	Cols() int
	// Copy 复制矩阵内容到另一个矩阵
	Copy(a Matrix)
	// Get 获取指定位置的元素值
	Get(row int, col int) float64
	// GetRow 获取指定行的所有元素
	GetRow(row int) ([]int, []float64)
	// Increment 增量设置矩阵元素（累加值）
	Increment(row int, col int, value float64)
	// IsSquare 检查矩阵是否为方阵
	IsSquare() bool
	// MatrixVectorMultiply 执行矩阵向量乘法
	MatrixVectorMultiply(x []float64) []float64
	// NonZeroCount 返回非零元素数量
	NonZeroCount() int
	// Rows 返回矩阵行数
	Rows() int
	// Set 设置矩阵元素值
	Set(row int, col int, value float64)
	// String 返回矩阵的字符串表示
	String() string
}
