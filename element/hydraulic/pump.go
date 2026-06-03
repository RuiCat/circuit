// 油路液压泵元件实现。
package hydraulic

import (
	"circuit/element"
	"circuit/mna"
)

// HPType 液压泵类型标识(NodeType=33)，网表"HP<name> <n1,n2> [pressure]"，默认10MPa。
var HPType element.NodeType = element.AddElement(33, &HP{
	&element.Config{
		Name:      "hp",
		Pin:       element.SetPin(element.PinHydraulic, "hp+", "hp-"),
		ValueInit: []any{float64(10e6)},
		ValueName: []string{"pressure"},
		Voltage:   []string{"hpsrc"},
	},
})

// HPPump 液压泵/压力源。与电压源同构，通过内部电压源加盖压力约束。
type HP struct{ *element.Config }

// Stamp 加盖液压泵的MNA贡献，使用StampVoltageSource在n1-n2间建立压力约束。
func (HP) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	pressure := value.GetFloat64(0)
	mna.StampVoltageSource(value.GetNodes(0), value.GetNodes(1), value.GetVoltSource(0), pressure)
}
