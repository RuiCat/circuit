// macroPreset.go：宏模型辅助——从 54b 网表的 sram8_p* 预置单元提取 init_hex，
// 并提供子电路段定位/替换。供测试与网表处理复用（生产代码，非测试内字符串切片）。
package load

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// SubcktRange 返回 .subckt name ... 到下一个 .ends 行的 [start,end)。
// 用「下一个 .ends」而非精确名字匹配：部分子电路的 .ends 带参数化名字（如 sram8_p{addr}）。
func SubcktRange(text, name string) (start, end int, ok bool) {
	start = strings.Index(text, ".subckt "+name+" ")
	if start < 0 {
		return 0, 0, false
	}
	rel := strings.Index(text[start:], "\n.ends ")
	if rel < 0 {
		return 0, 0, false
	}
	endStart := start + rel + 1
	if lineEnd := strings.Index(text[endStart:], "\n"); lineEnd >= 0 {
		return start, endStart + lineEnd, true
	}
	return start, len(text), true
}

// SubcktSegment 返回 .subckt name ... .ends 的文本；未找到返回 false。
func SubcktSegment(text, name string) (string, bool) {
	start, end, ok := SubcktRange(text, name)
	if !ok {
		return "", false
	}
	return text[start:end], true
}

// ReplaceSubckt 把 .subckt name ... .ends 整段替换为 repl；未找到返回 false。
func ReplaceSubckt(text, name, repl string) (string, bool) {
	start, end, ok := SubcktRange(text, name)
	if !ok {
		return text, false
	}
	return text[:start] + repl + text[end:], true
}

// PresetByte 从 sram8_pX 子电路的 SR1..SR8 初始状态提取字节（bit0..bit7）。
func PresetByte(text, name string) (byte, bool) {
	seg, ok := SubcktSegment(text, name)
	if !ok {
		return 0, false
	}
	var b byte
	for bit := 0; bit < 8; bit++ {
		marker := fmt.Sprintf("SR%d [", bit+1)
		idx := strings.Index(seg, marker)
		if idx < 0 {
			return 0, false
		}
		line := seg[idx:]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		if strings.Contains(line, "true") {
			b |= 1 << bit
		}
	}
	return b, true
}

// IramHex 从 sram8_pi0..pi10 提取 16 字节程序 hex（其余地址为 0）。
func IramHex(text string) (string, bool) {
	buf := make([]byte, 16)
	for addr := 0; addr < 11; addr++ {
		b, ok := PresetByte(text, fmt.Sprintf("sram8_pi%d", addr))
		if !ok {
			return "", false
		}
		buf[addr] = b
	}
	return hex.EncodeToString(buf), true
}

// RamHex 从 sram8_p8..p13 提取 16 字节数据 hex（RAM[8..13]）。
func RamHex(text string) (string, bool) {
	buf := make([]byte, 16)
	for addr := 8; addr <= 13; addr++ {
		b, ok := PresetByte(text, fmt.Sprintf("sram8_p%d", addr))
		if !ok {
			return "", false
		}
		buf[addr] = b
	}
	return hex.EncodeToString(buf), true
}
