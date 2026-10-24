// macroSynth.go：D5 最小规则——把 54b 的 sram8 门级存储阵列合成为宏 RAM 子电路。
// 规则：iram16x8 用 sram8_pi0..pi10（16 字节，其余 0），ram16x8 用 sram8_p8..p13。
package load

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// SynthMacroSubckt 用 sram8_{prefix}{addrStart..addrStart+addrCount-1} 的预置把 .subckt name ... .ends
// 替换为一个宏 RAM 子电路（实例名 instance，参数 [16,"hex"]）；未找到预置返回 false。
// prefix 为 "pi"（iram）或 "p"（数据 RAM）。
func SynthMacroSubckt(text, name, instance, prefix string, addrStart, addrCount int) (string, bool) {
	buf := make([]byte, 16)
	for i := 0; i < addrCount; i++ {
		b, ok := PresetByte(text, fmt.Sprintf("sram8_%s%d", prefix, addrStart+i))
		if !ok {
			return text, false
		}
		buf[addrStart+i] = b
	}
	hexStr := hex.EncodeToString(buf)
	repl := fmt.Sprintf(".subckt %s A[0:3] RW D[0:7] Q[0:7]\n%s [A0,A1,A2,A3,-1,-1,-1,-1,D0,D1,D2,D3,D4,D5,D6,D7,RW,Q0,Q1,Q2,Q3,Q4,Q5,Q6,Q7] [16,\"%s\"]\n.ends %s",
		name, instance, hexStr, name)
	return ReplaceSubckt(text, name, repl)
}

// AutoSynthMacro 自动识别 54b 风格：含 sram8_pi*/sram8_p* 实例的 iram16x8/ram16x8 段，
// 按实例地址集合合成宏 RAM 段（地址范围不写死）；无可识别段返回 false。
func AutoSynthMacro(text string) (string, bool) {
	out := text
	changed := false
	for _, spec := range []struct {
		name, instance, prefix string
	}{
		{"iram16x8", "RAM1", "pi"},
		{"ram16x8", "RAM2", "p"},
	} {
		seg, ok := SubcktSegment(out, spec.name)
		if !ok {
			continue
		}
		addrs := CollectSramAddrs(seg, spec.prefix)
		if len(addrs) == 0 {
			continue
		}
		buf := make([]byte, 16)
		for _, addr := range addrs {
			if addr < 0 || addr >= 16 {
				continue
			}
			b, ok := PresetByte(text, fmt.Sprintf("sram8_%s%d", spec.prefix, addr))
			if !ok {
				return text, false
			}
			buf[addr] = b
		}
		repl := fmt.Sprintf(".subckt %s A[0:3] RW D[0:7] Q[0:7]\n%s [A0,A1,A2,A3,-1,-1,-1,-1,D0,D1,D2,D3,D4,D5,D6,D7,RW,Q0,Q1,Q2,Q3,Q4,Q5,Q6,Q7] [16,\"%s\"]\n.ends %s",
			spec.name, spec.instance, hex.EncodeToString(buf), spec.name)
		out, ok = ReplaceSubckt(out, spec.name, repl)
		if !ok {
			return text, false
		}
		changed = true
	}
	return out, changed
}

// CollectSramAddrs 从段文本收集 sram8_<prefix><n> 的地址集合（去重升序）。
func CollectSramAddrs(seg, prefix string) []int {
	marker := "sram8_" + prefix
	seen := map[int]bool{}
	for pos := 0; ; {
		idx := strings.Index(seg[pos:], marker)
		if idx < 0 {
			break
		}
		next := pos + idx + len(marker)
		n, digits := 0, 0
		for next < len(seg) && seg[next] >= '0' && seg[next] <= '9' {
			n = n*10 + int(seg[next]-'0')
			next++
			digits++
		}
		if digits > 0 {
			seen[n] = true
		}
		pos = pos + idx + len(marker)
	}
	addrs := make([]int, 0, len(seen))
	for a := range seen {
		addrs = append(addrs, a)
	}
	sort.Ints(addrs)
	return addrs
}
