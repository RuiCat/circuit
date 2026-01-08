package mna

import "circuit/maths"

// NodeID 定义了电路节点的唯一标识符。
type NodeID int

// VoltageID 定义了电压源（包括受控源）的唯一标识符，
// 用于在MNA方程中定位其对应的电流未知量。
type VoltageID int

// Gnd 表示电路的接地节点，其电位为零。
const Gnd NodeID = -1

// UpdateMNA 扩展了 MNA 接口，提供了对MNA矩阵和向量进行更新与回滚的功能。
// 这对于需要迭代计算或状态管理的仿真（如时域分析）至关重要。
type UpdateMNA interface {
	MNA
	Update()    // Update 将暂存的修改应用到矩阵A和向量Z。
	Rollback()  // Rollback 丢弃暂存的修改，恢复A和Z。
	UpdateX()   // UpdateX 将暂存的修改应用到解向量X。
	RollbackX() // RollbackX 丢弃对X的暂存修改。
}

// MNA (Modified Nodal Analysis) 接口定义了构建和操作电路方程（Ax=Z）所需的核心功能。
// 它通过一系列“加盖”(Stamp)操作来构建MNA矩阵，并最终求解得到节点电压和支路电流。
type MNA interface {
	// String 返回MNA求解器的内部状态的字符串表示，包括矩阵A、向量Z和解向量X，主要用于调试。
	String() string

	// GetA 返回MNA方程 (Ax=Z) 中的矩阵A。
	GetA() maths.Matrix[float64]

	// GetZ 返回MNA方程 (Ax=Z) 中的已知向量Z。
	GetZ() maths.Vector[float64]

	// GetX 返回MNA方程 (Ax=Z) 的解向量X，其中包含节点电压和支路电流。
	GetX() maths.Vector[float64]

	// Zero 将MNA系统（矩阵A、向量Z和X）重置为零，以便重新构建电路方程。
	Zero()

	// GetNodeVoltage 从解向量X中获取并返回指定节点的电压。如果节点为地(Gnd)，则返回0。
	GetNodeVoltage(i NodeID) float64

	// GetNodeCurrent 从解向量X中获取并返回流经指定电压源的电流。如果ID无效，则返回0。
	GetNodeCurrent(i VoltageID) float64

	// GetNodeNum 获取电路中独立节点的数量（不包括地节点）。
	GetNodeNum() int

	// GetVoltageSourcesNum 获取电路中电压源和受控源的总数，这决定了MNA矩阵的扩展维度。
	GetVoltageSourcesNum() int

	// StampMatrix 将一个值加到矩阵A的(i,j)元素上。地节点相关的操作将被忽略。
	StampMatrix(i, j NodeID, value float64)

	// StampMatrixSet 直接设置矩阵A的(i,j)元素的值，覆盖原有值。地节点相关的操作将被忽略。
	StampMatrixSet(i, j NodeID, v float64)

	// StampRightSide 将一个值加到向量Z的第i个元素上。地节点相关的操作将被忽略。
	StampRightSide(i NodeID, value float64)

	// StampRightSideSet 直接设置向量Z的第i个元素的值，覆盖原有值。地节点相关的操作将被忽略。
	StampRightSideSet(i NodeID, v float64)

	// StampImpedance 为阻抗元件（如电阻）添加MNA加盖。
	// 数学模型: G=1/r，在矩阵A的对角元(n1,n1)和(n2,n2)加上G，非对角元(n1,n2)和(n2,n1)减去G。
	//   n1: 元件的第一个节点ID。
	//   n2: 元件的第二个节点ID。
	//   r:  阻值（欧姆），必须大于0。
	StampImpedance(n1, n2 NodeID, r float64)

	// StampAdmittance 为电导元件添加MNA加盖，直接将其电导值g贡献到MNA矩阵A中。
	// 数学模型: 在矩阵A的对角元(n1,n1)和(n2,n2)加上g，非对角元(n1,n2)和(n2,n1)减去g。
	//   n1: 元件的第一个节点ID。
	//   n2: 元件的第二个节点ID。
	//   g:  电导值。
	StampAdmittance(n1, n2 NodeID, g float64)

	// StampCurrentSource 为独立电流源添加MNA加盖。
	// 数学模型: 电流从n1流向n2，在向量Z的n1位置减去i，n2位置加上i。
	//   n1: 电流源的流出节点ID。
	//   n2: 电流源的流入节点ID。
	//   i:  电流值（安培），正方向为n1→n2。
	StampCurrentSource(n1, n2 NodeID, i float64)

	// StampVoltageSource 为独立电压源添加MNA加盖。
	// 数学模型: 引入电流I(vs)作为新变量，建立约束 V(n1)-V(n2)=v。
	//   n1: 电压源的正极节点ID。
	//   n2: 电压源的负极节点ID。
	//   vs: 电压源的唯一ID。
	//   v:  电压值（伏特）。
	StampVoltageSource(n1, n2 NodeID, vs VoltageID, v float64)

	// StampVCVS 为电压控制电压源(VCVS)添加MNA加盖。
	// 数学模型: 建立约束 V(on1)-V(on2) = gain × (V(cn1)-V(cn2))。
	//   on1, on2: 输出电压的节点。
	//   cn1, cn2: 控制电压的节点。
	//   vs:       VCVS的唯一ID。
	//   gain:     电压增益。
	StampVCVS(on1, on2, cn1, cn2 NodeID, vs VoltageID, gain float64)

	// StampCCCS 为电流控制电流源(CCCS)添加MNA加盖。
	// 数学模型: 输出电流 I(out) = gain × I(control)。
	//   n1, n2: 输出电流的节点。
	//   vs:     控制电流所在支路的电压源ID。
	//   gain:   电流增益。
	StampCCCS(n1, n2 NodeID, vs VoltageID, gain float64)

	// StampCCVS 为电流控制电压源(CCVS)添加MNA加盖。
	// 数学模型: 建立约束 V(on1)-V(on2) = gain × I(control)。
	//   on1, on2: 输出电压的节点。
	//   cs:       控制电流所在支路的电压源ID。
	//   vs:       CCVS的唯一ID。
	//   gain:     跨阻增益（欧姆）。
	StampCCVS(on1 NodeID, on2 NodeID, cs VoltageID, vs VoltageID, gain float64)

	// StampVCCS 为电压控制电流源(VCCS)添加MNA加盖。
	// 数学模型: 输出电流 I(out) = gain × (V(vn1)-V(vn2))。
	//   cn1:      输出电流的流出节点ID。
	//   cn2:      输出电流的流入节点ID。
	//   vn1, vn2: 控制电压的节点。
	//   gain:     跨导增益。
	StampVCCS(cn1 NodeID, cn2 NodeID, vn1 NodeID, vn2 NodeID, gain float64)

	// UpdateVoltageSource 更新一个已存在的电压源（独立或受控）的电压值。
	// 此操作仅修改向量Z中对应的项。
	UpdateVoltageSource(vs VoltageID, v float64)

	// IncrementVoltageSource 在一个已存在的电压源（独立或受控）的电压值上增加一个增量。
	// 此操作仅修改向量Z中对应的项。
	IncrementVoltageSource(vs VoltageID, v float64)
}
