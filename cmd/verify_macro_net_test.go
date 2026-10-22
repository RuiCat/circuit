// verify_macro_net_test.go 验证 58 宏模型网表（自带 init_hex）行为与 54b 一致。
package main_test

import (
	"encoding/hex"
	"strings"
	"testing"

	_ "circuit/element/register"
	"circuit/element"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

// TestCPU8MacroNetlist 加载 58 宏模型网表，跑 24ms 验证 ACC 序列与 RAM[14]。
func TestCPU8MacroNetlist(t *testing.T) {
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	t.Setenv("CIRCUIT_GMCONT", "1")
	t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
	t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")
	nl := readNetlist(t, "58_cpu8v2_macro.net")
	con, err := load.LoadString(nl)
	if err != nil {
		t.Fatalf("加载 58 宏模型网表失败: %v", err)
	}
	var ram2 element.NodeFace
	for _, e := range con.Nodelist {
		base := e.Base()
		if base == nil {
			continue
		}
		if strings.HasSuffix(base.InstanceName, "RAM2") {
			ram2 = base
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
	t.Logf("58 宏模型网表：元件=%d ACC 序列=%v", len(con.Nodelist), samples)
	want := []int{0x3C, 0x60, 0x50, 0x00, 0x05, 0x06, 0x0C, 0x06, 0xF9}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("ACC 未出现期望值 %02X，序列=%v", w, samples)
		}
	}
	r14 := 0
	if ram2 != nil {
		mem := ram2.GetString(2)
		if len(mem) >= 30 {
			if b, err := hex.DecodeString(mem[28:30]); err == nil {
				r14 = int(b[0])
			}
		}
	}
	t.Logf("58 宏模型网表 RAM[14]=%02X（期望 F9）", r14)
	if r14 != 0xF9 {
		t.Errorf("RAM[14]=%02X 期望 F9", r14)
	}
}
