package mna

import (
	"circuit/maths"
	"fmt"
	"math"
)

// updateMNA 带更新的矩阵
type updateMNA struct {
	MNA
	A maths.UpdateMatrix // 求解矩阵
	Z maths.UpdateVector // 已知向量
	X maths.UpdateVector // 未知向量
}

// Zero 重置
func (mna *updateMNA) Zero() {
	mna.A.Zero()
	mna.Z.Zero()
	mna.X.Zero()
}

// Update 更新数据到底层
func (mna *updateMNA) Update() {
	mna.X.Update()
	mna.A.Rollback()
	mna.Z.Rollback()
}

// UpdateStamp 更新线性数据
func (mna *updateMNA) UpdateStamp() {
	mna.A.Update()
	mna.Z.Update()
}

// Rollback 放弃数据
func (mna *updateMNA) Rollback() {
	mna.X.Rollback()
}

// NewUpdateMNA 创建更新求解矩阵
func NewUpdateMNA(NodesNum, VoltageSourcesNum int) UpdateMNA {
	n := NodesNum + VoltageSourcesNum
	a, z, x := maths.NewDenseMatrix(n, n), maths.NewDenseVector(n), maths.NewDenseVector(n)
	return &updateMNA{
		MNA: &mna{
			A:                 a,
			Z:                 z,
			X:                 x,
			NodesNum:          NodesNum,
			VoltageSourcesNum: VoltageSourcesNum,
		},
		A: maths.NewUpdateMatrixPtr(a),
		Z: maths.NewUpdateVectorPtr(z),
		X: maths.NewUpdateVectorPtr(x),
	}
}

// mna 结构体实现 MNA 接口
type mna struct {
	A                 maths.Matrix // 求解矩阵 (NodesNum + VoltageSourcesNum) x (NodesNum + VoltageSourcesNum)
	Z                 maths.Vector // 已知向量 (NodesNum + VoltageSourcesNum)
	X                 maths.Vector // 未知向量 (NodesNum + VoltageSourcesNum)
	NodesNum          int          // 电路节点数量（不含地）
	VoltageSourcesNum int          // 独立电压源 + 受控源数量（扩展未知量）
}

// NewMNA 创建MNA求解器实例
// nodesNum: 电路节点数（不含地）
// vsNum: 独立电压源+受控源数量（扩展未知量）
func NewMNA(nodesNum, vsNum int) MNA {
	n := nodesNum + vsNum
	return &mna{
		A:                 maths.NewDenseMatrix(n, n),
		Z:                 maths.NewDenseVector(n),
		X:                 maths.NewDenseVector(n),
		NodesNum:          nodesNum,
		VoltageSourcesNum: vsNum,
	}
}

// ------------------------------ 基础矩阵/向量操作 ------------------------------
func (m *mna) GetA() maths.Matrix { return m.A }
func (m *mna) GetZ() maths.Vector { return m.Z }
func (m *mna) GetX() maths.Vector { return m.X }

func (m *mna) Zero() {
	m.A.Zero()
	m.Z.Zero()
	m.X.Zero()
}

func (m *mna) GetNodeNum() int           { return m.NodesNum }
func (m *mna) GetVoltageSourcesNum() int { return m.VoltageSourcesNum }

// GetNodeVoltage 获取节点电压（地节点返回0）
func (m *mna) GetNodeVoltage(i NodeID) float64 {
	if i == Gnd {
		return 0
	}
	if int(i) >= m.NodesNum {
		return 0 // 超出节点范围返回0
	}
	return m.X.Get(int(i))
}

// GetVoltageSourceCurrent 获取电压源/受控源的电流（扩展未知量）
func (m *mna) GetVoltageSourceCurrent(vs NodeID) float64 {
	idx := int(vs) + m.NodesNum
	if vs < 0 || idx >= m.VoltageSourcesNum {
		return 0 // 超出矩阵范围返回0
	}
	return m.X.Get(idx)
}

// StampMatrix 矩阵A的(i,j)位置叠加值（过滤地节点）
func (m *mna) StampMatrix(i, j NodeID, value float64) {
	if i == Gnd || j == Gnd {
		return
	}
	m.A.Increment(int(i), int(j), value)
}

// StampMatrixSet 矩阵A的(i,j)位置设置值（过滤地节点）
func (m *mna) StampMatrixSet(i, j NodeID, v float64) {
	if i == Gnd || j == Gnd {
		return
	}
	m.A.Set(int(i), int(j), v)
}

// StampRightSide 向量Z的i位置叠加值（过滤地节点）
func (m *mna) StampRightSide(i NodeID, value float64) {
	if i == Gnd {
		return
	}
	m.Z.Increment(int(i), value)
}

// StampRightSideSet 向量Z的i位置设置值（过滤地节点）
func (m *mna) StampRightSideSet(i NodeID, v float64) {
	if i == Gnd {
		return
	}
	m.Z.Set(int(i), v)
}

// ------------------------------ 无源元件加盖 ------------------------------
// StampResistor 加盖电阻元件 (n1-n2, 阻值r)
func (m *mna) StampResistor(n1, n2 NodeID, r float64) {
	if r <= 0 || math.IsInf(r, 0) {
		return // 无效电阻值
	}
	g := 1.0 / r // 电导
	m.StampConductance(n1, n2, g)
}

// StampConductance 加盖电导元件 (n1-n2, 电导g)
func (m *mna) StampConductance(n1, n2 NodeID, g float64) {
	// 对角元素：+g
	m.StampMatrix(n1, n1, g)
	m.StampMatrix(n2, n2, g)
	// 非对角元素：-g
	m.StampMatrix(n1, n2, -g)
	m.StampMatrix(n2, n1, -g)
}

// ------------------------------ 独立源加盖 ------------------------------
// StampCurrentSource 加盖独立电流源 (从n1流向n2，电流值i)
func (m *mna) StampCurrentSource(n1, n2 NodeID, i float64) {
	m.StampRightSide(n1, -i) // 电流流出n1，节点电流方程减i
	m.StampRightSide(n2, i)  // 电流流入n2，节点电流方程加i
}

// StampVoltageSource 加盖独立电压源 (n1正端, n2负端, vs为源ID, 电压值v)
func (m *mna) StampVoltageSource(n1, n2, vs NodeID, v float64) {
	vsRow := NodeID(int(vs) + m.NodesNum) // 电压源对应矩阵行（扩展未知量）
	// 节点电流方程：I(vs) 对 n1/n2 的贡献
	m.StampMatrix(n1, vsRow, 1.0)
	m.StampMatrix(n2, vsRow, -1.0)
	// 电压源约束方程：V(n1) - V(n2) = v
	m.StampMatrix(vsRow, n1, 1.0)
	m.StampMatrix(vsRow, n2, -1.0)
	m.StampRightSideSet(vsRow, v) // 右侧向量设置电压值
}

// ------------------------------ 受控源加盖（核心） ------------------------------
// StampVCCS 加盖电压控制电流源 (VCCS)
// cn1: 输出电流正端, cn2: 输出电流负端
// vn1: 控制电压正端, vn2: 控制电压负端
// gain: 传输增益G (I_out = G*(V(vn1)-V(vn2)))
func (m *mna) StampVCCS(cn1, cn2, vn1, vn2 NodeID, gain float64) {
	// 输出电流对节点cn1/cn2的贡献：I_out = G*(Vvn1 - Vvn2)
	// 节点电流方程：dI(cn1)/dV(vn1) += G, dI(cn1)/dV(vn2) -= G
	m.StampMatrix(cn1, vn1, gain)
	m.StampMatrix(cn1, vn2, -gain)
	// 节点电流方程：dI(cn2)/dV(vn1) -= G, dI(cn2)/dV(vn2) += G
	m.StampMatrix(cn2, vn1, -gain)
	m.StampMatrix(cn2, vn2, gain)
}

// StampCCCS 加盖电流控制电流源 (CCCS)
// cn1: 输出电流正端, cn2: 输出电流负端
// cs: 控制电流源ID（需为已定义的独立电压源/电流源，对应扩展未知量）
// gain: 传输增益G (I_out = G*I_cs)
func (m *mna) StampCCCS(cn1, cn2, cs NodeID, gain float64) {
	csCol := NodeID(int(cs) + m.NodesNum) // 控制电流对应矩阵列
	// 输出电流对节点cn1/cn2的贡献：I_out = G*I_cs
	m.StampMatrix(cn1, csCol, gain)
	m.StampMatrix(cn2, csCol, -gain)
}

// StampVCVS 加盖电压控制电压源 (VCVS)
// on1: 输出电压正端, on2: 输出电压负端
// cn1: 控制电压正端, cn2: 控制电压负端
// vs: 受控源ID（扩展未知量）
// gain: 传输增益G (V(on1)-V(on2) = G*(V(cn1)-V(cn2)))
func (m *mna) StampVCVS(on1, on2, cn1, cn2, vs NodeID, gain float64) {
	vsRow := NodeID(int(vs) + m.NodesNum) // 受控源对应矩阵行
	// 节点电流方程：I(vs) 对 on1/on2 的贡献
	m.StampMatrix(on1, vsRow, 1.0)
	m.StampMatrix(on2, vsRow, -1.0)
	// VCVS约束方程：V(on1) - V(on2) - G*(V(cn1)-V(cn2)) = 0
	m.StampMatrix(vsRow, on1, 1.0)
	m.StampMatrix(vsRow, on2, -1.0)
	m.StampMatrix(vsRow, cn1, -gain)
	m.StampMatrix(vsRow, cn2, gain)
	// 右侧向量为0（受控源无独立电压）
	m.StampRightSideSet(vsRow, 0)
}

// StampCCVS 加盖电流控制电压源 (CCVS)
// on1: 输出电压正端, on2: 输出电压负端
// cn1: 控制电流正端, cn2: 控制电流负端
// cs: 控制电流源ID, vs: 受控源ID（扩展未知量）
// gain: 传输增益G (V(on1)-V(on2) = G*I_cs)
func (m *mna) StampCCVS(on1, on2, cn1, cn2, cs, vs NodeID, gain float64) {
	vsRow := NodeID(int(vs) + m.NodesNum) // 受控源对应矩阵行
	csCol := NodeID(int(cs) + m.NodesNum) // 控制电流对应矩阵列
	// 节点电流方程：I(vs) 对 on1/on2 的贡献
	m.StampMatrix(on1, vsRow, 1.0)
	m.StampMatrix(on2, vsRow, -1.0)
	// CCVS约束方程：V(on1) - V(on2) - G*I_cs = 0
	m.StampMatrix(vsRow, on1, 1.0)
	m.StampMatrix(vsRow, on2, -1.0)
	m.StampMatrix(vsRow, csCol, -gain)
	// 右侧向量为0
	m.StampRightSideSet(vsRow, 0)
}

// ------------------------------ 辅助方法 ------------------------------
// UpdateVoltageSource 更新电压源（独立/受控）的电压值
func (m *mna) UpdateVoltageSource(vs NodeID, v float64) {
	vsRow := NodeID(int(vs) + m.NodesNum)
	m.StampRightSideSet(vsRow, v)
}

// String MNA信息格式化输出
func (m *mna) String() string {
	return fmt.Sprintf("MNA Matrix (rows=%d, cols=%d):\n%s\nZ Vector:\n%s\nX Vector:\n%s\n",
		m.A.Rows(), m.A.Cols(), m.A.String(), m.Z.String(), m.X.String())
}
