// fake_test.go 提供与 K2 槽表同语义的内存假实现：
// fakeEngine 模拟引擎的单值事件 + PushEvents 消费语义与 UART 元件行为，
// fakeLink 模拟串口通道（可注入超时/断开错误）。它们只被本包测试使用。
package serialbridge

import (
	"fmt"
	"sort"
)

// fakeEngine 是 SlotIo 的内存实现。
// 它刻意复刻两处引擎语义，否则桥的用例会"通过得没有意义"：
//  1. WriteEvent 是单值存储——同名事件在同一"步"内再次写入会覆盖前值（overwrites 计数）；
//  2. Advance 消费事件（消费即删），并驱动一个最小的 UART 元件模型。
type fakeEngine struct {
	txName  string
	ackName string

	slots   [SlotCount]float64
	events  map[string]float64 // 待消费事件值（镜像 eventValues）
	targets map[string]int     // 事件名 → 目标槽（镜像 Config.EventSlots）

	overwrites int    // 同名事件在同一"步"内被覆盖的次数（大于 0 即发生了静默覆盖）
	txLog      []byte // 元件从槽 3 取走的入站字节（按顺序）
	rxQueue    []byte // 元件待交付的出站字节
	rxFrame    []bool // 与 rxQueue 对齐的帧错误标记

	reportConsumption bool  // false = 模拟"元件没取走事件"（Advance 不报告消费）
	closed            bool  // true = 通道已关闭
	stepErr           error // 非 nil = Advance 返回该错误
}

// newFakeEngine 构造 J2 槽表 + "UART1" 事件名的假引擎。
func newFakeEngine() *fakeEngine {
	e := &fakeEngine{
		txName:            "UART1.TX",
		ackName:           "UART1.ACK",
		events:            map[string]float64{},
		targets:           map[string]int{"UART1.TX": SlotTxByte, "UART1.ACK": SlotAckFlag},
		reportConsumption: true,
	}
	// D16：真元件在帧间空闲时槽 5（txBit）= TxIdleBit；假引擎必须复刻这一点，
	// 否则桥的 WaitTxIdle 会永远等不到空闲而耗尽预算（用例会"红得没有意义"）。
	e.slots[SlotTxBit] = TxIdleBit
	return e
}

// ReadSlot 读槽；越界返回 ErrSlotValue（不返回 0 冒充数据）。
func (e *fakeEngine) ReadSlot(idx int) (float64, error) {
	if idx < 0 || idx >= SlotCount {
		return 0, fmt.Errorf("%w: 槽 %d 越界", ErrSlotValue, idx)
	}
	return e.slots[idx], nil
}

// WriteEvent 单值写入事件；未知事件名按 ErrSlotValue 上报。
func (e *fakeEngine) WriteEvent(name string, value float64) error {
	if e.closed {
		return ErrLinkClosed
	}
	if _, ok := e.targets[name]; !ok {
		return fmt.Errorf("%w: 未知事件 %s", ErrSlotValue, name)
	}
	if _, dup := e.events[name]; dup {
		e.overwrites++ // 引擎语义：后值覆盖前值，前值不会被排队
	}
	e.events[name] = value
	return nil
}

// Advance 消费事件并推进元件一步；返回本步被消费的事件名。
func (e *fakeEngine) Advance() ([]string, error) {
	if e.stepErr != nil {
		return nil, e.stepErr
	}
	names := make([]string, 0, len(e.events))
	for name := range e.events {
		names = append(names, name)
	}
	sort.Strings(names) // 确定性顺序，避免 map 迭代顺序让用例偶发
	consumedTx := false
	for _, name := range names {
		value := e.events[name]
		delete(e.events, name) // 引擎语义：消费后置 nil
		switch name {
		case e.ackName:
			if value != 0 { // K2：槽 15 非 0 ⇒ 本步内 dataReady=false、ackFlag=0
				e.slots[SlotDataReady] = 0
				e.slots[SlotAckFlag] = 0
			}
		case e.txName:
			e.slots[SlotTxByte] = value
			e.txLog = append(e.txLog, byte(int(value)))
			consumedTx = true
		}
	}
	// D16：取走字节的这一步进入"帧内"（非空闲），下一步起回到帧间空闲。
	if consumedTx {
		e.slots[SlotTxBit] = 1
	} else if e.slots[SlotTxBit] != TxIdleBit {
		e.slots[SlotTxBit] = TxIdleBit
	}
	if e.slots[SlotDataReady] == 0 && len(e.rxQueue) > 0 {
		e.slots[SlotRxByte] = float64(e.rxQueue[0])
		if len(e.rxFrame) > 0 {
			if e.rxFrame[0] {
				e.slots[SlotFrameError] = 1
			} else {
				e.slots[SlotFrameError] = 0
			}
			e.rxFrame = e.rxFrame[1:]
		}
		e.rxQueue = e.rxQueue[1:]
		e.slots[SlotDataReady] = 1
	}
	if !e.reportConsumption {
		return nil, nil
	}
	return names, nil
}

// fakeLink 是 Link 的内存实现：in 为待读字节，out 为已写字节，err 注入读错误。
type fakeLink struct {
	in     []byte
	out    []byte
	err    error
	closed bool
}

// ReadByte 优先返回注入的错误；无数据时返回 ErrTimeout（与真实 Link 语义一致）。
func (l *fakeLink) ReadByte() (byte, error) {
	if l.err != nil {
		return 0, l.err
	}
	if len(l.in) == 0 {
		return 0, ErrTimeout
	}
	b := l.in[0]
	l.in = l.in[1:]
	return b, nil
}

// WriteBytes 追加写出；关闭后返回 ErrLinkClosed。
func (l *fakeLink) WriteBytes(p []byte) (int, error) {
	if l.closed {
		return 0, ErrLinkClosed
	}
	l.out = append(l.out, p...)
	return len(p), nil
}

// Close 标记通道关闭。
func (l *fakeLink) Close() error {
	l.closed = true
	return nil
}
