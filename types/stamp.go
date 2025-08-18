package types

// NodeID 节点
type NodeID = int

// VoltageID 电压
type VoltageID = int

// PinID 引脚
type PinID = int

// PinList 引脚列表
type PinList []NodeID

// VoltageList 电压索引列表
type VoltageList []VoltageID

// WireID 连接
type WireID int

// WireList 连接列表
type WireList []WireID

// ElementID 元件
type ElementID int

// ElementType 元件类型
type ElementType uint

// Element 元件信息
type Element struct {
	ElementFace  // 元件实现
	*ElementBase // 元件基础信息
}

// Stamp 直流分析矩阵加盖接口
type Stamp interface {
	GetTime() *StampTime                                                          // 仿真时间
	GetConfig() *StampConfig                                                      // 仿真参数
	GetNumNodes() int                                                             // 返回电路节点数量,不包含电压数量
	GetDampingFactor() float64                                                    // 阻尼
	GetNumVoltageSources() int                                                    // 返回电路电压数量
	GetVoltage(i NodeID) float64                                                  // 返回节点电压
	SetVoltage(i NodeID, v float64) error                                         // 设置节点电压
	StampMatrix(i, j NodeID, value float64) error                                 // 在矩阵A的(i,j)位置叠加值
	StampRightSide(i VoltageID, value float64) error                              // 在右侧向量B的i位置叠加值
	StampResistor(n1, n2 NodeID, r float64) error                                 // 加盖电阻元件
	StampConductance(n1, n2 NodeID, g float64) error                              // 加盖电导元件
	StampCurrentSource(n1, n2 NodeID, i float64) error                            // 加盖电流源
	StampVoltageSource(n1, n2 NodeID, vs VoltageID, v float64) error              // 加盖电压源
	StampVCVS(n1, n2 NodeID, coef float64, vs VoltageID) error                    // 加盖电压控制电压源
	StampVCCurrentSource(cn1, cn2 NodeID, vn1, vn2 VoltageID, gain float64) error // 加盖电压控制电流源
	StampCCCS(n1, n2 NodeID, vs VoltageID, gain float64) error                    // 加盖电流控制电流源
	UpdateVoltageSource(n1, n2 NodeID, vs VoltageID, v float64) error             // 更新电压源值
}

// StampTime 仿真时间
type StampTime struct {
	Time           float64 // 当前仿真时间(秒)
	TimeStep       float64 // 当前时间步长(秒)
	MaxTimeStep    float64 // 最大允许步长(秒)
	MinTimeStep    float64 // 最小允许步长(秒)
	GoodIterations int     // 当前步数
}

// Zero 初始化
func (time *StampTime) Zero() {
	time.Time = 0
	time.TimeStep = DefaultTimeStep
}

// StampConfig 仿真参数
type StampConfig struct {
	IsDCAnalysis  bool // DC分析
	IsTrapezoidal bool // 梯形法
}
