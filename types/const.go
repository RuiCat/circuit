package types

// 默认连接常量定义
const (
	ElementGndNodeID  NodeID = -1 // 标记为地
	ElementHeghNodeID NodeID = -2 // 标记为高阻
	ElementHeghWireID WireID = -2 // 引脚未连接标记
)

// 默认参数常量定义
var (
	Tolerance           = 1e-6      // 收敛容差
	MaxIterations       = 50        // 最大迭代错误次数
	MaxOscillationCount = 15        // 最大震荡次数
	MaxGoodIter         = 10        // 最大失败数
	DefaultTimeStep     = 0.000005  // 默认时间步长
	MinTimeStep         = 0.0000001 // 最小时间步长
	MaxTimeStep         = 0.0001    // 最大时间步长
	MaxSparseMNA        = 1000      // 设置超过节点数量后使用稀疏矩阵
)
