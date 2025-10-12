package types

// ElementGraph 元件图表
type ElementGraph struct {
	*StampTime   // 仿真时间
	*StampConfig // 仿真参数
	// 参数
	Iter                int     // 当前迭代次数
	MaxIter             int     // 最大迭代次数
	MaxGoodIter         int     // 最大失败数
	NumNodes            int     // 电路节点数量
	NumVoltageSources   int     // 独立电压源数量
	ConvergenceTol      float64 // 收敛容差
	OscillationCount    int     // 振荡计数器
	OscillationCountMax int     // 震荡最大值
	// 元件信息
	NodeList    [][]ElementID          // 节点连接
	ElementList map[ElementID]*Element // 元件列表
}

// Zero 初始化
func (ele *ElementGraph) Zero() {
	ele.Iter = 0
	ele.MaxIter = MaxIterations
	ele.MaxGoodIter = MaxGoodIter
	ele.ConvergenceTol = Tolerance
	ele.OscillationCount = 0
	ele.OscillationCountMax = MaxOscillationCount
	ele.StampTime.Zero()
}

// Time 仿真时间
func (ele *ElementGraph) GetTime() *StampTime {
	return ele.StampTime
}

// Config 仿真参数
func (ele *ElementGraph) GetConfig() *StampConfig {
	return ele.StampConfig
}
