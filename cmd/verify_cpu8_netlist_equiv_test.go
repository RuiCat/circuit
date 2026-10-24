// verify_cpu8_netlist_equiv_test.go：G4 m2 的 CPU8 网表等价性。
//
// 静态层：58/59 的宏 RAM 预置与 54b 的 sram8 预置逐字节一致；59 的串口程序末字节 0xAB。
// 运行层：54b 与 58 跑同一预置程序 24ms，比较 ACC 序列与 iram/data RAM 全量；
//         59 的运行级由 TestCPU8v2SerialLoad（V5）覆盖，60 的由 TestCPU8v3MacroNetlist 覆盖。
package main_test

import (
	"strings"
	"testing"

	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

// TestCPU8NetlistEquivalence 断言 58/59 的宏 RAM 预置与 54b 的 sram8 预置一致，
// 并真跑 54b/58 比较 ACC 序列与 RAM 全量。
func TestCPU8NetlistEquivalence(t *testing.T) {
	nl54b := readNetlist(t, "54b_cpu8_preloaded.net")
	nl58 := readNetlist(t, "58_cpu8v2_macro.net")
	nl59 := readNetlist(t, "59_cpu8v2_serial_macro.net")
	nl60 := readNetlist(t, "60_cpu8v3_macro.net")

	wantIram, okIram := load.IramHex(nl54b)
	if !okIram {
		t.Fatalf("从 54b 提取 iRAM 预置失败")
	}
	wantRam, okRam := load.RamHex(nl54b)
	if !okRam {
		t.Fatalf("从 54b 提取数据 RAM 预置失败")
	}
	if !strings.Contains(nl58, `[16,"`+wantIram+`"]`) {
		t.Errorf("58 的 iRAM 预置不等于 54b 的 sram8_pi0..pi10：%s", wantIram)
	}
	if !strings.Contains(nl58, `[16,"`+wantRam+`"]`) {
		t.Errorf("58 的数据 RAM 预置不等于 54b 的 sram8_p8..p13：%s", wantRam)
	}
	if !strings.Contains(nl59, `[16,"`+wantRam+`"]`) {
		t.Errorf("59 的 RAM2 预置不等于 58/54b 的数据 RAM：%s", wantRam)
	}
	if !strings.Contains(nl59, "RAM1 [") || !strings.Contains(nl59, `RAM2 [`) {
		t.Fatalf("59 缺 RAM1/RAM2 宏实例")
	}
	if strings.Contains(nl59, `RAM2 [A0,A1,A2,A3,-1,-1,-1,-1,D0,D1,D2,D3,D4,D5,D6,D7,RW,Q0,Q1,Q2,Q3,Q4,Q5,Q6,Q7] [16]`) {
		t.Errorf("59 的 RAM2 仍是裸 [16]（缺 init_hex）")
	}
	if got := cpu8v2Program[len(cpu8v2Program)-1]; got != 0xAB {
		t.Errorf("59 串口程序末字节=0x%02X，期望 0xAB（JMP 11）", got)
	}
	for _, want := range []string{"ALU1 [", "REGF1 ["} {
		if !strings.Contains(nl60, want) {
			t.Errorf("60 缺宏实例 %s", want)
		}
	}

	t.Run("54b_vs_58", func(t *testing.T) {
		t.Setenv("CIRCUIT_LUSPARSE", "1")
		t.Setenv("CIRCUIT_GMCONT", "1")
		t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
		t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")
		acc54, iram54, dram54 := runPreloadedCPU8(t, "54b_cpu8_preloaded.net")
		acc58, iram58, dram58 := runPreloadedCPU8(t, "58_cpu8v2_macro.net")
		if !equalInts(iram54, iram58) {
			t.Errorf("iram 不等价：54b=%02X 58=%02X", iram54, iram58)
		}
		if !equalInts(dram54, dram58) {
			t.Errorf("data RAM 不等价：54b=%02X 58=%02X", dram54, dram58)
		}
		if !equalInts(acc54, acc58) {
			t.Errorf("ACC 序列不等价：54b=%v 58=%v", acc54, acc58)
		}
		t.Logf("54b/58 等价：iram=%02X data=%02X ACC=%v", iram54, dram54, acc54)
	})
}

// runPreloadedCPU8 加载预置版 CPU8（54b/58），跑 24ms，返回 ACC 序列与 iram/data RAM 全量。
func runPreloadedCPU8(t *testing.T, name string) (accSeq, iram, dram []int) {
	t.Helper()
	con, err := load.LoadString(readNetlist(t, name))
	if err != nil {
		t.Fatalf("%s 加载失败: %v", name, err)
	}
	con.Time, err = etime.NewTimeMNA(24e-3)
	if err != nil {
		t.Fatalf("%s time: %v", name, err)
	}
	if err := con.Time.SetStepLimits(1e-6, 5e-5); err != nil {
		t.Fatalf("%s 步长: %v", name, err)
	}
	var trigs []mna.Trigger
	for tms := 0.5; tms <= 24; tms++ {
		trigs = append(trigs, mna.Trigger{Time: tms * 1e-3})
	}
	con.Time.SetTriggers(trigs)

	prev := -1
	if err := etime.TransientSimulation(con, func([]float64) {
		q := cpu8v2ReadBus(con, "Q")
		if q != prev {
			accSeq = append(accSeq, q)
			prev = q
		}
	}); err != nil {
		t.Fatalf("%s 仿真失败: %v", name, err)
	}
	ram1 := cpu8v2FindMacroRam(con, "RAM1")
	ram2 := cpu8v2FindMacroRam(con, "RAM2")
	for addr := 0; addr < 16; addr++ {
		if ram1 != nil {
			iram = append(iram, cpu8v2ReadMacroRamByte(ram1, addr))
		} else {
			iram = append(iram, cpu8v2ReadRamByte(con, "X1.X2", addr))
		}
		if ram2 != nil {
			dram = append(dram, cpu8v2ReadMacroRamByte(ram2, addr))
		} else {
			dram = append(dram, cpu8v2ReadRamByte(con, "X1.X5", addr))
		}
	}
	return accSeq, iram, dram
}

// equalInts 判断两个整型切片逐元素相等。
func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
