// macroPreset_test.go：load 的生产宏辅助函数（C12/D8）单测。
package load

import (
	"os"
	"strings"
	"testing"
)

// TestMacroPreset 验证 54b 网表的 iram/数据 RAM 预置提取与子电路段替换。
func TestMacroPreset(t *testing.T) {
	data, err := os.ReadFile("../cmd/circuits/54b_cpu8_preloaded.net")
	if err != nil {
		t.Fatalf("读取 54b 网表失败: %v", err)
	}
	nl := string(data)

	iram, ok := IramHex(nl)
	if !ok {
		t.Fatalf("IramHex 失败")
	}
	if want := "08192a3b4c5d7080609eaa0000000000"; iram != want {
		t.Errorf("iram 预置=%s，期望 %s", iram, want)
	}
	ram, ok := RamHex(nl)
	if !ok {
		t.Fatalf("RamHex 失败")
	}
	if want := "00000000000000003c24100f05030000"; ram != want {
		t.Errorf("数据 RAM 预置=%s，期望 %s", ram, want)
	}
	if _, _, ok := SubcktRange(nl, "iram16x8"); !ok {
		t.Errorf("SubcktRange 未找到 iram16x8")
	}
	if _, ok := SubcktSegment(nl, "sram8_pi0"); !ok {
		t.Errorf("SubcktSegment 未找到 sram8_pi0")
	}
	repl := ".subckt iram16x8 A[0:3] RW D[0:7] Q[0:7]\nRAM1 [A0,A1,A2,A3,-1,-1,-1,-1,D0,D1,D2,D3,D4,D5,D6,D7,RW,Q0,Q1,Q2,Q3,Q4,Q5,Q6,Q7] [16,\"08192a3b4c5d7080609eaa0000000000\"]\n.ends iram16x8"
	newText, ok := ReplaceSubckt(nl, "iram16x8", repl)
	if !ok {
		t.Fatalf("ReplaceSubckt 失败")
	}
	if !strings.Contains(newText, "RAM1 [A0,A1") {
		t.Errorf("替换后未写入 RAM1")
	}
}
