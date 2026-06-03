// 模拟比较器元件实现，比较两路模拟输入并输出数字电平。
package logic

import (
	"circuit/element"
	"circuit/mna"
)

// CMPType 模拟比较器类型标识(NodeType=41)，网表"CMP<name> <vp,vn,vout> [V_high,V_low]"。
var ComparatorType element.NodeType = element.AddElement(41, &Comparator{
	&element.Config{
		Name: "CMP",
		Pin:  element.SetPin(element.PinLowVoltage, "vp", "vn", "vout"),
		ValueInit: []any{
			float64(5.0), // 0: 高电平输出电压 V_high
			float64(0.0), // 1: 低电平输出电压 V_low
		},
		ValueName: []string{"V_high", "V_low"},
		Voltage:   []string{"vout"},
	},
})

// Comparator 模拟比较器。比较Vp与Vn：Vp>Vn→Vout=V_high，否则→V_low。
// 直接电压源驱动输出，无滞回。
type Comparator struct{ *element.Config }

// Stamp 初始化比较器输出电压源为0V。
func (c *Comparator) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	m.StampVoltageSource(value.GetNodes(2), -1, value.GetVoltSource(0), 0)
}

// DoStep 比较器每步仿真。比较Vp与Vn，输出V_high或V_low，
// 若输出变化超过span的1%则标记未收敛。
func (c *Comparator) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	vHigh := value.GetFloat64(0)
	vLow := value.GetFloat64(1)

	vp := m.GetNodeVoltage(value.GetNodes(0))
	vn := m.GetNodeVoltage(value.GetNodes(1))

	var desired float64
	if vp > vn {
		desired = vHigh
	} else {
		desired = vLow
	}

	currentV := m.GetNodeVoltage(value.GetNodes(2))
	m.UpdateVoltageSource(value.GetVoltSource(0), desired)

	diff := currentV - desired
	if diff < 0 {
		diff = -diff
	}
	span := vHigh - vLow
	if span < 0 {
		span = -span
	}
	if diff > 0.01*span {
		t.NoConverged()
	}
}
