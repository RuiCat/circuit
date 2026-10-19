// loaderSlots.go：LOADER 的 NodeValue 槽位表、初始值模板、运行态回滚登记与默认参数。
package loader

import (
	"math"
	"strconv"

	"circuit/element"
)

// 槽位布局（K2）：
//   - 0..3   实例参数（网表位置参数），是唯一可被网表改写的槽；
//   - 4..21  保留不用——追加槽一律从 22 起，避免与既有 K2 槽号空间（UART 0..15）混用；
//   - 22..24 宿主可观测的追加槽（error/done/addr，K2 冻结）；
//   - 25..   控制器内部运行态。
//
// 运行态槽全部登记在 Config.OrigValue：引擎每步 MarkRollbackElements 只回滚登记过的槽，
// 未登记的槽在收敛重试时会「推进一次又被重试」，破坏幂等。
const (
	slotLenMax    = iota // 0：帧长度上限（float64，默认 32；K2/C14）。
	slotStepLimit        // 1：StepGuard 上界（float64，<= 0 时按 9600 baud 推导，K2/C5）。
	slotCpuMode          // 2：CPU 时序模式（string，"v3" 默认 / "v2"）。
	slotVHigh            // 3：输出高电平（float64，默认 5 V，与 UART 的 VHigh 默认对齐）。
)

// 保留槽区间（对齐「追加槽从 22 起」的约定；loader 不使用这些槽）。
const (
	slotReservedFirst = 4
	slotReservedLast  = 21
)

// 宿主可观测的追加槽（K2：22 起至少含 error、done、addr）。
const (
	slotError = 22 // error：非 0 表示帧错误且 CPU 保持冻结（只可 Reset 清除）。
	slotDone  = 23 // done：非 0 表示加载完成且已释放 CPU。
	slotAddr  = 24 // addr：当前写地址（写阶段）；完成后停在最后一个字节地址。
)

// 控制器内部运行态槽。
const (
	slotPhase        = iota + 25 // phase：协议/写时序相位（phase* 常量）。
	slotFrame                    // frame：帧解码状态（frameWait* 常量）。
	slotFrameLen                 // frameLen：本帧 LEN 字段。
	slotFramePos                 // framePos：已收到的数据字节数。
	slotCheckSum                 // checkSum：累计校验和（LEN + DATA，按 8 位截断）。
	slotFrameData                // frameData：本帧数据（[]byte，写时复制，见 loaderFrame.go）。
	slotPulseLeft                // pulseLeft：LWE 高电平剩余保持步数。
	slotPendingByte              // pendingByte：握手期间锁存的字节。
	slotLastStepTime             // lastStepTime：幂等步进戳（NaN = 本时刻尚未推进）。
)

// slotNum 是槽位总数。
const slotNum = slotLastStepTime + 1

// 宿主可观测的追加槽（D5）：网表外不硬编码槽号，桥/e2e 一律引用这些导出常量。
// 宿主只读——这些槽由 LOADER 的 DoStep 写入，宿主写它们不会改变加载流程。
//
//   - SlotError：非 0 表示帧错误，CPU 保持冻结（LOAD=1），只能经 Reset 清除；
//   - SlotDone：非 0 表示加载成功且已释放 CPU（LOAD=0、LWE=0）；
//   - SlotAddr：当前写地址（写阶段）；加载完成后停在最后一个字节地址，失败帧为 0。
const (
	SlotError = slotError // 22：error。
	SlotDone  = slotDone  // 23：done。
	SlotAddr  = slotAddr  // 24：addr。
)

// 内部相位的只读观测槽（宿主只读）：仅供诊断/日志，取值语义属实现细节、不承诺稳定；
// 流程判据只允许用 SlotError/SlotDone 这类追加槽，不得依赖本组槽的数值。
const (
	SlotPhase       = slotPhase       // 25：协议/写时序相位（phase* 常量）。
	SlotFrame       = slotFrame       // 26：帧解码状态（frameWait* 常量）。
	SlotPendingByte = slotPendingByte // 32：握手期间锁存的字节。
)

// 默认参数（K2）。
const (
	defaultLenMax  = 32   // LenMax 默认值。
	defaultVHigh   = 5.0  // 输出高电平默认值（V）。
	referenceBaud  = 9600 // StepLimit 缺省推导所用参考波特率。
	stepsPerBitRef = 10   // StepLimit 缺省推导：每比特至少 10 个仿真步（uartStep.go 的 T/10 口径）。
)

// loaderRuntimeSlots 是必须登记进 Config.OrigValue 的运行态与追加槽。
// 参数槽 0..3 不登记：它们由网表给定、运行期不变。
var loaderRuntimeSlots = []int{
	slotError, slotDone, slotAddr,
	slotPhase, slotFrame, slotFrameLen, slotFramePos, slotCheckSum,
	slotFrameData, slotPulseLeft, slotPendingByte, slotLastStepTime,
}

// loaderValueNames 返回与槽位下标一一对应的可读名（调试与宿主展示用）。
func loaderValueNames() []string {
	names := make([]string, slotNum)
	for i := range names {
		names[i] = "reserved" + strconv.Itoa(i)
	}
	names[slotLenMax] = "LenMax"
	names[slotStepLimit] = "StepLimit"
	names[slotCpuMode] = "CpuMode"
	names[slotVHigh] = "VHigh"
	names[slotError] = "error"
	names[slotDone] = "done"
	names[slotAddr] = "addr"
	names[slotPhase] = "phase"
	names[slotFrame] = "frame"
	names[slotFrameLen] = "frameLen"
	names[slotFramePos] = "framePos"
	names[slotCheckSum] = "checkSum"
	names[slotFrameData] = "frameData"
	names[slotPulseLeft] = "pulseLeft"
	names[slotPendingByte] = "pendingByte"
	names[slotLastStepTime] = "lastStepTime"
	return names
}

// loaderValueInit 返回槽位初始值模板。
// 上电即 LOAD 高、LWE/ACK 低：CPU 在加载完成前保持冻结（K2 的失败语义要求「不释放」）。
// 幂等戳必须是 NaN（digital.SinceLastStamp 约定）。
func loaderValueInit() []any {
	init := make([]any, slotNum)
	for i := range init {
		init[i] = float64(0)
	}
	init[slotLenMax] = float64(defaultLenMax)
	init[slotCpuMode] = modeV3
	init[slotVHigh] = defaultVHigh
	init[slotFrame] = float64(frameWaitSync)
	init[slotFrameData] = []byte(nil)
	init[slotLastStepTime] = math.NaN()
	return init
}

// loaderStepLimit 返回本实例的 StepGuard 上界：显式参数优先，否则按参考波特率推导 T/10。
func loaderStepLimit(value element.NodeFace) float64 {
	if limit := value.GetFloat64(slotStepLimit); limit > 0 {
		return limit
	}
	return 1 / (referenceBaud * float64(stepsPerBitRef))
}
