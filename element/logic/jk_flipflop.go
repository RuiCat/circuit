// JK触发器元件实现，正边沿触发，支持保持/置位/复位/翻转四种模式。
package logic

import (
	"circuit/element"
	"circuit/mna"
)

// JKType JK触发器类型标识(NodeType=38)，网表"JK<name> <j,k,clk,q,nq> [V_high]"。
var JKFlipFlopType element.NodeType = element.AddElement(38, &JKFlipFlop{
	&element.Config{
		Name: "JK",
		Pin:  element.SetPin(element.PinBoolean, "j", "k", "clk", "q", "nq"),
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

// JKFlipFlop JK触发器。边沿检测(90%V_high阈值)+直接电压源驱动Q/NQ。
// 真值表: J=K=0保持, J=0/K=1复位, J=1/K=0置位, J=K=1翻转。
type JKFlipFlop struct{ *element.Config }

// Stamp 根据当前Q状态设置Q和NQ输出电压源初始值。
func (f *JKFlipFlop) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	highVoltage := value.GetFloat64(0)
	qState := value.GetBool(2)

	qVoltage := 0.0
	nqVoltage := highVoltage
	if qState {
		qVoltage = highVoltage
		nqVoltage = 0.0
	}

	m.StampVoltageSource(value.GetNodes(3), -1, value.GetVoltSource(0), qVoltage)
	m.StampVoltageSource(value.GetNodes(4), -1, value.GetVoltSource(1), nqVoltage)
}

// DoStep JK触发器每步仿真。检测CLK正边沿(>90%V_high)后根据J/K输入更新Q状态，
// 并通过电压源驱动Q和NQ输出。
func (f *JKFlipFlop) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	highVoltage := value.GetFloat64(0)
	prevClk := value.GetFloat64(1)
	qState := value.GetBool(2)
	initialized := value.GetBool(3)

	clkNode := value.GetNodes(2)
	clkNow := m.GetNodeVoltage(clkNode)

	edgeThreshold := highVoltage * 0.9
	isHigh := clkNow > edgeThreshold

	if !initialized && (t.Time() > 1e-9 || !isHigh) {
		initialized = true
		value.SetBool(3, true)
	}
	wasHigh := prevClk > highVoltage*0.9

	if initialized && isHigh && !wasHigh {
		jNode := value.GetNodes(0)
		kNode := value.GetNodes(1)
		j := m.GetNodeVoltage(jNode) > highVoltage*0.5
		k := m.GetNodeVoltage(kNode) > highVoltage*0.5

		if j && k {
			qState = !qState
		} else if j {
			qState = true
		} else if k {
			qState = false
		}

		value.SetBool(2, qState)
		t.NoConverged()
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
func (f *JKFlipFlop) StepFinished(m mna.Mna, t mna.Time, value element.NodeFace) {
	if !value.GetBool(3) {
		clkNow := m.GetNodeVoltage(value.GetNodes(2))
		highVoltage := value.GetFloat64(0)
		if clkNow < highVoltage*0.3 || t.Time() > 1e-9 {
			value.SetBool(3, true)
		}
	}
}
