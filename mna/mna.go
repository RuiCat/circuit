package mna

import (
	"circuit/maths"
	"fmt"
	"math"
)

// updateMNA 带更新功能的MNA求解器结构体
// 继承MNA接口，同时提供矩阵和向量的更新/回滚功能
// 用于支持迭代求解和状态管理
type updateMNA struct {
	MNA                    // 嵌入MNA接口，继承所有MNA方法
	A   maths.UpdateMatrix // 可更新的求解矩阵A，支持缓存和回滚
	Z   maths.UpdateVector // 可更新的已知向量Z，支持缓存和回滚
	X   maths.UpdateVector // 可更新的未知向量X，支持缓存和回滚
}

// Zero 重置所有矩阵和向量为零状态
// 将求解矩阵A、已知向量Z和未知向量X的所有元素设置为零
func (mna *updateMNA) Zero() {
	mna.A.Zero()
	mna.Z.Zero()
	mna.X.Zero()
}

// Update 将缓存的数据更新到底层存储
// 将矩阵A和向量Z的修改从缓存提交到底层数据结构
// 用于在迭代求解中确认当前修改
func (mna *updateMNA) Update() {
	mna.A.Update()
	mna.Z.Update()
}

// Rollback 回滚矩阵A和向量Z的修改
// 放弃缓存中的修改，恢复到上一次Update之前的状态
// 用于在迭代求解失败时撤销当前修改
func (mna *updateMNA) Rollback() {
	mna.A.Rollback()
	mna.Z.Rollback()
}

// UpdateX 将未知向量X的缓存数据更新到底层存储
// 将解向量X的修改从缓存提交到底层数据结构
func (mna *updateMNA) UpdateX() {
	mna.X.Update()
}

// RollbackX 回滚未知向量X的修改
// 放弃解向量X缓存中的修改，恢复到上一次UpdateX之前的状态
func (mna *updateMNA) RollbackX() {
	mna.X.Rollback()
}

// NewUpdateMNA 创建带更新功能的MNA求解器实例
// 参数NodesNum: 电路节点数量（不含地节点）
// 参数VoltageSourcesNum: 独立电压源和受控源的总数量
// 返回：UpdateMNA接口实例，支持矩阵和向量的更新/回滚操作
func NewUpdateMNA(NodesNum, VoltageSourcesNum int) UpdateMNA {
	n := NodesNum + VoltageSourcesNum // 总方程数量
	// 创建可更新的矩阵和向量
	A := maths.NewUpdateMatrixPtr(maths.NewDenseMatrix(n, n))
	Z := maths.NewUpdateVectorPtr(maths.NewDenseVector(n))
	X := maths.NewUpdateVectorPtr(maths.NewDenseVector(n))
	return &updateMNA{
		MNA: &mna{
			A:                 A,
			Z:                 Z,
			X:                 X,
			NodesNum:          NodesNum,
			VoltageSourcesNum: VoltageSourcesNum,
		},
		A: A,
		Z: Z,
		X: X,
	}
}

// mna 基础MNA求解器结构体，实现MNA接口
// 存储电路方程的矩阵表示，支持各类电路元件的MNA加盖操作
type mna struct {
	A                 maths.Matrix // 求解矩阵A，维度为(NodesNum+VoltageSourcesNum)×(NodesNum+VoltageSourcesNum)
	Z                 maths.Vector // 已知向量Z，维度为(NodesNum+VoltageSourcesNum)×1
	X                 maths.Vector // 未知向量X，存储节点电压和电压源电流，维度与Z一致
	NodesNum          int          // 电路节点数量（不含地节点）
	VoltageSourcesNum int          // 独立电压源和受控源的总数量（扩展未知量）
}

// NewMNA 创建基础MNA求解器实例
// 参数nodesNum: 电路节点数量（不含地节点）
// 参数vsNum: 独立电压源和受控源的总数量（扩展未知量）
// 返回：MNA接口实例，支持电路方程的构建和求解
func NewMNA(nodesNum, vsNum int) MNA {
	n := nodesNum + vsNum // 总方程数量
	return &mna{
		A:                 maths.NewDenseMatrix(n, n), // 创建稠密矩阵
		Z:                 maths.NewDenseVector(n),    // 创建稠密向量
		X:                 maths.NewDenseVector(n),    // 创建稠密向量
		NodesNum:          nodesNum,
		VoltageSourcesNum: vsNum,
	}
}

// ------------------------------ 基础矩阵/向量操作 ------------------------------

// GetA 获取求解矩阵A
// 返回：数学矩阵接口，可用于直接访问和操作矩阵A
func (m *mna) GetA() maths.Matrix { return m.A }

// GetZ 获取已知向量Z
// 返回：数学向量接口，可用于直接访问和操作向量Z
func (m *mna) GetZ() maths.Vector { return m.Z }

// GetX 获取未知向量X
// 返回：数学向量接口，可用于直接访问和操作向量X
func (m *mna) GetX() maths.Vector { return m.X }

// Zero 重置所有矩阵和向量为零状态
// 将矩阵A、向量Z和向量X的所有元素设置为零，用于重新构建电路方程
func (m *mna) Zero() {
	m.A.Zero()
	m.Z.Zero()
	m.X.Zero()
}

// GetNodeNum 获取电路节点数量（不含地节点）
// 返回：电路中的独立节点数量
func (m *mna) GetNodeNum() int { return m.NodesNum }

// GetVoltageSourcesNum 获取电压源和受控源的总数量
// 返回：扩展未知量的数量，即独立电压源和受控源的总数
func (m *mna) GetVoltageSourcesNum() int { return m.VoltageSourcesNum }

// GetNodeVoltage 获取节点电压（地节点返回0）
func (m *mna) GetNodeVoltage(i NodeID) float64 {
	x := int(i)
	if i > Gnd && x < m.NodesNum {
		return m.X.Get(x)
	}
	return 0
}

// GetNodeCurrent 获取电压源/受控源的电流
func (m *mna) GetNodeCurrent(i VoltageID) float64 {
	x := int(i)
	if x > -1 && x < m.VoltageSourcesNum {
		return m.X.Get(m.NodesNum + x)
	}
	return 0
}

// StampMatrix 矩阵A的(i,j)位置叠加值（过滤地节点）
func (m *mna) StampMatrix(i, j NodeID, value float64) {
	if i > Gnd && j > Gnd {
		m.A.Increment(int(i), int(j), value)
	}
}

// StampMatrixSet 矩阵A的(i,j)位置设置值（过滤地节点）
func (m *mna) StampMatrixSet(i, j NodeID, v float64) {
	if i > Gnd && j > Gnd {
		m.A.Set(int(i), int(j), v)
	}
}

// StampRightSide 向量Z的i位置叠加值（过滤地节点）
func (m *mna) StampRightSide(i NodeID, value float64) {
	if i > Gnd {
		m.Z.Increment(int(i), value)
	}
}

// StampRightSideSet 向量Z的i位置设置值（过滤地节点）
func (m *mna) StampRightSideSet(i NodeID, v float64) {
	if i > Gnd {
		m.Z.Set(int(i), v)
	}
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
func (m *mna) StampVoltageSource(n1, n2 NodeID, vs VoltageID, v float64) {
	if vs < 0 {
		return
	}
	vsRow := NodeID(vs) + NodeID(m.NodesNum) // 电压源对应矩阵行（扩展未知量）
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
func (m *mna) StampCCCS(cn1, cn2 NodeID, cs VoltageID, gain float64) {
	if cs < 0 {
		return
	}
	csCol := NodeID(cs) + NodeID(m.NodesNum) // 控制电流对应矩阵列
	// 输出电流对节点cn1/cn2的贡献：I_out = G*I_cs
	m.StampMatrix(cn1, csCol, gain)
	m.StampMatrix(cn2, csCol, -gain)
}

// StampVCVS 加盖电压控制电压源 (VCVS)
// on1: 输出电压正端, on2: 输出电压负端
// cn1: 控制电压正端, cn2: 控制电压负端
// vs: 受控源ID（扩展未知量）
// gain: 传输增益G (V(on1)-V(on2) = G*(V(cn1)-V(cn2)))
func (m *mna) StampVCVS(on1, on2, cn1, cn2 NodeID, vs VoltageID, gain float64) {
	if vs < 0 {
		return
	}
	vsRow := NodeID(vs) + NodeID(m.NodesNum) // 受控源对应矩阵行
	// 节点电流方程：电压源电流对输出节点的贡献
	// 只有输出节点（on1, on2）与电压源电流耦合
	m.StampMatrix(on1, vsRow, 1.0)  // 输出正节点：+I(vs)
	m.StampMatrix(on2, vsRow, -1.0) // 输出负节点：-I(vs)
	// VCVS约束方程：V(on1) - V(on2) - gain*(V(cn1)-V(cn2)) = 0
	// 约束方程需要所有四个节点的电压
	m.StampMatrix(vsRow, on1, 1.0)   // V(on1)项
	m.StampMatrix(vsRow, on2, -1.0)  // -V(on2)项
	m.StampMatrix(vsRow, cn1, -gain) // -gain*V(cn1)项
	m.StampMatrix(vsRow, cn2, gain)  // +gain*V(cn2)项
	// 右侧向量为0（受控源无独立电压）
	m.StampRightSideSet(vsRow, 0)
}

// StampCCVS 加盖电流控制电压源 (CCVS)
// on1: 输出电压正端, on2: 输出电压负端
// cs: 控制电流源ID, vs: 受控源ID（扩展未知量）
// gain: 传输增益G (V(on1)-V(on2) = G*I_cs)
func (m *mna) StampCCVS(on1, on2 NodeID, cs, vs VoltageID, gain float64) {
	if vs < 0 {
		return
	}
	vsRow := NodeID(vs) + NodeID(m.NodesNum) // 受控源对应矩阵行
	csCol := NodeID(cs) + NodeID(m.NodesNum) // 控制电流对应矩阵列
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
func (m *mna) UpdateVoltageSource(vs VoltageID, v float64) {
	if vs < 0 {
		return
	}
	vsRow := NodeID(vs) + NodeID(m.NodesNum)
	m.StampRightSideSet(vsRow, v)
}

// IncrementVoltageSource 叠加电压源（独立/受控）的电压值
func (m *mna) IncrementVoltageSource(vs VoltageID, v float64) {
	if vs < 0 {
		return
	}
	vsRow := NodeID(vs) + NodeID(m.NodesNum)
	m.StampRightSide(vsRow, v)
}

// String MNA信息格式化输出
func (m *mna) String() string {
	return fmt.Sprintf("MNA Matrix (rows=%d, cols=%d):\n%s\nZ Vector:\n%s\nX Vector:\n%s\n",
		m.A.Rows(), m.A.Cols(), m.A.String(), m.Z.String(), m.X.String())
}
