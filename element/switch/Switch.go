package switches

import (
	"circuit/element"
	"circuit/mna"
)

// SwitchType 定义元件
var SwitchType element.NodeType = element.AddElement(7, &Switch{
	&element.Config{
		Name: "sw",
		Pin:  element.SetPin(element.PinLowVoltage, "sw1", "sw2"),
		ValueInit: []any{
			int(0),        // 0: 开关状态 (0=关, 1=开)
			float64(1e-6), // 1: 导通电阻
			float64(1e12), // 2: 关断电阻
			int(0),        // 3: 上次盖印状态
			float64(0),    // 4: 电流 I (A)（CalculateCurrent 写入）
		},
		ValueName: []string{"state", "R_on", "R_off", "last_state", "I"},
		Current:   []int{4},
		OrigValue: []int{3},
	},
})

// Switch 开关
type Switch struct{ *element.Config }

// DoStep 检测开关状态变化并驱动重新盖章。
// 当开关状态发生变化时（state != lastState），标记未收敛并触发重新 Stamp 以更新阻抗矩阵。
func (s *Switch) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	state := value.GetInt(0)
	lastState := value.GetInt(3)
	if state != lastState {
		time.NoConverged()
		s.Stamp(mna, time, value)
		value.SetInt(3, state)
	}
}

// Stamp 根据开关状态将导通/关断电阻加盖到 MNA 导纳矩阵。
// 状态为1（闭合）时使用导通电阻 R_on，状态为0（断开）时使用关断电阻 R_off。
func (Switch) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	state := value.GetInt(0)
	var resistance float64
	if state == 1 {
		resistance = value.GetFloat64(1) // 导通状态
	} else {
		resistance = value.GetFloat64(2) // 关断状态
	}
	mna.StampImpedance(value.GetNodes(0), value.GetNodes(1), resistance)
}

// CalculateCurrent 根据开关当前状态和欧姆定律计算流经开关的电流。
func (Switch) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	state := value.GetInt(0)
	var resistance float64
	if state == 1 {
		resistance = value.GetFloat64(1)
	} else {
		resistance = value.GetFloat64(2)
	}

	// 计算电流（欧姆定律）
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	if resistance > 0 {
		current := (v1 - v2) / resistance
		mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), -current)
		value.SetFloat64(4, current) // 记录电流供输出
	} else {
		value.SetFloat64(4, 0)
	}
}
