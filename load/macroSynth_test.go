// macroSynth_test.go：D5 最小规则的宏子电路合成单测（54b → 宏 RAM 段）。
package load

import (
	"os"
	"strings"
	"testing"
)

// TestAutoSynthMacro 验证自动识别地址范围得到的宏段与手动规则一致。
func TestAutoSynthMacro(t *testing.T) {
	data, err := os.ReadFile("../cmd/circuits/54b_cpu8_preloaded.net")
	if err != nil {
		t.Fatalf("读取 54b 网表失败: %v", err)
	}
	nl := string(data)
	auto, ok := AutoSynthMacro(nl)
	if !ok {
		t.Fatalf("AutoSynthMacro 失败")
	}
	manual, ok := SynthMacroSubckt(nl, "iram16x8", "RAM1", "pi", 0, 11)
	if !ok {
		t.Fatalf("手动合成 iram16x8 失败")
	}
	manual, ok = SynthMacroSubckt(manual, "ram16x8", "RAM2", "p", 8, 6)
	if !ok {
		t.Fatalf("手动合成 ram16x8 失败")
	}
	if auto != manual {
		t.Errorf("自动合成与手动规则不一致")
	}
	if got := CollectSramAddrs(".subckt x\nX2 [a] sram8_pi0\nX3 [b] sram8_pi5", "pi"); len(got) != 2 || got[0] != 0 || got[1] != 5 {
		t.Errorf("CollectSramAddrs=%v，期望 [0 5]", got)
	}
}

// TestSynthMacroSubckt 验证 iram16x8/ram16x8 合成出与预置一致的宏 RAM 段。
func TestSynthMacroSubckt(t *testing.T) {
	data, err := os.ReadFile("../cmd/circuits/54b_cpu8_preloaded.net")
	if err != nil {
		t.Fatalf("读取 54b 网表失败: %v", err)
	}
	nl := string(data)
	wantIram, ok := IramHex(nl)
	if !ok {
		t.Fatalf("IramHex 失败")
	}
	wantRam, ok := RamHex(nl)
	if !ok {
		t.Fatalf("RamHex 失败")
	}

	if _, _, ok := SubcktRange(nl, "iram16x8"); !ok {
		t.Fatalf("SubcktRange(iram16x8) 失败")
	}
	if _, ok := PresetByte(nl, "sram8_pi10"); !ok {
		t.Fatalf("PresetByte(sram8_pi10) 失败")
	}

	out, ok := SynthMacroSubckt(nl, "iram16x8", "RAM1", "pi", 0, 11)
	if !ok {
		t.Fatalf("合成 iram16x8 失败")
	}
	if !strings.Contains(out, `[16,"`+wantIram+`"]`) {
		t.Errorf("合成 iram 段缺预置 %s", wantIram)
	}
	out2, ok := SynthMacroSubckt(out, "ram16x8", "RAM2", "p", 8, 6)
	if !ok {
		t.Fatalf("合成 ram16x8 失败")
	}
	if !strings.Contains(out2, `[16,"`+wantRam+`"]`) {
		t.Errorf("合成 data RAM 段缺预置 %s", wantRam)
	}
	for _, want := range []string{
		".subckt iram16x8 A[0:3] RW D[0:7] Q[0:7]",
		".subckt ram16x8 A[0:3] RW D[0:7] Q[0:7]",
		"RAM1 [A0,A1,A2,A3,",
		"RAM2 [A0,A1,A2,A3,",
	} {
		if !strings.Contains(out2, want) {
			t.Errorf("合成结果缺 %q", want)
		}
	}
}
