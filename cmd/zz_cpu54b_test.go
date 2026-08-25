package main_test

import (
	"testing"

	_ "circuit/element/register"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

func TestCPU8v2Preloaded(t *testing.T) {
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	t.Setenv("CIRCUIT_GMCONT", "1")
	t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
	t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")
	nl := readNetlist(t, "54b_cpu8_preloaded.net")
	con, err := load.LoadString(nl)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
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
	etime.TransientSimulation(con, func([]float64) {
		q := readQ()
		seen[q] = true
		if q != prev {
			samples = append(samples, q)
			prev = q
		}
	})
	t.Logf("ACC 序列: %v", samples)
	want := []int{0x3C, 0x60, 0x50, 0x00, 0x05, 0x06, 0x0C, 0x06, 0xF9}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("ACC 未出现期望值 %02X，序列=%v", w, samples)
		}
	}
	r14 := 0
	for i := 0; i < 8; i++ {
		if con.GetHierarchicalNodeVoltage("X1.X5.q14"+string(rune('0'+i))) >= 3.0 {
			r14 |= 1 << i
		}
	}
	t.Logf("RAM[14] = %02X（期望 F9）", r14)
	if r14 != 0xF9 {
		t.Errorf("RAM[14] = %02X 期望 F9", r14)
	}
}
