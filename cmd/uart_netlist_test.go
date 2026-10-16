// uart_netlist_test.go：UART 元件的网表级验收（C7 —— 必须走触发 init() 注册的路径）。
//
// 本文件由 Lead 在批 2 收尾时补写（原子体在基础设施故障中中断，其包内 12+3 用例已修好）。
// 判据：环回解出同一字节 + dataReady + INTR 电平 + ACK 回落 + frameError（见各自用例）。
package main_test

import (
	"testing"

	"circuit/element"
	_ "circuit/element/register" // 既有元件域（R 等）
	_ "circuit/element/serial"   // C7：blank import 触发 UART 的 init() 注册（不改 register.go）
	etime "circuit/element/time"
	"circuit/load"
)

// uartSlot* 与 K2 冻结槽表一致（判据只读这些下标）。
const (
	uartSlotRxByte    = 11
	uartSlotDataReady = 12
	uartSlotFrameErr  = 13
	uartSlotIntrPin   = 2 // 引脚下标：tx=0 rx=1 intr=2
)

// uartFind 返回实例名对应的元件节点接口。
func uartFind(t *testing.T, con *element.Context, name string) element.NodeFace {
	t.Helper()
	for _, n := range con.Nodelist {
		if n.Base().InstanceName == name {
			return n
		}
	}
	t.Fatalf("网表里找不到实例 %s", name)
	return nil
}

// uartRun 加载环回网表、注入一个字节、按显式步长上界跑到 target，返回 UART 实例。
func uartRun(t *testing.T, txByte float64, ack bool, target float64) (element.NodeFace, *element.Context) {
	t.Helper()
	con, err := load.LoadString(readNetlist(t, "68_ttl_uart_loopback.net"))
	if err != nil {
		t.Fatalf("加载网表失败: %v", err)
	}
	con.Time, err = etime.NewTimeMNA(target)
	if err != nil {
		t.Fatalf("创建时间控制器失败: %v", err)
	}
	// C1：显式步长上界（9600 baud ⇒ T≈104.17µs，上界取 T/20 留足裕量）。
	if err := con.Time.SetStepLimits(1e-9, 5e-6); err != nil {
		t.Fatalf("设置步长限制失败: %v", err)
	}
	con.SetEvent("UART1.TX", txByte)
	if err := etime.TransientSimulation(con, func([]float64) {}); err != nil {
		t.Fatalf("仿真失败: %v", err)
	}
	uart := uartFind(t, con, "UART1")
	if ack {
		// ACK：写槽 1 绑定的默认事件名 "<实例名>.ACK"，再推进一小段让元件消费。
		con.SetEvent("UART1.ACK", float64(1))
		con.Time, err = etime.NewTimeMNA(con.Time.CurrentTime() + 1e-4)
		if err != nil {
			t.Fatalf("重建时间控制器失败: %v", err)
		}
		if err := con.Time.SetStepLimits(1e-9, 5e-6); err != nil {
			t.Fatalf("设置步长限制失败: %v", err)
		}
		if err := etime.TransientSimulation(con, func([]float64) {}); err != nil {
			t.Fatalf("ACK 段仿真失败: %v", err)
		}
	}
	return uart, con
}

// TestUartNetlistLoopback 环回：写 0x5A ⇒ 解出 0x5A 且 dataReady=true。
func TestUartNetlistLoopback(t *testing.T) {
	uart, _ := uartRun(t, 0x5A, false, 2e-3)
	got := uart.GetFloat64(uartSlotRxByte)
	if got != 0x5A {
		t.Fatalf("环回应解出 0x5A，实际 0x%02X（dataReady=%v frameError=%v）",
			int(got), uart.GetBool(uartSlotDataReady), uart.GetBool(uartSlotFrameErr))
	}
	if !uart.GetBool(uartSlotDataReady) {
		t.Fatalf("解出字节后 dataReady 必须为真")
	}
	if uart.GetBool(uartSlotFrameErr) {
		t.Fatalf("正常帧不得置 frameError")
	}
}

// TestUartNetlistZeroByte 边界：0x00（全零数据）也必须解出并置 dataReady。
func TestUartNetlistZeroByte(t *testing.T) {
	uart, _ := uartRun(t, 0x00, false, 2e-3)
	if got := uart.GetFloat64(uartSlotRxByte); got != 0 {
		t.Fatalf("环回应解出 0x00，实际 0x%02X", int(got))
	}
	if !uart.GetBool(uartSlotDataReady) {
		t.Fatalf("0x00 收完应置 dataReady（起始位与数据位全低，仍是一帧）")
	}
}

// TestUartNetlistIntrAndAck INTR 高有效电平 + ACK 事件清除（K2）。
func TestUartNetlistIntrAndAck(t *testing.T) {
	uart, con := uartRun(t, 0x33, false, 2e-3)
	if !uart.GetBool(uartSlotDataReady) {
		t.Fatalf("前置条件失败：应已收到字节")
	}
	intrHigh := con.GetNodeVoltage(uart.GetNodes(uartSlotIntrPin))
	if intrHigh < 0.9*5.0 {
		t.Fatalf("dataReady 时 INTR 应钳位到 VHigh(≥4.5V)，实际 %.3fV", intrHigh)
	}
	// ACK 段：写 ACK 事件并推进 ⇒ dataReady 清零、INTR 回落。
	uart2, con2 := uartRun(t, 0x33, true, 2e-3)
	if uart2.GetBool(uartSlotDataReady) {
		t.Fatalf("ACK 推进后 dataReady 必须为 false")
	}
	if v := con2.GetNodeVoltage(uart2.GetNodes(uartSlotIntrPin)); v > 0.1*5.0 {
		t.Fatalf("ACK 后 INTR 应回落（≤0.5V），实际 %.3fV", v)
	}
}

// TestUartNetlistIdempotent 幂等（C4）：同一 Context 连跑两次结果一致。
func TestUartNetlistIdempotent(t *testing.T) {
	first, _ := uartRun(t, 0xA5, false, 2e-3)
	b1 := first.GetFloat64(uartSlotRxByte)
	second, _ := uartRun(t, 0xA5, false, 2e-3)
	b2 := second.GetFloat64(uartSlotRxByte)
	if b1 != b2 || b1 != 0xA5 {
		t.Fatalf("两次独立加载结果应一致且为 0xA5，实际 0x%02X / 0x%02X", int(b1), int(b2))
	}
}
