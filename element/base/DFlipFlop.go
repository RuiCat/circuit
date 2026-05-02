package base

import (
	"circuit/element"
	"circuit/mna"
)

// DFlipFlopType 原生 D 触发器元件类型标识
var DFlipFlopType element.NodeType = element.AddElement(20, &DFlipFlop{
	&element.Config{
		Name: "F",
		Pin:  element.SetPin(element.PinBoolean, "clk", "d", "q", "nq"),
		ValueInit: []any{
			float64(5.0), // 0: 高电平电压 (V)
			float64(0.0), // 1: 上一时刻 clk 电压 (用于边沿检测)
			false,        // 2: 内部 Q 状态
			false,        // 3: 是否已完成首次收敛初始化
			float64(0.0), // 4: 上一时间步收敛时的 D 输入电压
		},
		ValueName: []string{"V_high", "prev_clk", "q_state", "initialized", "prev_d"},
		Voltage:   []string{"vq", "vnq"},
		Flags:     element.FlagNonlinear,
		OrigValue: []int{1, 2, 3, 4},
	},
})

// DFlipFlop 原生 D 触发器元件，直接驱动输出无需求阻尼迭代。
type DFlipFlop struct{ *element.Config }

// Stamp 加盖 Q 和 NQ 输出的电压源。
// 保持当前的 Q/NQ 状态，避免自适应步长触发重新加盖时丢失状态。
func (f *DFlipFlop) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
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

// DoStep 检测时钟正边沿，采样 D 输入，并直接驱动 Q/NQ 输出电压。
//
// 边沿判断使用上一时间步收敛时保存的 D 输入电压 (prev_d)，
// 而非当前迭代中的实时电压。这确保：
//   - 同一时间步内所有牛顿迭代使用相同的 D 值
//   - D 值不受当前步中其他元件输出变化的影响
//   - 不受 Gate.Stamp() 导致的虚假时钟瞬态影响
func (f *DFlipFlop) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	highVoltage := value.GetFloat64(0)
	prevClk := value.GetFloat64(1)
	qState := value.GetBool(2)
	initialized := value.GetBool(3)

	clkNode := value.GetNodes(0)

	clkNow := m.GetNodeVoltage(clkNode)

	// 使用较高的边沿检测阈值（90% V_high），过滤门的阻尼中间电压
	// 和重新加盖期间的瞬态毛刺。只接受干净的满幅时钟边沿。
	edgeThreshold := highVoltage * 0.9
	isHigh := clkNow > edgeThreshold
	wasHigh := prevClk > highVoltage*0.5

	if initialized && isHigh && !wasHigh {
		prevD := value.GetFloat64(4)
		qState = prevD > highVoltage*0.5
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

// StepFinished 在时间步收敛完成后调用。
// 保存 D 输入电压供下一边沿判断，并延迟初始化直到时钟达到干净低电平。
func (f *DFlipFlop) StepFinished(m mna.Mna, t mna.Time, value element.NodeFace) {
	dVoltage := m.GetNodeVoltage(value.GetNodes(1))
	value.SetFloat64(4, dVoltage)

	if !value.GetBool(3) {
		clkNow := m.GetNodeVoltage(value.GetNodes(0))
		highVoltage := value.GetFloat64(0)
		if clkNow < highVoltage*0.3 {
			value.SetBool(3, true)
		}
	}
}
