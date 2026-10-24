// RAM 元件实现：随机存取存储器（冯·诺依曼统一内存）。
// 网表语法：RAM<name> [A0..A7,D0..D7,WE,Q0..Q7] [size, init_hex]
//   - A[0:7]：8 位地址（256 字节）
//   - D[0:7]：写数据总线
//   - WE    ：写使能（高电平写，低电平保持）
//   - Q[0:7]：读数据输出（电压源驱动，组合读：Q = mem[A]）
//   - 参数 0：size（字节数，默认 256，范围 1..65536）
//   - 参数 1：init_hex（十六进制初始化内容，如 "08 19 2A"；空=全 0）
//
// 内存状态存于参数 2（string，十六进制，每字节 2 字符）：
//   - 仿真中 DoStep 写入时更新该字符串
//   - 外部可通过 SetString(2, ...) 动态装载程序（MCP set_element_param）
//
// 用途：CPU 的冯·诺依曼统一内存——程序与数据同址存放；
//
//	装载模式（WE 由外部控制）逐字节写入程序后切换运行。
package logic

import (
	"encoding/hex"
	"fmt"
	"strings"

	"circuit/element"
	"circuit/mna"
)

// RAMType RAM 元件类型标识(NodeType=47)，网表"RAM<name> <A[0:7],D[0:7],WE,Q[0:7]> [size, init]"。
// 引脚布局（24 引脚）：
//
//	0-7:  A0..A7 地址
//	8-15: D0..D7 写数据
//	16:   WE 写使能
//	17-24:Q0..Q7 读输出
var RAMType element.NodeType = element.AddElement(47, &RAM{
	&element.Config{
		Name: "RAM",
		Pin: element.SetPin(element.PinBoolean,
			"A0", "A1", "A2", "A3", "A4", "A5", "A6", "A7",
			"D0", "D1", "D2", "D3", "D4", "D5", "D6", "D7",
			"WE",
			"Q0", "Q1", "Q2", "Q3", "Q4", "Q5", "Q6", "Q7"),
		ValueInit: []any{
			float64(256), // 0: 容量（字节）
			"",           // 1: 初始化内容（十六进制，可选）
			"",           // 2: 当前内存状态（十六进制）
			float64(0),   // 3: WE 上升沿锁存的写地址
			float64(0),   // 4: WE 上升沿锁存的写数据
			float64(0),   // 5: 上一步 WE（0/1）
			float64(-1),  // 6: 幂等戳（上次更新锁存的接受步时刻）
		},
		ValueName: []string{"size", "init_hex", "mem_hex", "latch_addr", "latch_data", "prev_we", "last_step"},
		Voltage:   []string{"vq0", "vq1", "vq2", "vq3", "vq4", "vq5", "vq6", "vq7"},
		Flags:     element.FlagNonlinear,
		OrigValue: []int{2},
	},
})

// RAM 随机存取存储器。组合读（Q=mem[A]）；写为 WE 上升沿锁存 A/D、WE 高期间只写锁存值。
type RAM struct{ *element.Config }

// RAM 写锁存槽（ValueInit 3..6）：WE 上升沿锁存 A/D，WE 高期间只写锁存值，
// 避免地址/数据在 WE 高期间切换时写到旧地址（与门级 SRAM 的 WL 时序对齐）。
const (
	slotRamLatchAddr = 3
	slotRamLatchData = 4
	slotRamPrevWE    = 5
	slotRamLastStep  = 6
)

// ensureMem 确保 mem_hex 与 size 匹配：扩展/截断到 size 字节。
func ensureMem(value element.NodeFace) string {
	size := int(value.GetFloat64(0))
	if size <= 0 {
		size = 256
	}
	mem := value.GetString(2)
	if len(mem) >= size*2 {
		return mem[:size*2]
	}
	// 补齐（保留已有内容，尾部补 0）
	b := strings.Builder{}
	b.WriteString(mem)
	for i := len(mem); i < size*2; i++ {
		b.WriteByte('0')
	}
	return b.String()
}

// normalizeHex 按 size 把十六进制串去空白后截断/补零（与 ensureMem 的 mem_hex 长度口径一致）。
func normalizeHex(value element.NodeFace, hexStr string) string {
	size := int(value.GetFloat64(0))
	if size <= 0 {
		size = 256
	}
	s := strings.ReplaceAll(hexStr, " ", "")
	if len(s) > size*2 {
		return s[:size*2]
	}
	if len(s) < size*2 {
		s += strings.Repeat("0", size*2-len(s))
	}
	return s
}

// Reset 把内存恢复到初值（K5/C12），重复调用幂等。
//
// 语义（Lead D4 批准的 K5 修订）：
//   - init_hex 非空：mem_hex 恢复为 init_hex（去空白，按 size 截断/补零）；
//   - init_hex 为空：首次 Reset 把当前 mem_hex（含 InitFromHex/MCP 外部预载）快照进
//     init_hex 作为初值，之后每次 Reset 都恢复该初值。
//
// 之所以对空 init_hex 做首次快照而非直接清空：引擎每次 TransientSimulation 启动都会
// 自动调用 MarkReset（element/time/simulation.go:57），直接清空会抹掉仿真前的程序预载
// （如 cmd/verify_digital_test.go 的 InitFromHex 用法）。快照口径下，未预载的新 Context
// 初值仍为全 0，等价于清空。
func (r *RAM) Reset(value element.NodeFace) {
	initHex := strings.TrimSpace(value.GetString(1))
	if initHex == "" {
		initHex = normalizeHex(value, value.GetString(2))
		value.SetString(1, initHex)
	} else {
		initHex = normalizeHex(value, initHex)
	}
	value.SetString(2, initHex)
	// 同步刷新备份：引擎每步的 MarkRollbackElements 只回滚 OrigValue 里登记的槽。
	if node := value.Base(); node != nil {
		if _, ok := node.OrigValue[2]; ok {
			node.OrigValue[2] = initHex
		}
	}
}

// Stamp 驱动 Q0..Q7 输出电压源为当前地址对应字节。
func (r *RAM) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	high := 5.0
	mem := ensureMem(value)
	// 初始读地址 0（DoStep 会立即更新）
	byteVal := byte(0)
	if len(mem) >= 2 {
		if v, err := hex.DecodeString(mem[:2]); err == nil && len(v) == 1 {
			byteVal = v[0]
		}
	}
	for i := 0; i < 8; i++ {
		volt := 0.0
		if byteVal&(1<<i) != 0 {
			volt = high
		}
		m.StampVoltageSource(value.GetNodes(17+i), -1, value.GetVoltSource(i), volt)
	}
}

// DoStep 组合读 + 电平写。
func (r *RAM) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	high := 5.0
	size := int(value.GetFloat64(0))
	if size <= 0 {
		size = 256
	}
	memHex := ensureMem(value)
	mem, _ := hex.DecodeString(memHex)
	if len(mem) < size {
		mem = append(mem, make([]byte, size-len(mem))...)
	}

	// 读地址（A0..A7 引脚 0-7，50% 阈值判高）
	addr := 0
	for i := 0; i < 8; i++ {
		if m.GetNodeVoltage(value.GetNodes(i)) > high*0.5 {
			addr |= 1 << i
		}
	}
	if addr >= size {
		addr = size - 1
	}

	// WE 上升沿锁存 A/D；WE 高期间只写锁存值（与门级 SRAM 的 WL 时序对齐）。
	// 每个接受步只更新一次锁存（幂等戳），同一步的牛顿迭代不重复锁存。
	data := 0
	for i := 0; i < 8; i++ {
		if m.GetNodeVoltage(value.GetNodes(8+i)) > high*0.5 {
			data |= 1 << i
		}
	}
	we := m.GetNodeVoltage(value.GetNodes(16)) > high*0.5
	now := 0.0
	if t != nil {
		now = t.Time()
	}
	if last := value.GetFloat64(slotRamLastStep); last != now {
		prev := value.GetFloat64(slotRamPrevWE) != 0
		if we && !prev {
			value.SetFloat64(slotRamLatchAddr, float64(addr))
			value.SetFloat64(slotRamLatchData, float64(data))
		}
		if we {
			value.SetFloat64(slotRamPrevWE, 1)
		} else {
			value.SetFloat64(slotRamPrevWE, 0)
		}
		value.SetFloat64(slotRamLastStep, now)
	}
	if we {
		latchAddr := int(value.GetFloat64(slotRamLatchAddr))
		latchData := int(value.GetFloat64(slotRamLatchData))
		if latchAddr >= 0 && latchAddr < size && mem[latchAddr] != byte(latchData) {
			mem[latchAddr] = byte(latchData)
			value.SetString(2, hex.EncodeToString(mem))
			t.NoConverged()
		}
	}

	// 读输出 Q = mem[addr]
	byteVal := mem[addr]
	for i := 0; i < 8; i++ {
		volt := 0.0
		if byteVal&(1<<i) != 0 {
			volt = high
		}
		m.UpdateVoltageSource(value.GetVoltSource(i), volt)
	}
}

// SetByte 便捷方法：通过 SetString 装载单个字节（供外部/MCP 装载程序用）。
func (r *RAM) SetByte(value element.NodeFace, addr int, b byte) {
	size := int(value.GetFloat64(0))
	if size <= 0 {
		size = 256
	}
	memHex := ensureMem(value)
	mem, _ := hex.DecodeString(memHex)
	if len(mem) < size {
		mem = append(mem, make([]byte, size-len(mem))...)
	}
	if addr >= 0 && addr < size {
		mem[addr] = b
		value.SetString(2, hex.EncodeToString(mem))
	}
}

// GetByte 便捷方法：读取单个字节。
func (r *RAM) GetByte(value element.NodeFace, addr int) byte {
	size := int(value.GetFloat64(0))
	if size <= 0 {
		size = 256
	}
	memHex := ensureMem(value)
	mem, _ := hex.DecodeString(memHex)
	if addr >= 0 && addr < len(mem) {
		return mem[addr]
	}
	return 0
}

// InitFromHex 从十六进制字符串初始化内存（MCP 装载程序入口）。
func (r *RAM) InitFromHex(value element.NodeFace, hexStr string) error {
	s := strings.ReplaceAll(hexStr, " ", "")
	if len(s)%2 != 0 {
		return fmt.Errorf("RAM: 十六进制长度必须为偶数")
	}
	mem, err := hex.DecodeString(s)
	if err != nil {
		return fmt.Errorf("RAM: 十六进制解析失败: %v", err)
	}
	value.SetString(2, hex.EncodeToString(mem))
	return nil
}
