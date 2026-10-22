// verify_60_macro_test.go 验证 60 宏模型网表（55 的 alu8/reg8 实例换成 ALU/REGF 行为元件）：
// 跑 24ms 程序，与 55 的实跑结果逐项对照（R0..R3、RAM[14]、RAM 内容），
// 并打印 55/60 的元件与节点数对比。
package main_test

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"circuit/element"
	"circuit/element/logic"
	_ "circuit/element/register"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

// cpu8Run 保存一次 CPU8 运行的结果与规模统计。
type cpu8Run struct {
	regs   [4]int
	ramHex string
	accSeq []int
	nodes  int
	elems  int
}

// runCPU8Netlist 加载网表、预载 55 的 12B 程序、跑 24ms，并采集结果、ALU 序列与规模。
func runCPU8Netlist(t *testing.T, name string) cpu8Run {
	t.Helper()
	con, err := load.LoadString(readNetlist(t, name))
	if err != nil {
		t.Fatalf("加载 %s 失败: %v", name, err)
	}
	var ram element.NodeFace
	for _, e := range con.Nodelist {
		base := e.Base()
		if base != nil && strings.HasSuffix(base.InstanceName, "RAM1") {
			ram = base
		}
	}
	if ram == nil {
		t.Fatalf("%s 中未找到 RAM1", name)
	}
	prog := []int{0x64, 0x2C, 0x09, 0x16, 0x36, 0x46, 0x56, 0x74, 0x84, 0xA4, 0x94, 0x02}
	mem := make([]byte, 256)
	for i, v := range prog {
		mem[i] = byte(v)
	}
	if err := (&logic.RAM{}).InitFromHex(ram, hex.EncodeToString(mem)); err != nil {
		t.Fatalf("%s 预载程序失败: %v", name, err)
	}

	con.Time, err = etime.NewTimeMNA(24e-3)
	if err != nil {
		t.Fatalf("%s time: %v", name, err)
	}
	if err := con.Time.SetStepLimits(1e-6, 5e-5); err != nil {
		t.Fatalf("%s 步长: %v", name, err)
	}
	var trigs []mna.Trigger
	for tms := 0.5; tms <= 23.5; tms++ {
		trigs = append(trigs, mna.Trigger{Time: tms * 1e-3})
	}
	con.Time.SetTriggers(trigs)

	var accSeq []int
	prev := -1
	if err := etime.TransientSimulation(con, func([]float64) {
		acc := readAcc(con)
		if acc != prev {
			accSeq = append(accSeq, acc)
			prev = acc
		}
	}); err != nil {
		t.Fatalf("%s 仿真失败: %v", name, err)
	}
	return cpu8Run{
		regs: [4]int{
			readMacroByte(con, "X1.R0_"), readMacroByte(con, "X1.R1_"),
			readMacroByte(con, "X1.R2_"), readMacroByte(con, "X1.R3_"),
		},
		ramHex: ram.GetString(2),
		accSeq: accSeq,
		nodes:  con.GetNodeNum(),
		elems:  len(con.Nodelist),
	}
}

// ramByteAt 从 mem_hex 字符串取指定地址的字节（越界或非法返回 0）。
func ramByteAt(hexStr string, addr int) int {
	if addr < 0 || len(hexStr) < addr*2+2 {
		return 0
	}
	b, err := hex.DecodeString(hexStr[addr*2 : addr*2+2])
	if err != nil {
		return 0
	}
	return int(b[0])
}

// accString 把 ALU 采样序列格式化为十六进制字符串（仅日志用）。
func accString(seq []int) string {
	parts := make([]string, len(seq))
	for i, v := range seq {
		parts[i] = fmt.Sprintf("%02X", v)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// TestCPU8v3MacroNetlist 加载 60 跑 24ms，与 55 实跑结果一致（R0..R3 / RAM[14] / RAM），
// 并断言 60 的元件数、节点数明显低于 55（55 基线 894 节点）。
func TestCPU8v3MacroNetlist(t *testing.T) {
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	t.Setenv("CIRCUIT_GMCONT", "1")
	t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
	t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")

	run60 := runCPU8Netlist(t, "60_cpu8v3_macro.net")
	run55 := runCPU8Netlist(t, "55_cpu8_regfile.net")
	t.Logf("55: 元件=%d 节点=%d ACC 序列=%s", run55.elems, run55.nodes, accString(run55.accSeq))
	t.Logf("60: 元件=%d 节点=%d ACC 序列=%s", run60.elems, run60.nodes, accString(run60.accSeq))

	// R0..R3 与 55 既有验收（FF/00/FF/00）一致，且与 55 实跑逐项相同。
	r := run60.regs
	t.Logf("60 运行后: R0=%02X R1=%02X R2=%02X R3=%02X（55 实跑 %02X/%02X/%02X/%02X）",
		r[0], r[1], r[2], r[3], run55.regs[0], run55.regs[1], run55.regs[2], run55.regs[3])
	if r[0] != 0xFF || r[1] != 0x00 || r[2] != 0xFF || r[3] != 0x00 {
		t.Errorf("指令集验证失败: R0=%02X R1=%02X R2=%02X R3=%02X", r[0], r[1], r[2], r[3])
	}
	if r != run55.regs {
		t.Errorf("60 寄存器结果与 55 不一致: 60=%v 55=%v", r, run55.regs)
	}

	// RAM[14] 与 RAM 内容与 55 实跑一致（55 的该程序只写 RAM[0]，RAM[14]=00）。
	r14 := ramByteAt(run60.ramHex, 14)
	r14_55 := ramByteAt(run55.ramHex, 14)
	t.Logf("RAM[14]: 60=%02X 55=%02X（期望一致）", r14, r14_55)
	if r14 != r14_55 {
		t.Errorf("RAM[14] 与 55 不一致: 60=%02X 55=%02X", r14, r14_55)
	}
	if r14 != 0x00 {
		t.Errorf("RAM[14]=%02X 期望 00", r14)
	}
	if run60.ramHex != run55.ramHex {
		t.Errorf("60 的 RAM 内容与 55 不一致（60=%s 55=%s）", run60.ramHex, run55.ramHex)
	}

	// 规模对比：60 节点数须明显低于 55（要求 <75%）。
	t.Logf("对比：55 元件=%d 节点=%d；60 元件=%d 节点=%d（节点降低 %.1f%%）",
		run55.elems, run55.nodes, run60.elems, run60.nodes,
		100*float64(run55.nodes-run60.nodes)/float64(run55.nodes))
	if run60.nodes*4 >= run55.nodes*3 {
		t.Errorf("60 节点数未明显低于 55: 60=%d 55=%d（要求 <75%%）", run60.nodes, run55.nodes)
	}
	if run60.elems >= run55.elems {
		t.Errorf("60 元件数未低于 55: 60=%d 55=%d", run60.elems, run55.elems)
	}
}

// readAcc 读取 cpu8 内部 ALU 输出 ALU0..ALU7（X1.ALU*）的电平字节。
func readAcc(con *element.Context) int {
	v := 0
	for i := 0; i < 8; i++ {
		if con.GetHierarchicalNodeVoltage("X1.ALU"+string(rune('0'+i))) >= 3.0 {
			v |= 1 << i
		}
	}
	return v
}

// readMacroByte 读取以 base 为前缀的 8 位层级节点字节（如 X1.R0_）。
func readMacroByte(con *element.Context, base string) int {
	v := 0
	for i := 0; i < 8; i++ {
		if con.GetHierarchicalNodeVoltage(base+string(rune('0'+i))) >= 3.0 {
			v |= 1 << i
		}
	}
	return v
}
