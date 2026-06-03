// 气路压力源元件实现，提供恒定或时变气压激励。
package pneumatic

import (
	"circuit/element"
	"circuit/mna"
)

// PSType 气源元件类型标识(NodeType=28)，网表"PS<name> <n1,n2> [pressure]"，默认101325Pa(1atm)。
var PSType element.NodeType = element.AddElement(28, &PS{
	&element.Config{
		Name:      "ps",
		Pin:       element.SetPin(element.PinPneumatic, "ps+", "ps-"),
		ValueInit: []any{float64(101325)},
		ValueName: []string{"pressure"},
		Voltage:   []string{"psrc"},
	},
})

// PSPressureSource 气路压力源。与电压源完全同构：p=n1-n2，通过内部电压源加盖。
type PS struct{ *element.Config }

// Stamp 加盖气源的MNA贡献，使用StampVoltageSource在n1-n2间建立压力约束。
func (PS) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	pressure := value.GetFloat64(0)
	mna.StampVoltageSource(value.GetNodes(0), value.GetNodes(1), value.GetVoltSource(0), pressure)
}
