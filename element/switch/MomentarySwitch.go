package switches

import (
	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
	"fmt"
)

// MomentarySwitchType 非自锁按钮 (NodeType=16)
// 网表语法: MB<name> [n1, n2] [eventName, threshold, normallyOpen, Ron, Roff, holdSteps]
//
// 自动复位机制:
//   autoResetValue(NodeValue[9]) 初始 nil → PullEvents 跳过不传播
//   用户设置事件 → PushEvents 推送 → 按钮闭合 → StepFinished 计数
//   保持期内 autoResetValue 保持 nil（不干扰事件系统）
//   holdSteps 到期 → autoResetValue=0 → PullEvents 写消费者 → eventValue=0 → 复位
var MomentarySwitchType element.NodeType = element.AddElement(16, &MomentarySwitch{
	&element.Config{
		Name: "MB",
		Pin:  element.SetPin(element.PinLowVoltage, "mb1a", "mb1b"),
		ValueInit: []any{
			"MB1",         // 0: eventName
			float64(0.5),  // 1: threshold
			int(1),        // 2: normallyOpen
			float64(1e-6), // 3: R_on
			float64(1e12), // 4: R_off
			int(1),        // 5: holdSteps
			int(0),        // 6: lastState
			int(0),        // 7: stepCounter
			float64(0),    // 8: eventValue
			nil,           // 9: autoResetValue (nil=跳过, float64(0)=复位)
			"MB1",         // 10: resetEventName
			float64(0),    // 11: 总电流 I (A)（CalculateCurrent 写入，各触点对之和）
		},
		ValueName: []string{"eventName", "threshold", "normallyOpen",
			"R_on", "R_off", "holdSteps", "lastState", "stepCounter",
			"eventValue", "autoResetValue", "resetName", "I"},
		Current:    []int{11},
		OrigValue:  []int{6, 7},
		EventSlots: map[int]int{0: 8, 10: -9},
	},
})

// MomentarySwitch 非自锁按钮（脉冲型事件开关，StepFinished 直接清零 eventValue 自动复位）
type MomentarySwitch struct{ *element.Config }

// Base 根据网表动态生成配置。引脚数为偶数（每两个一组作为触点对），
// 事件名从网表读取。
func (ms *MomentarySwitch) Base(elem ast.ElementNode) *element.Config {
	nPins := len(elem.Pins)
	if nPins < 2 || nPins%2 != 0 {
		nPins = 2
	}
	nPairs := nPins / 2
	pinNames := make([]string, nPins)
	for i := 0; i < nPairs; i++ {
		pinNames[2*i] = fmt.Sprintf("mb%da", i+1)
		pinNames[2*i+1] = fmt.Sprintf("mb%db", i+1)
	}

	eventName := "MB1"
	if len(elem.Values) > 0 {
		eventName = elem.Values[0].Value
	}

	cfg := ms.Config
	valInit := make([]any, len(cfg.ValueInit))
	copy(valInit, cfg.ValueInit)
	valInit[0] = eventName
	valInit[10] = eventName

	return &element.Config{
		Name:       cfg.Name,
		Pin:        element.SetPin(element.PinLowVoltage, pinNames...),
		ValueInit:  valInit,
		ValueName:  cfg.ValueName,
		Current:    cfg.Current,
		OrigValue:  cfg.OrigValue,
		EventSlots: cfg.EventSlots,
	}
}

// Stamp 仅计算并保存初始开关状态，不盖章阻抗。
// 阻抗由 DoStep 动态维护。
func (ms *MomentarySwitch) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	state := calcEventSwitchState(value.GetFloat64(8), value.GetFloat64(1), value.GetInt(2))
	value.SetInt(6, state)
}

// DoStep 根据当前事件值刷新阻抗，并检测状态切换。
// 每次被调用都重新盖章正确阻抗，以兼容 MNA 矩阵回滚后的重盖章。
func (ms *MomentarySwitch) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	newState := calcEventSwitchState(value.GetFloat64(8), value.GetFloat64(1), value.GetInt(2))
	oldState := value.GetInt(6)

	var R float64
	if newState == 1 {
		R = value.GetFloat64(3)
	} else {
		R = value.GetFloat64(4)
	}

	cfg := value.Config()
	if cfg == nil {
		return
	}
	nPairs := cfg.PinNum() / 2
	for i := 0; i < nPairs; i++ {
		mna.StampImpedance(value.GetNodes(2*i), value.GetNodes(2*i+1), R)
	}

	if newState != oldState {
		value.SetInt(6, newState)
		time.NoConverged()
	}
}

// CalculateCurrent 根据当前状态和欧姆定律计算每对触点的电流。
func (ms *MomentarySwitch) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	state := value.GetInt(6)
	var R float64
	if state == 1 {
		R = value.GetFloat64(3)
	} else {
		R = value.GetFloat64(4)
	}

	cfg := value.Config()
	if cfg == nil {
		return
	}
	var total float64
	nPairs := cfg.PinNum() / 2
	for i := 0; i < nPairs; i++ {
		v1 := mna.GetNodeVoltage(value.GetNodes(2 * i))
		v2 := mna.GetNodeVoltage(value.GetNodes(2*i + 1))
		if R > 0 {
			current := (v1 - v2) / R
			total += current
			mna.StampCurrentSource(value.GetNodes(2*i), value.GetNodes(2*i+1), -current)
		}
	}
	value.SetFloat64(11, total) // 记录总电流供输出
}

// StepFinished 保持步数计数：持续触发 holdSteps 步后写入 autoResetValue=0，
// 经 PullEvents 清零 eventValue 完成自动复位。
func (ms *MomentarySwitch) StepFinished(mna mna.Mna, time mna.Time, value element.NodeFace) {
	holdSteps := value.GetInt(5)
	stepCounter := value.GetInt(7)
	eventValue := value.GetFloat64(8)
	threshold := value.GetFloat64(1)

	triggered := eventValue > threshold
	if triggered && stepCounter < holdSteps {
		stepCounter++
		value.SetInt(7, stepCounter)
	} else if triggered {
		value.SetInt(7, 0)
		value.Base().NodeValue[9] = float64(0) // autoResetValue=0 触发 PullEvents 复位
	} else {
		value.SetInt(7, 0)
	}
}

