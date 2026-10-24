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

// TestCPU8MacroModel 用 RAM 行为模型替换 54b 的 iram16x8/ram16x8，跑 24ms 验证行为。
func TestCPU8MacroModel(t *testing.T) {
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	t.Setenv("CIRCUIT_GMCONT", "1")
	t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
	t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")
	nl := readNetlist(t, "54b_cpu8_preloaded.net")
	prog, okIram := load.IramHex(nl)
	if !okIram {
		t.Fatalf("从 54b 提取 iRAM 预置失败")
	}
	data, okRam := load.RamHex(nl)
	if !okRam {
		t.Fatalf("从 54b 提取数据 RAM 预置失败")
	}
	t.Logf("宏模型预置：iram=%s ram=%s", prog, data)

	iram := fmt.Sprintf(`.subckt iram16x8 A[0:3] RW D[0:7] Q[0:7]
RAM1 [A0,A1,A2,A3,-1,-1,-1,-1,D0,D1,D2,D3,D4,D5,D6,D7,RW,Q0,Q1,Q2,Q3,Q4,Q5,Q6,Q7] [16,"%s"]
.ends iram16x8`, prog)
	ram := fmt.Sprintf(`.subckt ram16x8 A[0:3] RW D[0:7] Q[0:7]
RAM2 [A0,A1,A2,A3,-1,-1,-1,-1,D0,D1,D2,D3,D4,D5,D6,D7,RW,Q0,Q1,Q2,Q3,Q4,Q5,Q6,Q7] [16,"%s"]
.ends ram16x8`, data)
	nl, okIram = load.ReplaceSubckt(nl, "iram16x8", iram)
	if !okIram {
		t.Fatalf("替换 iram16x8 失败")
	}
	nl, okRam = load.ReplaceSubckt(nl, "ram16x8", ram)
	if !okRam {
		t.Fatalf("替换 ram16x8 失败")
	}

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
