// uartRx.go：接收侧——检测起始沿，按 (1.5+n)·T 采样 8 个数据位与 9.5·T 采样停止位并置帧错误。
package serial

import (
	"circuit/element"
	"circuit/element/digital"
	"circuit/mna"
)

// stepUartRx 推进接收侧一步。
//
// 空闲（rxPhase == 0）时线为高；线变低即起始沿，记录 lastRxTime 并进入帧内。
// 帧内按 K2 采样点取值：第 n 个数据位在起始沿后 (1.5+n)·T（LSB 先），停止位在 9.5·T。
// 停止位采样为低 ⇒ frameError=true，不打断仿真。
func stepUartRx(m mna.Mna, value element.NodeFace, now, period float64) {
	if m == nil || value == nil {
		return
	}
	level := digital.Level{VHigh: value.GetFloat64(slotVHigh)}
	lineHigh := level.IsHigh(m.GetNodeVoltage(value.GetNodes(pinRx)))

	if value.GetFloat64(slotRxPhase) == 0 {
		if !lineHigh {
			// 起始沿：新帧从此刻起算，并清除上一帧的错误标志。
			value.SetFloat64(slotRxShifter, 0)
			value.SetFloat64(slotRxBit, 0)
			value.SetFloat64(slotLastRxTime, now)
			value.SetFloat64(slotRxPhase, 1)
			value.SetBool(slotFrameError, false)
		}
		return
	}

	lastRxTime := value.GetFloat64(slotLastRxTime)
	rxBit := int(value.GetFloat64(slotRxBit))
	shifter := value.GetFloat64(slotRxShifter)
	for rxBit < 8 {
		if !uartTimeReached(now, uartRxSampleAt(lastRxTime, rxBit, period), period) {
			break
		}
		if lineHigh {
			shifter += float64(int(1) << uint(rxBit)) // LSB 先
		}
		rxBit++
	}
	value.SetFloat64(slotRxShifter, shifter)
	value.SetFloat64(slotRxBit, float64(rxBit))

	if rxBit < 8 || !uartTimeReached(now, uartRxStopAt(lastRxTime, period), period) {
		return
	}
	value.SetFloat64(slotRxByte, shifter)
	value.SetBool(slotDataReady, true)
	if !lineHigh {
		value.SetBool(slotFrameError, true)
	}
	value.SetFloat64(slotRxPhase, 0)
}

// uartRxSampleAt 返回第 n 个数据位的采样时刻：起始沿后 (1.5+n)·T。
func uartRxSampleAt(lastRxTime float64, n int, period float64) float64 {
	return lastRxTime + (rxSampleOffset+float64(n))*period
}

// uartRxStopAt 返回停止位的采样时刻：起始沿后 9.5·T。
func uartRxStopAt(lastRxTime, period float64) float64 {
	return lastRxTime + rxStopOffset*period
}

// uartTimeEps 是事件时刻比较的相对容差（以比特周期为单位）。
//
// 存在理由（本包实测缺陷）：仿真时刻按 k·(T/10) 逐步累加，而采样时刻按
// lastRxTime+(1.5+n)·T 计算，两者在浮点下**不保证相等**——恰好落在边界的那一步
// 会因 `now < at` 为真而被推迟；若仿真恰在该边界结束（测试与真实用例都会这样），
// 停止位永不被采样 ⇒ dataReady/frameError 恒不置位。事件时刻是连续量，
// 而引擎只在离散时刻观测，故比较必须带容差。
const uartTimeEps = 1e-9

// uartTimeReached 判断仿真时刻 now 是否已到达事件时刻 at（带相对容差）。
func uartTimeReached(now, at, period float64) bool {
	return now >= at-uartTimeEps*period
}
