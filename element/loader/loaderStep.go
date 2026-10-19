// loaderStep.go：DoStep 编排——步长自检、每时刻只推进一次的幂等门、相位推进与输出驱动。
package loader

import (
	"circuit/element"
	"circuit/element/digital"
	"circuit/mna"
)

// 协议/写时序相位（槽 25 的取值）。
const (
	phaseIdle    = iota // 等 RDY 高（ACK 低）。
	phaseHold           // 已锁存字节、ACK 高，等 RDY 回低。
	phaseRelease        // ACK 已回低，下一步消费锁存字节。
	phaseSetup          // LWE=0，LA/LD 已定位到当前写地址。
	phasePulse          // LWE=1，LA/LD 保持不变，直到 pulseLeft 用尽。
	phaseAdvance        // LWE=0，地址自增并进入下一字节的 setup。
	phaseSettle         // LWE=0 且 LOAD 仍高的最后一个保持步。
	phaseDone           // 加载成功：done=1、LOAD=0、LWE=0。
	phaseError          // 帧错误：error=1、LOAD=1 保持冻结、不再接受字节。
)

// DoStep 推进 LOADER 一步。
//
// 顺序：① 步长自检（超界即 NoConverged 并放弃本步，不推进状态）；② 幂等门——同一仿真时刻
// 只推进一个相位；③ 无条件重驱动全部输出（牛顿迭代之间的回滚会丢掉上一次电压源写入，
// 故驱动必须在幂等门之外）。输入只按引脚电平采样，不读 UART 槽/事件。
func (l *Loader) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	if t == nil || value == nil {
		return
	}
	// K2/C5：上界来自参数（或参考波特率推导），超界必须由元件自己调 NoConverged。
	if err := digital.StepGuard(t, loaderStepLimit(value)); err != nil {
		t.NoConverged()
		return
	}
	if m == nil {
		return
	}
	now := t.Time()
	if digital.SinceLastStamp(value, slotLastStepTime, now) != 0 {
		advanceLoader(m, value)
		digital.MarkStamp(value, slotLastStepTime, now)
	}
	driveLoaderOutputs(m, value)
}

// advanceLoader 在一个新的被接受仿真时刻推进一个相位。
// 任何时刻最多前进一步：一个字节的握手（idle→hold→release）与写脉冲各自跨时刻完成。
func advanceLoader(m mna.Mna, value element.NodeFace) {
	switch int(value.GetFloat64(slotPhase)) {
	case phaseIdle:
		if loaderRDYHigh(m, value) {
			value.SetFloat64(slotPendingByte, float64(loaderInputByte(m, value)))
			value.SetFloat64(slotPhase, phaseHold)
		}
	case phaseHold:
		if !loaderRDYHigh(m, value) {
			value.SetFloat64(slotPhase, phaseRelease)
		}
	case phaseRelease:
		decodeFrameByte(value, int(value.GetFloat64(slotPendingByte)), loaderModeOf(value))
		// 解码可能进入写时序或错误锁存；仍停在 release 时说明该字节只是帧内推进，
		// 回到 idle 等待（并允许 UART 给出）下一字节。
		if int(value.GetFloat64(slotPhase)) == phaseRelease {
			value.SetFloat64(slotPhase, phaseIdle)
		}
	case phaseSetup:
		value.SetFloat64(slotPulseLeft, float64(loaderModeOf(value).pulseSteps))
		value.SetFloat64(slotPhase, phasePulse)
	case phasePulse:
		left := value.GetFloat64(slotPulseLeft) - 1
		if left <= 0 {
			value.SetFloat64(slotPulseLeft, 0)
			value.SetFloat64(slotPhase, phaseAdvance)
		} else {
			value.SetFloat64(slotPulseLeft, left)
		}
	case phaseAdvance:
		addr := int(value.GetFloat64(slotAddr)) + 1
		value.SetFloat64(slotAddr, float64(addr))
		if addr < len(frameDataOf(value)) {
			value.SetFloat64(slotPhase, phaseSetup)
		} else {
			value.SetFloat64(slotPhase, phaseSettle)
		}
	case phaseSettle:
		value.SetFloat64(slotDone, 1)
		value.SetFloat64(slotPhase, phaseDone)
	}
}

// driveLoaderOutputs 按当前相位把 19 根输出引脚的电平写回电压源。每步无条件执行。
// 不变量：phasePulse 期间 LA/LD 只由 slotAddr/帧数据决定（保持稳定）；相位切换时 LWE 先回 0。
func driveLoaderOutputs(m mna.Mna, value element.NodeFace) {
	high := loaderHigh(value)
	phase := int(value.GetFloat64(slotPhase))
	addr := int(value.GetFloat64(slotAddr))
	data := int(loaderDataAt(value, addr))
	for i := 0; i < 8; i++ {
		digital.DriveNow(m, value.GetVoltSource(volLA0+i), loaderBitLevel(addr, i, high))
		digital.DriveNow(m, value.GetVoltSource(volLD0+i), loaderBitLevel(data, i, high))
	}
	load := high
	if phase == phaseDone {
		load = 0
	}
	lwe := 0.0
	if phase == phasePulse {
		lwe = high
	}
	ack := 0.0
	if phase == phaseHold {
		ack = high
	}
	digital.DriveNow(m, value.GetVoltSource(volLOAD), load)
	digital.DriveNow(m, value.GetVoltSource(volLWE), lwe)
	digital.DriveNow(m, value.GetVoltSource(volACK), ack)
}

// loaderBitLevel 返回整数 n 的第 bit 位对应的输出电平。
func loaderBitLevel(n, bit int, high float64) float64 {
	if n&(1<<uint(bit)) != 0 {
		return high
	}
	return 0
}

// loaderHigh 返回本实例的输出高电平；非法值回退默认值。
func loaderHigh(value element.NodeFace) float64 {
	high := value.GetFloat64(slotVHigh)
	if !(high > 0) {
		return defaultVHigh
	}
	return high
}

// loaderLevel 返回本实例的输入判决电平规格（阈值取摆幅中点）。
func loaderLevel(value element.NodeFace) digital.Level {
	return digital.Level{VHigh: loaderHigh(value)}
}

// loaderInputByte 采样 D0..D7 的当前字节（LSB 在 D0）。
func loaderInputByte(m mna.Mna, value element.NodeFace) int {
	level := loaderLevel(value)
	b := 0
	for i := 0; i < 8; i++ {
		if level.IsHigh(m.GetNodeVoltage(value.GetNodes(pinD0 + i))) {
			b |= 1 << uint(i)
		}
	}
	return b
}

// loaderRDYHigh 返回 RDY 引脚是否为高（高 = UART 有字节可用）。
func loaderRDYHigh(m mna.Mna, value element.NodeFace) bool {
	return loaderLevel(value).IsHigh(m.GetNodeVoltage(value.GetNodes(pinRDY)))
}
