// bridge.go 实现宿主串口桥的核心：入站一次一字节 + 确认流控（C15）、出站直读槽 11/12/13、ACK 握手。
package serialbridge

import (
	"errors"
	"fmt"
)

// DefaultStepBudget 是单次等待（入站字节被消费 / 出站字节到来）允许推进的最大步数。
const DefaultStepBudget = 256

// Config 是桥的参数；事件名默认由 Instance 推导。
type Config struct {
	Instance   string // UART 实例名（例 "UART1"）⇒ 缺省事件名 "UART1.TX" / "UART1.ACK"
	TxEvent    string // 覆盖入站事件名（非空则优先于 Instance）
	AckEvent   string // 覆盖 ACK 事件名（非空则优先于 Instance）
	StepBudget int    // 单次等待的步数预算；≤0 时取 DefaultStepBudget
}

// Bridge 把串口字节流与 K2 槽表对接。
// 不变量：pendingTx 为 true 时，入站事件在一步内不得再写第二次——
// 引擎的 SetEvent 是单值存储、PushEvents 消费后置 nil（element/context.go:304,371-396），
// 不确认就连发会让先写的字节被静默覆盖（C15）。
type Bridge struct {
	io         SlotIo
	txEvent    string
	ackEvent   string
	stepBudget int
	pendingTx  bool // 已写入 txEvent 但引擎尚未在 Advance 中报告消费
}

// NewBridge 校验配置并构造桥；io 为空或事件名无法确定时返回错误。
func NewBridge(io SlotIo, cfg Config) (*Bridge, error) {
	if io == nil {
		return nil, errors.New("serialbridge: SlotIo 为空")
	}
	txEvent := cfg.TxEvent
	ackEvent := cfg.AckEvent
	if txEvent == "" || ackEvent == "" {
		if cfg.Instance == "" {
			return nil, ErrMissingInstance
		}
		if txEvent == "" {
			txEvent = cfg.Instance + ".TX"
		}
		if ackEvent == "" {
			ackEvent = cfg.Instance + ".ACK"
		}
	}
	budget := cfg.StepBudget
	if budget <= 0 {
		budget = DefaultStepBudget
	}
	return &Bridge{io: io, txEvent: txEvent, ackEvent: ackEvent, stepBudget: budget}, nil
}

// TxEvent 返回入站事件名（供诊断与网表对照）。
func (br *Bridge) TxEvent() string { return br.txEvent }

// AckEvent 返回 ACK 事件名。
func (br *Bridge) AckEvent() string { return br.ackEvent }

// Pending 报告是否有入站字节在途（已写事件但未被引擎消费）。
func (br *Bridge) Pending() bool { return br.pendingTx }

// SendByte 把一个字节写进入站事件（K2：入站事件是槽 3 的唯一来源）。
// 流控：上一字节未被引擎消费时直接返回 ErrByteInFlight——拒绝而不是排队，
// 因为排队会掩盖"元件没取走字节"这一故障，且桥无法知道元件何时能收。
func (br *Bridge) SendByte(b byte) error {
	if br.pendingTx {
		return ErrByteInFlight
	}
	if err := br.io.WriteEvent(br.txEvent, float64(b)); err != nil {
		return err
	}
	br.pendingTx = true
	return nil
}

// Advance 推进一个仿真步；只有引擎报告消费了入站事件时才解除在途标记（背压）。
func (br *Bridge) Advance() error {
	consumed, err := br.io.Advance()
	if err != nil {
		return err
	}
	for _, name := range consumed {
		if name == br.txEvent {
			br.pendingTx = false
			break
		}
	}
	return nil
}

// WaitTxDrained 在步数预算内反复推进直到入站字节被消费；预算内未消费返回 ErrStepBudget。
func (br *Bridge) WaitTxDrained() error {
	for i := 0; i < br.stepBudget && br.pendingTx; i++ {
		if err := br.Advance(); err != nil {
			return err
		}
	}
	if br.pendingTx {
		return ErrStepBudget
	}
	return nil
}

// TxIdleBit 是 K2 槽 5（txBit）的「帧间空闲」取值。
// 依据 D16：元件侧 uartSlots.go 的 txBitIdle = 10（起始位 0、数据位 1..8、停止位 9、10 = 帧间空闲）；
// 该取值由本常量冻结为契约的一部分，桥据此判断"上一字节是否真的发完"。
const TxIdleBit = 10

// WaitTxIdle 在步数预算内反复推进，直到 UART 发送侧回到帧间空闲。
//
// 存在理由（D16，实测缺口）：入站字节槽只有一个，元件只在帧间空闲时取走它。
// 「事件被消费」只说明元件收到了字节，**不等于已经发完**；此时再写下一字节会让
// 槽 3 被覆盖 ⇒ 静默丢字节（16 字节连发只剩最后 1 个）。故每个字节之后必须等到空闲。
// 本方法先推进一步再判定，保证"刚写入的字节已被元件取走"这一状态可见。
func (br *Bridge) WaitTxIdle() error {
	for i := 0; i < br.stepBudget; i++ {
		if err := br.Advance(); err != nil {
			return err
		}
		bit, err := br.io.ReadSlot(SlotTxBit)
		if err != nil {
			return err
		}
		if int(bit) == TxIdleBit {
			return nil
		}
	}
	return ErrStepBudget
}

// ReceiveByte 读一个出站字节，并在同一调用内完成 ACK 握手。
// 直读槽 12（dataReady）/11（rxByte）/13（frameError）——运行中可读，不依赖 circuit_get_element（C2）。
// 返回 (字节, 是否帧错误, 错误)：dataReady==0 时返回 ErrNoData；
// 读到字节后写 ACK 事件并推进一步，使元件按 K2 在步内置 dataReady=false、ackFlag=0。
func (br *Bridge) ReceiveByte() (byte, bool, error) {
	ready, err := br.io.ReadSlot(SlotDataReady)
	if err != nil {
		return 0, false, err
	}
	if ready == 0 {
		return 0, false, ErrNoData
	}
	raw, err := br.io.ReadSlot(SlotRxByte)
	if err != nil {
		return 0, false, err
	}
	if raw < 0 || raw > 255 || raw != float64(int(raw)) {
		return 0, false, fmt.Errorf("%w: rxByte=%v（要求 0..255 整数）", ErrSlotValue, raw)
	}
	frame, err := br.io.ReadSlot(SlotFrameError)
	if err != nil {
		return 0, false, err
	}
	if err := br.io.WriteEvent(br.ackEvent, 1); err != nil {
		return 0, false, err
	}
	if err := br.Advance(); err != nil {
		return 0, false, err
	}
	return byte(int(raw)), frame != 0, nil
}
