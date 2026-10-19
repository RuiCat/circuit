// verify_cpu8v2_serial_test.go：57_cpu8v2_serial_load.net 的串口加载验收（K3 / 用户裁决 D1）。
//
// 判据（50-subtasks/m3.md）：
//  1. 12B 程序经 UART1.TX 事件逐字节送入（SYNC+LEN+DATA+CHK 帧），由 LOADER1（CpuMode=v2）
//     写入 iram16x8（独立 LWE 写使能），全程不做 SetString/InitFromHex/SetByte 旁路；
//  2. 加载期间 PC 冻结（顶层 PCLK=clk·nLOAD 门控），且 LWE 与 LOAD 相互独立；
//  3. 加载完成后运行，ACC 序列与 X1 的数据 RAM[14] 与 54b 预置期望一致
//     （期望取自 cmd/zz_cpu54b_test.go:66 与 :79，程序字节取自其 :60-65 的逐条指令注释）。
//
// 逐字节背压：每字节只在上一字节已被 UART 交给加载器（UART 槽 22 byteAck=1）后才写下一个
// 宿主事件，避免引擎单值事件槽被覆盖（与 serialbridge.Bridge 的 SendByte 背压语义同口径）。
package main_test

import (
	"fmt"
	"testing"
	"time"

	"circuit/element"
	_ "circuit/element/register" // 触发 UART/LOADER 的 init() 注册（K6/K8）
	"circuit/load"
	"circuit/serialbridge/elementio"
)

const (
	// cpu8v2SlotLoadError / cpu8v2SlotLoadDone 是 LOADER 的 K2 冻结追加槽（22/23）。
	cpu8v2SlotLoadError = 22
	cpu8v2SlotLoadDone  = 23
	// cpu8v2SlotUartByteAck 是 UART 的 K1 追加槽（22：加载器已 ACK 当前字节）。
	cpu8v2SlotUartByteAck = 22

	// cpu8v2LowStep / cpu8v2MaxStep：UART 与 LOADER 的 StepGuard 上界都是 T/10=1/(9600*10)≈1.04e-5 s，
	// 故引擎步长上界取 1e-5 s（C1/C5：时序类元件必须显式步长上界）。
	cpu8v2MinStep = 1e-6
	cpu8v2MaxStep = 1e-5

	// cpu8v2LoadStepBudget 是单字节等待预算：一帧 8N1=10 比特周期≈104 步，留 3 倍余量。
	cpu8v2LoadStepBudget = 320
	// cpu8v2RunStepBudget 是 CPU 运行预算：10 kHz 时钟下 11 条指令≈22 个时钟，按最小步长 1e-6 留足余量。
	cpu8v2RunStepBudget = 1500
)

// cpu8v2Program 是送入 iram 的 12 字节：1 字节起始校准 + 54b 的 11 条指令（地址各 +1）。
//
// 为什么要 +1：54 的 cpu8 没有复位/起始约定（原 54b 靠 t=0 的 DFF 首次边沿取全零初始态）。
// 本网表在 LOAD=1 期间冻结 CPU，释放后的第一个 PCLK 上升沿会把冻结期间采样的 D 装进寄存器：
// T←1、PC←1、IR←0（PC 因此跳过 iram[0]）。因此把 iram[0] 留作校准字节 0x00，
// 54b 的指令序列放到 iram[1..11]；第 10 条 JMP 原目标 10 随之改为 JMP 11（原地自循环）。
// 于是加载后执行的指令序列与 54b 完全相同，ACC 序列与 RAM[14] 期望不变。
var cpu8v2Program = []int{
	0x00, // 起始校准字节（不被取指；首个 PCLK 沿后 PC=1，取指从 iram[1] 开始）
	0x08, // LDA 8
	0x19, // ADD 9
	0x2A, // SUB 10
	0x3B, // AND 11
	0x4C, // OR 12
	0x5D, // XOR 13
	0x70, // SHL
	0x80, // SHR
	0x60, // NOT
	0x9E, // STA 14
	0xAB, // JMP 11（原 54b 为 JMP 10，因整体 +1 而同步 +1）
}

// cpu8v2WantAcc 是 54b 用例断言必须出现的 ACC 值（cmd/zz_cpu54b_test.go:66）。
var cpu8v2WantAcc = []int{0x3C, 0x60, 0x50, 0x00, 0x05, 0x06, 0x0C, 0x06, 0xF9}

// cpu8v2Frame 返回 SYNC+LEN+DATA+CHK 帧（协议常量与 element/loader 一致：SYNC=0xA5、
// CHK=(LEN+ΣDATA)&0xFF；本用例不引用包内常量，只按冻结协议构造）。
func cpu8v2Frame(prog []int) []int {
	frame := []int{0xA5, len(prog)}
	sum := len(prog)
	for _, b := range prog {
		frame = append(frame, b)
		sum += b
	}
	return append(frame, sum&0xFF)
}

// cpu8v2FindInstance 在顶层元件列表里按实例名取 NodeFace（与 elementio 的 findInstance 同口径）。
func cpu8v2FindInstance(t *testing.T, con *element.Context, name string) element.NodeFace {
	t.Helper()
	for _, face := range con.Nodelist {
		if base := face.Base(); base != nil && base.InstanceName == name {
			return face
		}
	}
	t.Fatalf("网表顶层找不到实例 %s", name)
	return nil
}

// cpu8v2ReadBus 读一组按位命名的节点（path 前缀 + 位下标），返回 8 位值。
// 例：cpu8v2ReadBus(con, "Q") 读顶层 Q0..Q7；cpu8v2ReadBus(con, "X1.PC") 读 cpu8 内部 PC0..PC7。
func cpu8v2ReadBus(con *element.Context, prefix string) int {
	v := 0
	for i := 0; i < 8; i++ {
		if con.GetHierarchicalNodeVoltage(fmt.Sprintf("%s%d", prefix, i)) >= 3.0 {
			v |= 1 << uint(i)
		}
	}
	return v
}

// cpu8v2ReadRamByte 读某个 16x8 RAM 实例的第 addr 字节：该实例内部 q<addr>[0:7] 总线展开为
// q<addr>0..q<addr>7，地址 0..15 时路径同时兼容 q0x/q10 两种拼法（q+addr+bit）。
func cpu8v2ReadRamByte(con *element.Context, instance string, addr int) int {
	v := 0
	for i := 0; i < 8; i++ {
		if con.GetHierarchicalNodeVoltage(fmt.Sprintf("%s.q%d%d", instance, addr, i)) >= 3.0 {
			v |= 1 << uint(i)
		}
	}
	return v
}

// cpu8v2AdvanceUntil 推进适配器直到 done() 成立，返回推进步数；超预算或引擎报错即终止用例。
func cpu8v2AdvanceUntil(t *testing.T, ad *elementio.Adapter, what string, budget int, done func() bool) int {
	t.Helper()
	for steps := 0; steps < budget; steps++ {
		if _, err := ad.Advance(); err != nil {
			t.Fatalf("%s：仿真推进失败（已推进 %d 步）：%v", what, steps, err)
		}
		if done() {
			return steps + 1
		}
	}
	t.Fatalf("%s：%d 步内未达成条件", what, budget)
	return 0
}

// TestCPU8v2SerialLoad 是 m3 的验收用例：串口加载 12B → 运行 → 与 54b 期望一致。
func TestCPU8v2SerialLoad(t *testing.T) {
	t.Setenv("CIRCUIT_LUSPARSE", "1")
	t.Setenv("CIRCUIT_GMCONT", "1")
	t.Setenv("CIRCUIT_GMINFINAL", "1e-5")
	t.Setenv("CIRCUIT_GLOBALGMIN", "1e-3")

	con, err := load.LoadString(readNetlist(t, "57_cpu8v2_serial_load.net"))
	if err != nil {
		t.Fatalf("加载 57 网表失败: %v", err)
	}
	uart := cpu8v2FindInstance(t, con, "UART1")
	loader := cpu8v2FindInstance(t, con, "LOADER1")
	// 顶层实例形态与加载参数（K1/K2）：13 针 UART；LOADER 为 v2 模式、LenMax ≥ 12。
	if got := uart.Config().PinNum(); got != 13 {
		t.Fatalf("UART1 应为 13 针字节口，实际 %d 针", got)
	}
	if got := loader.GetFloat64(0); got < float64(len(cpu8v2Program)) {
		t.Fatalf("LOADER1 的 LenMax=%v，必须 >= %d", got, len(cpu8v2Program))
	}
	if got := loader.GetString(2); got != "v2" {
		t.Fatalf("LOADER1 的 CpuMode=%q，期望 v2", got)
	}

	ad, err := elementio.NewAdapter(con, "UART1", elementio.Options{
		TargetTime:   1,
		MinStep:      cpu8v2MinStep,
		MaxStep:      cpu8v2MaxStep,
		StepTimeout:  60 * time.Second,
		CloseTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建 UART1 适配器失败: %v", err)
	}
	t.Cleanup(func() {
		if err := ad.Close(); err != nil {
			t.Errorf("关闭适配器失败: %v", err)
		}
	})

	started := time.Now()
	totalSteps := 0
	frame := cpu8v2Frame(cpu8v2Program)

	// ①② 逐字节送帧；每字节都等「UART 已把它交给加载器」（槽 22=1）后才写下一个事件，
	// 并在加载期间断言 PC 冻结（PCLK=clk·nLOAD 的门控生效）。
	for i, b := range frame {
		if err := ad.WriteEvent("UART1.TX", float64(b)); err != nil {
			t.Fatalf("写第 %d 字节事件失败: %v", i, err)
		}
		totalSteps += cpu8v2AdvanceUntil(t, ad, fmt.Sprintf("第 %d 字节被 UART 消费", i), cpu8v2LoadStepBudget,
			func() bool { return con.GetEvent("UART1.TX") == nil })
		// 先等 byteAck 回 0：只有 UART 真正把该字节取走（离开槽 3）时才会清 0；
		// 直接等 1 会被上一字节留下的 1 满足，导致下一事件覆盖槽 3（静默丢字节）。
		totalSteps += cpu8v2AdvanceUntil(t, ad, fmt.Sprintf("第 %d 字节离开 UART 槽 3", i), cpu8v2LoadStepBudget,
			func() bool { return uart.GetFloat64(cpu8v2SlotUartByteAck) == 0 })
		totalSteps += cpu8v2AdvanceUntil(t, ad, fmt.Sprintf("第 %d 字节被加载器 ACK", i), cpu8v2LoadStepBudget,
			func() bool { return uart.GetFloat64(cpu8v2SlotUartByteAck) == 1 })
		if pc := cpu8v2ReadBus(con, "X1.PC"); pc != 0 {
			t.Fatalf("第 %d 字节后加载仍在进行，但 PC=%02X：LOAD=1 期间 PC 未冻结", i, pc)
		}
		if i%4 == 3 || i == len(frame)-1 {
			t.Logf("加载进度 %d/%d 步=%d simT=%.6g", i+1, len(frame), totalSteps, con.Time.CurrentTime())
		}
	}

	// ③ 等加载完成：LOADER 槽 23 done=1；错误槽必须为 0（坏帧不得静默）。
	totalSteps += cpu8v2AdvanceUntil(t, ad, "加载器 done 或 error", cpu8v2LoadStepBudget,
		func() bool {
			return loader.GetFloat64(cpu8v2SlotLoadDone) != 0 || loader.GetFloat64(cpu8v2SlotLoadError) != 0
		})
	if got := loader.GetFloat64(cpu8v2SlotLoadError); got != 0 {
		t.Fatalf("加载报错 error=%v（帧/写时序失败），addr=%v", got, loader.GetFloat64(24))
	}
	if pc := cpu8v2ReadBus(con, "X1.PC"); pc != 0 {
		t.Fatalf("加载完成时 PC=%02X，期望 0（PC 冻结）", pc)
	}
	loadSteps := totalSteps
	if loadSteps > 2000 {
		t.Errorf("12B 串口加载 steps=%d > 2000（C5/C14 预算）", loadSteps)
	}
	// 引擎在两次接受步之间 MnaType.X 指向上一解（UpdateX 交换语义），故节点电压读数比元件槽
	// 晚一个接受步：再推进 2 步后判 LOAD/LWE 电平，避免把 done 前的 LOAD=5V 当成释放失败。
	runSteps := 0
	for ; runSteps < 2; runSteps++ {
		if _, err := ad.Advance(); err != nil {
			t.Fatalf("加载完成后推进失败（第 %d 步）：%v", runSteps, err)
		}
	}
	if v := con.GetHierarchicalNodeVoltage("LOAD"); v >= 2.5 {
		t.Fatalf("加载完成后 LOAD=%.3fV，期望已释放（<2.5V）", v)
	}
	if v := con.GetHierarchicalNodeVoltage("LWE"); v >= 2.5 {
		t.Fatalf("加载完成后 LWE=%.3fV，期望 0（写使能不得保持高）", v)
	}

	// ④ 程序确实被串口写入 iram（X1.X2= cpu8 的 iram16x8）：逐字节比对，未写地址保持 0。
	for addr, want := range cpu8v2Program {
		if got := cpu8v2ReadRamByte(con, "X1.X2", addr); got != want {
			t.Errorf("iram[%d]=%02X，期望 %02X（串口加载未按字节写入）", addr, got, want)
		}
	}
	for addr := len(cpu8v2Program); addr < 16; addr++ {
		if got := cpu8v2ReadRamByte(con, "X1.X2", addr); got != 0 {
			t.Errorf("iram[%d]=%02X，未加载地址应保持 0", addr, got)
		}
	}

	// ⑤ 运行 CPU：采样顶层 Q（ACC）与数据 RAM[14]（X1.X5= cpu8 的 ram16x8）。
	seen := make(map[int]bool, len(cpu8v2WantAcc))
	sampled := make([]int, 0, 16)
	prev := -1
	for ; runSteps < cpu8v2RunStepBudget; runSteps++ {
		if _, err := ad.Advance(); err != nil {
			t.Fatalf("CPU 运行阶段推进失败（已推进 %d 步）：%v", runSteps, err)
		}
		acc := cpu8v2ReadBus(con, "Q")
		seen[acc] = true
		if acc != prev {
			sampled = append(sampled, acc)
			prev = acc
			t.Logf("ACC 变化 step=%d simT=%.6g Q=%02X PC=%02X IR=%02X RAM14=%02X",
				runSteps, con.Time.CurrentTime(), acc, cpu8v2ReadBus(con, "X1.PC"),
				cpu8v2ReadBus(con, "X1.IRQ"), cpu8v2ReadRamByte(con, "X1.X5", 14))
		}
		if runSteps%100 == 0 {
			t.Logf("运行诊断 step=%d simT=%.6g dt=%.3g Q=%02X RAM14=%02X 序列=%v",
				runSteps, con.Time.CurrentTime(), con.Time.CurrentStep(), acc,
				cpu8v2ReadRamByte(con, "X1.X5", 14), sampled)
		}
		if time.Since(started) > 9*time.Minute {
			t.Fatalf("CPU 运行超过 9 分钟墙钟（step=%d simT=%.6g Q=%02X RAM14=%02X 序列=%v）",
				runSteps, con.Time.CurrentTime(), acc, cpu8v2ReadRamByte(con, "X1.X5", 14), sampled)
		}
		if cpu8v2AllSeen(seen) && cpu8v2ReadRamByte(con, "X1.X5", 14) == 0xF9 {
			runSteps++
			break
		}
	}
	totalSteps += runSteps
	wall := time.Since(started).Round(time.Millisecond)

	for _, want := range cpu8v2WantAcc {
		if !seen[want] {
			t.Errorf("ACC 未出现期望值 %02X（采样序列 %v）", want, sampled)
		}
	}
	if got := cpu8v2ReadRamByte(con, "X1.X5", 14); got != 0xF9 {
		t.Errorf("RAM[14]=%02X，期望 F9（与 54b 预置期望一致）", got)
	}
	if got := cpu8v2ReadBus(con, "Q"); got != 0xF9 {
		// 54b 只断言 ACC 序列出现过期望值与 RAM[14]；JMP 之后的取指相位可能再执行一条
		// 未加载地址（0x00=LDA 0），故最终 ACC 不作硬断言，只记录。
		t.Logf("运行结束 ACC=%02X（54b 判据为 ACC 序列含期望值 + RAM[14]）", got)
	}
	t.Logf("ACC 序列: %v（期望含 %02X..）", sampled, cpu8v2WantAcc)
	t.Logf("12B 加载 steps=%d（含 %d 字节帧）；CPU 运行 steps=%d；wall=%s", loadSteps, len(frame), runSteps, wall)
}

// cpu8v2AllSeen 报告 54b 期望的 ACC 值是否全部出现过。
func cpu8v2AllSeen(seen map[int]bool) bool {
	for _, want := range cpu8v2WantAcc {
		if !seen[want] {
			return false
		}
	}
	return true
}
