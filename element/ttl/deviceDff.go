// deviceDff.go：TTLDFF —— 74LS74 系列正沿 D 触发器（单触发元件）。
//
// 引脚（6 根，与 74LS74 的功能引脚对应）：d（数据）、clk（时钟，正沿采样）、
// pre（异步置位，**低有效**）、clr（异步复位，**低有效**）、q、nq（互补输出）。
// 型号默认 74LS74，可由器件类型默认值给出（D21：型号不写进网表参数）。
//
// 语义（与 74LS74 一致的部分）：
//   - pre=0 ⇒ q=1、nq=0；clr=0 ⇒ q=0、nq=1；两者异步、与 clk 无关；
//   - pre 与 clr **同时为 0** 在真器件上是"两输出同为高"的非法态，本模型按 **pre 优先**处理（已在测试里注明）；
//   - 两者均为 1 时，clk 正沿采样 d。
//   - 两个输出（q/nq）各用一份门模型：各自按 TpdLH/TpdHL 走惯性延时；
//     输入负载（IIH/IIL）只在 q 侧模型盖一次（d/clk/pre/clr 四个输入），避免同一引脚被重复计数。
package ttl

import (
	"math"

	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
)

// TTLDFF 的槽位下标：两份门模型 + 触发器自身状态 + 型号字符串。
const (
	dffModel   = 0  // 型号字符串（K4：必须在槽 0，见 deviceGate.go 的同名说明）
	dffQueueQ  = 1  // q 侧延时队列（1..4）
	dffGoalQ   = 5  // q 侧目标电平
	dffDriveQ  = 6  // q 侧驱动电平
	dffStampQ  = 7  // q 侧幂等戳
	dffQueueNQ = 8  // nq 侧延时队列（8..11）
	dffGoalNQ  = 12 // nq 侧目标电平
	dffDriveNQ = 13 // nq 侧驱动电平
	dffStampNQ = 14 // nq 侧幂等戳
	dffPrevClk = 15 // 上一（收敛）时刻的 clk 电平（正沿检测）
	dffQ       = 16 // 触发器状态
	dffSlots   = 17
)

// dffRuntimeSlots 是运行态槽（C4），型号槽 16 不登记。
var dffRuntimeSlots = []int{dffQueueQ, dffQueueQ + 1, dffQueueQ + 2, dffQueueQ + 3,
	dffGoalQ, dffDriveQ, dffStampQ,
	dffQueueNQ, dffQueueNQ + 1, dffQueueNQ + 2, dffQueueNQ + 3,
	dffGoalNQ, dffDriveNQ, dffStampNQ, dffPrevClk, dffQ}

// TtlDffType 是 TTLDFF 注册后的类型号（NodeType 54，K4 号段）。
var TtlDffType = element.AddElement(54, &ttlDff{gateBase: gateBase{&element.Config{
	Name:      "TTLDFF",
	Pin:       element.SetPin(element.PinLowVoltage, "d", "clk", "pre", "clr", "q", "nq"),
	ValueInit: dffValueInit("74LS74"),
	ValueName: dffValueNames(),
	OrigValue: dffRuntimeSlots,
	Voltage:   []string{"v_q", "v_nq"},
	Internal:  []string{"int_q", "int_nq"},
	Flags:     element.FlagNonlinear,
}}})

// dffValueInit 返回 D 触发器的初始值模板（Base 会按网表参数替换型号槽）。
func dffValueInit(defaultModel string) []any {
	init := make([]any, dffSlots)
	for i := range init {
		init[i] = float64(0)
	}
	init[dffModel] = defaultModel
	init[dffStampQ] = math.NaN()
	init[dffStampNQ] = math.NaN()
	return init
}

// dffValueNames 返回槽位可读名（与 dffValueInit 一一对应）。
func dffValueNames() []string {
	return []string{"model", "q_armed", "q_at", "q_value", "q_now", "q_goal", "q_drive", "q_stamp",
		"nq_armed", "nq_at", "nq_value", "nq_now", "nq_goal", "nq_drive", "nq_stamp",
		"prev_clk", "q_state"}
}

// ttlDff 是正沿 D 触发器元件（TTL 电气级）。
type ttlDff struct{ gateBase }

// Base 按网表参数生成本实例配置（型号可覆盖，默认 74LS74）。
func (d *ttlDff) Base(elem ast.ElementNode) *element.Config {
	cfg := gateConfigWithModel(d.Config, elem, "74LS74")
	return cfg
}

// dffModelOf 构建 q 或 nq 一侧的门模型（槽位与输出映射由参数给出）。
func dffModelOf(value element.NodeFace, params GateParams, outPin, internal, source, queueBase, goal, drive, stamp int, inputs []int) (GateModel, error) {
	return NewGateModel(params, ttlVcc,
		GateOutput{Pin: outPin, Internal: internal, Source: source},
		GateSlots{QueueBase: queueBase, Goal: goal, Drive: drive, Stamp: stamp},
		inputs...)
}

// DoStep 正沿采样 d 并驱动 q/nq；延时与负载由门模型负责。
// 型号未知或参数不完整 ⇒ t.NoConverged()（引擎减半步长直至报错，不静默建模）。
func (d *ttlDff) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	if m == nil || value == nil {
		return
	}
	params, err := ParamsFromModel(value.GetString(dffModel))
	if err != nil {
		t.NoConverged()
		return
	}
	qModel, err := dffModelOf(value, params, 4, 0, 0, dffQueueQ, dffGoalQ, dffDriveQ, dffStampQ, []int{0, 1, 2, 3})
	if err != nil {
		t.NoConverged()
		return
	}
	// nq 侧不盖输入负载（inputs 为空）：d/clk 的 IIH/IIL 已由 q 侧模型盖过一次。
	nqModel, err := dffModelOf(value, params, 5, 1, 1, dffQueueNQ, dffGoalNQ, dffDriveNQ, dffStampNQ, nil)
	if err != nil {
		t.NoConverged()
		return
	}

	dHigh := m.GetNodeVoltage(value.GetNodes(0)) > ttlVcc/2
	clkHigh := m.GetNodeVoltage(value.GetNodes(1)) > ttlVcc/2
	preLow := m.GetNodeVoltage(value.GetNodes(2)) <= ttlVcc/2 // 低有效
	clrLow := m.GetNodeVoltage(value.GetNodes(3)) <= ttlVcc/2 // 低有效
	prevClk := value.GetFloat64(dffPrevClk) != 0
	qState := value.GetFloat64(dffQ) != 0

	switch {
	case preLow: // 异步置位（与 clk 无关）；pre 与 clr 同时有效时 pre 优先
		qState = true
	case clrLow: // 异步复位
		qState = false
	case clkHigh && !prevClk: // 正沿采样
		qState = dHigh
	}
	value.SetFloat64(dffPrevClk, levelOf(clkHigh))
	value.SetFloat64(dffQ, levelOf(qState))

	if err := qModel.Step(m, t, value, qState); err != nil {
		t.NoConverged()
		return
	}
	if err := nqModel.Step(m, t, value, !qState); err != nil {
		t.NoConverged()
	}
}

// Reset 复位运行态槽（C5）：状态与两级延时队列都要回到初值。
func (d *ttlDff) Reset(value element.NodeFace) { resetSlots(value, dffRuntimeSlots) }

// levelOf 把逻辑电平折算成槽位值（1 = 高、0 = 低）。
func levelOf(high bool) float64 {
	if high {
		return 1
	}
	return 0
}
