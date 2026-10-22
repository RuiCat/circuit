// alu.go：8 位 ALU 行为元件（55 的 alu8 子电路宏模型，端口顺序与 alu8 兼容）。
// 网表语法：ALU<name> [S0,S1,S2,S3,A0..A7,B0..B7,Y0..Y7] [V_high]
// 操作码 S[0:3]（与 55 的 alu8 解码一致）：
//
//	0000 MOV/LDA → Y=B   0001 ADD → Y=A+B   0010 SUB → Y=A-B
//	0011 AND            0100 OR            0101 XOR
//	0110 NOT → Y=~A     0111 SHL → Y=A<<1  1000 SHR → Y=A>>1
//	1010 ST → Y=A
//	其余（1001 LD / 1011 JMP / 未定义）→ Y=0
//
// 输入 S/A/B 以 50% V_high 判电平，输出 Y[0:7] 由电压源驱动。
package logic

import (
	"circuit/element"
	"circuit/mna"
)

// AluType ALU 元件类型标识（NodeType=55）。
var AluType element.NodeType = element.AddElement(55, &Alu{
	&element.Config{
		Name: "ALU",
		Pin: element.SetPin(element.PinBoolean,
			"S0", "S1", "S2", "S3",
			"A0", "A1", "A2", "A3", "A4", "A5", "A6", "A7",
			"B0", "B1", "B2", "B3", "B4", "B5", "B6", "B7",
			"Y0", "Y1", "Y2", "Y3", "Y4", "Y5", "Y6", "Y7"),
		ValueInit: []any{
			float64(5.0), // 0: 高电平电压 (V)
		},
		ValueName: []string{"V_high"},
		Voltage:   []string{"vy0", "vy1", "vy2", "vy3", "vy4", "vy5", "vy6", "vy7"},
		Flags:     element.FlagNonlinear,
	},
})

// Alu 8 位组合 ALU 行为元件。
type Alu struct{ *element.Config }

// Stamp 为 8 个输出放置地参考电压源，初值取当前节点电压以避免时间步间虚假瞬态。
func (a *Alu) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	for i := 0; i < 8; i++ {
		node := value.GetNodes(20 + i) // Y0..Y7
		m.StampVoltageSource(node, -1, value.GetVoltSource(i), m.GetNodeVoltage(node))
	}
}

// DoStep 读取操作码与 A/B 操作数，按操作码驱动 Y[0:7]。
func (a *Alu) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	high := value.GetFloat64(0)
	if high <= 0 {
		high = 5.0
	}
	isHigh := func(i int) bool { return m.GetNodeVoltage(value.GetNodes(i)) > high*0.5 }
	readByte := func(base int) uint8 {
		var v uint8
		for i := 0; i < 8; i++ {
			if isHigh(base + i) {
				v |= 1 << i
			}
		}
		return v
	}

	op := 0
	for i := 0; i < 4; i++ {
		if isHigh(i) {
			op |= 1 << i
		}
	}
	x := readByte(4)  // A0..A7
	y := readByte(12) // B0..B7

	var res uint8
	switch op {
	case 0: // MOV/LDA：Y=B
		res = y
	case 1: // ADD
		res = x + y
	case 2: // SUB（8 位回绕，与 adder8 忽略进位一致）
		res = x - y
	case 3: // AND
		res = x & y
	case 4: // OR
		res = x | y
	case 5: // XOR
		res = x ^ y
	case 6: // NOT
		res = ^x
	case 7: // SHL
		res = x << 1
	case 8: // SHR
		res = x >> 1
	case 10: // ST：Y=A
		res = x
	default: // 1001 LD / 1011 JMP / 未定义：无使能，Y=0
		res = 0
	}

	for i := 0; i < 8; i++ {
		desired := 0.0
		if res&(1<<i) != 0 {
			desired = high
		}
		node := value.GetNodes(20 + i) // Y0..Y7
		current := m.GetNodeVoltage(node)
		// 与 Gate 相同的 0.75 阻尼：ALU 处在 CPU 组合反馈环上，避免迭代振荡。
		m.UpdateVoltageSource(value.GetVoltSource(i), current+0.75*(desired-current))
		diff := current - desired
		if diff < 0 {
			diff = -diff
		}
		if diff > high*0.1 {
			t.NoConverged()
		}
	}
}
