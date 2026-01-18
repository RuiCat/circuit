package base

import (
	"circuit/element"
	"circuit/mna"
)

// VCVSType 定义元件
var VCVSType element.NodeType = element.AddElement(10, &VCVS{
	&element.Config{
		Name:      "e",
		Pin:       element.SetPin(element.PinLowVoltage, "cp", "cn", "op", "on"), // 控制正、控制负、输出正、输出负
		ValueInit: []any{float64(1)},                                             // 0: 增益系数
		Voltage:   []string{"v"},                                                 // 电压源
	},
})

// VCVS 电压控制电压源
type VCVS struct{ *element.Config }

func (VCVS) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// VCVS: V_out = Gain * V_in
	// 控制节点: value.GetNodes[0], value.GetNodes[1] (输入)
	// 输出节点: value.GetNodes[2], value.GetNodes[3] (输出)
	gain := value.GetFloat64(0)

	// 获取节点ID
	cp := value.GetNodes(0) // 控制正
	cn := value.GetNodes(1) // 控制负
	op := value.GetNodes(2) // 输出正
	on := value.GetNodes(3) // 输出负

	// 使用StampVCVS方法，参数为: 输出正, 输出负, 控制正, 控制负, 电压源ID, 增益
	// StampVCVS已经包含了电压源约束，不需要额外的StampVoltageSource调用
	mna.StampVCVS(op, on, cp, cn, value.GetVoltSource(0), gain)
}
