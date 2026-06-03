// 施密特触发器元件实现，带滞回特性的比较器，抗噪声干扰。
package logic

import (
	"circuit/element"
	"circuit/mna"
)

// STType 施密特触发器类型标识(NodeType=42)，网表"ST<name> <vin,vout> [V_high,V_low,Vth_high,Vth_low]"。
var SchmittType element.NodeType = element.AddElement(42, &Schmitt{
	&element.Config{
		Name: "ST",
		Pin:  element.SetPin(element.PinLowVoltage, "vin", "vout"),
		ValueInit: []any{
			float64(5.0), // 0: 高电平输出电压 V_high
			float64(0.0), // 1: 低电平输出电压 V_low
			float64(3.0), // 2: 上升阈值 Vth_high
			float64(2.0), // 3: 下降阈值 Vth_low
			false,        // 4: 当前输出状态 (true=高, false=低)
		},
		ValueName: []string{"V_high", "V_low", "Vth_high", "Vth_low", "current_state"},
		Voltage:   []string{"vout"},
		OrigValue: []int{4},
	},
})

// Schmitt 施密特触发器。双阈值滞回: 上升沿>Vth_high翻高, 下降沿<Vth_low翻低。
// 滞回窗口防止输入噪声引起的输出振荡。
type Schmitt struct{ *element.Config }

// Stamp 初始化施密特触发器输出电压源为0V。
func (s *Schmitt) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	m.StampVoltageSource(value.GetNodes(1), -1, value.GetVoltSource(0), 0)
}

// DoStep 施密特触发器每步仿真。双阈值滞回逻辑：当前为低时Vin>Vth_high才翻高，
// 当前为高时Vin<Vth_low才翻低。状态变化时标记未收敛。
func (s *Schmitt) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	vHigh := value.GetFloat64(0)
	vLow := value.GetFloat64(1)
	vthHigh := value.GetFloat64(2)
	vthLow := value.GetFloat64(3)
	currentState := value.GetBool(4)

	vin := m.GetNodeVoltage(value.GetNodes(0))

	prevState := currentState

	if !currentState && vin > vthHigh {
		currentState = true
	} else if currentState && vin < vthLow {
		currentState = false
	}

	if currentState != prevState {
		value.SetBool(4, currentState)
		t.NoConverged()
	}

	desired := vLow
	if currentState {
		desired = vHigh
	}

	m.UpdateVoltageSource(value.GetVoltSource(0), desired)
}
