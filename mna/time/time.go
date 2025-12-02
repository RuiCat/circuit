package time

import (
	"circuit/mna"
	"errors"
	"fmt"
	"math"
)

// 常量定义（通用配置阈值）
const (
	minValidStep   = 1e-12 // 最小有效步长（避免数值下溢）
	maxValidStep   = 1e-2  // 最大有效步长（避免步长过大发散）
	defaultAbsTol  = 1e-6  // 默认绝对误差容差
	defaultRelTol  = 1e-4  // 默认相对误差容差
	defaultSafety  = 0.85  // 默认步长调整安全系数
	defaultMaxNonl = 200   // 默认最大非线性迭代次数
	defaultMaxElem = 100   // 默认单个元件最大收敛迭代次数
)

// 3阶Adams方法系数常量
const (
	// Adams-Bashford预测器系数 (3阶)
	abCoeff1 = 23.0 / 12.0 // 23/12
	abCoeff2 = 16.0 / 12.0 // 16/12
	abCoeff3 = 5.0 / 12.0  // 5/12

	// Adams-Moulton校正器系数 (3阶)
	amCoeff1 = 5.0 / 12.0 // 5/12
	amCoeff2 = 8.0 / 12.0 // 8/12
	amCoeff3 = 1.0 / 12.0 // 1/12

	// 局部截断误差(LTE)系数
	lteCoeffPredictor = 3.0 / 8.0   // 预测器误差系数 C*
	lteCoeffCorrector = -1.0 / 24.0 // 校正器误差系数 C
	lteFactor         = lteCoeffCorrector / (lteCoeffPredictor - lteCoeffCorrector)

	// 步长调整参数
	maxStepScale       = 2.5                              // 最大步长增长倍数
	minStepScale       = 0.4                              // 最小步长缩减倍数
	stepAdjustOrder    = 3                                // 积分阶数
	stepAdjustExponent = 1.0 / float64(stepAdjustOrder+1) // 步长调整指数
)

// TimeMNA 通用时间管理与数值积分核心
// 支持多变量电路状态、MNA残差计算、自适应步长、3阶预测-校正积分
type TimeMNA struct {
	// 时间核心参数
	currentTime float64 // 当前仿真时间（累积值）
	targetTime  float64 // 仿真目标总时间
	currentStep float64 // 当前自适应步长
	minStep     float64 // 最小允许步长
	maxStep     float64 // 最大允许步长

	// 时间步控制
	maxTimeSteps  int // 最大时间步数限制
	timeStepCount int // 当前时间步计数

	// 收敛状态管理
	globalConverged   bool  // 全局收敛标记
	unconvergedElems  []int // 未收敛元件索引列表
	residualConverged bool  // 残差收敛标记
	elementConverged  bool  // 元件收敛标记（用于单个元件迭代）

	// 误差控制参数（外部可配置）
	absTol float64 // 绝对误差容差
	relTol float64 // 相对误差容差
	safety float64 // 步长调整安全系数（0~1）

	// 3阶预测-校正法核心（多变量历史缓存）
	historyStates [3][]float64 // 历史状态向量：state[n], state[n-1], state[n-2]
	historyDers   [3][]float64 // 历史导数向量：der[n], der[n-1], der[n-2]
	historyInited bool         // 历史数据初始化标记（仅初始化一次）

	// MNA残差计算核心（L2范数）
	residualNorm float64    // 残差范数 ||A*X - Z||
	solutionNorm float64    // 解向量范数 ||X||
	residualTol  float64    // 动态残差阈值：absTol + relTol*||X||
	residualHist [3]float64 // 残差历史（用于趋势分析）

	// 非线性迭代控制
	maxNonlinIter  int // 全局最大非线性迭代次数
	currNonlinIter int // 当前非线性迭代计数
	maxElemIter    int // 单个元件最大收敛迭代次数
	currElemIter   int // 当前元件收敛迭代计数

	// 局部截断误差（LTE）相关
	localTruncError  float64 // 局部截断误差估计
	maxErrorQuotient float64 // 步长最大增长倍数
}

// NewTimeMNA 创建通用TimeMNAImpl实例
// 参数：targetTime - 仿真目标总时间
func NewTimeMNA(targetTime float64) (*TimeMNA, error) {
	// 校验目标时间合法性
	if targetTime <= 0 {
		return nil, errors.New("目标仿真时间必须大于0")
	}

	// 初始化默认参数（兼顾精度与效率）
	return &TimeMNA{
		currentTime:       0.0,
		targetTime:        targetTime,
		currentStep:       1e-6,
		minStep:           1e-9,
		maxStep:           5e-4,
		maxTimeSteps:      10000, // 默认最大时间步数
		timeStepCount:     0,
		globalConverged:   true,
		unconvergedElems:  make([]int, 0, 16), // 预分配容量优化
		residualConverged: false,
		absTol:            defaultAbsTol,
		relTol:            defaultRelTol,
		safety:            defaultSafety,
		historyStates:     [3][]float64{},
		historyDers:       [3][]float64{},
		historyInited:     false,
		residualNorm:      0.0,
		solutionNorm:      0.0,
		residualTol:       0.0,
		residualHist:      [3]float64{0, 0, 0},
		maxNonlinIter:     defaultMaxNonl,
		currNonlinIter:    0,
		maxElemIter:       defaultMaxElem,
		currElemIter:      0,
		localTruncError:   0.0,
		maxErrorQuotient:  maxStepScale,
	}, nil
}

// ------------------------------
// 外部配置方法（通用化参数调整）
// ------------------------------

// SetTolerances 配置误差容差（外部可定制精度）
func (t *TimeMNA) SetTolerances(absTol, relTol float64) error {
	if absTol <= 0 || relTol <= 0 {
		return errors.New("容差必须大于0")
	}
	t.absTol = absTol
	t.relTol = relTol
	return nil
}

// ------------------------------
// TransientSimulation 支持方法
// ------------------------------

// MaxNonlinearIter 返回最大非线性迭代次数
func (t *TimeMNA) MaxNonlinearIter() int {
	return t.maxNonlinIter
}

// MaxElemIter 返回最大元件迭代次数
func (t *TimeMNA) MaxElemIter() int {
	return t.maxElemIter
}

// ResetUnconvergedList 重置未收敛元件列表
func (t *TimeMNA) ResetUnconvergedList() {
	t.unconvergedElems = t.unconvergedElems[:0]
	t.globalConverged = true
}

// SetElementConverged 设置元件收敛状态
func (t *TimeMNA) SetElementConverged(converged bool) {
	t.elementConverged = converged
}

// IsElementConverged 检查元件是否收敛
func (t *TimeMNA) IsElementConverged() bool {
	// 检查元件收敛标记和未收敛元件列表
	return t.elementConverged && len(t.unconvergedElems) == 0
}

// UpdateResidualHistory 更新残差历史记录
func (t *TimeMNA) UpdateResidualHistory() {
	// 移动历史记录
	t.residualHist[2] = t.residualHist[1]
	t.residualHist[1] = t.residualHist[0]
	t.residualHist[0] = t.residualNorm
}

// ShouldAdjustStepSize 判断是否需要调整步长
func (t *TimeMNA) ShouldAdjustStepSize() bool {
	// 简单策略：如果残差变化较大或接近收敛极限，则调整步长
	if t.residualHist[0] == 0 && t.residualHist[1] == 0 && t.residualHist[2] == 0 {
		return false // 初始状态
	}
	// 检查残差变化率
	var changeRate float64
	if t.residualHist[1] > 0 {
		changeRate = math.Abs(t.residualHist[0]-t.residualHist[1]) / t.residualHist[1]
	} else {
		changeRate = math.Abs(t.residualHist[0] - t.residualHist[1])
	}
	// 调整条件：残差变化率 > 50% 或残差超过容限10倍
	return changeRate > 0.5 || t.residualNorm > t.residualTol*10
}

// IsResidualConverged 检查残差是否收敛
func (t *TimeMNA) IsResidualConverged() bool {
	return t.residualConverged
}

// IsElemIterExhausted 检查元件迭代是否耗尽
func (t *TimeMNA) IsElemIterExhausted() bool {
	return t.currElemIter >= t.maxElemIter
}

// SetTime 设置当前仿真时间（用于测试和调试）
func (t *TimeMNA) SetTime(time float64) {
	t.currentTime = time
}

// SetTimeStep 设置当前步长（用于测试和调试）
func (t *TimeMNA) SetTimeStep(step float64) {
	t.currentStep = step
}

// SetStepLimits 配置步长范围（适配不同仿真场景）
func (t *TimeMNA) SetStepLimits(minStep, maxStep float64) error {
	if minStep <= 0 || maxStep <= minStep || maxStep > maxValidStep {
		return fmt.Errorf("步长范围无效：需满足 0 < minStep < maxStep ≤ %v", maxValidStep)
	}
	t.minStep = minStep
	t.maxStep = maxStep
	t.currentStep = math.Max(minStep, math.Min(t.currentStep, maxStep))
	return nil
}

// SetIterationLimits 配置迭代次数限制（控制收敛效率）
func (t *TimeMNA) SetIterationLimits(maxNonlinIter, maxElemIter int) error {
	if maxNonlinIter <= 0 || maxElemIter <= 0 {
		return errors.New("迭代次数必须大于0")
	}
	t.maxNonlinIter = maxNonlinIter
	t.maxElemIter = maxElemIter
	return nil
}

// SetMaxTimeSteps 设置最大时间步数限制
func (t *TimeMNA) SetMaxTimeSteps(maxSteps int) error {
	if maxSteps <= 0 {
		return errors.New("最大时间步数必须大于0")
	}
	t.maxTimeSteps = maxSteps
	return nil
}

// ResetTimeStepCount 重置时间步计数
func (t *TimeMNA) ResetTimeStepCount() {
	t.timeStepCount = 0
}

// IncrementTimeStepCount 增加时间步计数，返回是否超过限制
func (t *TimeMNA) IncrementTimeStepCount() bool {
	t.timeStepCount++
	return t.timeStepCount <= t.maxTimeSteps
}

// IsTimeStepLimitExceeded 检查是否超过时间步限制
func (t *TimeMNA) IsTimeStepLimitExceeded() bool {
	return t.timeStepCount > t.maxTimeSteps
}

// ------------------------------
// 状态查询方法（外部监控接口）
// ------------------------------

// CurrentTime 获取当前仿真时间
func (t *TimeMNA) CurrentTime() float64 {
	return t.currentTime
}

// CurrentStep 获取当前自适应步长
func (t *TimeMNA) CurrentStep() float64 {
	return t.currentStep
}

// TargetTime 获取仿真目标总时间
func (t *TimeMNA) TargetTime() float64 {
	return t.targetTime
}

// Time 获取当前仿真时间（实现 TimeMNA 接口）
func (t *TimeMNA) Time() float64 {
	return t.currentTime
}

// TimeStep 获取当前时间步长（实现 TimeMNA 接口）
func (t *TimeMNA) TimeStep() float64 {
	return t.currentStep
}

// MaxTimeStep 获取最大允许步长（实现 TimeMNA 接口）
func (t *TimeMNA) MaxTimeStep() float64 {
	return t.maxStep
}

// MinTimeStep 获取最小允许步长（实现 TimeMNA 接口）
func (t *TimeMNA) MinTimeStep() float64 {
	return t.minStep
}

// GoodIterations 获取当前步数（实现 TimeMNA 接口）
func (t *TimeMNA) GoodIterations() int {
	// 返回已成功完成的时间步数
	return t.timeStepCount
}

// ResidualNorm 获取当前MNA残差范数
func (t *TimeMNA) ResidualNorm() float64 {
	return t.residualNorm
}

// IsConverged 获取全局收敛状态（残差收敛+无未收敛元件）
func (t *TimeMNA) IsConverged() bool {
	return t.globalConverged && t.residualConverged
}

// Converged 标记收敛状态（实现 TimeMNA 接口）
func (t *TimeMNA) Converged() {
	t.globalConverged = true
	t.residualConverged = true
	t.elementConverged = true
}

// IsSimulationFinished 检查仿真是否完成（达到目标时间）
func (t *TimeMNA) IsSimulationFinished() bool {
	return t.currentTime >= t.targetTime
}

// UnconvergedElems 获取未收敛元件索引列表
func (t *TimeMNA) UnconvergedElems() []int {
	return append([]int(nil), t.unconvergedElems...) // 返回拷贝，避免外部修改
}

// ------------------------------
// 多变量3阶预测-校正积分核心
// ------------------------------

// DerivativeFunc 多变量导数函数类型定义
// 输入：当前状态向量（MNA解向量X）
// 输出：状态导数向量（dx/dt）+ 错误信息
type DerivativeFunc func([]float64) ([]float64, error)

// Predict 3阶Adams-Bashford预测：计算下一时间步预测状态
func (t *TimeMNA) Predict() ([]float64, error) {
	if !t.historyInited {
		return nil, errors.New("历史数据未初始化，无法执行预测")
	}
	// 获取历史数据（n: 当前步，n-1: 前1步，n-2: 前2步）
	stateN := t.historyStates[0]
	derN := t.historyDers[0]
	derN1 := t.historyDers[1]
	derN2 := t.historyDers[2]
	h := t.currentStep
	// 校验历史数据维度一致性
	if len(stateN) != len(derN) || len(derN) != len(derN1) || len(derN1) != len(derN2) {
		return nil, fmt.Errorf("历史状态/导数向量维度不一致: state=%d, der=%d, derN1=%d, derN2=%d",
			len(stateN), len(derN), len(derN1), len(derN2))
	}
	// 3阶Adams-Bashford公式（逐元素计算多变量预测值）
	// x_pred[i] = x[n][i] + h*(abCoeff1*der[n][i] - abCoeff2*der[n-1][i] + abCoeff3*der[n-2][i])
	predState := make([]float64, len(stateN))
	for i := range stateN {
		predState[i] = stateN[i] + h*(abCoeff1*derN[i]-abCoeff2*derN1[i]+abCoeff3*derN2[i])
	}
	return predState, nil
}

// Correct 3阶Adams-Moulton校正：基于预测值优化状态
func (t *TimeMNA) Correct(predState []float64, predDer []float64) ([]float64, error) {
	if !t.historyInited {
		return nil, errors.New("历史数据未初始化，无法执行校正")
	}
	// 获取历史数据
	stateN := t.historyStates[0]
	derN := t.historyDers[0]
	derN1 := t.historyDers[1]
	h := t.currentStep
	// 校验维度一致性
	if len(predState) != len(stateN) || len(predDer) != len(stateN) {
		return nil, fmt.Errorf("预测状态/导数与历史状态维度不一致: predState=%d, predDer=%d, stateN=%d",
			len(predState), len(predDer), len(stateN))
	}
	// 3阶Adams-Moulton公式（逐元素计算多变量校正值）
	// x_corr[i] = x[n][i] + h*(amCoeff1*predDer[i] + amCoeff2*der[n][i] - amCoeff3*der[n-1][i])
	corrState := make([]float64, len(stateN))
	for i := range stateN {
		corrState[i] = stateN[i] + h*(amCoeff1*predDer[i]+amCoeff2*derN[i]-amCoeff3*derN1[i])
	}
	return corrState, nil
}

// ------------------------------
// 历史数据初始化与更新（多变量适配）
// ------------------------------

// InitHistory 初始化多变量历史数据（用改进欧拉法生成前3步）
// 参数：
//
//	initialState - 初始状态向量（MNA解向量X的初始值）
//	derFunc      - 导数计算函数
func (t *TimeMNA) InitHistory(initialState []float64, derFunc DerivativeFunc) error {
	// 校验初始状态合法性
	if len(initialState) == 0 {
		return errors.New("初始状态向量不能为空")
	}
	for i, val := range initialState {
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return fmt.Errorf("初始状态向量包含无效值（NaN/Inf）在索引 %d", i)
		}
	}
	stepHalf := t.currentStep / 2 // 半步长提升初始精度
	// t=0，初始状态（state0）和初始导数（der0）
	der0, err := derFunc(initialState)
	if err != nil {
		return fmt.Errorf("计算初始导数失败: %v", err)
	}
	if len(der0) != len(initialState) {
		return fmt.Errorf("初始导数向量与状态向量维度不匹配: der=%d, state=%d", len(der0), len(initialState))
	}
	// t=stepHalf，用改进欧拉法计算（state1, der1）
	// 预测：欧拉法半步
	state1Pred := make([]float64, len(initialState))
	for i := range initialState {
		state1Pred[i] = initialState[i] + stepHalf*der0[i]
	}
	// 校正：梯形积分（用预测状态算导数，取平均）
	der1Pred, err := derFunc(state1Pred)
	if err != nil {
		return fmt.Errorf("计算第二步预测导数失败: %v", err)
	}
	state1 := make([]float64, len(initialState))
	der1 := make([]float64, len(initialState))
	for i := range initialState {
		der1[i] = (der0[i] + der1Pred[i]) / 2
		state1[i] = initialState[i] + stepHalf*der1[i]
	}
	// t=currentStep，用改进欧拉法计算（state2, der2）
	// 预测：欧拉法半步
	state2Pred := make([]float64, len(state1))
	for i := range state1 {
		state2Pred[i] = state1[i] + stepHalf*der1[i]
	}
	// 校正：梯形积分
	der2Pred, err := derFunc(state2Pred)
	if err != nil {
		return fmt.Errorf("计算第三步预测导数失败: %v", err)
	}
	state2 := make([]float64, len(state1))
	der2 := make([]float64, len(state1))
	for i := range state1 {
		der2[i] = (der1[i] + der2Pred[i]) / 2
		state2[i] = state1[i] + stepHalf*der2[i]
	}
	// 填充历史数据（state[n] = state2, state[n-1] = state1, state[n-2] = initialState）
	t.historyStates[0] = state2
	t.historyStates[1] = state1
	t.historyStates[2] = initialState
	t.historyDers[0] = der2
	t.historyDers[1] = der1
	t.historyDers[2] = der0
	t.historyInited = true
	return nil
}

// UpdateHistory 推进历史数据缓存（使用循环缓冲区优化，避免频繁内存分配）
func (t *TimeMNA) UpdateHistory(newState, newDer []float64) error {
	if !t.historyInited {
		return errors.New("历史数据未初始化，无法更新")
	}
	if len(newState) != len(t.historyStates[0]) || len(newDer) != len(t.historyDers[0]) {
		return fmt.Errorf("新状态/导数与历史数据维度不一致: newState=%d, historyState=%d, newDer=%d, historyDer=%d",
			len(newState), len(t.historyStates[0]), len(newDer), len(t.historyDers[0]))
	}
	// 使用循环缓冲区思想：将新数据放在位置0，旧数据向后移动
	// 保存最旧的数据（位置2）的引用，以便重用内存
	oldestState := t.historyStates[2]
	oldestDer := t.historyDers[2]
	// 移动数据：2 <- 1, 1 <- 0
	t.historyStates[2] = t.historyStates[1]
	t.historyDers[2] = t.historyDers[1]
	t.historyStates[1] = t.historyStates[0]
	t.historyDers[1] = t.historyDers[0]
	// 重用最旧的内存或分配新内存
	if len(oldestState) == len(newState) {
		// 重用内存
		copy(oldestState, newState)
		t.historyStates[0] = oldestState
	} else {
		// 分配新内存
		t.historyStates[0] = make([]float64, len(newState))
		copy(t.historyStates[0], newState)
	}
	if len(oldestDer) == len(newDer) {
		// 重用内存
		copy(oldestDer, newDer)
		t.historyDers[0] = oldestDer
	} else {
		// 分配新内存
		t.historyDers[0] = make([]float64, len(newDer))
		copy(t.historyDers[0], newDer)
	}
	return nil
}

// ------------------------------
// MNA残差计算（严格对齐原逻辑，多变量适配）
// ------------------------------

// CalculateMNAResidual 基于MNA方程计算残差范数和解向量范数
// 参数：mnaSolver - MNA矩阵求解器（提供A、Z、X）
func (t *TimeMNA) CalculateMNAResidual(mnaSolver mna.MNA) error {
	if mnaSolver == nil {
		return errors.New("MNA求解器不能为空")
	}
	// 从MNA求解器获取核心数据
	A := mnaSolver.GetA()
	Z := mnaSolver.GetZ()
	X := mnaSolver.GetX()
	// 校验维度一致性（A为N×N矩阵，Z/X为N维向量）
	n := A.Rows()
	if A.Cols() != n || Z.Length() != n || X.Length() != n {
		return fmt.Errorf("MNA维度不匹配：A是%d×%d矩阵，Z/X是%d维向量", A.Rows(), A.Cols(), Z.Length())
	}
	if n == 0 {
		return errors.New("MNA矩阵/向量为空")
	}
	// 计算 A*X（矩阵向量乘法）
	AX := A.MatrixVectorMultiply(X)
	if AX.Length() != n {
		return errors.New("矩阵向量乘法结果维度异常")
	}
	// 计算残差向量 R = A*X - Z 和解向量范数（合并循环优化）
	t.solutionNorm = 0.0
	t.residualNorm = 0.0
	for i := 0; i < n; i++ {
		xVal := X.Get(i)
		t.solutionNorm += xVal * xVal
		residual := AX.Get(i) - Z.Get(i)
		t.residualNorm += residual * residual
	}
	t.solutionNorm = math.Sqrt(t.solutionNorm)
	t.residualNorm = math.Sqrt(t.residualNorm)
	// 计算动态残差收敛阈值
	t.residualTol = t.absTol + t.relTol*t.solutionNorm
	// 更新残差历史
	t.residualHist[2] = t.residualHist[1]
	t.residualHist[1] = t.residualHist[0]
	t.residualHist[0] = t.residualNorm
	return nil
}

// CheckResidualConvergence 检查残差是否收敛
func (t *TimeMNA) CheckResidualConvergence() {
	// 收敛条件：残差范数 ≤ 动态阈值
	if t.residualHist[0] == t.residualHist[1] && t.residualHist[1] == t.residualHist[2] {
		t.residualConverged = true
	} else {
		t.residualConverged = t.residualNorm <= t.residualTol
	}
}

// ------------------------------
// 局部截断误差（LTE）估计与自适应步长
// ------------------------------

// EstimateLTE 基于预测/校正状态估计局部截断误差（多变量取最大误差）
func (t *TimeMNA) EstimateLTE(predState, corrState []float64) error {
	if len(predState) != len(corrState) {
		return errors.New("预测状态与校正状态维度不一致")
	}
	if len(predState) == 0 {
		return errors.New("状态向量为空，无法估计LTE")
	}
	// 多变量场景：取所有元素的最大LTE（保证最严格的误差控制）
	maxLTE := 0.0
	for i := range predState {
		stateDiff := math.Abs(corrState[i] - predState[i])
		lte := math.Abs(lteFactor * stateDiff)
		if lte > maxLTE {
			maxLTE = lte
		}
	}
	t.localTruncError = maxLTE
	return nil
}

// AdjustStepSize 基于LTE和残差自适应调整步长
func (t *TimeMNA) AdjustStepSize() error {
	if t.localTruncError < 0 {
		return errors.New("局部截断误差不能为负")
	}
	// 计算允许误差（基于当前状态向量的最大元素）
	currentState := t.historyStates[0]
	maxState := 0.0
	for _, val := range currentState {
		if absVal := math.Abs(val); absVal > maxState {
			maxState = absVal
		}
	}
	allowableError := t.absTol + t.relTol*maxState
	// 保护性检查：避免除以极小值
	if allowableError < minValidStep {
		allowableError = minValidStep
	}
	// 计算误差商（实际误差 / 允许误差）
	errorQuotient := t.localTruncError / allowableError
	// 步长调整公式：h_new = h_old * (safety / errorQuotient)^(1/(order+1))
	stepScale := math.Pow(t.safety/errorQuotient, stepAdjustExponent)
	// 限制步长变化幅度（避免突变）
	stepScale = math.Max(minStepScale, math.Min(stepScale, maxStepScale))
	newStep := t.currentStep * stepScale
	// 约束步长在合法范围
	newStep = math.Max(t.minStep, math.Min(newStep, t.maxStep))
	// 校验步长有效性
	if newStep < minValidStep {
		return fmt.Errorf("步长过小（%v），已低于数值计算下限（%v）", newStep, minValidStep)
	}
	t.currentStep = newStep
	return nil
}

// ------------------------------
// 非线性迭代控制（通用状态管理）
// ------------------------------

// ResetNonlinearIter 重置非线性迭代状态（每时间步开始时调用）
func (t *TimeMNA) ResetNonlinearIter() {
	t.currNonlinIter = 0
	t.unconvergedElems = t.unconvergedElems[:0]
	t.globalConverged = true
	t.elementConverged = true
}

// NextNonlinearIter 推进非线性迭代计数，返回是否未超限
func (t *TimeMNA) NextNonlinearIter() bool {
	t.currNonlinIter++
	return t.currNonlinIter < t.maxNonlinIter
}

// AddUnconvergedElem 记录未收敛的元件索引
func (t *TimeMNA) AddUnconvergedElem(elemIdx int) {
	if elemIdx < 0 {
		return // 忽略无效索引
	}
	t.unconvergedElems = append(t.unconvergedElems, elemIdx)
	t.globalConverged = false
	t.elementConverged = false
}

// ResetElemIter 重置单个元件的收敛迭代计数
func (t *TimeMNA) ResetElemIter() {
	t.currElemIter = 0
	t.elementConverged = true
}

// NextElemIter 推进单个元件迭代计数，返回是否未超限
func (t *TimeMNA) NextElemIter() bool {
	t.currElemIter++
	return t.currElemIter < t.maxElemIter
}

// IsNonlinIterExhausted 检查非线性迭代是否耗尽
func (t *TimeMNA) IsNonlinIterExhausted() bool {
	return t.currNonlinIter >= t.maxNonlinIter
}

// ------------------------------
// 仿真推进核心方法（完整流程闭环）
// ------------------------------

// initializeIfNeeded 初始化历史数据（如果需要）
func (t *TimeMNA) initializeIfNeeded(mnaSolver mna.MNA, derFunc DerivativeFunc) error {
	if t.historyInited {
		return nil
	}
	// 从MNA求解器获取初始状态向量X（完整向量，通用化）
	xVec := mnaSolver.GetX()
	initialState := make([]float64, xVec.Length())
	validCount := 0
	for i := 0; i < xVec.Length(); i++ {
		val := xVec.Get(i)
		if !math.IsNaN(val) && !math.IsInf(val, 0) {
			initialState[i] = val
			validCount++
		} else {
			initialState[i] = 0.0 // 无效值兜底
		}
	}
	if validCount == 0 {
		return errors.New("MNA解向量无有效初始值，无法启动仿真")
	}
	// 初始化历史数据
	if err := t.InitHistory(initialState, derFunc); err != nil {
		return fmt.Errorf("历史数据初始化失败: %v", err)
	}
	// 初始时间推进到第一个步长
	t.currentTime = t.currentStep
	return nil
}

// performPredictionCorrection 执行预测-校正步骤
func (t *TimeMNA) performPredictionCorrection(derFunc DerivativeFunc) ([]float64, []float64, []float64, error) {
	// 预测步骤：计算预测状态和预测导数
	predState, err := t.Predict()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("预测步骤失败: %v", err)
	}
	predDer, err := derFunc(predState)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("计算预测导数失败: %v", err)
	}
	// 校正步骤：计算校正状态和校正导数
	corrState, err := t.Correct(predState, predDer)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("校正步骤失败: %v", err)
	}
	corrDer, err := derFunc(corrState)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("计算校正导数失败: %v", err)
	}
	return predState, corrState, corrDer, nil
}

// estimateAndAdjustStep 估计误差并调整步长
func (t *TimeMNA) estimateAndAdjustStep(predState, corrState []float64, mnaSolver mna.MNA) error {
	// 估计局部截断误差（LTE）
	if err := t.EstimateLTE(predState, corrState); err != nil {
		return fmt.Errorf("LTE估计失败: %v", err)
	}
	// 计算MNA残差（基于当前MNA解向量）
	if err := t.CalculateMNAResidual(mnaSolver); err != nil {
		return fmt.Errorf("残差计算失败: %v", err)
	}
	// 检查残差收敛
	t.CheckResidualConvergence()
	// 自适应调整步长
	if err := t.AdjustStepSize(); err != nil {
		return fmt.Errorf("步长调整失败: %v", err)
	}
	return nil
}

// AdvanceTimeStep 推进一个时间步（完整流程：预测-校正-残差-步长调整）
// 参数：
//
//	mnaSolver - MNA矩阵求解器（用于获取A/Z/X，计算残差）
//	derFunc   - 多变量导数计算函数（dx/dt）
func (t *TimeMNA) AdvanceTimeStep(mnaSolver mna.MNA, derFunc DerivativeFunc) error {
	// 检查仿真状态
	if t.IsSimulationFinished() {
		return errors.New("仿真已完成，无需继续推进")
	}
	if mnaSolver == nil {
		return errors.New("MNA求解器不能为空")
	}
	if derFunc == nil {
		return errors.New("导数计算函数不能为空")
	}
	// 初始化历史数据（首次调用时）
	if err := t.initializeIfNeeded(mnaSolver, derFunc); err != nil {
		return err
	}
	// 执行预测-校正步骤
	predState, corrState, corrDer, err := t.performPredictionCorrection(derFunc)
	if err != nil {
		return err
	}
	// 估计误差并调整步长
	if err := t.estimateAndAdjustStep(predState, corrState, mnaSolver); err != nil {
		return err
	}
	// 更新历史数据（用校正后的状态/导数）
	if err := t.UpdateHistory(corrState, corrDer); err != nil {
		return fmt.Errorf("历史数据更新失败: %v", err)
	}
	// 推进仿真时间（确保不超过目标时间）
	nextTime := t.currentTime + t.currentStep
	if nextTime > t.targetTime {
		nextTime = t.targetTime
		t.currentStep = nextTime - t.currentTime // 最后一步调整步长
	}
	t.currentTime = nextTime
	return nil
}
