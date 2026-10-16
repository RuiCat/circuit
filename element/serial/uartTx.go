// uartTx.go：发送侧——取走槽 3 的待发字节，按 8N1 生成 tx 位流（每个跳变经 DelayQueue 排程）。
package serial

import (
	"math"

	"circuit/element"
	"circuit/element/digital"
	"circuit/mna"
)

// uartTxQueue 是 TX 翻转队列；digital.DelayQueue 自身不持有跨步状态，故包级实例安全。
var uartTxQueue digital.DelayQueue

// stepUartTx 推进发送侧：先 Apply 到期跳变，再按 K2 时序排程下一个跳变。
//
// 顺序固定「先 Apply 再排程」（K1 约定⑤）：Apply 把已到期跳变写回 tx 电压源，
// 排程只排「尚未发出的下一个位边沿」，故不会丢掉已到期事件。
// 一个 base 同时只允许一个未到期事件 ⇒ 用 Plan（不丢跳变），Plan 报错必须处理而不是忽略。
func stepUartTx(m mna.Mna, t mna.Time, value element.NodeFace, now, period float64) {
	uartTxQueue.Apply(m, t, value, slotTxQueue, value.GetVoltSource(volTx))

	txBit := int(value.GetFloat64(slotTxBit))
	lastTxTime := value.GetFloat64(slotLastTxTime)
	needPlan := false

	// 当前位的边沿已到期（Apply 已写回）→ 前进一位。
	if txBit >= 0 && txBit <= txBitLast && now >= uartTxEdgeAt(lastTxTime, txBit, period) {
		txBit++
		needPlan = true
	}

	// 帧间空闲且上一帧（含停止位）已发完 → 取走待发字节并立即发出起始位。
	if txBit == txBitIdle && now >= lastTxTime+float64(txBitIdle)*period {
		if b, ok := takeTxByte(value); ok {
			shifter := float64(b)
			value.SetFloat64(slotTxShifter, shifter)
			value.SetFloat64(slotLastTxTime, now)
			lastTxTime = now
			txBit = 1 // 起始位（位 0）已由下面立即驱动，下一个待排边沿是位 1
			digital.DriveNow(m, value.GetVoltSource(volTx), uartBitVolts(value.GetFloat64(slotVHigh), shifter, 0))
			needPlan = true
		}
	}
	value.SetFloat64(slotTxBit, float64(txBit))

	if !needPlan || txBit < 1 || txBit > txBitLast {
		return
	}
	shifter := value.GetFloat64(slotTxShifter)
	level := uartBitVolts(value.GetFloat64(slotVHigh), shifter, txBit)
	if err := uartTxQueue.Plan(value, slotTxQueue, uartTxEdgeAt(lastTxTime, txBit, period), level); err != nil {
		// K1 约定③：Plan 报错不得忽略（否则丢跳变）——标记未收敛，让步长减半后重试本步。
		t.NoConverged()
	}
}

// takeTxByte 取走槽 3 的待发字节：值为非负有限数时取低 8 位并清空槽位（避免重复发送）；
// 负数与 NaN 视为「无待发字节」。
func takeTxByte(value element.NodeFace) (int, bool) {
	raw := value.GetFloat64(slotTxByte)
	if math.IsNaN(raw) || raw < 0 {
		return 0, false
	}
	value.SetFloat64(slotTxByte, txByteNone)
	return int(raw) & 0xFF, true
}

// uartTxEdgeAt 返回发送侧第 bit 位的边沿时刻：本帧起始时刻 + bit·T。
// 排程与到期判断用同一表达式，保证与 DelayQueue 内部判据「now >= at」逐位一致。
func uartTxEdgeAt(lastTxTime float64, bit int, period float64) float64 {
	return lastTxTime + float64(bit)*period
}

// uartBitLevel 返回 8N1 帧第 bit 位的归一化电平：0 起始位（低）、1..8 数据位（LSB 先）、
// 9 停止位（高）；越界位按停止位处理（帧尾保持高）。
func uartBitLevel(shifter float64, bit int) float64 {
	switch {
	case bit <= 0:
		return 0
	case bit <= 8:
		return float64((int(shifter) >> (bit - 1)) & 1)
	default:
		return 1
	}
}

// uartBitVolts 返回 8N1 帧第 bit 位对应的引脚电压（VHigh 为逻辑高、0V 为逻辑低）。
func uartBitVolts(high, shifter float64, bit int) float64 {
	return high * uartBitLevel(shifter, bit)
}

// uartTxLineLevel 返回当前时刻 tx 引脚应处的电平：帧内取当前位电平（txBit 的边沿已发出，
// 当前正在发送的是 txBit-1 位），帧间为高（空闲）。
func uartTxLineLevel(value element.NodeFace, high float64) float64 {
	txBit := int(value.GetFloat64(slotTxBit))
	if txBit < 0 || txBit > txBitLast {
		return high
	}
	return uartBitVolts(high, value.GetFloat64(slotTxShifter), txBit-1)
}
