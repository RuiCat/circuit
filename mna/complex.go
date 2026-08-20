// Package mna 复数扩展:提供任意数值类型(含 complex128)的 MNA 求解器工厂。
// 时域仿真使用 float64;AC 相量分析 / 潮流计算使用 complex128。
package mna

import "circuit/maths"

// NewMnaT 泛型工厂:创建任意数值类型的基础 MNA 求解器。
// 与原 NewMna 等价,但支持 complex64/complex128,用于相量域求解。
func NewMnaT[T maths.Number](nodesNum, vsNum int) *MnaType[T] {
	n := nodesNum + vsNum // 总方程数量
	return &MnaType[T]{
		A:                 maths.NewDenseMatrix[T](n, n),
		Z:                 maths.NewDenseVector[T](n),
		X:                 maths.NewDenseVector[T](n),
		NodesNum:          nodesNum,
		VoltageSourcesNum: vsNum,
	}
}

// NewMnaUpdateT 泛型工厂:创建带更新/回滚能力的 MNA 求解器。
// 与原 NewMnaUpdate 等价,但支持复数类型。
func NewMnaUpdateT[T maths.Number](nodesNum, vsNum int) *MnaUpdateType[T] {
	n := nodesNum + vsNum // 总方程数量
	mna := &MnaUpdateType[T]{
		MnaType: NewMnaT[T](nodesNum, vsNum),
		A:       maths.NewUpdateMatrixPtr(maths.NewDenseMatrix[T](n, n)),
		Z:       maths.NewUpdateVectorPtr(maths.NewDenseVector[T](n)),
		X:       maths.NewDenseVector[T](n),
		LastX:   maths.NewDenseVector[T](n),
	}
	mna.MnaType.A = mna.A
	mna.MnaType.Z = mna.Z
	mna.MnaType.X = mna.X
	return mna
}

// NewMnaUpdateSparseT 泛型工厂：与 NewMnaUpdateT 相同，但 A 使用 CSR 稀疏基矩阵。
// 供瞬态引擎在稀疏模式下使用（与稀疏 LU 分解器配套）。
func NewMnaUpdateSparseT[T maths.Number](nodesNum, vsNum int) *MnaUpdateType[T] {
	n := nodesNum + vsNum
	mna := &MnaUpdateType[T]{
		MnaType: NewMnaT[T](nodesNum, vsNum),
		A:       maths.NewUpdateMatrixPtr(maths.NewSparseMatrix[T](n, n)),
		Z:       maths.NewUpdateVectorPtr(maths.NewDenseVector[T](n)),
		X:       maths.NewDenseVector[T](n),
		LastX:   maths.NewDenseVector[T](n),
	}
	mna.MnaType.A = mna.A
	mna.MnaType.Z = mna.Z
	mna.MnaType.X = mna.X
	return mna
}

// NewMnaComplex 创建复数(complex128)基础 MNA 求解器,用于 AC 相量分析。
func NewMnaComplex(nodesNum, vsNum int) *MnaType[complex128] {
	return NewMnaT[complex128](nodesNum, vsNum)
}

// NewMnaUpdateComplex 创建带更新/回滚的复数 MNA 求解器。
func NewMnaUpdateComplex(nodesNum, vsNum int) *MnaUpdateType[complex128] {
	return NewMnaUpdateT[complex128](nodesNum, vsNum)
}

// oneOf 返回数值类型 T 的单位元 1。
// 泛型类型断言 any(1.0).(T) 在 T 为复数时 panic(数值类型不自动转换),
// 必须按动态类型分支构造(与 StampImpedance 的模式一致)。
func oneOf[T maths.Number]() T {
	var zero T
	switch any(zero).(type) {
	case float32:
		return any(float32(1.0)).(T)
	case float64:
		return any(float64(1.0)).(T)
	case complex64:
		return any(complex64(1 + 0i)).(T)
	default:
		return any(complex128(1 + 0i)).(T)
	}
}
