// 收敛与步长策略对象（契约 K3/K4 的 m5 落点）。
//
// 本文件承载三件事，全部是**读出口 + 状态同步**，不改任何算法：
//
//  1. StrategyBinding → TimeMNA 的同步入口 ApplyStrategy：每次 TransientSimulation
//     启动无条件调用一次。`GminFinal <= 0` 的语义是**回默认值**（不是"不修改"）——
//     这是 C5 的用户裁决，也是"同一 Context 连跑两次结果不依赖运行历史"的唯一保证
//     （历史教训：GminFinal 只写不回退会让第二次运行沿用第一次的终值）。
//  2. 有界牛顿事件计数 BoundedNewtonEvents：四个计数器按 K4 的
//     「减半步长重试 / 拒绝当前解 / 元素迭代耗尽 / NaN 拒绝」各一次自增，
//     每次 TransientSimulation 启动重置。计数只增不减，且**不新增任何
//     "强制推进 / 静默接受不收敛解"路径**（P8 已移除那些路径，回退即缺陷）。
//  3. 迭代数读出口：GoodSteps / CurrentStepNonlinearIterations /
//     CurrentStepElemIterations。最后一个是缺陷 D18 的修正出口——
//     器件侧把 GoodIterations()（累计成功步数）当迭代数用，见其方法注释。
package time

import (
	"circuit/element"
	"circuit/mna"
	"fmt"
	"math"
	"sync/atomic"
)

// ------------------------------
// 有界牛顿事件计数（契约 K4）
// ------------------------------

// BoundedNewtonEvents 有界牛顿事件计数（契约 K4，定义在 element/time/strategy.go）。
//
// 四个字段用 atomic.Uint64 承载：主循环写入在仿真 goroutine，读取可能来自
// 外部 goroutine（CLI 收尾、MCP 查询、测试），故按 K4「线程安全读接口」要求原子化。
// 复制本结构体会复制原子值（go vet 会报告 copylocks）——按值传递需先 Copy()。
type BoundedNewtonEvents struct {
	StepHalved    atomic.Uint64 // 因分解/求解失败或拒绝当前解而减半步长重试的次数
	StepRejected  atomic.Uint64 // 当前时间步的解被拒绝（残差不可接受 / 元件迭代耗尽）的次数
	IterExhausted atomic.Uint64 // 元件次级迭代额度耗尽的次数
	NaNRejected   atomic.Uint64 // 检出 NaN/Inf 无效电压而拒绝该解的次数
}

// AddStepHalved 记录一次减半步长重试。
func (e *BoundedNewtonEvents) AddStepHalved() { e.StepHalved.Add(1) }

// AddStepRejected 记录一次「拒绝当前解」。
func (e *BoundedNewtonEvents) AddStepRejected() { e.StepRejected.Add(1) }

// AddIterExhausted 记录一次元件次级迭代额度耗尽。
func (e *BoundedNewtonEvents) AddIterExhausted() { e.IterExhausted.Add(1) }

// AddNaNRejected 记录一次 NaN/Inf 拒绝。
func (e *BoundedNewtonEvents) AddNaNRejected() { e.NaNRejected.Add(1) }

// Reset 把四个计数器清零（每次 TransientSimulation 启动调用一次）。
func (e *BoundedNewtonEvents) Reset() {
	e.StepHalved.Store(0)
	e.StepRejected.Store(0)
	e.IterExhausted.Store(0)
	e.NaNRejected.Store(0)
}

// NewtonEventCounts 是 BoundedNewtonEvents 的无锁快照：四个普通 uint64 字段，
// 可安全按值传递、比较、序列化（BoundedNewtonEvents 本身含 atomic 值，
// 按值复制会被 go vet 判为 copylocks）。
type NewtonEventCounts struct {
	StepHalved    uint64
	StepRejected  uint64
	IterExhausted uint64
	NaNRejected   uint64
}

// Counts 返回四个计数的快照（线程安全读接口，K4）。
func (e *BoundedNewtonEvents) Counts() NewtonEventCounts {
	return NewtonEventCounts{
		StepHalved:    e.StepHalved.Load(),
		StepRejected:  e.StepRejected.Load(),
		IterExhausted: e.IterExhausted.Load(),
		NaNRejected:   e.NaNRejected.Load(),
	}
}

// Total 返回四计数之和（m4 的 C6 口径：收益 = 该和下降）。
func (c NewtonEventCounts) Total() uint64 {
	return c.StepHalved + c.StepRejected + c.IterExhausted + c.NaNRejected
}

// NewtonEvents 返回四计数快照（线程安全读接口）。
func (t *TimeMNA) NewtonEvents() NewtonEventCounts {
	return t.boundedEvents.Counts()
}

// BeginBoundedNewtonEvents 复位四个事件计数（每次 TransientSimulation 启动时调用一次）。
func (t *TimeMNA) BeginBoundedNewtonEvents() {
	t.boundedEvents.Reset()
}

// ------------------------------
// 策略同步（契约 K3/K4/C5）
// ------------------------------

// ApplyStrategy 把策略绑定无条件同步到 TimeMNA（契约 K3 冻结签名，m5 实现）。
//
// 语义（engineConfig.go 文件末尾的冻结原文）：
//   - 每次仿真启动调用一次，**无条件写** TimeMNA（不是"仅当非零才写"）；
//   - s.GminFinal <= 0 → 回默认值 gminStepFinal（不是"不修改"）；>0 为无效值
//     （NaN/Inf）时报错，不回退也不部分生效；
//   - 其它数值字段（NaturalGmin/GlobalGmin）遇 NaN 或**负值** → 报错且
//     **不改动** TimeMNA 现有状态（校验先于任何写入，禁止部分生效）；
//   - 不引入跨 run 残留：写入的每个字段都能在下一次调用时被重新覆盖。
//
// 数值字段的 0 是合法值（NaturalGmin=0 关闭自然 gmin；GlobalGmin=0 关闭全局 gmin），
// 故只有负值被拒；GminFinal<=0 是"回默认"的显式语义，不是错误。
func ApplyStrategy(tm *TimeMNA, s StrategyBinding) error {
	if tm == nil {
		return fmt.Errorf("ApplyStrategy: TimeMNA 不能为空")
	}
	if invalidStrategyValue(s.NaturalGmin) {
		return fmt.Errorf("ApplyStrategy: NaturalGmin 无效（%v），须为有限非负值", s.NaturalGmin)
	}
	if invalidStrategyValue(s.GlobalGmin) {
		return fmt.Errorf("ApplyStrategy: GlobalGmin 无效（%v），须为有限非负值", s.GlobalGmin)
	}
	// GminFinal 的无效值必须先于 "<=0 回默认" 判定：NaN 与任何值比较恒为 false，
	// 若先判 >0 会把 NaN 当成"回默认"静默接受（违反"遇 NaN 报错"）。
	if math.IsNaN(s.GminFinal) || math.IsInf(s.GminFinal, 0) {
		return fmt.Errorf("ApplyStrategy: GminFinal 无效（%v），须为有限值或 <=0（回默认）", s.GminFinal)
	}
	// 校验全部通过后才写入：拒绝时 TimeMNA 状态保持原样。
	tm.SetGminFinal(s.GminFinal) // <=0 → 默认值（SetGminFinal 的既有语义）
	tm.SetGminCont(s.GminCont)
	tm.SetNaturalGmin(s.NaturalGmin)
	tm.SetGlobalGmin(s.GlobalGmin)
	tm.SetGminDbg(s.GminDbg)
	tm.SetNoEq(s.NoEq)
	// 存归一化后的绑定：GminFinal<=0 已折算为默认值，故 Strategy() 与实际
	// gminFinal 状态逐字段一致（不会出现策略对象存 0、状态存 1e-12 的错位）。
	s.GminFinal = tm.GminFinal()
	tm.strategy = s
	return nil
}

// invalidStrategyValue 判定策略数值字段是否无效：NaN 或负值。
func invalidStrategyValue(v float64) bool {
	return math.IsNaN(v) || v < 0
}

// Strategy 返回最近一次 ApplyStrategy 写入的策略绑定（用于核对同步结果）。
func (t *TimeMNA) Strategy() StrategyBinding {
	return t.strategy
}

// GminCont 返回 Gmin 延续开关（由 ApplyStrategy 同步）。
func (t *TimeMNA) GminCont() bool { return t.gminCont }

// SetGminCont 写入 Gmin 延续开关。
func (t *TimeMNA) SetGminCont(v bool) { t.gminCont = v }

// NaturalGmin 返回自然 gmin（对角叠加）。
func (t *TimeMNA) NaturalGmin() float64 { return t.naturalGmin }

// SetNaturalGmin 写入自然 gmin。
func (t *TimeMNA) SetNaturalGmin(v float64) { t.naturalGmin = v }

// GlobalGmin 返回全局对角 gmin（0=关闭）。
func (t *TimeMNA) GlobalGmin() float64 { return t.globalGmin }

// SetGlobalGmin 写入全局对角 gmin。
func (t *TimeMNA) SetGlobalGmin(v float64) { t.globalGmin = v }

// GminFinal 返回 Gmin 延续终值（恒 >0：ApplyStrategy 已把 <=0 归一为默认值）。
func (t *TimeMNA) GminFinal() float64 { return t.gminFinal }

// GminDbg 返回 gmin 步进轨迹打印开关。
func (t *TimeMNA) GminDbg() bool { return t.gminDbg }

// SetGminDbg 写入 gmin 步进轨迹打印开关。
func (t *TimeMNA) SetGminDbg(v bool) { t.gminDbg = v }

// NoEq 返回"跳过行/列均衡化"开关。
func (t *TimeMNA) NoEq() bool { return t.noEq }

// SetNoEq 写入"跳过行/列均衡化"开关。
func (t *TimeMNA) SetNoEq(v bool) { t.noEq = v }

// ------------------------------
// 步长减半重试（主循环的失败路径）
// ------------------------------

// halveStepOnFailure 处理"当前解失败 → 减半步长重试"这一条既有路径：
// 把步长减半、标记本步需要重新加盖线性元件、置位 luRetry（时间循环据此重算本步），
// 并按 K4 记一次减半步长事件。**不新增任何推进/接受语义**——它只做原有三步并计数。
//
// 返回 error 时是既有终止条件（步长已到下限），交由调用方 return；返回 nil 时
// 调用方必须 break 出当前迭代循环，由时间循环按 luRetry 重算本时间步。
func halveStepOnFailure(con *element.Context, needLinearStamp, luRetry *bool) error {
	newStep := con.CurrentStep() / 2
	if newStep < con.MinTimeStep() {
		return fmt.Errorf("步长已降至最小限制 %.6e（时间 %.6e）", con.MinTimeStep(), con.CurrentTime())
	}
	con.SetTimeStep(newStep)
	*needLinearStamp = true
	*luRetry = true
	// 经 EngineTime 计数：TransientSimulation 入口已保证 con.Time 实现 EngineTime，
	// 此处不再用具体类型断言（断言失败会静默少计，UD7）。
	con.Time.(EngineTime).AddStepHalved()
	return nil
}

// ------------------------------
// EngineTime：仿真循环需要的引擎状态接口（UD7）
// ------------------------------

// EngineTime 是 TransientSimulation 在本层需要的**引擎状态**面：策略同步、有界牛顿事件
// 计数、强制重盖标志、gmin 策略读值与连续模式。*TimeMNA 实现它。
//
// 为什么不放进 mna.Time：这些方法里 ApplyStrategyBinding 的参数是 element/time 自己的
// StrategyBinding 类型，而 mna 不能反向 import element/time（会成环）。
//
// 装饰器约定（重要）：想在 TransientSimulation 外面包一层观测/拦截时，
// **匿名嵌入 *TimeMNA 或 element/time.EngineTime** —— 这样上述方法会被整体转发。
// 只嵌入 mna.Time 的包装会缺少这些方法，入口会**显式报错**而不是静默丢失语义（UD7）。
type EngineTime interface {
	mna.Time
	// ApplyStrategyBinding 按当前绑定同步策略状态（封装 ApplyStrategy，含"GminFinal<=0 回默认"）。
	ApplyStrategyBinding(s StrategyBinding) error
	// BeginBoundedNewtonEvents 重置本次仿真的有界牛顿事件计数。
	BeginBoundedNewtonEvents()
	// ConsumeForceStamp 消费一次"强制重新加盖线性元件"请求。
	ConsumeForceStamp() bool
	// AddNewtonIters 累加本步牛顿迭代轮数（跨重试累加）。
	AddNewtonIters(n int)
	// 下面四个是 K4 有界牛顿事件计数的自增入口（诊断口径，见 NewtonEventCounts）。
	AddStepHalved()
	AddStepRejected()
	AddIterExhausted()
	AddNaNRejected()
	// gmin 策略读值（仿真循环据其决定是否启用延续/全局 gmin）。
	GminCont() bool
	NaturalGmin() float64
	GlobalGmin() float64
	// SetContinuousMode 置连续模式（TransientSimulationContinuous 用）。
	SetContinuousMode()
}

// ApplyStrategyBinding 见 EngineTime：把绑定同步到自身。
func (t *TimeMNA) ApplyStrategyBinding(s StrategyBinding) error {
	return ApplyStrategy(t, s)
}

// AddStepHalved 见 EngineTime。
func (t *TimeMNA) AddStepHalved() { t.boundedEvents.AddStepHalved() }

// AddStepRejected 见 EngineTime。
func (t *TimeMNA) AddStepRejected() { t.boundedEvents.AddStepRejected() }

// AddIterExhausted 见 EngineTime。
func (t *TimeMNA) AddIterExhausted() { t.boundedEvents.AddIterExhausted() }

// AddNaNRejected 见 EngineTime。
func (t *TimeMNA) AddNaNRejected() { t.boundedEvents.AddNaNRejected() }

// ------------------------------
// 迭代数读出口（K4/C11，含 D18 修正出口）
// ------------------------------

// GoodSteps 返回已成功完成的时间步数（= GoodIterations() 的准确命名）。
// 该计数在仿真内**从不重置**，语义是"累计成功步数"，**不是**迭代数。
func (t *TimeMNA) GoodSteps() int {
	return t.goodStepCount
}

// AddNewtonIters 把一段迭代轮数累加进总数（在本步收敛/重试返回时间循环之前调用）。
// 用累加而不是写入：每次"拒绝当前解 → 减半步长重算"都会把被丢弃的那几轮也算进
// 真实工作量，写入只会留下最后一次尝试的轮数（CLI 上表现为恒 1，已是错误读数）。
func (t *TimeMNA) AddNewtonIters(n int) {
	if n <= 0 {
		return
	}
	t.totalNewtonIters.Add(int64(n))
}

// NewtonIterations 返回本次仿真的牛顿迭代总数（每次 TransientSimulation 启动重置）。
func (t *TimeMNA) NewtonIterations() int {
	return int(t.totalNewtonIters.Load())
}

// CurrentStepNonlinearIterations 返回**当前时间步**内的牛顿外循环轮数
// （由 ResetNonlinearIter 在每个时间步开始时清零）。
func (t *TimeMNA) CurrentStepNonlinearIterations() int {
	return int(t.currNonlinIter)
}

// CurrentStepElemIterations 返回**当前时间步**内的元件次级迭代计数
// （由 ResetElemIter 在每轮外循环开始时清零，NextElemIter 每次 doStep 自增）。
//
// 本方法替代对 GoodIterations() 的错误用法（缺陷 D18）：GoodIterations() 返回的是
// **累计成功步数**（goodStepCount，仿真内从不重置），被 element/semiconductor 的
// Diode/led/zener 当作"本步子迭代数"喂给 `subIterations > 100` 的加性 gmin 启发式；
// 长仿真（>100 步）后该条件恒成立、extraGmin 恒被钳到 1e-6，每个 PN 结长期并联
// ~1MΩ，模型随仿真时长漂移。器件侧应改读本方法（详见
// runs/circuit-perf-optimization/_results/m5/summary.md 的待办交接清单）。
//
// 注意归零时刻：每轮牛顿外循环开始时归零，故同一时间步内多次外循环之间不累加，
// 读到的是**最近一次**外循环的元件迭代数（量级与器件侧判据一致，远小于 100）。
func (t *TimeMNA) CurrentStepElemIterations() int {
	return t.currElemIter
}
