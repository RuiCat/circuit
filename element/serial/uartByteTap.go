// uartByteTap.go：13 针字节口的 4 相握手（RDY/ACK）与 D0..D7 锁存输出；3 针形态全程短路。
//
// 4 相握手（K1 冻结）：RDY 高 → 加载器拉 ACK → UART 拉低 RDY → 加载器拉低 ACK。
// 不变量：D 在 RDY 高到 ACK 低期间不得变化 ⇒ 取下一字节同时在「RDY 已回低」与「ACK 为低」
// 两个条件成立时才允许（见 uartTakeAllowed），而不是只看 ACK。
package serial

import (
	"circuit/element"
	"circuit/element/digital"
	"circuit/mna"
)

// uartIsByteTap 报告实例是否为 13 针字节口形态（按实例配置的引脚数判定，不看包级原型）。
func uartIsByteTap(value element.NodeFace) bool {
	if value == nil {
		return false
	}
	cfg := value.Config()
	return cfg != nil && cfg.PinNum() == uartTapPinNum
}

// uartAckHigh 读 ACK 引脚电平（PinBoolean 域，阈值取 VHigh/2，与接收侧同口径）。
// m 为空时按低电平处理：该函数只在 13 针分支被调用，不能因无矩阵而 panic。
func uartAckHigh(m mna.Mna, value element.NodeFace) bool {
	if m == nil || value == nil {
		return false
	}
	level := digital.Level{VHigh: value.GetFloat64(slotVHigh)}
	return level.IsHigh(m.GetNodeVoltage(value.GetNodes(pinAck)))
}

// uartTakeAllowed 判断本步能否从槽 3 取走待发字节。
// 3 针形态恒为 true（逐位等价于改动前的取字节条件，C1：不得附加 ACK 门控）；
// 13 针形态要求上一字节的握手已完成（RDY 已回低）且 ACK 已回低，保证 D 在
// 「RDY 高 → ACK 低」区间内保持不变。
func uartTakeAllowed(m mna.Mna, value element.NodeFace) bool {
	if !uartIsByteTap(value) {
		return true
	}
	if value.GetFloat64(slotByteRdy) != 0 {
		return false
	}
	return !uartAckHigh(m, value)
}

// stepUartByteTap 推进 13 针握手：见到 ACK 为高即拉低 RDY 并记录「加载器已 ACK」。
// 在取字节之前调用，保证同一步内「ACK 高」优先于「取下一字节」。
func stepUartByteTap(m mna.Mna, value element.NodeFace) {
	if !uartIsByteTap(value) || value.GetFloat64(slotByteRdy) == 0 {
		return
	}
	if !uartAckHigh(m, value) {
		return
	}
	value.SetFloat64(slotByteRdy, 0)
	value.SetFloat64(slotByteAck, 1)
}

// uartByteTaken 记录刚从槽 3 取走的字节：13 针形态锁存到 D 引脚并拉起 RDY、
// 清掉上一字节的 ACK 标记；3 针形态只更新不参与输出的锁存槽。
func uartByteTaken(value element.NodeFace, b int) {
	if value == nil {
		return
	}
	value.SetFloat64(slotByteLatch, float64(b))
	if !uartIsByteTap(value) {
		return
	}
	value.SetFloat64(slotByteRdy, 1)
	value.SetFloat64(slotByteAck, 0)
}

// driveUartByteOutputs 把锁存字节写到 D0..D7、把 RDY 内部状态写到 RDY 电压源。
// 每步、每次 DoStep 调用都必须执行（与 tx/intr 同理：牛顿迭代之间的 Z 回滚会丢写入）。
func driveUartByteOutputs(m mna.Mna, value element.NodeFace, high float64) {
	if !uartIsByteTap(value) {
		return
	}
	raw := int(value.GetFloat64(slotByteLatch))
	for i := 0; i < 8; i++ {
		level := 0.0
		if (raw>>uint(i))&1 == 1 {
			level = high
		}
		digital.DriveNow(m, value.GetVoltSource(volD0+i), level)
	}
	rdy := 0.0
	if value.GetFloat64(slotByteRdy) != 0 {
		rdy = high
	}
	digital.DriveNow(m, value.GetVoltSource(volRdy), rdy)
}
