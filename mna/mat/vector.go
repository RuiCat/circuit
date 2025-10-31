package mat

// Vector 通用向量接口
// 定向量的基本操作，支持稀疏和密集两种实现
type Vector interface {
	// BuildFromDense 从稠密向量构建向量
	BuildFromDense(dense []float64)
	// Clear 清空向量，重置为零向量
	Clear()
	// Copy 复制向量内容到另一个向量
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
