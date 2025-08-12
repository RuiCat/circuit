package types

// 默认连接常量定义
const (
	ElementHeghWireID WireID = -1 // 引脚未连接标记
	ElementGndWireID  WireID = -1 // 用于虚拟接地标记
	ElementGndNodeID  NodeID = -1 // 标记为地
	ElementHeghNodeID NodeID = -2 // 标记为高阻
)

// 默认参数常量定义
var (
	Tolerance           = 1e-6     // 收敛容差
	MaxIterations       = 10       // 最大迭代次数
	MaxOscillationCount = 6        // 最大震荡次数
	MaxGoodIter         = 5        // 最大失败数
	DefaultTimeStep     = 0.0005   // 默认时间步长
	MinTimeStep         = 0.000001 // 最小时间步长
	MaxTimeStep         = 0.01     // 最大时间步长
)
