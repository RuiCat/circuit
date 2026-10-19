// uartStep.go：DoStep 编排——步长自检、每时刻只推进一次的幂等门、状态推进与输出驱动。
package serial

import (
	"circuit/element"
	"circuit/element/digital"
	"circuit/mna"
)

// uartStepsPerBit 是标称时序（比特周期）与最大允许步长之比：步长必须 ≤ T/10。
const uartStepsPerBit = 10

// DoStep 推进 UART 一步。
//
// 顺序：① 参数与步长自检（不合法或超界即 NoConverged 并放弃本步，不驱动输出）；
// ② 幂等门——同一仿真时刻只推进一次状态（引擎会在一个时间步内多次调用 DoStep）；
// ③ 无条件写回 tx/intr 电平——牛顿迭代之间的 Z 回滚会丢掉上一次写入，故驱动必须在幂等门之外。
func (u *Uart) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	if t == nil || value == nil {
		return
	}
	baud := value.GetFloat64(slotBaud)
	if !(baud > 0) {
		// 非法 baud：DoStep 没有 error 通道，按 K2 置 NoConverged 且不驱动输出。
		t.NoConverged()
		return
	}
	period := 1 / baud
	if err := digital.StepGuard(t, period/uartStepsPerBit); err != nil {
		// K1 约定②：StepGuard 是纯函数，元件必须自己调 NoConverged，引擎才会减半步长。
		t.NoConverged()
		return
	}
	now := t.Time()
	if digital.SinceLastStamp(value, slotLastStepTime, now) != 0 {
		advanceUart(m, t, value, now, period)
		digital.MarkStamp(value, slotLastStepTime, now)
	}
	driveUartOutputs(m, value)
}

// advanceUart 推进一个仿真时刻的 UART 状态，顺序固定：ACK 清除 → 13 针握手 → 发送 → 接收。
// 先清 ACK 再收新字节，保证「确认上一字节」不会吃掉同一步内刚到的新字节；
// 13 针握手先于发送推进，保证「ACK 高」优先于「从槽 3 取下一字节」（K1 的 4 相握手）。
func advanceUart(m mna.Mna, t mna.Time, value element.NodeFace, now, period float64) {
	clearUartAck(value)
	stepUartByteTap(m, value)
	stepUartTx(m, t, value, now, period)
	stepUartRx(m, value, now, period)
}

// driveUartOutputs 把 tx（当前位电平）、intr（dataReady 高有效）与 13 针形态的
// D0..D7/RDY 写回各自的电压源。每步、每次 DoStep 调用都必须执行。
// （K2：槽 12 == true ⇒ intr 钳位 VHigh，否则 0V。）
func driveUartOutputs(m mna.Mna, value element.NodeFace) {
	high := value.GetFloat64(slotVHigh)
	digital.DriveNow(m, value.GetVoltSource(volTx), uartTxLineLevel(value, high))
	intrLevel := 0.0
	if value.GetBool(slotDataReady) {
		intrLevel = high
	}
	digital.DriveNow(m, value.GetVoltSource(volIntr), intrLevel)
	driveUartByteOutputs(m, value, high)
}
