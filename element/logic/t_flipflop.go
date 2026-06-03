// T触发器元件实现，正边沿触发的二分频器。
package logic

import (
	"circuit/element"
	"circuit/mna"
)

// TType T触发器类型标识(NodeType=39)，网表"T<name> <t,clk,q,nq> [V_high]"。
var TFlipFlopType element.NodeType = element.AddElement(39, &TFlipFlop{
	&element.Config{
		Name: "T",
		Pin:  element.SetPin(element.PinBoolean, "t", "clk", "q", "nq"),
		ValueInit: []any{
			float64(5.0), // 0: 高电平电压 V_high
			float64(0.0), // 1: 上一时刻 clk 电压
			false,        // 2: 内部 Q 状态
			false,        // 3: 是否已完成首次收敛初始化
		},
		ValueName: []string{"V_high", "prev_clk", "q_state", "initialized"},
		Voltage:   []string{"vq", "vnq"},
		Flags:     element.FlagNonlinear,
		OrigValue: []int{1, 2, 3},
	},
})

// TFlipFlop T触发器。正边沿触发翻转：T=1时CLK上升沿→Q取反(二分频)，T=0保持。
// 边沿检测(90%V_high阈值)+直接电压源驱动Q/NQ。
type TFlipFlop struct{ *element.Config }

// Stamp 根据当前Q状态设置Q和NQ输出电压源初始值。
func (f *TFlipFlop) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	highVoltage := value.GetFloat64(0)
	qState := value.GetBool(2)

	qVoltage := 0.0
	nqVoltage := highVoltage
	if qState {
		qVoltage = highVoltage
		nqVoltage = 0.0
	}

	m.StampVoltageSource(value.GetNodes(2), -1, value.GetVoltSource(0), qVoltage)
	m.StampVoltageSource(value.GetNodes(3), -1, value.GetVoltSource(1), nqVoltage)
}

// DoStep T触发器每步仿真。检测CLK正边沿(>90%V_high)后若T=1则翻转Q，
// 并通过电压源驱动Q和NQ输出。
func (f *TFlipFlop) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	highVoltage := value.GetFloat64(0)
	prevClk := value.GetFloat64(1)
	qState := value.GetBool(2)
	initialized := value.GetBool(3)

	clkNode := value.GetNodes(1)
	clkNow := m.GetNodeVoltage(clkNode)

	edgeThreshold := highVoltage * 0.9
	isHigh := clkNow > edgeThreshold

	if !initialized && (t.Time() > 1e-9 || !isHigh) {
		initialized = true
		value.SetBool(3, true)
	}
	wasHigh := prevClk > highVoltage*0.9

	if initialized && isHigh && !wasHigh {
		tNode := value.GetNodes(0)
		tInput := m.GetNodeVoltage(tNode) > highVoltage*0.5

		if tInput {
			qState = !qState
			value.SetBool(2, qState)
			t.NoConverged()
		}
	}

	value.SetFloat64(1, clkNow)

	desiredQ := 0.0
	if qState {
		desiredQ = highVoltage
	}
	desiredNQ := 0.0
	if !qState {
		desiredNQ = highVoltage
	}

	m.UpdateVoltageSource(value.GetVoltSource(0), desiredQ)
	m.UpdateVoltageSource(value.GetVoltSource(1), desiredNQ)
}

// StepFinished 收敛后检查：若CLK为低或仿真时间>1ns则标记已初始化。
func (f *TFlipFlop) StepFinished(m mna.Mna, t mna.Time, value element.NodeFace) {
	if !value.GetBool(3) {
		clkNow := m.GetNodeVoltage(value.GetNodes(1))
		highVoltage := value.GetFloat64(0)
		if clkNow < highVoltage*0.3 || t.Time() > 1e-9 {
			value.SetBool(3, true)
		}
	}
}
