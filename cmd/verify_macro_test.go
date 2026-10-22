// verify_macro_test.go 验证宏模型替换：把 54b 的 iram16x8/ram16x8 门级实现
// 换成 logic.RAM 行为模型，断言行为与 54b 一致并对比元件数。
package main_test

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	_ "circuit/element/register"
	"circuit/element"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

// subcktRange 返回 .subckt name ... 到下一个 .ends 行的 [start,end) 范围。
// 用「下一个 .ends」而非精确名字匹配：网表里部分子电路的 .ends 带参数化名字
// （如 sram8_p{addr}），精确匹配会找不到。
func subcktRange(t *testing.T, nl, name string) (int, int) {
	t.Helper()
	start := strings.Index(nl, ".subckt "+name+" ")
	if start < 0 {
		t.Fatalf("找不到 .subckt %s", name)
	}
	rel := strings.Index(nl[start:], "\n.ends ")
	if rel < 0 {
		t.Fatalf("找不到 %s 的 .ends", name)
	}
	endStart := start + rel + 1
	if lineEnd := strings.Index(nl[endStart:], "\n"); lineEnd >= 0 {
		return start, endStart + lineEnd
	}
	return start, len(nl)
}

// replaceSubckt 把网表中的 .subckt name ... .ends 整段替换为 repl。
func replaceSubckt(t *testing.T, nl, name, repl string) string {
	t.Helper()
	start, end := subcktRange(t, nl, name)
	return nl[:start] + repl + nl[end:]
}

// subcktSegment 返回 .subckt name ... .ends 的文本。
func subcktSegment(t *testing.T, nl, name string) string {
	t.Helper()
	start, end := subcktRange(t, nl, name)
	return nl[start:end]
}

// presetByte 从 sram8_pX 子电路的 SR1..SR8 初始状态提取字节（bit0..bit7）。
func presetByte(t *testing.T, nl, name string) byte {
	t.Helper()
	seg := subcktSegment(t, nl, name)
	var b byte
	for bit := 0; bit < 8; bit++ {
		marker := fmt.Sprintf("SR%d [", bit+1)
		idx := strings.Index(seg, marker)
		if idx < 0 {
			t.Fatalf("%s 缺 %s", name, marker)
		}
		line := seg[idx:]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		if strings.Contains(line, "true") {
			b |= 1 << bit
		}
	}
	return b
}

// iramHex 从 sram8_pi0..pi10 提取 16 字节程序（其余地址为 0）。
func iramHex(t *testing.T, nl string) string {
	t.Helper()
	buf := make([]byte, 16)
	for addr := 0; addr < 11; addr++ {
		buf[addr] = presetByte(t, nl, fmt.Sprintf("sram8_pi%d", addr))
	}
	return hex.EncodeToString(buf)
}

// ramHex 从 sram8_p8..p13 提取 16 字节数据（RAM[8..13]）。
func ramHex(t *testing.T, nl string) string {
	t.Helper()
	buf := make([]byte, 16)
	for addr := 8; addr <= 13; addr++ {
		buf[addr] = presetByte(t, nl, fmt.Sprintf("sram8_p%d", addr))
	}
	return hex.EncodeToString(buf)
}

// TestCPU8MacroModel 用 RAM 行为模型替换 54b 的 iram16x8/ram16x8，跑 24ms 验证行为。
func TestCPU8MacroModel(t *testing.T) {
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	t.Setenv("CIRCUIT_GMCONT", "1")
	t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
	t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")
	nl := readNetlist(t, "54b_cpu8_preloaded.net")
	prog := iramHex(t, nl)
	data := ramHex(t, nl)
	t.Logf("宏模型预置：iram=%s ram=%s", prog, data)

	iram := fmt.Sprintf(`.subckt iram16x8 A[0:3] RW D[0:7] Q[0:7]
RAM1 [A0,A1,A2,A3,-1,-1,-1,-1,D0,D1,D2,D3,D4,D5,D6,D7,RW,Q0,Q1,Q2,Q3,Q4,Q5,Q6,Q7] [16,"%s"]
.ends iram16x8`, prog)
	ram := fmt.Sprintf(`.subckt ram16x8 A[0:3] RW D[0:7] Q[0:7]
RAM2 [A0,A1,A2,A3,-1,-1,-1,-1,D0,D1,D2,D3,D4,D5,D6,D7,RW,Q0,Q1,Q2,Q3,Q4,Q5,Q6,Q7] [16,"%s"]
.ends ram16x8`, data)
	nl = replaceSubckt(t, nl, "iram16x8", iram)
	nl = replaceSubckt(t, nl, "ram16x8", ram)

	con, err := load.LoadString(nl)
	if err != nil {
		t.Fatalf("加载宏模型网表失败: %v", err)
	}
	var ram2Node element.NodeFace
	for _, e := range con.Nodelist {
		base := e.Base()
		if base == nil {
			continue
		}
		switch {
		case strings.HasSuffix(base.InstanceName, "RAM1"):
			base.SetString(2, prog) // 预载程序（Reset 首次快照为初值）
		case strings.HasSuffix(base.InstanceName, "RAM2"):
			base.SetString(2, data) // 预载数据
			ram2Node = base
		}
		if strings.Contains(base.InstanceName, "RAM1") || strings.Contains(base.InstanceName, "RAM2") {
			t.Logf("%s size=%v init=%q mem=%q", base.InstanceName, base.NodeValue[0], base.NodeValue[1], base.NodeValue[2])
		}
	}
	con.Time, err = etime.NewTimeMNA(24e-3)
	if err != nil {
		t.Fatalf("time: %v", err)
	}
	if err := con.Time.SetStepLimits(1e-6, 5e-5); err != nil {
		t.Fatalf("步长: %v", err)
	}
	var trigs []mna.Trigger
	for tms := 0.5; tms <= 24; tms++ {
		trigs = append(trigs, mna.Trigger{Time: tms * 1e-3})
	}
	con.Time.SetTriggers(trigs)

	readQ := func() int {
		v := 0
		for i := 0; i < 8; i++ {
			if con.GetHierarchicalNodeVoltage("Q"+string(rune('0'+i))) >= 3.0 {
				v |= 1 << i
			}
		}
		return v
	}
	var samples []int
	seen := map[int]bool{}
	prev := -1
	if err := etime.TransientSimulation(con, func([]float64) {
		q := readQ()
		seen[q] = true
		if q != prev {
			samples = append(samples, q)
			prev = q
		}
	}); err != nil {
		t.Fatalf("仿真失败: %v", err)
	}
	t.Logf("宏模型：元件=%d ACC 序列=%v", len(con.Nodelist), samples)
	want := []int{0x3C, 0x60, 0x50, 0x00, 0x05, 0x06, 0x0C, 0x06, 0xF9}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("ACC 未出现期望值 %02X，序列=%v", w, samples)
		}
	}
	r14 := 0
	if ram2Node != nil {
		mem := ram2Node.GetString(2)
		if len(mem) >= 30 {
			if b, err := hex.DecodeString(mem[28:30]); err == nil {
				r14 = int(b[0])
			}
		}
	}
	t.Logf("宏模型 RAM[14]=%02X（期望 F9）", r14)
	if r14 != 0xF9 {
		t.Errorf("RAM[14]=%02X 期望 F9", r14)
	}
}
