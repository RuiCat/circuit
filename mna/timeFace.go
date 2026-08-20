package mna

import (
	"circuit/maths"
)

// DerivativeFunc 多变量导数函数类型定义
// 输入：当前状态向量（MNA解向量X）
// 输出：状态导数向量（dx/dt）+ 错误信息
type DerivativeFunc func([]float64) ([]float64, error)

// Trigger 仿真触发点，当仿真时间到达 Time 时触发
type Trigger struct {
	Time      float64 // 触发时间
	Triggered bool    // 是否已触发
}

// SimStatus 表示仿真运行状态
type SimStatus = int32

const (
	StatusRunning  SimStatus = 0 // 正在运行
	StatusPaused   SimStatus = 1 // 已暂停
	StatusStopped  SimStatus = 2 // 已停止
	StatusStepping SimStatus = 3 // 单步模式（执行一步后自动暂停）
)

// Time 仿真时间接口，提供仿真过程中的时间相关信息
// 用于管理仿真时间步长、控制收敛行为，支持自适应步长调整
type Time interface {
	// ------------------------------
	// 通用化参数调整
	// ------------------------------

	// SetTolerances 配置误差容差
	SetTolerances(absTol, relTol float64) error
	// MaxNonlinearIter 返回最大非线性迭代次数
	MaxNonlinearIter() int
	// MaxElemIter 返回最大元件迭代次数
	MaxElemIter() int
	// UpdateResidualHistory 更新残差历史记录
	UpdateResidualHistory()
	// ShouldAdjustStepSize 判断是否需要调整步长
	ShouldAdjustStepSize() bool
	// IsResidualConverged 检查残差是否收敛
	IsResidualConverged() bool
	// IsElemIterExhausted 检查元件迭代是否耗尽
	IsElemIterExhausted() bool
	// SetTime 设置当前仿真时间
	SetTime(time float64)
	// SetTimeStep 设置当前步长
	SetTimeStep(step float64)
	// SetStepLimits 配置步长范围
	SetStepLimits(minStep, maxStep float64) error
	// SetIterationLimits 配置迭代次数限制（控制收敛效率）
	SetIterationLimits(maxNonlinIter, maxElemIter int) error
	// SetMaxTimeSteps 设置最大时间步数限制
	SetMaxTimeSteps(maxSteps int) error
	// ResetTimeStepCount 重置时间步计数
	ResetTimeStepCount()
	// IncrementTimeStepCount 增加时间步计数，返回是否超过限制
	IncrementTimeStepCount() bool
	// IsTimeStepLimitExceeded 检查是否超过时间步限制
	IsTimeStepLimitExceeded() bool

	// ------------------------------
	// 状态查询方法
	// ------------------------------

	// CurrentTime 获取当前仿真时间
	CurrentTime() float64
	// CurrentStep 获取当前自适应步长
	CurrentStep() float64
	// TargetTime 获取仿真目标总时间
	TargetTime() float64
	// Time 获取当前仿真时间
	Time() float64
	// TimeStep 获取当前时间步长
	TimeStep() float64
	// MaxTimeStep 获取最大允许步长
	MaxTimeStep() float64
	// MinTimeStep 获取最小允许步长
	MinTimeStep() float64
	// GoodIterations 获取已成功完成的时间步数
	GoodIterations() int
	// IncrementGoodSteps 递增已成功完成的时间步计数
	IncrementGoodSteps()
	// ResidualNorm 获取当前MNA残差范数
	ResidualNorm() float64
	// Status 返回当前仿真运行状态
	Status() SimStatus
	// SetStatus 设置运行状态
	SetStatus(old, new SimStatus)

	// ------------------------------
	// 多变量3阶预测-校正积分核心
	// ------------------------------

	// Predict 3阶Adams-Bashford预测：计算下一时间步预测状态
	Predict() error
	// CopyPredStateToX 将预测状态复制到MNA解向量X，用于Newton初始猜测
	CopyPredStateToX(x maths.Vector[float64])
	// Correct 3阶Adams-Moulton校正：基于预测值优化状态
	Correct() error
	// ------------------------------
	// 历史数据初始化与更新
	// ------------------------------

	// InitHistory 初始化多变量历史数据（用改进欧拉法生成前3步）
	// 参数：
	//
	//	initialState - 初始状态向量（MNA解向量X的初始值）
	//	derFunc      - 导数计算函数
	InitHistory(initialState []float64, derFunc DerivativeFunc) error
	// BootstrapHistory 在前3个成功步累积历史数据，为3阶Adams方法初始化
	// stepIdx: 0, 1, 2 分别对应 historyStates[2], [1], [0]
	BootstrapHistory(state, deriv []float64, stepIdx int)
	// UpdateHistory 推进历史数据缓存（使用循环缓冲区优化，避免频繁内存分配）
	UpdateHistory()
	// SetCorrStateFromX 从MNA解向量X复制当前收敛状态到校正缓冲区，供UpdateHistory使用
	SetCorrStateFromX(x maths.Vector[float64])
	// CorrState 返回校正状态缓冲区的指针，供外部读取预测-校正积分器中
	// 当前收敛后的状态向量（即解向量 X 的副本）。
	// 该缓冲区在 Correct 完成后由 SetCorrStateFromX 更新。
	CorrState() *[]float64
	// CorrDer 返回校正导数缓冲区的指针，供外部读取预测-校正积分器中
	// 当前收敛后的导数向量（dx/dt）的副本。
	// 该缓冲区在 Correct 完成后由 UpdateHistory 更新。
	CorrDer() *[]float64

	// ------------------------------
	// MNA残差计算
	// ------------------------------

	// CalculateMNAResidual 基于MNA方程计算残差范数和解向量范数
	// 参数：mnaSolver - MNA矩阵求解器（提供A、Z、X）
	CalculateMNAResidual(mnaSolver Mna) error
	// CheckResidualConvergence 检查残差是否收敛
	CheckResidualConvergence()

	// ------------------------------
	// 局部截断误差（LTE）估计与自适应步长
	// ------------------------------

	// EstimateLTE 基于预测/校正状态估计局部截断误差（多变量取最大误差）
	EstimateLTE()
	// AdjustStepSize 基于LTE和残差自适应调整步长
	AdjustStepSize() error

	// ------------------------------
	// 非线性迭代控制（通用状态管理）
	// ------------------------------

	// GetContinuationGmin 返回当前 Gmin 延续值（0 = 关闭），用于 t=0 直流求解的阻尼收敛。
	GetContinuationGmin() float64
	// SetContinuationGmin 设置 Gmin 延续值（仅 TimeMNA 实现使用）。
	SetContinuationGmin(v float64)
	// SetGminFinal 设置 Gmin 延续终值（仅 TimeMNA 实现使用）。
	SetGminFinal(v float64)
	// BeginGminStepping 启动 Gmin 延续（延续值设为起始大值，复位恢复计数）。
	BeginGminStepping()
	// EndGminStepping 结束 Gmin 延续（恢复自然 gmin）。
	EndGminStepping()
	// GminSteppingActive 返回延续是否激活（>0 表示正在步进）。
	GminSteppingActive() bool
	// StepGminDown 下调延续 Gmin 一档，返回是否仍需继续步进。
	StepGminDown() bool
	// RecoverGmin 牛顿未收敛时增大阻尼一档，返回是否还有恢复额度。
	RecoverGmin() bool

	// ResetNonlinearIter 重置非线性迭代状态（每时间步开始时调用）
	ResetNonlinearIter()
	// NextNonlinearIter 推进非线性迭代计数，返回是否未超限
	NextNonlinearIter() bool
	// ResetElemIter 重置单个元件的收敛迭代计数
	ResetElemIter()
	// NextElemIter 推进单个元件迭代计数，返回是否未超限
	NextElemIter() bool
	// NoConverged 标记元件没有收敛
	NoConverged()
	// IsNonlinIterExhausted 检查非线性迭代是否耗尽
	IsNonlinIterExhausted() bool
	// IsConverged 获取全局收敛状态（残差收敛+无未收敛元件）
	IsConverged() bool
	// IsSimulationFinished 检查仿真是否完成（达到目标时间）
	IsSimulationFinished() bool

	// ------------------------------
	// 触发点管理
	// ------------------------------

	// SetTriggers 设置触发点列表
	SetTriggers(triggers []Trigger)
	// Triggers 获取触发点列表
	Triggers() []Trigger

	// ------------------------------
	// 仿真推进核心方法（完整流程闭环）
	// ------------------------------

	// AdvanceTimeStep 推进一个时间步（完整流程：预测-校正-残差-步长调整）
	// 参数：
	//
	//	mnaSolver - MNA矩阵求解器（用于获取A/Z/X，计算残差）
	//	derFunc   - 多变量导数计算函数（dx/dt）
	AdvanceTimeStep(mnaSolver Mna, derFunc DerivativeFunc) error
	// AdvanceTimeSimple 简单时间推进：currentTime += currentStep + 触发点截断 + 目标时间截断
	AdvanceTimeSimple() error
}
