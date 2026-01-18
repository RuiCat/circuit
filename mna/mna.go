package mna

import (
	"circuit/maths"
	"fmt"
	"math/cmplx"
)

// MnaUpdate 扩展了 MNA 接口，提供了对MNA矩阵和向量进行更新与回滚的功能。
// 这对于需要迭代计算或状态管理的仿真（如时域分析）至关重要。
type MnaUpdate = UpdateFace[float64]

// Mna (Modified Nodal Analysis) 接口定义了构建和操作电路方程（Ax=Z）所需的核心功能。
// 它通过一系列“加盖”(Stamp)操作来构建Mna矩阵，并最终求解得到节点电压和支路电流。
type Mna = MNAFace[float64]

// MnaUpdateType 实现了 UpdateMNA 接口，封装了一个标准的 MNA 求解器，
// 并为其矩阵和向量提供了更新与回滚的功能。
type MnaUpdateType[T maths.Number] struct {
	*MnaType[T]                       // 嵌入MNA接口，继承所有MNA方法
	A           maths.UpdateMatrix[T] // 可更新的求解矩阵A
	Z           maths.UpdateVector[T] // 可更新的已知向量Z
	X           maths.Vector[T]       // 可更新的未知向量X
	LastX       maths.Vector[T]       // 上一个未知向量X
}

// NewMnaUpdate 创建一个带更新功能的MNA求解器实例。
//
//	NodesNum: 电路节点数量（不含地节点）。
//	VoltageSourcesNum: 独立电压源和受控源的总数量。
//	返回:一个新的 UpdateMNA 实例。
func NewMnaUpdate(NodesNum, VoltageSourcesNum int) MnaUpdate {
	n := NodesNum + VoltageSourcesNum // 总方程数量
	// 创建可更新的矩阵和向量
	mna := &MnaUpdateType[float64]{
		MnaType: &MnaType[float64]{
			NodesNum:          NodesNum,
			VoltageSourcesNum: VoltageSourcesNum,
		},
		A:     maths.NewUpdateMatrixPtr(maths.NewDenseMatrix[float64](n, n)),
		Z:     maths.NewUpdateVectorPtr(maths.NewDenseVector[float64](n)),
		X:     maths.NewDenseVector[float64](n),
		LastX: maths.NewDenseVector[float64](n),
	}
	mna.MnaType.A = mna.A
	mna.MnaType.Z = mna.Z
	mna.MnaType.X = mna.X
	return mna
}

// Update 将对矩阵A和向量Z的暂存修改应用到底层数据结构中。
func (mna *MnaUpdateType[T]) Update() {
	mna.A.Update()
	mna.Z.Update()
}

// Rollback 丢弃对矩阵A和向量Z的暂存修改，将其恢复到上次更新或初始状态。
func (mna *MnaUpdateType[T]) Rollback() {
	mna.A.Rollback()
	mna.Z.Rollback()
}

// UpdateX 将对解向量X的暂存修改应用到底层数据结构中。
func (mna *MnaUpdateType[T]) UpdateX() {
	mna.X, mna.LastX = mna.LastX, mna.X
	mna.MnaType.X = mna.X
}

// RollbackX 丢弃对解向量X的暂存修改。
func (mna *MnaUpdateType[T]) RollbackX() {
	mna.X, mna.LastX = mna.LastX, mna.X
	mna.MnaType.X = mna.LastX
}

// MnaType 结构体是 MNA 接口的基础实现，包含了求解电路所需的核心矩阵和向量。
type MnaType[T maths.Number] struct {
	A                 maths.Matrix[T] // 求解矩阵A
	Z                 maths.Vector[T] // 已知向量Z
	X                 maths.Vector[T] // 未知向量X (解)
	NodesNum          int             // 电路节点数量（不含地节点）
	VoltageSourcesNum int             // 独立电压源和受控源的总数量
}

// NewMna 创建一个基础MNA求解器实例。
//
//	nodesNum: 电路节点数量（不含地节点）。
//	vsNum: 独立电压源和受控源的总数量。
//	返回:一个新的 MNA 实例。
func NewMna(nodesNum, vsNum int) Mna {
	n := nodesNum + vsNum // 总方程数量
	return &MnaType[float64]{
		A:                 maths.NewDenseMatrix[float64](n, n),
		Z:                 maths.NewDenseVector[float64](n),
		X:                 maths.NewDenseVector[float64](n),
		NodesNum:          nodesNum,
		VoltageSourcesNum: vsNum,
	}
}

// ------------------------------ 矩阵/向量访问 ------------------------------

func (m *MnaType[T]) GetA() maths.Matrix[T] { return m.A }
func (m *MnaType[T]) GetZ() maths.Vector[T] { return m.Z }
func (m *MnaType[T]) GetX() maths.Vector[T] { return m.X }

// Zero 将MNA系统（矩阵A、向量Z和X）重置为零。
func (m *MnaType[T]) Zero() {
	m.A.Zero()
	m.Z.Zero()
	m.X.Zero()
}

// ------------------------------ 系统信息查询 ------------------------------

func (m *MnaType[T]) GetNodeNum() int           { return m.NodesNum }
func (m *MnaType[T]) GetVoltageSourcesNum() int { return m.VoltageSourcesNum }

// GetNodeVoltage 从解向量X中获取指定节点的电压。
func (m *MnaType[T]) GetNodeVoltage(i NodeID) (zero T) {
	if i > Gnd && int(i) < m.NodesNum {
		return m.X.Get(int(i))
	}
	return zero // 地节点或无效节点返回0
}

// GetVoltageSourceCurrent 从解向量X中获取流经指定电压源的电流。
func (m *MnaType[T]) GetVoltageSourceCurrent(i VoltageID) (zero T) {
	if int(i) > -1 && int(i) < m.VoltageSourcesNum {
		return m.X.Get(m.NodesNum + int(i))
	}
	return zero // 无效ID返回0
}

// ------------------------------ MNA矩阵操作 ------------------------------

// StampMatrix 将一个值加到矩阵A的(i,j)元素上。地节点索引将被忽略。
func (m *MnaType[T]) StampMatrix(i, j NodeID, value T) {
	if i > Gnd && j > Gnd {
		m.A.Increment(int(i), int(j), value)
	}
}

// StampMatrixSet 直接设置矩阵A的(i,j)元素的值。地节点索引将被忽略。
func (m *MnaType[T]) StampMatrixSet(i, j NodeID, v T) {
	if i > Gnd && j > Gnd {
		m.A.Set(int(i), int(j), v)
	}
}

// StampRightSide 将一个值加到向量Z的第i个元素上。地节点索引将被忽略。
func (m *MnaType[T]) StampRightSide(i NodeID, value T) {
	if i > Gnd {
		m.Z.Increment(int(i), value)
	}
}

// StampRightSideSet 直接设置向量Z的第i个元素的值。地节点索引将被忽略。
func (m *MnaType[T]) StampRightSideSet(i NodeID, v T) {
	if i > Gnd {
		m.Z.Set(int(i), v)
	}
}

// ------------------------------ 无源元件加盖 ------------------------------

// StampImpedance 为阻抗元件添加MNA加盖。内部通过计算电导 y=1/z 并调用 StampAdmittance 来实现。
func (m *MnaType[T]) StampImpedance(n1, n2 NodeID, z T) {
	var y T
	switch v := any(z).(type) {
	case float64:
		if v > 1e-9 {
			y = any(1.0 / v).(T)
		} else {
			y = any(1e9).(T) // 避免除零
		}
	case complex128:
		if cmplx.Abs(v) > 1e-9 {
			y = any(1.0 / v).(T)
		} else {
			y = any(complex(0, 1e9)).(T) // 避免除零
		}
	default:
		var one T = any(float64(1.0)).(T)
		var zero T
		if z != zero {
			y = one / z
		} else {
			y = any(float64(1e9)).(T) // 避免除零
		}
	}
	m.StampAdmittance(n1, n2, y)
}

// StampAdmittance 为导纳元件添加MNA加盖，通过修改矩阵A的四个相关元素来反映其对电路的贡献。
func (m *MnaType[T]) StampAdmittance(n1, n2 NodeID, y T) {
	m.StampMatrix(n1, n1, y)
	m.StampMatrix(n2, n2, y)
	m.StampMatrix(n1, n2, -y)
	m.StampMatrix(n2, n1, -y)
}

// ------------------------------ 独立源加盖 ------------------------------

// StampCurrentSource 为独立电流源添加MNA加盖。它通过在向量Z的相应位置上加/减电流值来修改节点方程。
func (m *MnaType[T]) StampCurrentSource(n1, n2 NodeID, i T) {
	m.StampRightSide(n1, -i)
	m.StampRightSide(n2, i)
}

// StampVoltageSource 为独立电压源添加MNA加盖。该操作会引入一个新的电流未知量，并修改矩阵A和向量Z以建立电压约束方程。
func (m *MnaType[T]) StampVoltageSource(n1, n2 NodeID, vs VoltageID, v T) {
	if vs < 0 {
		return
	}
	vsRow := NodeID(vs) + NodeID(m.NodesNum)
	one := any(float64(1.0)).(T)
	// KCL方程: I(vs) 对 n1/n2 节点的贡献
	m.StampMatrix(n1, vsRow, one)
	m.StampMatrix(n2, vsRow, -one)
	// 电压源约束方程: V(n1) - V(n2) = v
	m.StampMatrix(vsRow, n1, one)
	m.StampMatrix(vsRow, n2, -one)
	m.StampRightSideSet(vsRow, v)
}

// ------------------------------ 受控源加盖 ------------------------------

// StampVCCS 为电压控制电流源(VCCS)添加MNA加盖。它修改矩阵A中的四个元素，以建立输出电流和控制电压之间的跨导关系。
func (m *MnaType[T]) StampVCCS(cn1, cn2, vn1, vn2 NodeID, gain T) {
	m.StampMatrix(cn1, vn1, gain)
	m.StampMatrix(cn1, vn2, -gain)
	m.StampMatrix(cn2, vn1, -gain)
	m.StampMatrix(cn2, vn2, gain)
}

// StampCCCS 为电流控制电流源(CCCS)添加MNA加盖。它通过修改矩阵A的两个元素来反映控制电流对输出节点的影响。
func (m *MnaType[T]) StampCCCS(cn1, cn2 NodeID, cs VoltageID, gain T) {
	if cs < 0 {
		return
	}
	csCol := NodeID(cs) + NodeID(m.NodesNum)
	m.StampMatrix(cn1, csCol, gain)
	m.StampMatrix(cn2, csCol, -gain)
}

// StampVCVS 为电压控制电压源(VCVS)添加MNA加盖。它引入一个新的电流未知量，并通过修改矩阵A中的一行和两列来建立电压增益关系。
func (m *MnaType[T]) StampVCVS(on1, on2, cn1, cn2 NodeID, vs VoltageID, gain T) {
	if vs < 0 {
		return
	}
	vsRow := NodeID(vs) + NodeID(m.NodesNum)
	one := any(float64(1.0)).(T)
	// KCL: 电压源电流对输出节点的贡献
	m.StampMatrix(on1, vsRow, one)
	m.StampMatrix(on2, vsRow, -one)
	// VCVS约束方程: V(on1) - V(on2) - gain*(V(cn1)-V(cn2)) = 0
	m.StampMatrix(vsRow, on1, one)
	m.StampMatrix(vsRow, on2, -one)
	m.StampMatrix(vsRow, cn1, -gain)
	m.StampMatrix(vsRow, cn2, gain)
	var zero T
	m.StampRightSideSet(vsRow, zero)
}

// StampCCVS 为电流控制电压源(CCVS)添加MNA加盖。它引入一个新的电流未知量，并通过修改矩阵A中的一行和两列来建立跨阻关系。
func (m *MnaType[T]) StampCCVS(on1, on2 NodeID, cs, vs VoltageID, gain T) {
	if vs < 0 {
		return
	}
	vsRow := NodeID(vs) + NodeID(m.NodesNum)
	csCol := NodeID(cs) + NodeID(m.NodesNum)
	one := any(float64(1.0)).(T)
	// KCL: 电压源电流对输出节点的贡献
	m.StampMatrix(on1, vsRow, one)
	m.StampMatrix(on2, vsRow, -one)
	// CCVS约束方程: V(on1) - V(on2) - gain*I_cs = 0
	m.StampMatrix(vsRow, on1, one)
	m.StampMatrix(vsRow, on2, -one)
	m.StampMatrix(vsRow, csCol, -gain)
	var zero T
	m.StampRightSideSet(vsRow, zero)
}

// ------------------------------ 辅助方法 ------------------------------

// UpdateVoltageSource 更新一个已存在的电压源的电压值。此操作仅修改向量Z中对应的项。
func (m *MnaType[T]) UpdateVoltageSource(vs VoltageID, v T) {
	if vs < 0 {
		return
	}
	vsRow := NodeID(vs) + NodeID(m.NodesNum)
	m.StampRightSideSet(vsRow, v)
}

// IncrementVoltageSource 在一个已存在的电压源的电压值上增加一个增量。此操作仅修改向量Z中对应的项。
func (m *MnaType[T]) IncrementVoltageSource(vs VoltageID, v T) {
	if vs < 0 {
		return
	}
	vsRow := NodeID(vs) + NodeID(m.NodesNum)
	m.StampRightSide(vsRow, v)
}

// String 返回MNA求解器内部状态（矩阵A, 向量Z, X）的字符串表示。
func (m *MnaType[T]) String() string {
	return fmt.Sprintf("MNA Matrix (rows=%d, cols=%d):\n%snZ ector:\n%s\nnX ector:\n%s",
		m.A.Rows(), m.A.Cols(), m.A.String(), m.Z.String(), m.X.String())
}
