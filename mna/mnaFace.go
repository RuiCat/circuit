package mna

import "circuit/maths"

// NodeID 定义了电路节点的唯一标识符。
type NodeID int

// VoltageID 定义了电压源（包括受控源）的唯一标识符，
// 用于在MNA方程中定位其对应的电流未知量。
type VoltageID int

// Gnd 表示电路的接地节点，其电位为零。
const Gnd NodeID = -1

// Stamp 加盖接口
type Stamp[T maths.Number] interface {
	// GetNodeVoltage 从解向量X中获取并返回指定节点的电压。如果节点为地(Gnd)，则返回0。
	GetNodeVoltage(id NodeID) T

	// GetVoltageSourceCurrent 从解向量X中获取并返回流经指定电压源的电流。如果ID无效，则返回0。
	GetVoltageSourceCurrent(i VoltageID) T

	// StampMatrix 将一个值加到矩阵A的(i,j)元素上。地节点相关的操作将被忽略。
	StampMatrix(i, j NodeID, value T)

	// StampMatrixSet 直接设置矩阵A的(i,j)元素的值，覆盖原有值。地节点相关的操作将被忽略。
	StampMatrixSet(i, j NodeID, value T)

	// StampRightSide 将一个值加到向量Z的第i个元素上。地节点相关的操作将被忽略。
	StampRightSide(node NodeID, value T)

	// StampRightSideSet 直接设置向量Z的第i个元素的值，覆盖原有值。地节点相关的操作将被忽略。
	StampRightSideSet(node NodeID, value T)

	// StampImpedance 为阻抗元件（如电阻）添加MNA加盖。
	// 数学模型: G=1/resistance，在矩阵A的对角元(n1,n1)和(n2,n2)加上G，非对角元(n1,n2)和(n2,n1)减去G。
	//   n1:         元件的第一个节点ID。
	//   n2:         元件的第二个节点ID。
	//   resistance: 阻值（欧姆），必须大于0。
	StampImpedance(n1, n2 NodeID, resistance T)

	// StampAdmittance 为电导元件添加MNA加盖，直接将其电导值g贡献到MNA矩阵A中。
	// 数学模型: 在矩阵A的对角元(n1,n1)和(n2,n2)加上admittance，非对角元(n1,n2)和(n2,n1)减去admittance。
	//   n1:         元件的第一个节点ID。
	//   n2:         元件的第二个节点ID。
	//   admittance: 电导值。
	StampAdmittance(n1, n2 NodeID, admittance T)

	// StampCurrentSource 为独立电流源添加MNA加盖。
	// 数学模型: 电流从n1流向n2，在向量Z的n1位置减去current，n2位置加上current。
	//   n1:      电流源的流出节点ID。
	//   n2:      电流源的流入节点ID。
	//   current: 电流值（安培），正方向为n1→n2。
	StampCurrentSource(n1, n2 NodeID, current T)

	// StampVoltageSource 为独立电压源添加MNA加盖。
	// 数学模型: 引入电流I(id)作为新变量，建立约束 V(n1)-V(n2)=voltage。
	//   n1:      电压源的正极节点ID。
	//   n2:      电压源的负极节点ID。
	//   id:      电压源的唯一ID。
	//   voltage: 电压值（伏特）。
	StampVoltageSource(n1, n2 NodeID, id VoltageID, voltage T)

	// StampVCVS 为电压控制电压源(VCVS)添加MNA加盖。
	// 数学模型: 建立约束 V(on1)-V(on2) = gain × (V(cn1)-V(cn2))。
	//   on1, on2: 输出电压的节点。
	//   cn1, cn2: 控制电压的节点。
	//   id:       VCVS的唯一ID。
	//   gain:     电压增益。
	StampVCVS(on1, on2, cn1, cn2 NodeID, id VoltageID, gain T)

	// StampCCCS 为电流控制电流源(CCCS)添加MNA加盖。
	// 数学模型: 输出电流 I(out) = gain × I(control)。
	//   n1, n2:      输出电流的节点。
	//   controlVSID: 控制电流所在支路的电压源ID。
	//   gain:        电流增益。
	StampCCCS(n1, n2 NodeID, controlVSID VoltageID, gain T)

	// StampCCVS 为电流控制电压源(CCVS)添加MNA加盖。
	// 数学模型: 建立约束 V(on1)-V(on2) = gain × I(control)。
	//   on1, on2:    输出电压的节点。
	//   controlVSID: 控制电流所在支路的电压源ID。
	//   id:          CCVS的唯一ID。
	//   gain:        跨阻增益（欧姆）。
	StampCCVS(on1, on2 NodeID, controlVSID VoltageID, id VoltageID, gain T)

	// StampVCCS 为电压控制电流源(VCCS)添加MNA加盖。
	// 数学模型: 输出电流 I(out) = gain × (V(vn1)-V(vn2))。
	//   cn1:      输出电流的流出节点ID。
	//   cn2:      输出电流的流入节点ID。
	//   vn1, vn2: 控制电压的节点。
	//   gain:     跨导增益。
	StampVCCS(cn1 NodeID, cn2 NodeID, vn1 NodeID, vn2 NodeID, gain T)

	// UpdateVoltageSource 更新一个已存在的电压源（独立或受控）的电压值。
	// 此操作仅修改向量Z中对应的项。
	UpdateVoltageSource(id VoltageID, voltage T)

	// IncrementVoltageSource 在一个已存在的电压源（独立或受控）的电压值上增加一个增量。
	// 此操作仅修改向量Z中对应的项。
	IncrementVoltageSource(id VoltageID, increment T)
}

// UpdateFace 扩展了 MNAFace 接口，增加了在迭代计算（如时域分析）中管理状态变更的能力。
// 它提供了对MNA矩阵、已知向量和解向量进行修改、应用和回滚的方法，
// 从而支持高效的增量式更新，避免完全重构方程组。
type UpdateFace[T maths.Number] interface {
	MNAFace[T]
	Update()    // Update 将暂存的修改应用到矩阵A和向量Z。
	Rollback()  // Rollback 丢弃暂存的修改，恢复A和Z。
	UpdateX()   // UpdateX 将暂存的修改应用到解向量X。
	RollbackX() // RollbackX 丢弃对X的暂存修改。
}

// MNAFace (Modified Nodal Analysis Interface) 定义了构建和操作电路MNA方程（Ax=Z）所需的核心功能。
// 它提供了一系列“加盖”(Stamp)方法，用于根据电路元件的特性来填充MNA矩阵A和向量Z。
// 此外，它还提供了查询节点电压、支路电流以及系统矩阵的方法。
type MNAFace[T maths.Number] interface {
	// Stamp 加盖接口
	Stamp[T]
	// String 返回MNA求解器的内部状态的字符串表示，包括矩阵A、向量Z和解向量X，主要用于调试。
	String() string

	// GetA 返回MNA方程 (Ax=Z) 中的矩阵A。
	GetA() maths.Matrix[T]

	// GetZ 返回MNA方程 (Ax=Z) 中的已知向量Z。
	GetZ() maths.Vector[T]

	// GetX 返回MNA方程 (Ax=Z) 的解向量X，其中包含节点电压和支路电流。
	GetX() maths.Vector[T]

	// Zero 将MNA系统（矩阵A、向量Z和X）重置为零，以便重新构建电路方程。
	Zero()

	// GetNodeNum 获取电路中独立节点的数量（不包括地节点）。
	GetNodeNum() int

	// GetVoltageSourcesNum 获取电路中电压源和受控源的总数，这决定了MNA矩阵的扩展维度。
	GetVoltageSourcesNum() int
}
