package base

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
		},
		Current: []int{0},
	},
})

// Switch 开关
type Switch struct{ *element.Config }

func (Switch) Stamp(mna mna.MNA, time mna.Time, value element.NodeFace) {
	state := value.GetInt(0)
	var resistance float64
	if state == 1 {
		resistance = value.GetFloat64(1) // 导通状态
	} else {
		resistance = value.GetFloat64(2) // 关断状态
	}
	mna.StampResistor(value.GetNodes(0), value.GetNodes(1), resistance)
}

func (Switch) CalculateCurrent(mna mna.MNA, time mna.Time, value element.NodeFace) {
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
	}
}
