// verify_cpu8_netlist_equiv_test.go：G4 m2 的网表级等价断言（54b/58/59/60）。
//
// 运行级证据由以下测试覆盖（本测试只做快速、可复算的“源/预置/端口”一致性）：
//   - 58：TestCPU8MacroNetlist
//   - 59：TestCPU8v2SerialLoad（V5）
//   - 60：TestCPU8v3MacroNetlist
package main_test

import (
	"strings"
	"testing"
)

// TestCPU8NetlistEquivalence 断言 58/59 的宏 RAM 预置与 54b 的 sram8 预置逐字节一致，
// 60 保留 ALU/REGF 宏实例；59 的串口程序末字节为 0xAB（有意 +1，不是 58 的 0xAA）。
func TestCPU8NetlistEquivalence(t *testing.T) {
	nl54b := readNetlist(t, "54b_cpu8_preloaded.net")
	nl58 := readNetlist(t, "58_cpu8v2_macro.net")
	nl59 := readNetlist(t, "59_cpu8v2_serial_macro.net")
	nl60 := readNetlist(t, "60_cpu8v3_macro.net")

	wantIram := iramHex(t, nl54b)
	wantRam := ramHex(t, nl54b)
	if !strings.Contains(nl58, `[16,"`+wantIram+`"]`) {
		t.Errorf("58 的 iRAM 预置不等于 54b 的 sram8_pi0..pi10：%s", wantIram)
	}
	if !strings.Contains(nl58, `[16,"`+wantRam+`"]`) {
		t.Errorf("58 的数据 RAM 预置不等于 54b 的 sram8_p8..p13：%s", wantRam)
	}
	if !strings.Contains(nl59, `[16,"`+wantRam+`"]`) {
		t.Errorf("59 的 RAM2 预置不等于 58/54b 的数据 RAM：%s", wantRam)
	}
	// 59 的 iRAM 由串口加载写入（无 init）；数据 RAM 必须有 init。
	if !strings.Contains(nl59, "RAM1 [") || !strings.Contains(nl59, `RAM2 [`) {
		t.Fatalf("59 缺 RAM1/RAM2 宏实例")
	}
	if strings.Contains(nl59, `RAM2 [A0,A1,A2,A3,-1,-1,-1,-1,D0,D1,D2,D3,D4,D5,D6,D7,RW,Q0,Q1,Q2,Q3,Q4,Q5,Q6,Q7] [16]`) {
		t.Errorf("59 的 RAM2 仍是裸 [16]（缺 init_hex）")
	}
	// 59 的串口程序末字节 0xAB（JMP 11），是测试程序有意 +1；不是 58 的 0xAA。
	if got := cpu8v2Program[len(cpu8v2Program)-1]; got != 0xAB {
		t.Errorf("59 串口程序末字节=0x%02X，期望 0xAB（JMP 11）", got)
	}
	// 60 的 ALU/REGF 宏实例存在；55 的 alu8/reg8 端口一致性由 TestCPU8v3MacroNetlist 运行级覆盖。
	for _, want := range []string{"ALU1 [", "REGF1 ["} {
		if !strings.Contains(nl60, want) {
			t.Errorf("60 缺宏实例 %s", want)
		}
	}
}
