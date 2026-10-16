// deviceGate.go：TTL 门类器件（74LS00 等）的**共用管路**——槽位约定、模型构建、推进与复位。
//
// 本文件不注册元件类型（器件注册在 device*.go 各自的 init 侧变量里），也不含参数数值
// （数值只在 spec.go）。槽位约定与 gate.go 的 GateSlots 一一对应，并与 element/ttl 的
// 测试器件同源（同一批约定已被 gate_test.go 的判据验证）。
package ttl

import (
	"fmt"
	"math"

	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
)

// ttlVcc 是 TTL 器件的标称电源电压（V）。阈值取 VCC/2，与 gate.go 的 digital.Level 一致。
const ttlVcc = 5.0

// 槽位下标（每个门器件实例）：0..3 延时队列、4 目标电平、5 驱动电平、6 幂等戳、7 型号字符串。
const (
	gateSlotModel = 0 // 型号字符串（K4：型号只作参数值）
	gateSlotQueue = 1 // 占 4 个连续槽位：1..4 = [armed, at, value, now]
	gateSlotGoal  = 5 // 最近一次判决/排程的目标电平（1 = 高）
	gateSlotDrive = 6 // 已经生效、当前由电压源驱动的电平（1 = 高）
	gateSlotStamp = 7 // 步进幂等戳（K1：初值必须 NaN）
	gateSlotCount = 8
)

// gateRuntimeSlots 是运行态槽（C4）：在 DoStep 内被改写，必须登记进 Config.OrigValue。
// 型号槽 7 是配置而非运行态，不登记。
var gateRuntimeSlots = []int{gateSlotQueue, gateSlotQueue + 1, gateSlotQueue + 2, gateSlotQueue + 3,
	gateSlotGoal, gateSlotDrive, gateSlotStamp}

// gateValueInit 返回某默认型号的门器件初始值模板（Base 会按网表参数替换型号槽）。
func gateValueInit(defaultModel string) []any {
	return []any{
		defaultModel,                                   // 0：型号（K4）
		float64(0), float64(0), float64(0), float64(0), // 1..4：延时队列 [armed, at, value, now]
		float64(0), // 5：目标电平
		float64(0), // 6：驱动电平
		math.NaN(), // 7：幂等戳（K1：初值必须 NaN，否则 t=0 首步被误判为已推进）
	}
}

// gateValueNames 返回槽位可读名（与 gateValueInit 一一对应）。
func gateValueNames() []string {
	return []string{"model", "q_armed", "q_at", "q_value", "q_now", "goal", "drive", "stamp"}
}

// gateConfigWithModel 由类型模板复制出一份按实例型号改写的配置（Base 的实现体）。
func gateConfigWithModel(template *element.Config, elem ast.ElementNode, defaultModel string) *element.Config {
	model := defaultModel
	if len(elem.Values) > 0 {
		if s := elem.Values[0].Value; s != "" {
			model = s
		}
	}
	values := make([]any, len(template.ValueInit))
	copy(values, template.ValueInit)
	values[gateSlotModel] = model
	return &element.Config{
		Name:       template.Name,
		Pin:        template.Pin,
		ValueInit:  values,
		ValueName:  template.ValueName,
		Current:    template.Current,
		OrigValue:  template.OrigValue,
		Voltage:    template.Voltage,
		Internal:   template.Internal,
		Flags:      template.Flags,
		EventSlots: template.EventSlots,
	}
}

// gateModelOf 按实例槽里的型号构建门模型。
// 失败（型号未知或参数不完整）必须由调用方转成"响亮失败"：调用方调 t.NoConverged()，
// 让引擎减半步长直至明确报错——**不得**用 0 参数静默建模（C11）。
func gateModelOf(value element.NodeFace, outPin int, inputPins ...int) (GateModel, error) {
	model := value.GetString(gateSlotModel)
	params, err := ParamsFromModel(model)
	if err != nil {
		return GateModel{}, err
	}
	return NewGateModel(params, ttlVcc,
		GateOutput{Pin: outPin, Internal: 0, Source: 0},
		GateSlots{QueueBase: gateSlotQueue, Goal: gateSlotGoal, Drive: gateSlotDrive, Stamp: gateSlotStamp},
		inputPins...)
}

// gateInputs 读输入引脚的逻辑电平（阈值 VCC/2，与模型内部口径一致）。
func gateInputs(m mna.Mna, value element.NodeFace, pins []int) []bool {
	out := make([]bool, len(pins))
	for i, p := range pins {
		out[i] = m.GetNodeVoltage(value.GetNodes(p)) > ttlVcc/2
	}
	return out
}

// driveGate 是门器件的统一推进：判决 → 模型推进（含步长自检、惯性延时、盖章）。
// 型号错误 ⇒ t.NoConverged()（引擎会减半步长，最终以明确错误终止，而不是给出貌似正常的波形）。
func driveGate(m mna.Mna, t mna.Time, value element.NodeFace, outPin int, inputs []int, decide func([]bool) bool) {
	if m == nil || value == nil {
		return
	}
	model, err := gateModelOf(value, outPin, inputs...)
	if err != nil {
		t.NoConverged()
		return
	}
	if err := model.Step(m, t, value, decide(gateInputs(m, value, inputs))); err != nil {
		t.NoConverged()
	}
}

// 参数表 → 门模型的接线（K3：数值只在 spec.go；接线由器件层做一次）。
//
// IIH/IIL 在参数表里是**电流（A）**，门模型要的是**导纳（S）**：基准取 TTL 的
// VIH(min)=2.7 V（手册在这一档测 IIH）与 VIL(max)=0.4 V（同一档测 IIL），故 G = |I| / V。
// 量纲转换属模型侧，故常量写在这里而不是 spec.go。
const (
	specInputHighVolt = 2.7
	specInputLowVolt  = 0.4
)

// init 把 SpecOf 的字段映射进 GateParams（器件层唯一一次接线）。
// 未取证字段（0）照原样传下去，由 GateParams.Validate 在建模时报错（不许静默用 0 建模）。
func init() {
	gateSpecLookup = func(model string) (GateParams, bool) {
		s, ok := SpecOf(model)
		if !ok {
			return GateParams{}, false
		}
		return GateParams{
			Model: s.Model,
			TpdLH: s.TpdLH,
			TpdHL: s.TpdHL,
			IIH:   math.Abs(s.IIH) / specInputHighVolt,
			IIL:   math.Abs(s.IIL) / specInputLowVolt,
			IOH:   math.Abs(s.IOH),
			IOL:   math.Abs(s.IOL),
			VOH:   s.VOH,
			VOL:   s.VOL,
		}, true
	}
}

// gateBase 提供门/触发器件不需要重写的 ElementFace 方法（一律 no-op）：
// 电学贡献全部由 gate.go 的 GateModel.Step 在 DoStep 内完成（与 Diode 在 DoStep 内盖章同口径）。
// 器件类型内嵌本结构即可满足接口，避免每个器件文件重复写 5 个空方法。
type gateBase struct{ *element.Config }

func (gateBase) StartIteration(mna.Mna, mna.Time, element.NodeFace)           {}
func (gateBase) Stamp(mna.Mna, mna.Time, element.NodeFace)                    {}
func (gateBase) CalculateCurrent(mna.Mna, mna.Time, element.NodeFace)         {}
func (gateBase) StepFinished(mna.Mna, mna.Time, element.NodeFace)             {}
func (gateBase) AddDerivative(mna.Mna, mna.Time, element.NodeFace, []float64) {}

// resetSlots 把运行态槽复位到初始值（C5：Reset 必须清掉上一轮仿真留下的状态）。
// 直接写 NodeValue：这些槽的初值就在 Config.ValueInit 里，接口没有"按槽设值"的通用方法。
func resetSlots(value element.NodeFace, slots []int) {
	if value == nil {
		return
	}
	base := value.Base()
	cfg := value.Config()
	if base == nil || cfg == nil {
		return
	}
	for _, i := range slots {
		if i >= 0 && i < len(base.NodeValue) && i < len(cfg.ValueInit) {
			base.NodeValue[i] = cfg.ValueInit[i]
		}
	}
}

// gateErrNoModel 把"型号错误"包装成可读错误（供器件层在需要时打印/上报）。
func gateErrNoModel(value element.NodeFace, err error) error {
	return fmt.Errorf("ttl: 元件 %s 的型号 %q 无法建模: %w",
		value.Base().InstanceName, value.GetString(gateSlotModel), err)
}
