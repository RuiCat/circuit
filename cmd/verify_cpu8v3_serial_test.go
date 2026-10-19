// verify_cpu8v3_serial_test.go：56 网表的串口加载验收——12B 程序经 UART1.TX 逐字节送入，
// LOADER1 写入 RAM 后运行 cpu8，断言 X1.R0_..R3_ = FF/00/FF/00（K3、C4、C10、C11）。
package main_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"circuit/element"
	_ "circuit/element/register"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

// K2 冻结的 LOADER 追加槽（element/loader/loaderSlots.go:33-50）：22 error、23 done、24 addr、
// 25 phase、26 frame、28 framePos、29 checkSum、32 pendingByte。m2 目前只导出 LoaderType、
// 未导出槽常量（K2/D5 的导出缺口已报 Lead），这里按冻结编号只读，不硬编码其它语义。
// K1 的 UART 13 针槽：3 txByte、22 byteAck、23 byteRdy、24 byteLatch。
const (
	SERIAL_LOADER_SLOT_ERROR   = 22
	SERIAL_LOADER_SLOT_DONE    = 23
	SERIAL_LOADER_SLOT_PENDING = 32
	SERIAL_LOADER_SLOT_FRAME   = 26
	SERIAL_LOADER_SLOT_POS     = 28
	SERIAL_LOADER_SLOT_CHKSUM  = 29
	SERIAL_RAM_SLOT_MEM_HEX    = 2  // RAM 的 mem_hex 槽（element/logic/ram.go:39-48）
	SERIAL_UART_SLOT_BYTEACK   = 22 // 加载器已 ACK 当前字节
	SERIAL_UART_TX_EVENT       = "UART1.TX"
	SERIAL_UART_INSTANCE       = "UART1"
	SERIAL_LOADER_INSTANCE     = "LOADER1"
	SERIAL_RAM_INSTANCE        = "X1.RAM1"
	SERIAL_LOAD_TARGET         = 42e-3 // 15B 加载（≈15.6ms）+ 12 条指令×2 时钟（=24ms）+ 余量
	SERIAL_TRIGGER_COUNT       = 42    // 0.5/1.5/.../41.5 ms，与 55 既有验收的采样节奏一致
	SERIAL_RAM_BYTES           = 256
	SERIAL_LOGIC_HIGH          = 3.0
)

// TestCPU8v3SerialLoad 走真实网表与真实事件入口：事件 "UART1.TX" 一次只写一个字节，
// 等 UART 的 byteAck（加载器已 ACK）后写下一字节（背压），15 字节帧由 LOADER1 解码并
// 用 LA/LD/LOAD/LWE 写进 RAM；加载完成后 CPU 释放、跑完 12 条指令，读回归寄存器。
// 不直接写 RAM 槽/内存映像，写入只能经 LOADER1 的电压源时序（C4）。
func TestCPU8v3SerialLoad(t *testing.T) {
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	t.Setenv("CIRCUIT_GMCONT", "1")
	t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
	t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")

	con, err := load.LoadString(readNetlist(t, "56_cpu8v3_serial_load.net"))
	if err != nil {
		t.Fatalf("加载 56 网表失败: %v", err)
	}
	uartNode, err := netlistInstance(con, SERIAL_UART_INSTANCE)
	if err != nil {
		t.Fatalf("顶层缺少 %s（C10）：%v", SERIAL_UART_INSTANCE, err)
	}
	loaderNode, err := netlistInstance(con, SERIAL_LOADER_INSTANCE)
	if err != nil {
		t.Fatalf("顶层缺少 %s：%v", SERIAL_LOADER_INSTANCE, err)
	}
	ramNode, err := netlistInstance(con, SERIAL_RAM_INSTANCE)
	if err != nil {
		t.Fatalf("找不到 RAM 实例 %s：%v", SERIAL_RAM_INSTANCE, err)
	}

	prog := cpu8v3Program()
	frame := serialLoadFrame(prog)
	con.Time, err = etime.NewTimeMNA(SERIAL_LOAD_TARGET)
	if err != nil {
		t.Fatalf("创建时间控制器失败: %v", err)
	}
	// C1/K3：UART 比特周期 T=1/9600≈104.2µs，步长必须 ≤T/10；这里统一取 1.04e-5。
	if err := con.Time.SetStepLimits(1e-6, 1.04e-5); err != nil {
		t.Fatalf("设置步长限制失败: %v", err)
	}
	con.Time.SetTriggers(serialLoadTriggers())

	con.SetEvent(SERIAL_UART_TX_EVENT, float64(frame[0]))
	sent := 1
	seenAck := false
	var ramHexAtRelease string
	ramCaptured := false
	latched := make([]int, 0, len(frame))
	// 背压：一次 ACK 只喂一个字节。byteAck 在下一字节被取走前保持 1，
	// 若只看「非 0 就写」会每步覆盖事件表单值，导致中间字节被吞。
	call := func([]float64) {
		if !ramCaptured && loaderNode.GetFloat64(SERIAL_LOADER_SLOT_DONE) != 0 {
			ramHexAtRelease = ramNode.GetString(SERIAL_RAM_SLOT_MEM_HEX)
			ramCaptured = true
		}
		if uartNode.GetFloat64(SERIAL_UART_SLOT_BYTEACK) == 0 {
			seenAck = false
			return
		}
		if seenAck {
			return
		}
		seenAck = true
		latched = append(latched, int(loaderNode.GetFloat64(SERIAL_LOADER_SLOT_PENDING)))
		if sent >= len(frame) || loaderNode.GetFloat64(SERIAL_LOADER_SLOT_ERROR) != 0 {
			return
		}
		con.SetEvent(SERIAL_UART_TX_EVENT, float64(frame[sent]))
		sent++
	}
	if err := etime.TransientSimulation(con, call); err != nil {
		t.Fatalf("仿真失败（已喂 %d/%d 字节）: %v", sent, len(frame), err)
	}
	t.Logf("steps=%d fed=%d/%d", con.Time.GoodIterations(), sent, len(frame))

	if got := loaderNode.GetFloat64(SERIAL_LOADER_SLOT_ERROR); got != 0 {
		t.Logf("帧诊断: frame=%v framePos=%v checkSum=%v latched=% X want=% X",
			loaderNode.GetFloat64(SERIAL_LOADER_SLOT_FRAME),
			loaderNode.GetFloat64(SERIAL_LOADER_SLOT_POS),
			loaderNode.GetFloat64(SERIAL_LOADER_SLOT_CHKSUM),
			latched, frame)
		t.Fatalf("LOADER1 置 error=%v，帧未被接受（fed=%d/%d）", got, sent, len(frame))
	}
	if got := loaderNode.GetFloat64(SERIAL_LOADER_SLOT_DONE); got == 0 {
		t.Fatalf("LOADER1 未完成加载（fed=%d/%d，CpuMode 或握手不符）", sent, len(frame))
	}
	if sent != len(frame) {
		t.Fatalf("只喂入 %d/%d 字节", sent, len(frame))
	}
	if !ramCaptured {
		t.Fatalf("加载完成时未捕获 RAM.mem_hex")
	}
	// C4：加载完成、CPU 释放前，RAM 前 12 字节必须与程序逐字节一致，其余为 0。
	mem := make([]byte, SERIAL_RAM_BYTES)
	raw, err := hex.DecodeString(ramHexAtRelease)
	if err != nil {
		t.Fatalf("RAM 的 mem_hex 槽不是合法十六进制: %v", err)
	}
	copy(mem, raw)
	for addr, want := range prog {
		if mem[addr] != want {
			t.Errorf("RAM[%d]=%02X，期望 %02X（加载写路径）", addr, mem[addr], want)
		}
	}
	for addr := len(prog); addr < len(mem); addr++ {
		if mem[addr] != 0 {
			t.Errorf("越界写入：RAM[%d]=%02X，期望 00", addr, mem[addr])
		}
	}

	r0 := readHierarchicalByte(con, "X1.R0_")
	r1 := readHierarchicalByte(con, "X1.R1_")
	r2 := readHierarchicalByte(con, "X1.R2_")
	r3 := readHierarchicalByte(con, "X1.R3_")
	t.Logf("运行后: R0=%02X R1=%02X R2=%02X R3=%02X（期望 FF 00 FF 00）", r0, r1, r2, r3)
	if r0 != 0xFF || r1 != 0x00 || r2 != 0xFF || r3 != 0x00 {
		t.Errorf("指令集验证失败: R0=%02X R1=%02X R2=%02X R3=%02X", r0, r1, r2, r3)
	}
}

// cpu8v3Program 是 55 既有验收用的 12 字节程序（cmd/verify_digital_test.go:633）。
func cpu8v3Program() []byte {
	return []byte{0x64, 0x2C, 0x09, 0x16, 0x36, 0x46, 0x56, 0x74, 0x84, 0xA4, 0x94, 0x02}
}

// serialLoadFrame 按 K2 冻结协议组帧：SYNC=0xA5、LEN、DATA、CHK=(LEN+ΣDATA)&0xFF。
func serialLoadFrame(prog []byte) []byte {
	sum := len(prog)
	frame := make([]byte, 0, len(prog)+3)
	frame = append(frame, 0xA5, byte(len(prog)))
	for _, b := range prog {
		frame = append(frame, b)
		sum += int(b)
	}
	return append(frame, byte(sum&0xFF))
}

// serialLoadTriggers 生成 0.5ms 间隔的强制时间点，让 1kHz 时钟边沿被稳定采样。
func serialLoadTriggers() []mna.Trigger {
	trigs := make([]mna.Trigger, SERIAL_TRIGGER_COUNT)
	for i := range trigs {
		trigs[i] = mna.Trigger{Time: (0.5 + float64(i)) * 1e-3}
	}
	return trigs
}

// netlistInstance 按实例名在顶层 Nodelist 里找元件（C10：UART 必须在顶层）。
func netlistInstance(con *element.Context, name string) (element.NodeFace, error) {
	for _, n := range con.Nodelist {
		if n.Base().InstanceName == name {
			return n, nil
		}
	}
	return nil, fmt.Errorf("网表里找不到实例 %q", name)
}

// readHierarchicalByte 读层级路径 base+0..7 的 8 根节点并拼成一个字节（LSB 在 base+0）。
func readHierarchicalByte(con *element.Context, base string) int {
	v := 0
	for i := 0; i < 8; i++ {
		if con.GetHierarchicalNodeVoltage(base+string(rune('0'+i))) >= SERIAL_LOGIC_HIGH {
			v |= 1 << i
		}
	}
	return v
}
