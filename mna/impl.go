package mna

// NodeID 电路节点
type NodeID int

// ElementType 元件类型
type ElementType uint

// Gnd 地节点
const Gnd NodeID = -1

// MNA 矩阵操作
type MNA interface {
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

// Element 元件实现
type Element interface {
	StartIteration(mna MNA, base *ElementBase)   // 步长迭代开始
	Stamp(mna MNA, base *ElementBase)            // 加盖线性贡献
	DoStep(mna MNA, base *ElementBase)           // 执行仿真
	CalculateCurrent(mna MNA, base *ElementBase) // 电流计算
	StepFinished(mna MNA, base *ElementBase)     // 步长迭代结束
}

// ElementConfig 元件配置信息
type ElementConfig interface {
	Type() ElementType        // 元件类型
	Init() *ElementBase       // 得到底层
	Reset(base *ElementBase)  // 元件值初始化
	CirLoad(netlist *NetList) // 网表文件写入值
	CirExport() *NetList      // 网表文件导出值
}
