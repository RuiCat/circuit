package mna

import "circuit/maths"

// NodeID 电路节点
type NodeID int

// Gnd 地节点
const Gnd NodeID = -1

// MNA 矩阵操作
type MNA interface {
	String() string                                               // 格式化输出
	GetA() maths.Matrix                                           // 求解矩阵
	GetZ() maths.Vector                                           // 已知向量
	GetX() maths.Vector                                           // 未知向量
	Zero()                                                        // 重置
	GetVoltage(i NodeID) float64                                  // 得到节点电压
	GetCurrent(vs NodeID) float64                                 // 得到电压源电流
	GetNodesNum() int                                             // 得到电路节点数量
	GetVoltageSourcesNum() int                                    // 得到电路电压源数量
	StampMatrix(i, j NodeID, value float64)                       // 在矩阵A的(i,j)位置叠加值
	StampMatrixSet(i, j NodeID, v float64)                        // 在矩阵A的(i,j)位置设置值
	StampRightSide(i NodeID, value float64)                       // 在右侧向量B的i位置叠加值
	StampRightSideSet(i NodeID, v float64)                        // 在右侧向量B的i位置设置值
	StampResistor(n1, n2 NodeID, r float64)                       // 加盖电阻元件
	StampConductance(n1, n2 NodeID, g float64)                    // 加盖电导元件
	StampCurrentSource(n1, n2 NodeID, i float64)                  // 加盖电流源
	StampVoltageSource(n1, n2 NodeID, vs NodeID, v float64)       // 加盖电压源
	StampVCVS(n1, n2 NodeID, vs NodeID, coef float64)             // 加盖电压控制电压源
	StampCCCS(n1, n2 NodeID, vs NodeID, gain float64)             // 加盖电流控制电流源
	StampVCCurrentSource(cn1, cn2, vn1, vn2 NodeID, gain float64) // 加盖电压控制电流源
	UpdateVoltageSource(vs NodeID, v float64)                     // 更新电压源值
}

// TimeMNA 仿真时间
type TimeMNA interface {
	Time() float64        // 当前仿真时间(秒)
	TimeStep() float64    // 当前时间步长(秒)
	MaxTimeStep() float64 // 最大允许步长(秒)
	MinTimeStep() float64 // 最小允许步长(秒)
	GoodIterations() int  // 当前步数
}

// ValueMNA 将 ElementBase 封装为接口
type ValueMNA interface {
	TimeMNA                      // 继承时间
	Base() *ElementBase          // 获取基础元件信息
	Update()                     // 更新元件状态
	Rollback()                   // 回滚元件状态
	Nodes(i int) NodeID          // 获取第i个引脚连接的节点
	VoltSource(i int) NodeID     // 获取第i个电压源节点
	NodesInternal(i int) NodeID  // 获取第i个内部节点
	GetFloat64(i int) float64    // 获取第i个浮点数值参数
	GetInt(i int) int            // 获取第i个整数值参数
	SetFloat64(i int, v float64) // 设置第i个浮点数值参数
	SetInt(i int, v int)         // 设置第i个整数值参数
	PinNum() int                 // 获取引脚数量
	ValueNum() int               // 获取参数值数量
	VoltageNum() int             // 获取电压源数量
	InternalNum() int            // 获取内部节点数量
}

// UpdateMNA 矩阵操作
type UpdateMNA interface {
	MNA
	Update()   // 缓存数据刷到底层存储
	Rollback() // 回溯操作（清空缓存，放弃修改）
}

// Element 元件实现
type Element interface {
	StartIteration(mna MNA, base ValueMNA)   // 步长迭代开始
	Stamp(mna MNA, base ValueMNA)            // 加盖线性贡献
	DoStep(mna MNA, base ValueMNA)           // 执行仿真
	CalculateCurrent(mna MNA, base ValueMNA) // 电流计算
	StepFinished(mna MNA, base ValueMNA)     // 步长迭代结束
}

// ElementConfig 元件配置信息
type ElementConfig interface {
	Init() ValueMNA      // 得到底层
	Reset(base ValueMNA) // 元件值初始化
	CirLoad(ValueMNA)    // 网表文件写入值
	CirExport(ValueMNA)  // 网表文件导出值
}
