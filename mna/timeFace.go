package mna

// Time 仿真时间接口，提供仿真过程中的时间相关信息
// 用于管理仿真时间步长、控制收敛行为，支持自适应步长调整
type Time interface {
	// Time 获取当前仿真时间
	// 返回：当前仿真时间，单位为秒
	Time() float64

	// TimeStep 获取当前时间步长
	// 返回：当前使用的时间步长，单位为秒
	TimeStep() float64

	// MaxTimeStep 获取最大允许步长
	// 返回：仿真器允许的最大时间步长，单位为秒
	// 用于限制步长增长，保证数值稳定性
	MaxTimeStep() float64

	// MinTimeStep 获取最小允许步长
	// 返回：仿真器允许的最小时间步长，单位为秒
	// 用于防止步长过小导致仿真效率过低
	MinTimeStep() float64

	// GoodIterations 获取当前步数
	// 返回：当前时间点已成功完成的迭代次数
	// 用于收敛判断和步长调整策略
	GoodIterations() int

	// Converged 标记当前迭代已收敛
	// 调用此方法通知时间管理器当前迭代已收敛，可以继续下一步仿真
	Converged()
}
