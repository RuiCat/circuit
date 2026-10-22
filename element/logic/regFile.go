// regFile.go：8×8 位寄存器堆行为元件（55 的 reg8 子电路宏模型，端口顺序兼容）。
// 网表语法：REGF<name> [CLK,D0..D7,Q0..Q7] [V_high]
// 时序与 55 的 reg8（8 个 DFF + nq 下拉）一致：CLK 正沿（>0.9*V_high）采样
// 上一收敛时间步的 D 值并更新 Q；无沿时保持。Q[0:7] 由电压源驱动。
package logic

import (
	"circuit/element"
	"circuit/mna"
)

// RegFileType 寄存器堆元件类型标识（NodeType=56）。
var RegFileType element.NodeType = element.AddElement(56, &RegFile{
	&element.Config{
		Name: "REGF",
		Pin: element.SetPin(element.PinBoolean, "CLK",
			"D0", "D1", "D2", "D3", "D4", "D5", "D6", "D7",
			"Q0", "Q1", "Q2", "Q3", "Q4", "Q5", "Q6", "Q7"),
		ValueInit: []any{
			float64(5.0), // 0: 高电平电压 (V)
			float64(0.0), // 1: 上一时刻 CLK 电压（边沿检测）
			int(0),       // 2: 8 位输出状态
			false,        // 3: 是否已完成首次收敛初始化
			int(0),       // 4: 上一收敛步采到的 8 位 D 输入
		},
		ValueName: []string{"V_high", "prev_clk", "q_state", "initialized", "prev_d"},
		Voltage:   []string{"vq0", "vq1", "vq2", "vq3", "vq4", "vq5", "vq6", "vq7"},
		Flags:     element.FlagNonlinear,
		OrigValue: []int{1, 2, 3, 4},
	},
})

// RegFile 8×8 位寄存器堆行为元件。
type RegFile struct{ *element.Config }

// Stamp 按当前状态驱动 Q[0:7] 的电压源（初值即状态，避免虚假瞬态）。
func (r *RegFile) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	high := value.GetFloat64(0)
	if high <= 0 {
		high = 5.0
	}
	q := value.GetInt(2)
	for i := 0; i < 8; i++ {
		v := 0.0
		if q&(1<<i) != 0 {
			v = high
		}
		m.StampVoltageSource(value.GetNodes(9+i), -1, value.GetVoltSource(i), v) // Q0..Q7
	}
}

// DoStep 检测 CLK 正沿：初始化完成后，正沿把上一收敛步的 D 锁存进状态并更新 Q。
func (r *RegFile) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	high := value.GetFloat64(0)
	if high <= 0 {
		high = 5.0
	}
	prevClk := value.GetFloat64(1)
	q := value.GetInt(2)
	initialized := value.GetBool(3)

	clkNow := m.GetNodeVoltage(value.GetNodes(0))
	isHigh := clkNow > high*0.9
	// 与 DFF 相同的首次收敛初始化：t=0 时钟为高时不采样，避免上电假沿。
	if !initialized && (t.Time() > 1e-9 || !isHigh) {
		initialized = true
		value.SetBool(3, true)
	}
	wasHigh := prevClk > high*0.9
	if initialized && isHigh && !wasHigh {
		q = value.GetInt(4) // 上一收敛步采到的 D
		value.SetInt(2, q)
		t.NoConverged()
	}
	value.SetFloat64(1, clkNow)

	for i := 0; i < 8; i++ {
		v := 0.0
		if q&(1<<i) != 0 {
			v = high
		}
		m.UpdateVoltageSource(value.GetVoltSource(i), v)
	}
}

// StepFinished 保存本步收敛后的 D 输入（供下一步正沿采样），并延迟初始化到干净低电平。
func (r *RegFile) StepFinished(m mna.Mna, t mna.Time, value element.NodeFace) {
	high := value.GetFloat64(0)
	if high <= 0 {
		high = 5.0
	}
	d := 0
	for i := 0; i < 8; i++ {
		if m.GetNodeVoltage(value.GetNodes(1+i)) > high*0.5 { // D0..D7
			d |= 1 << i
		}
	}
	value.SetInt(4, d)

	if !value.GetBool(3) {
		clkNow := m.GetNodeVoltage(value.GetNodes(0))
		if clkNow < high*0.3 || t.Time() > 1e-9 {
			value.SetBool(3, true)
		}
	}
}

// Reset 把运行态槽复位到初值（Q=0，清边沿检测/初始化标记），与 DFlipFlop.Reset 同口径。
func (r *RegFile) Reset(value element.NodeFace) {
	if value == nil {
		return
	}
	cfg := value.Config()
	node := value.Base()
	if cfg == nil || node == nil {
		return
	}
	for i := 1; i <= 4; i++ {
		if i < len(cfg.ValueInit) && i < len(node.NodeValue) {
			node.NodeValue[i] = cfg.ValueInit[i]
			if _, ok := node.OrigValue[i]; ok {
				node.OrigValue[i] = cfg.ValueInit[i]
			}
		}
	}
}
