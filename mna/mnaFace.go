package mna

import "circuit/maths"

// NodeID 电路节点
type NodeID int

// VoltageID 电路电压节点
type VoltageID int

const Gnd NodeID = -1

// UpdateMNA 矩阵操作
type UpdateMNA interface {
	MNA
	Update()    // 更新 A,Z 内容
	Rollback()  // 回溯操作 将 A,Z 内容还原
	UpdateX()   // 更新 X 内容
	RollbackX() // 回溯操作 将 X 内容还原
}

// MNA 接口定义了改进节点分析法(Modified Nodal Analysis)的核心操作，
// 用于构建电路的MNA矩阵和向量，并实现各类电路元件的"加盖"(MNA)操作，最终求解 Ax=Z 得到节点电压和支路电流。
type MNA interface {
	// String 格式化输出MNA的矩阵A、向量Z和X的完整信息，便于调试和日志打印
	String() string

	// GetA 获取MNA求解的核心矩阵A，维度为 (节点数+电压源数) × (节点数+电压源数)
	GetA() maths.Matrix

	// GetZ 获取MNA方程的右侧已知向量Z，维度为 (节点数+电压源数) × 1
	GetZ() maths.Vector

	// GetX 获取MNA方程的解向量X，存储节点电压和电压源/受控源的支路电流，维度与Z一致
	GetX() maths.Vector

	// Zero 重置MNA的矩阵A、向量Z和X为全零，用于重新构建电路方程
	Zero()

	// GetNodeVoltage 获取指定节点的电压值，地节点(Gnd)返回0
	// 参数i: 目标节点的ID（NodeID类型）
	GetNodeVoltage(i NodeID) float64

	// GetNodeCurrent 获取指定电压源的电流值，地节点(Gnd)返回0
	// 参数i: 目标电压源的ID（VoltageID类型）
	GetNodeCurrent(i VoltageID) float64

	// GetNodeNum 获取电路的独立节点数量（不含地节点）
	GetNodeNum() int

	// GetVoltageSourcesNum 获取电路中独立电压源和受控源的总数量（MNA扩展未知量的数量）
	GetVoltageSourcesNum() int

	// StampMatrix 在矩阵A的(i,j)位置叠加指定数值，地节点(Gnd)会被过滤不参与计算
	// 参数i: 矩阵行索引（对应节点/源ID）
	// 参数j: 矩阵列索引（对应节点/源ID）
	// 参数value: 要叠加的数值
	StampMatrix(i, j NodeID, value float64)

	// StampMatrixSet 在矩阵A的(i,j)位置直接设置指定数值，覆盖原有值，地节点(Gnd)会被过滤不参与计算
	// 参数i: 矩阵行索引（对应节点/源ID）
	// 参数j: 矩阵列索引（对应节点/源ID）
	// 参数v: 要设置的数值
	StampMatrixSet(i, j NodeID, v float64)

	// StampRightSide 在右侧向量Z的第i个位置叠加指定数值，地节点(Gnd)会被过滤不参与计算
	// 参数i: 向量Z的索引（对应节点/源ID）
	// 参数value: 要叠加的数值
	StampRightSide(i NodeID, value float64)

	// StampRightSideSet 在右侧向量Z的第i个位置直接设置指定数值，覆盖原有值，地节点(Gnd)会被过滤不参与计算
	// 参数i: 向量Z的索引（对应节点/源ID）
	// 参数v: 要设置的数值
	StampRightSideSet(i NodeID, v float64)

	// StampResistor 对电阻元件执行MNA加盖操作，将电阻的电导贡献写入矩阵A
	// 数学模型: G=1/R，矩阵对角元(n1,n1)和(n2,n2)加G，非对角元(n1,n2)和(n2,n1)减G
	// 参数n1: 电阻的第一个节点ID
	// 参数n2: 电阻的第二个节点ID
	// 参数r: 电阻的阻值（单位：欧姆），需大于0
	StampResistor(n1, n2 NodeID, r float64)

	// StampConductance 对电导元件执行MNA加盖操作，将电导贡献直接写入矩阵A
	// 数学模型: 矩阵对角元(n1,n1)和(n2,n2)加G，非对角元(n1,n2)和(n2,n1)减G
	// 参数n1: 电导的第一个节点ID
	// 参数n2: 电导的第二个节点ID
	// 参数g: 电导的电导值
	StampConductance(n1, n2 NodeID, g float64)

	// StampCurrentSource 对独立电流源执行MNA加盖操作，将电流贡献写入右侧向量Z
	// 数学模型: 电流从n1流出、流入n2，向量Z的n1位置减i，n2位置加i
	// 参数n1: 电流源的流出节点ID
	// 参数n2: 电流源的流入节点ID
	// 参数i: 电流源的电流值（单位：安培），正方向为n1→n2
	StampCurrentSource(n1, n2 NodeID, i float64)

	// StampVoltageSource 对独立电压源执行MNA加盖操作，同时更新矩阵A和右侧向量Z
	// 数学模型: V(n1)-V(n2)=v，矩阵A添加电压源的约束行和列，向量Z设置电压值v
	// 参数n1: 电压源的正极节点ID
	// 参数n2: 电压源的负极节点ID
	// 参数vs: 电压源的唯一ID（对应MNA扩展未知量的索引）
	// 参数v: 电压源的电压值（单位：伏特）
	StampVoltageSource(n1, n2 NodeID, vs VoltageID, v float64)

	// StampVCVS 对电压控制电压源(VCVS)执行MNA加盖操作，实现受控源的约束方程
	// 数学模型: V(on1)-V(on2) = gain × (V(cn1)-V(cn2))，通过扩展未知量行写入矩阵A
	// 参数on1: VCVS输出电压的正极节点ID
	// 参数on2: VCVS输出电压的负极节点ID
	// 参数cn1: VCVS控制电压的正极节点ID
	// 参数cn2: VCVS控制电压的负极节点ID
	// 参数vs: VCVS的唯一ID（对应MNA扩展未知量的索引）
	// 参数gain: VCVS的电压传输增益（无量纲）
	StampVCVS(on1, on2, cn1, cn2 NodeID, vs VoltageID, gain float64)

	// StampCCCS 对电流控制电流源(CCCS)执行MNA加盖操作，实现受控源的电流放大约束
	// 数学模型: I(out) = gain × I(control)，通过矩阵A的扩展列写入电流贡献
	// 参数n1: CCCS输出电流的流出节点ID
	// 参数n2: CCCS输出电流的流入节点ID
	// 参数vs: 控制电流对应的电压源/受控源ID（对应MNA扩展未知量的索引）
	// 参数gain: CCCS的电流传输增益（无量纲）
	StampCCCS(n1, n2 NodeID, vs VoltageID, gain float64)

	// StampCCVS 对电流控制电压源(CCVS)执行MNA加盖操作，实现受控源的约束方程
	// 数学模型: V(on1)-V(on2) = gain × I(control)，通过扩展行和列写入矩阵A
	// 参数on1: CCVS输出电压的正极节点ID
	// 参数on2: CCVS输出电压的负极节点ID
	// 参数cs: 控制电流对应的电压源/受控源ID（对应MNA扩展未知量的索引）
	// 参数vs: CCVS的唯一ID（对应MNA扩展未知量的索引）
	// 参数gain: CCVS的传输增益（单位：欧姆）
	StampCCVS(on1 NodeID, on2 NodeID, cs VoltageID, vs VoltageID, gain float64)

	// StampVCCS 对电压控制电流源(VCCS)执行MNA加盖操作，实现受控源的电流约束
	// 数学模型: I(out) = gain × (V(vn1)-V(vn2))，直接将电导贡献写入矩阵A的节点行
	// 参数cn1: VCCS输出电流的流出节点ID
	// 参数cn2: VCCS输出电流的流入节点ID
	// 参数vn1: VCCS控制电压的正极节点ID
	// 参数vn2: VCCS控制电压的负极节点ID
	// 参数gain: VCCS的跨导增益
	StampVCCS(cn1 NodeID, cn2 NodeID, vn1 NodeID, vn2 NodeID, gain float64)

	// UpdateVoltageSource 更新指定电压源/受控源的电压值，仅修改右侧向量Z的对应位置
	// 参数vs: 电压源/受控源的唯一ID（对应MNA扩展未知量的索引）
	// 参数v: 新的电压值（单位：伏特）
	UpdateVoltageSource(vs VoltageID, v float64)

	// IncrementVoltageSource 叠加指定电压源/受控源的电压值，仅修改右侧向量Z的对应位置
	// 参数vs: 电压源/受控源的唯一ID（对应MNA扩展未知量的索引）
	// 参数v: 叠加电压值（单位：伏特）
	IncrementVoltageSource(vs VoltageID, v float64)
}
