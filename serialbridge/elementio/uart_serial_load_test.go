//go:build unix

// uart_serial_load_test.go：13 针串口加载端到端（K3/K4/C6/C7/C14）。
//
// 链路：真实 pty 写 12B 帧 → serialbridge（开启 WaitByteAck 背压）→ elementio.Adapter
// → UART1 的 D0..D7/RDY/ACK 4 相握手 → LOADER1 解码并用 LA/LD/LOAD/LWE 写 RAM
// → CPU 释放后运行。判据只读节点 / RAM（V3 口径）：55/57 网表没有 CPU→UART 的数据通路，
// 故不把「CPU 输出经 pty 回读」当判据（C6）；Drain 也不参与断言。
package elementio_test

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"circuit/element"
	"circuit/element/loader"
	_ "circuit/element/register" // UART/LOADER 等元件的 init() 注册（K6）
	"circuit/load"
	"circuit/mna"
	"circuit/serialbridge"
	"circuit/serialbridge/elementio"
)

const (
	// serialLoadMinStep / serialLoadMaxStepV3：UART 的比特周期 T=1/9600≈104.2µs，
	// StepGuard 要求步长 ≤ T/10，故 V3 上界取 1.04e-5（与 m4 验收一致，C1/C5）。
	serialLoadMinStep   = 1e-6
	serialLoadMaxStepV3 = 1.04e-5
	// serialLoadMaxStepV2：57 用 10kHz 时钟（T=100µs），上界取 1e-5（与 m3 验收一致）。
	serialLoadMaxStepV2 = 1e-5

	// serialLoadTargetV3 是 55 版 cpu8 的验收时长：15B 帧加载（≈15.6ms）+ 12 条指令×2 时钟
	// （1kHz ⇒ 24ms）+ 余量 42ms（与 m4 验收的命令同口径）。
	serialLoadTargetV3 = 42e-3

	// serialLoadBridgeBudget 是桥对单字节 ACK 握手 / TX 空闲的步数预算：一帧 8N1=10 比特，
	// 每比特最多 10 步，握手再加余量；4096 步足以覆盖步长被 Gmin/Newton 折半的情形。
	serialLoadBridgeBudget = 4096
	// serialLoadStepCap 是加载 / 运行阶段的推进上限：防止实现退化时无界推进。
	serialLoadStepCap = 40000
	// serialLoadWallLimit 是端到端墙钟上限（C14：端到端墙钟必须单独测，超限即报错）。
	serialLoadWallLimit = 9 * time.Minute
	// serialLoadLogicHigh 是布尔节点判高的阈值（与既有 55/54 用例一致）。
	serialLoadLogicHigh = 3.0
	// serialLoadReadTimeout 是 pty 读期限；帧在 Ingest 前一次性写入，字节已在缓冲区内。
	serialLoadReadTimeout = 2 * time.Second
)

// serialLoadV3Program 是 55 既有验收的 12 字节程序（cmd/verify_digital_test.go:633）。
var serialLoadV3Program = []byte{0x64, 0x2C, 0x09, 0x16, 0x36, 0x46, 0x56, 0x74, 0x84, 0xA4, 0x94, 0x02}

// serialLoadV2Program 是 57 网表的 12 字节程序（m3 移交：起始校准 0x00 + 54b 的 11 条指令）。
var serialLoadV2Program = []byte{
	0x00, 0x08, 0x19, 0x2A, 0x3B, 0x4C, 0x5D, 0x70, 0x80, 0x60, 0x9E, 0xAB,
}

// TestUartSerialLoadV3Pty 是 56 网表的端到端验收（K3 的 V3 口径）：
// pty 写帧 → 桥逐字节背压 → LOADER1 写 RAM → CPU 运行 → R0..R3 = FF/00/FF/00，
// 且加载完成时 RAM[0:12] 与程序逐字节一致。
func TestUartSerialLoadV3Pty(t *testing.T) {
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	t.Setenv("CIRCUIT_GMCONT", "1")
	t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
	t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")

	con, err := load.LoadString(string(serialLoadNetlist(t, "56_cpu8v3_serial_load.net")))
	if err != nil {
		t.Fatalf("加载 56 网表失败: %v", err)
	}
	uart := serialLoadInstance(t, con, "UART1")
	if got := uart.Config().PinNum(); got != 13 {
		t.Fatalf("UART1 应为 13 针字节口（K1），实际 %d 针", got)
	}
	loaderNode := serialLoadInstance(t, con, "LOADER1")
	ram := serialLoadInstance(t, con, "X1.RAM1")
	memSlot := serialLoadSlotByName(t, ram, "mem_hex")

	ad, br, link, master := serialLoadOpen(t, con, serialLoadMinStep, serialLoadMaxStepV3, serialLoadTargetV3)
	serialLoadSetTriggers(t, con, serialLoadTargetV3)

	started := time.Now()
	loadStart := con.Time.GoodIterations()
	frame := serialLoadFrame(serialLoadV3Program)
	if _, err := master.Write(frame); err != nil {
		t.Fatalf("向 pty 写入帧失败: %v", err)
	}
	n, err := br.Ingest(link, len(frame))
	if err != nil {
		t.Fatalf("桥 Ingest 失败（已搬运 %d/%d 字节）: %v", n, len(frame), err)
	}
	if n != len(frame) {
		t.Fatalf("桥只搬运了 %d/%d 字节", n, len(frame))
	}
	serialLoadAdvanceUntil(t, ad, func() bool {
		return loaderNode.GetFloat64(loader.SlotDone) != 0 || loaderNode.GetFloat64(loader.SlotError) != 0
	}, "LOADER1 done 或 error")
	loadSteps := con.Time.GoodIterations() - loadStart
	if got := loaderNode.GetFloat64(loader.SlotError); got != 0 {
		t.Fatalf("LOADER1 置 error=%v（帧未被接受，addr=%v，已喂 %d 字节）",
			got, loaderNode.GetFloat64(loader.SlotAddr), n)
	}
	if got := loaderNode.GetFloat64(loader.SlotDone); got == 0 {
		t.Fatalf("LOADER1 未完成加载（已喂 %d 字节，steps=%d）", n, loadSteps)
	}

	// C4：加载完成、CPU 释放前 RAM[0:12] 必须与程序逐字节一致。
	mem := serialLoadDecodeRam(t, ram.GetString(memSlot))
	for addr, want := range serialLoadV3Program {
		if mem[addr] != want {
			t.Errorf("RAM[%d]=%02X，期望 %02X（串口加载写路径）", addr, mem[addr], want)
		}
	}

	runStart := con.Time.GoodIterations()
	serialLoadAdvanceToTime(t, ad, con, serialLoadTargetV3)
	runSteps := con.Time.GoodIterations() - runStart
	r0 := serialLoadReadByte(con, "X1.R0_")
	r1 := serialLoadReadByte(con, "X1.R1_")
	r2 := serialLoadReadByte(con, "X1.R2_")
	r3 := serialLoadReadByte(con, "X1.R3_")
	wall := time.Since(started).Round(time.Millisecond)
	t.Logf("55/V3: 加载 steps=%d 运行 steps=%d wall=%s simT=%.6g R0..R3=%02X %02X %02X %02X",
		loadSteps, runSteps, wall, con.Time.CurrentTime(), r0, r1, r2, r3)
	if r0 != 0xFF || r1 != 0x00 || r2 != 0xFF || r3 != 0x00 {
		t.Errorf("指令集验证失败: R0=%02X R1=%02X R2=%02X R3=%02X（期望 FF 00 FF 00）",
			r0, r1, r2, r3)
	}
}

// TestUartSerialLoadV2Pty 是 57 网表的可选端到端路径：同一条 pty→桥→LOADER1 链路，
// 断言数据 RAM[14]=F9（m3 已有网表级证据；本用例只补「经真实串口链路」这一层）。
// 该网表端到端需数分钟墙钟，故默认跳过，设 CIRCUIT_SERIAL_E2E_V2=1 显式运行。
func TestUartSerialLoadV2Pty(t *testing.T) {
	if os.Getenv("CIRCUIT_SERIAL_E2E_V2") == "" {
		t.Skip("57 网表端到端耗时数分钟；设 CIRCUIT_SERIAL_E2E_V2=1 显式运行")
	}
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	t.Setenv("CIRCUIT_GMCONT", "1")
	t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
	t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")

	con, err := load.LoadString(string(serialLoadNetlist(t, "57_cpu8v2_serial_load.net")))
	if err != nil {
		t.Fatalf("加载 57 网表失败: %v", err)
	}
	if got := serialLoadInstance(t, con, "UART1").Config().PinNum(); got != 13 {
		t.Fatalf("UART1 应为 13 针字节口（K1），实际 %d 针", got)
	}
	loaderNode := serialLoadInstance(t, con, "LOADER1")
	if got := loaderNode.GetString(2); got != "v2" {
		t.Fatalf("LOADER1 的 CpuMode=%q，期望 v2", got)
	}

	ad, br, link, master := serialLoadOpen(t, con, serialLoadMinStep, serialLoadMaxStepV2, 1)
	started := time.Now()
	loadStart := con.Time.GoodIterations()
	frame := serialLoadFrame(serialLoadV2Program)
	if _, err := master.Write(frame); err != nil {
		t.Fatalf("向 pty 写入帧失败: %v", err)
	}
	n, err := br.Ingest(link, len(frame))
	if err != nil {
		t.Fatalf("桥 Ingest 失败（已搬运 %d/%d 字节）: %v", n, len(frame), err)
	}
	serialLoadAdvanceUntil(t, ad, func() bool {
		return loaderNode.GetFloat64(loader.SlotDone) != 0 || loaderNode.GetFloat64(loader.SlotError) != 0
	}, "LOADER1 done 或 error")
	loadSteps := con.Time.GoodIterations() - loadStart
	if got := loaderNode.GetFloat64(loader.SlotError); got != 0 {
		t.Fatalf("LOADER1 置 error=%v（帧未被接受，已喂 %d 字节）", got, n)
	}

	runStart := con.Time.GoodIterations()
	serialLoadAdvanceUntil(t, ad, func() bool {
		return serialLoadReadRamByte(con, "X1.X5", 14) == 0xF9
	}, "CPU 运行到 RAM[14]=F9")
	runSteps := con.Time.GoodIterations() - runStart
	ram14 := serialLoadReadRamByte(con, "X1.X5", 14)
	wall := time.Since(started).Round(time.Millisecond)
	t.Logf("54/V2: 加载 steps=%d 运行 steps=%d wall=%s simT=%.6g RAM[14]=%02X",
		loadSteps, runSteps, wall, con.Time.CurrentTime(), ram14)
	if ram14 != 0xF9 {
		t.Errorf("RAM[14]=%02X，期望 F9（54b 预置期望）", ram14)
	}
}

// serialLoadNetlist 读取 cmd/circuits 下的网表（go test 工作目录是 serialbridge/elementio）。
func serialLoadNetlist(t *testing.T, name string) []byte {
	t.Helper()
	for _, p := range []string{
		filepath.Join("..", "..", "cmd", "circuits", name),
		filepath.Join("cmd", "circuits", name),
	} {
		if b, err := os.ReadFile(p); err == nil {
			return b
		}
	}
	t.Fatalf("找不到网表 %s", name)
	return nil
}

// serialLoadInstance 在顶层元件列表（含子电路平铺实例）里按实例名找元件。
func serialLoadInstance(t *testing.T, con *element.Context, name string) element.NodeFace {
	t.Helper()
	for _, face := range con.Nodelist {
		if base := face.Base(); base != nil && base.InstanceName == name {
			return face
		}
	}
	t.Fatalf("网表里找不到实例 %s", name)
	return nil
}

// serialLoadSlotByName 按 ValueName 找实例槽位下标：避免在用例里硬编码 RAM 的 mem_hex 槽号。
func serialLoadSlotByName(t *testing.T, face element.NodeFace, name string) int {
	t.Helper()
	cfg := face.Config()
	if cfg == nil {
		t.Fatalf("实例 %s 没有配置信息", face.Base().InstanceName)
	}
	for i, got := range cfg.ValueName {
		if got == name {
			return i
		}
	}
	t.Fatalf("实例 %s 没有名为 %q 的槽", face.Base().InstanceName, name)
	return 0
}

// serialLoadOpen 装配端到端链路：真实引擎适配器 → 开启 13 针 ACK 背压的桥 → pty 主/从设备。
// 返回的主设备用于写入待发送字节；从设备已由 OpenPort 置原始模式（字节不被行规程改写）。
func serialLoadOpen(t *testing.T, con *element.Context, minStep, maxStep, target float64,
) (*elementio.Adapter, *serialbridge.Bridge, serialbridge.Link, *os.File) {
	t.Helper()
	ad, err := elementio.NewAdapter(con, "UART1", elementio.Options{
		TargetTime: target, MinStep: minStep, MaxStep: maxStep,
		StepTimeout: 60 * time.Second, CloseTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("构造 UART1 适配器失败: %v", err)
	}
	t.Cleanup(func() {
		if err := ad.Close(); err != nil {
			t.Errorf("关闭适配器失败: %v", err)
		}
	})
	br, err := serialbridge.NewBridge(ad, serialbridge.Config{
		Instance: "UART1", StepBudget: serialLoadBridgeBudget, WaitByteAck: true,
	})
	if err != nil {
		t.Fatalf("构造桥失败: %v", err)
	}
	master, slavePath, err := serialbridge.OpenPty()
	if err != nil {
		t.Skipf("pty 自建不可用，跳过端到端: %v", err)
	}
	t.Cleanup(func() { _ = master.Close() })
	link, err := serialbridge.OpenPort(serialbridge.PortConfig{Name: slavePath, Timeout: serialLoadReadTimeout})
	if err != nil {
		t.Fatalf("打开 pty 从设备失败: %v", err)
	}
	t.Cleanup(func() { _ = link.Close() })
	return ad, br, link, master
}

// serialLoadSetTriggers 在 0.5ms 间隔上强制时间点，让 1kHz 时钟边沿被稳定采样
// （与 m4 验收的采样节奏一致）。此时引擎 goroutine 在 NewAdapter 里已 Pause、
// 尚未被任何 Advance 唤醒，写触发点没有并发读者。
func serialLoadSetTriggers(t *testing.T, con *element.Context, target float64) {
	t.Helper()
	var trigs []mna.Trigger
	for ms := 0.5; ms*1e-3 < target; ms++ {
		trigs = append(trigs, mna.Trigger{Time: ms * 1e-3})
	}
	con.Time.SetTriggers(trigs)
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

// serialLoadAdvanceUntil 逐仿真步推进直到 done() 成立；超步数或超墙钟即判失败。
func serialLoadAdvanceUntil(t *testing.T, ad *elementio.Adapter, done func() bool, what string) {
	t.Helper()
	started := time.Now()
	for steps := 0; steps < serialLoadStepCap; steps++ {
		if _, err := ad.Advance(); err != nil {
			t.Fatalf("%s：仿真推进失败（已推进 %d 步）：%v", what, steps, err)
		}
		if done() {
			return
		}
		if time.Since(started) > serialLoadWallLimit {
			t.Fatalf("%s：超过 %s 墙钟仍未达成", what, serialLoadWallLimit)
		}
	}
	t.Fatalf("%s：%d 步内未达成", what, serialLoadStepCap)
}

// serialLoadAdvanceToTime 推进到当前仿真时间 ≥ target（连续模式下由用例自己停）。
func serialLoadAdvanceToTime(t *testing.T, ad *elementio.Adapter, con *element.Context, target float64) {
	t.Helper()
	started := time.Now()
	for steps := 0; con.Time.CurrentTime() < target; steps++ {
		if steps >= serialLoadStepCap {
			t.Fatalf("推进到 simT=%g 超过 %d 步上限（当前 simT=%g）",
				target, serialLoadStepCap, con.Time.CurrentTime())
		}
		if _, err := ad.Advance(); err != nil {
			t.Fatalf("推进到 simT=%g 失败（已推进 %d 步）：%v", target, steps, err)
		}
		if time.Since(started) > serialLoadWallLimit {
			t.Fatalf("推进到 simT=%g 超过 %s 墙钟（当前 simT=%g）",
				target, serialLoadWallLimit, con.Time.CurrentTime())
		}
	}
}

// serialLoadReadByte 读 base+0..7 八根层级节点的电平并拼成字节（LSB 在 base+0）。
func serialLoadReadByte(con *element.Context, base string) int {
	value := 0
	for i := 0; i < 8; i++ {
		if con.GetHierarchicalNodeVoltage(base+fmt.Sprintf("%d", i)) >= serialLoadLogicHigh {
			value |= 1 << i
		}
	}
	return value
}

// serialLoadReadRamByte 读实例第 addr 字节：ram16x8 的 q<addr>[0:7] 展开为 q<addr>0..q<addr>7。
func serialLoadReadRamByte(con *element.Context, instance string, addr int) int {
	value := 0
	for i := 0; i < 8; i++ {
		node := fmt.Sprintf("%s.q%d%d", instance, addr, i)
		if con.GetHierarchicalNodeVoltage(node) >= serialLoadLogicHigh {
			value |= 1 << i
		}
	}
	return value
}

// serialLoadDecodeRam 把 RAM 的 mem_hex 槽解成 256 字节映像（空串视为全 0 存储）。
// 定长补 0：mem_hex 是紧凑十六进制串，短于容量时后续地址必须按 0 处理而不是越界。
func serialLoadDecodeRam(t *testing.T, memHex string) []byte {
	t.Helper()
	mem := make([]byte, 256)
	if memHex == "" {
		return mem
	}
	raw, err := hex.DecodeString(memHex)
	if err != nil {
		t.Fatalf("RAM 的 mem_hex 槽不是合法十六进制: %v", err)
	}
	copy(mem, raw)
	return mem
}
