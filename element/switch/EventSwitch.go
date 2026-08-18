package switches

import (
	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
	"fmt"
)

// EventSwitchType 事件驱动按钮开关元件类型 (NodeType=13)
// 网表语法: B<name> <n1, n2> [eventName, threshold, normallyOpen, Ron, Roff]
// 例: B1 [0, 1] [SB1, 0.5, 1]
// 事件值由 Context.PushEvents 自动同步到 NodeValue[6]
var EventSwitchType element.NodeType = element.AddElement(13, &EventSwitch{
	&element.Config{
		Name: "B",
		Pin:  element.SetPin(element.PinLowVoltage, "sw1a", "sw1b"),
		ValueInit: []any{
			"SB1",         // 0: eventName — 事件名称 (string，永不覆盖)
			float64(0.5),  // 1: threshold — 触发阈值
			int(1),        // 2: normallyOpen — 1=常开, 0=常闭
			float64(1e-6), // 3: R_on — 闭合导通电阻
			float64(1e12), // 4: R_off — 断开时关断电阻
			int(0),        // 5: lastState — 内部追踪
			float64(0),    // 6: eventValue — 事件值 (由 PushEvents 更新)
			float64(0),    // 7: 总电流 I (A)（CalculateCurrent 写入，各触点对之和）
		},
		ValueName:  []string{"eventName", "threshold", "normallyOpen", "R_on", "R_off", "lastState", "eventValue", "I"},
		Current:    []int{7},
		OrigValue:  []int{5},
		EventSlots: map[int]int{0: 6}, // nameIndex(0) → valueIndex(6): 事件名"SB1"→值写入索引6
	},
})

// EventSwitch 事件驱动按钮开关
// 事件值由 Context.PushEvents 自动同步，无需手动调用 GetEvent
type EventSwitch struct{ *element.Config }

// Base 根据网表引脚数动态生成引脚配置
// 引脚数为偶数，每两个一组作为一个触点对
// 例: B1 [1,2,3,4,5,6] → 3对触点
func (es *EventSwitch) Base(elem ast.ElementNode) *element.Config {
	nPins := len(elem.Pins)
	if nPins < 2 || nPins%2 != 0 {
		nPins = 2 // 无效引脚数回退为1对
	}
	nPairs := nPins / 2

	// 生成引脚名: sw1a,sw1b, sw2a,sw2b, ...
	pinNames := make([]string, nPins)
	for i := 0; i < nPairs; i++ {
		pinNames[2*i] = fmt.Sprintf("sw%da", i+1)
		pinNames[2*i+1] = fmt.Sprintf("sw%db", i+1)
	}

	// 复制模板的 ValueInit 等（每个实例独立）
	cfg := es.Config
	return &element.Config{
		Name:       cfg.Name,
		Pin:        element.SetPin(element.PinLowVoltage, pinNames...),
		ValueInit:  cfg.ValueInit,
		ValueName:  cfg.ValueName,
		Current:    cfg.Current,
		OrigValue:  cfg.OrigValue,
		EventSlots: cfg.EventSlots,
	}
}

// calcEventSwitchState 根据事件值、阈值和常开/常闭模式计算开关状态
func calcEventSwitchState(eventVal float64, threshold float64, normallyOpen int) int {
	state := 0
	if eventVal > threshold {
		state = 1
	}
	if normallyOpen == 0 {
		state = 1 - state
	}
	return state
}

// Stamp 仅计算并保存初始开关状态，不盖章阻抗。
// 阻抗由 DoStep 负责动态维护，确保回滚后仍能恢复正确值。
func (es *EventSwitch) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	state := calcEventSwitchState(value.GetFloat64(6), value.GetFloat64(1), value.GetInt(2))
	value.SetInt(5, state)
	// 阻抗全部由 DoStep 处理，Stamp 不再盖章
}

// DoStep 根据当前事件值刷新阻抗，并检测状态切换。
// 每次被调用都重新盖章正确阻抗，以兼容 MNA 矩阵回滚后的重盖章。
func (es *EventSwitch) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	newState := calcEventSwitchState(value.GetFloat64(6), value.GetFloat64(1), value.GetInt(2))
	oldState := value.GetInt(5)

	// 每次调用都盖章当前状态对应的阻抗（解决矩阵回滚后阻抗丢失的问题）
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

	// 状态切换时更新追踪值并标记未收敛
	if newState != oldState {
		value.SetInt(5, newState)
		time.NoConverged()
	}
}

// CalculateCurrent 根据当前状态和欧姆定律计算每对触点的电流
func (es *EventSwitch) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	state := value.GetInt(5)
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
	value.SetFloat64(7, total) // 记录总电流供输出
}
