// CCCS（流控电流源）元件实现，内部使用0V测量电压源检测控制支路电流。
package controlled

import (
	"circuit/element"
	"circuit/mna"
)

// CCCSType 流控电流源类型标识，网表标识为 "f"，4引脚，内部含vmeas测量电压源。
var CCCSType element.NodeType = element.AddElement(22, &CCCS{
	&element.Config{
		Name:      "f",
		Pin:       element.SetPin(element.PinLowVoltage, "cp", "cn", "op", "on"),
		ValueInit: []any{float64(1)}, // 默认电流增益 1
		ValueName: []string{"gain"},
		Voltage:   []string{"vmeas"}, // 内部 0V 测量电压源
	},
})

// CCCS 流控电流源结构体。
// 数学模型: I_out = gain * I_control，控制电流通过内部0V电压源(vmeas)串入控制支路测量。
type CCCS struct{ *element.Config }

// Stamp 加盖CCCS的MNA贡献。
// 第一步：在cp-cn间加盖0V测量电压源(vmeas)；第二步：在op-on间调用StampCCCS建立受控关系。
func (CCCS) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 在 cp-cn 之间加盖 0V 电压源用于测量控制电流
	mna.StampVoltageSource(value.GetNodes(0), value.GetNodes(1), value.GetVoltSource(0), 0)
	// 在 op-on 之间加盖 CCCS，控制量为测量电压源中的电流
	gain := value.GetFloat64(0)
	mna.StampCCCS(value.GetNodes(2), value.GetNodes(3), value.GetVoltSource(0), gain)
}
