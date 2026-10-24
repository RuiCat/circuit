// verify_cpu8v2_serial_short_test.go：C11——57 门级串口的短帧快速回归（进短门禁）。
// 只加载 3 字节并断言 iram 前 3 字节；完整 12B 运行仍由 nightly57.sh 覆盖。
package main_test

import (
	"fmt"
	"testing"
	"time"

	_ "circuit/element/register"
	"circuit/load"
	"circuit/serialbridge/elementio"
)

// TestCPU8v2SerialLoadShort 用 3 字节短帧验证 57 的 UART→LOADER→iram 写路径（不跑 CPU 运行）。
func TestCPU8v2SerialLoadShort(t *testing.T) {
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
	ad, err := elementio.NewAdapter(con, "UART1", elementio.Options{
		TargetTime:   1,
		MinStep:      cpu8v2MinStep,
		MaxStep:      cpu8v2MaxStep,
		StepTimeout:  60 * time.Second,
		CloseTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建适配器失败: %v", err)
	}
	t.Cleanup(func() {
		if err := ad.Close(); err != nil {
			t.Errorf("关闭适配器失败: %v", err)
		}
	})

	prog := cpu8v2Program[:2]
	frame := cpu8v2Frame(prog)
	steps := 0
	for i, b := range frame {
		if err := ad.WriteEvent("UART1.TX", float64(b)); err != nil {
			t.Fatalf("写第 %d 字节失败: %v", i, err)
		}
		steps += cpu8v2AdvanceUntil(t, ad, fmt.Sprintf("短帧第 %d 字节被消费", i), cpu8v2LoadStepBudget,
			func() bool { return con.GetEvent("UART1.TX") == nil })
		steps += cpu8v2AdvanceUntil(t, ad, fmt.Sprintf("短帧第 %d 字节离开槽 3", i), cpu8v2LoadStepBudget,
			func() bool { return uart.GetFloat64(cpu8v2SlotUartByteAck) == 0 })
		steps += cpu8v2AdvanceUntil(t, ad, fmt.Sprintf("短帧第 %d 字节被 ACK", i), cpu8v2LoadStepBudget,
			func() bool { return uart.GetFloat64(cpu8v2SlotUartByteAck) == 1 })
	}
	steps += cpu8v2AdvanceUntil(t, ad, "短帧 done 或 error", cpu8v2LoadStepBudget,
		func() bool {
			return loader.GetFloat64(cpu8v2SlotLoadDone) != 0 || loader.GetFloat64(cpu8v2SlotLoadError) != 0
		})
	if got := loader.GetFloat64(cpu8v2SlotLoadError); got != 0 {
		t.Fatalf("短帧加载报错 error=%v", got)
	}
	for addr, want := range prog {
		if got := cpu8v2ReadRamByte(con, "X1.X2", addr); got != want {
			t.Errorf("iram[%d]=%02X，期望 %02X", addr, got, want)
		}
	}
	if steps > 600 {
		t.Errorf("57 短帧 steps=%d > 600（短门禁预算）", steps)
	}
	t.Logf("57 短帧：%d 字节 steps=%d", len(prog), steps)
}
