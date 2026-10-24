// macroSynth.go：D5 最小规则——把 54b 的 sram8 门级存储阵列合成为宏 RAM 子电路。
// 规则：iram16x8 用 sram8_pi0..pi10（16 字节，其余 0），ram16x8 用 sram8_p8..p13。
package load

import (
	"encoding/hex"
	"fmt"
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
