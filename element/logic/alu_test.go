// alu_test.go：8 位 ALU 与 8×8 寄存器堆行为元件的单元用例（55 宏模型，P1）。
package logic

import (
	"fmt"
	"strings"
	"testing"

	"circuit/element"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

// aluNetlist 生成 ALU 单元测试网表：S0..S3/A/B 由直流电压源驱动，Y0..Y7 为输出节点。
func aluNetlist(op int, a, b uint8) string {
	var sb strings.Builder
	for i := 0; i < 4; i++ { // S0..S3：节点 1..4
		v := 0.0
		if op&(1<<i) != 0 {
			v = 5
		}
		fmt.Fprintf(&sb, "V%d [%d,-1] [0,0,0,0,%g]\n", i+1, i+1, v)
	}
	for i := 0; i < 8; i++ { // A0..A7：节点 11..18
		v := 0.0
		if a&(1<<i) != 0 {
			v = 5
		}
		fmt.Fprintf(&sb, "V%d [%d,-1] [0,0,0,0,%g]\n", 11+i, 11+i, v)
	}
	for i := 0; i < 8; i++ { // B0..B7：节点 21..28
		v := 0.0
		if b&(1<<i) != 0 {
			v = 5
		}
		fmt.Fprintf(&sb, "V%d [%d,-1] [0,0,0,0,%g]\n", 21+i, 21+i, v)
	}
	sb.WriteString("ALU1 [1,2,3,4,11,12,13,14,15,16,17,18,21,22,23,24,25,26,27,28,31,32,33,34,35,36,37,38] [5]\n")
	return sb.String()
}

// runAluOp 加载 ALU 网表跑一个短仿真，返回 Y0..Y7 的电平字节。
func runAluOp(t *testing.T, op int, a, b uint8) uint8 {
	t.Helper()
	con, err := load.LoadString(aluNetlist(op, a, b))
	if err != nil {
		t.Fatalf("ALU 网表加载失败（op=%d）: %v", op, err)
	}
	con.Time, err = etime.NewTimeMNA(2e-4)
	if err != nil {
		t.Fatalf("创建时间控制器失败: %v", err)
	}
	if err := con.Time.SetStepLimits(1e-6, 1e-5); err != nil {
		t.Fatalf("设置步长失败: %v", err)
	}
	if err := etime.TransientSimulation(con, func([]float64) {}); err != nil {
		t.Fatalf("ALU 仿真失败（op=%d a=%02X b=%02X）: %v", op, a, b, err)
	}
	var out uint8
	for i := 0; i < 8; i++ { // Y0..Y7：节点 31..38
		if con.GetRawNodeVoltage(mna.NodeID(31+i)) > 2.5 {
			out |= 1 << i
		}
	}
	return out
}

// TestAluOps 验证 ALU 全部操作码（与 55 的 alu8 解码一致）。
func TestAluOps(t *testing.T) {
	cases := []struct {
		name string
		op   int
		a    uint8
		b    uint8
		want uint8
	}{
		{"MOV_LDA", 0, 0x12, 0xA5, 0xA5},
		{"ADD", 1, 0x0F, 0x02, 0x11},
		{"ADD_wrap", 1, 0xFF, 0x02, 0x01},
		{"SUB", 2, 0x10, 0x03, 0x0D},
		{"SUB_wrap", 2, 0x00, 0x01, 0xFF},
		{"AND", 3, 0xCC, 0x0F, 0x0C},
		{"OR", 4, 0xC0, 0x0F, 0xCF},
		{"XOR", 5, 0xFF, 0x0F, 0xF0},
		{"NOT", 6, 0xAA, 0x00, 0x55},
		{"SHL", 7, 0x81, 0x00, 0x02},
		{"SHR", 8, 0x81, 0x00, 0x40},
		{"LD_noenable", 9, 0xFF, 0xFF, 0x00},
		{"ST", 10, 0x5A, 0xFF, 0x5A},
		{"JMP_noenable", 11, 0xFF, 0xFF, 0x00},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := runAluOp(t, c.op, c.a, c.b); got != c.want {
				t.Errorf("op=%d a=%02X b=%02X: Y=%02X，期望 %02X", c.op, c.a, c.b, got, c.want)
			}
		})
	}
}

// regFileNetlist 生成寄存器堆单元测试网表：CLK 为 1kHz 脉冲（0 高，0.5ms 低，1ms 升沿），
// D0..D7 由直流电压源驱动，Q0..Q7 为输出节点。
func regFileNetlist(d uint8) string {
	var sb strings.Builder
	sb.WriteString("V1 [1,-1] [5,0,1000,0,5,0.5]\n")
	for i := 0; i < 8; i++ { // D0..D7：节点 2..9
		v := 0.0
		if d&(1<<i) != 0 {
			v = 5
		}
		fmt.Fprintf(&sb, "V%d [%d,-1] [0,0,0,0,%g]\n", 2+i, 2+i, v)
	}
	sb.WriteString("REGF1 [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17] [5]\n")
	return sb.String()
}

// TestRegFileEdge 验证寄存器堆在 CLK 正沿采样 D，并验证 Reset 把输出回 0。
func TestRegFileEdge(t *testing.T) {
	const pattern = 0xA5
	con, err := load.LoadString(regFileNetlist(pattern))
	if err != nil {
		t.Fatalf("REGF 网表加载失败: %v", err)
	}
	con.Time, err = etime.NewTimeMNA(1.5e-3)
	if err != nil {
		t.Fatalf("创建时间控制器失败: %v", err)
	}
	if err := con.Time.SetStepLimits(1e-6, 1e-5); err != nil {
		t.Fatalf("设置步长失败: %v", err)
	}
	con.Time.SetTriggers([]mna.Trigger{{Time: 0.5e-3}, {Time: 1e-3}, {Time: 1.5e-3}})
	if err := etime.TransientSimulation(con, func([]float64) {}); err != nil {
		t.Fatalf("REGF 仿真失败: %v", err)
	}

	var got uint8
	for i := 0; i < 8; i++ { // Q0..Q7：节点 10..17
		if con.GetRawNodeVoltage(mna.NodeID(10+i)) > 2.5 {
			got |= 1 << i
		}
	}
	if got != pattern {
		t.Fatalf("时钟正沿采样失败: Q=%02X，期望 %02X", got, pattern)
	}

	var reg element.NodeFace
	for _, e := range con.Nodelist {
		if e.Type() == RegFileType {
			reg = e
			break
		}
	}
	if reg == nil {
		t.Fatalf("网表中未找到 REGF 元件")
	}
	if err := con.CallMark(element.MarkReset); err != nil {
		t.Fatalf("MarkReset 失败: %v", err)
	}
	if q := reg.GetInt(2); q != 0 {
		t.Fatalf("Reset 后 q_state=%02X，期望 0", q)
	}
}
