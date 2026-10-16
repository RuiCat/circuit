// uartSlots.go：UART 的 NodeValue 槽位表、初始值模板与事件/回滚登记表。
package serial

import "math"

// UART 的槽位下标：0..15 为 K2 冻结槽；16 起是按 K2「冻结后新增槽只能追加到末尾」
// 追加的实现槽（事件名落点、幂等戳与 TX 翻转队列），不改变 0..15 的语义。
const (
	slotBaud         = iota // 0：波特率（float64，必须 > 0）。
	slotAckEvent            // 1：ACK 事件名（string）。
	slotVHigh               // 2：逻辑高电平（float64，V）。
	slotTxByte              // 3：入站待发字节（事件落点，float64；-1 = 无）。
	slotTxShifter           // 4：发送移位寄存器。
	slotTxBit               // 5：发送位序号（txBitIdle = 帧间空闲）。
	slotTxPhase             // 6：发送相位（运行态保留槽，当前未使用）。
	slotLastTxTime          // 7：本帧起始时刻（从未发送过帧时为 -Inf）。
	slotRxShifter           // 8：接收移位寄存器。
	slotRxBit               // 9：已采样的数据位数（0..8）。
	slotRxPhase             // 10：接收相位（0 = 等起始沿，1 = 帧内）。
	slotRxByte              // 11：最近收到的字节（0..255）。
	slotDataReady           // 12：收到字节且未确认（bool）。
	slotFrameError          // 13：停止位采样为低（bool）。
	slotLastRxTime          // 14：本帧起始沿时刻。
	slotAckFlag             // 15：ACK 事件落点（float64，非 0 表示确认）。
	slotLastStepTime        // 16：追加——幂等步进戳（NaN = 本时刻尚未推进）。
	slotRxEventName         // 17：追加——入站字节事件名 "<实例名>.TX"（string）。
	slotTxQueue             // 18：追加——TX 翻转队列 base（占 4 个连续槽位）。
)

// queueSlots 是 digital.DelayQueue 单个 base 占用的连续槽位数（K1：4）。
const queueSlots = 4

// slotNum 是槽位总数（0..21）。
const slotNum = slotTxQueue + queueSlots

// txBitIdle 是发送空闲的位序号（10 = 一帧 10 位全部发完）。
const txBitIdle = 10

// txBitLast 是停止位序号（0 = 起始位、1..8 = 数据位 LSB 先、9 = 停止位）。
const txBitLast = 9

// txByteNone 是槽 3 的「无待发字节」标记。
const txByteNone = -1.0

// rxSampleOffset 是第 1 个数据位相对起始沿的采样偏移（1.5 个比特周期）。
const rxSampleOffset = 1.5

// rxStopOffset 是停止位相对起始沿的采样偏移（9.5 个比特周期）。
const rxStopOffset = 9.5

// uartValueNames 与槽位下标一一对应（调试与 MCP 展示用的可读名）。
var uartValueNames = []string{
	"baud", "ackEvent", "VHigh",
	"txByte", "txShifter", "txBit", "txPhase", "lastTxTime",
	"rxShifter", "rxBit", "rxPhase", "rxByte", "dataReady",
	"frameError", "lastRxTime", "ackFlag",
	"lastStepTime", "rxEventName",
	"txQueueArmed", "txQueueAt", "txQueueValue", "txQueueNow",
}

// uartRuntimeSlots 是必须登记进 Config.OrigValue 的运行态槽（C3/C4）。
// 事件落点槽 3/15 刻意不在此列：事件值由引擎每步推入，回滚会丢掉宿主刚写的事件。
var uartRuntimeSlots = []int{
	slotTxShifter, slotTxBit, slotTxPhase, slotLastTxTime,
	slotRxShifter, slotRxBit, slotRxPhase, slotRxByte,
	slotDataReady, slotFrameError, slotLastRxTime,
	slotLastStepTime,
	slotTxQueue, slotTxQueue + 1, slotTxQueue + 2, slotTxQueue + 3,
}

// uartEventSlots 是事件名槽 → 事件值槽的映射（两者都是消费者：引擎 PushEvents 写入值槽）。
// 槽 17 持有 "<实例名>.TX"，槽 1 持有 ackEvent，故同一元件可同时消费两类事件。
var uartEventSlots = map[int]int{
	slotAckEvent:    slotAckFlag,
	slotRxEventName: slotTxByte,
}

// uartValueInit 返回槽位初始值模板；Base 会按实例名替换名字槽 1/17。
// 幂等戳 slotLastStepTime 必须是 NaN（K1 约定④），槽 3 的 -1 表示尚无待发字节。
func uartValueInit() []any {
	init := make([]any, slotNum)
	for i := range init {
		init[i] = float64(0)
	}
	for i := slotTxQueue; i < slotTxQueue+queueSlots; i++ {
		init[i] = float64(0)
	}
	init[slotBaud] = float64(9600)
	init[slotAckEvent] = ""
	init[slotVHigh] = float64(5)
	init[slotTxByte] = float64(txByteNone)
	init[slotTxBit] = float64(txBitIdle)
	init[slotLastTxTime] = math.Inf(-1)
	init[slotDataReady] = false
	init[slotFrameError] = false
	init[slotLastStepTime] = math.NaN()
	init[slotRxEventName] = ""
	return init
}
