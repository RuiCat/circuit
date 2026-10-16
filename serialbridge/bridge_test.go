// bridge_test.go 覆盖桥的入站流控（C15）、出站直读槽与 ACK 握手、以及错误可判定性（不 panic）。
package serialbridge

import (
	"errors"
	"testing"
)

// boom 是本文件注入用的哨兵错误，用于断言"错误被原样返回"。
var boom = errors.New("boom")

// brokenSlotIo 的每个操作都返回注入的错误，用于错误传播用例。
type brokenSlotIo struct{ err error }

func (b brokenSlotIo) ReadSlot(int) (float64, error)    { return 0, b.err }
func (b brokenSlotIo) WriteEvent(string, float64) error { return b.err }
func (b brokenSlotIo) Advance() ([]string, error)       { return nil, b.err }

// advanceFailIo 的写入成功但推进失败，用于断言"推进失败不算已消费"。
type advanceFailIo struct{ inner *fakeEngine }

func (a advanceFailIo) ReadSlot(idx int) (float64, error)       { return a.inner.ReadSlot(idx) }
func (a advanceFailIo) WriteEvent(name string, v float64) error { return a.inner.WriteEvent(name, v) }
func (a advanceFailIo) Advance() ([]string, error)              { return nil, boom }

// newTestBridge 构造指向 fakeEngine 的桥。
func newTestBridge(t *testing.T, engine *fakeEngine) *Bridge {
	t.Helper()
	br, err := NewBridge(engine, Config{Instance: "UART1"})
	if err != nil {
		t.Fatalf("NewBridge 失败: %v", err)
	}
	return br
}

// TestNewBridgeValidates 断言配置缺失与空 SlotIo 都被拒绝（不落到 nil 解引用）。
func TestNewBridgeValidates(t *testing.T) {
	if _, err := NewBridge(newFakeEngine(), Config{}); !errors.Is(err, ErrMissingInstance) {
		t.Fatalf("缺实例名应返回 ErrMissingInstance，实际 %v", err)
	}
	if _, err := NewBridge(nil, Config{Instance: "UART1"}); err == nil {
		t.Fatal("SlotIo 为空应返回错误")
	}
	if br, err := NewBridge(nil, Config{TxEvent: "X.TX", AckEvent: "X.ACK"}); err == nil {
		t.Fatalf("SlotIo 为空应返回错误，实际得到 %v", br)
	}
}

// TestSendByteRefusesSecondWriteInSameStep 是 C15 的主判据：
// 同一步内连写两次同名事件，桥必须拒绝（或排队），不得静默覆盖。
// 同一个用例里同时给出反证——绕过桥直写两次时，引擎只保留最后一个值（第一字节丢失）。
func TestSendByteRefusesSecondWriteInSameStep(t *testing.T) {
	engine := newFakeEngine()
	br := newTestBridge(t, engine)

	if err := br.SendByte(0x11); err != nil {
		t.Fatalf("首次 SendByte 失败: %v", err)
	}
	if !br.Pending() {
		t.Fatal("首字节写入后应处于在途状态")
	}
	if err := br.SendByte(0x22); !errors.Is(err, ErrByteInFlight) {
		t.Fatalf("同一步第二次写入必须被拒绝或排队，实际 err=%v", err)
	}
	if engine.overwrites != 0 {
		t.Fatalf("被拒绝的写入不得落到引擎（覆盖计数=%d）", engine.overwrites)
	}

	if err := br.Advance(); err != nil {
		t.Fatalf("Advance 失败: %v", err)
	}
	if br.Pending() {
		t.Fatal("引擎报告消费后应解除在途标记")
	}
	if len(engine.txLog) != 1 || engine.txLog[0] != 0x11 {
		t.Fatalf("元件应取走首字节 0x11，实际 %v", engine.txLog)
	}

	// 反证：去掉这层保护（同一步直写两次）时，第一字节必须被静默覆盖丢失。
	engine.txLog = nil
	if err := engine.WriteEvent(engine.txName, 0x33); err != nil {
		t.Fatalf("直写事件失败: %v", err)
	}
	if err := engine.WriteEvent(engine.txName, 0x44); err != nil {
		t.Fatalf("直写事件失败: %v", err)
	}
	if engine.overwrites != 1 {
		t.Fatalf("假引擎未复刻引擎的单值语义（覆盖计数=%d）", engine.overwrites)
	}
	if _, err := engine.Advance(); err != nil {
		t.Fatalf("Advance 失败: %v", err)
	}
	if len(engine.txLog) != 1 || engine.txLog[0] != 0x44 {
		t.Fatalf("无流控时必须是后者覆盖前者（0x33 丢失），实际 txLog=%v", engine.txLog)
	}
}

// TestSendByteThenAdvanceAllowsNext 断言确认后可继续发送（流控不是"永久锁死"）。
func TestSendByteThenAdvanceAllowsNext(t *testing.T) {
	engine := newFakeEngine()
	br := newTestBridge(t, engine)
	for _, b := range []byte{0x01, 0x02, 0x03} {
		if err := br.SendByte(b); err != nil {
			t.Fatalf("SendByte(%#x) 失败: %v", b, err)
		}
		if err := br.Advance(); err != nil {
			t.Fatalf("Advance 失败: %v", err)
		}
	}
	if len(engine.txLog) != 3 {
		t.Fatalf("元件应取走 3 字节，实际 %v", engine.txLog)
	}
}

// TestWaitTxDrainedBudget 断言元件始终不消费时以 ErrStepBudget 收场（不无界等待）。
func TestWaitTxDrainedBudget(t *testing.T) {
	engine := newFakeEngine()
	engine.reportConsumption = false
	br, err := NewBridge(engine, Config{Instance: "UART1", StepBudget: 8})
	if err != nil {
		t.Fatalf("NewBridge 失败: %v", err)
	}
	if err := br.SendByte(0x55); err != nil {
		t.Fatalf("SendByte 失败: %v", err)
	}
	if err := br.WaitTxDrained(); !errors.Is(err, ErrStepBudget) {
		t.Fatalf("未被消费时应返回 ErrStepBudget，实际 %v", err)
	}
	if !br.Pending() {
		t.Fatal("未被消费的字节必须仍然是在途状态")
	}
}

// TestReceiveByteNoData 断言无数据时返回 ErrNoData（可判定，不 panic、不返回假字节）。
func TestReceiveByteNoData(t *testing.T) {
	br := newTestBridge(t, newFakeEngine())
	if _, _, err := br.ReceiveByte(); !errors.Is(err, ErrNoData) {
		t.Fatalf("dataReady=0 应返回 ErrNoData，实际 %v", err)
	}
}

// TestReceiveByteAcksAndClearsReady 断言出站直读槽 11/12 并完成 ACK 握手（槽 15 非 0 → 清 dataReady）。
func TestReceiveByteAcksAndClearsReady(t *testing.T) {
	engine := newFakeEngine()
	engine.rxQueue = []byte{0x5A}
	engine.rxFrame = []bool{false}
	br := newTestBridge(t, engine)
	if _, err := engine.Advance(); err != nil { // 元件交付第 1 字节
		t.Fatalf("Advance 失败: %v", err)
	}
	if engine.slots[SlotDataReady] != 1 {
		t.Fatalf("交付后 dataReady 应为 1，实际 %v", engine.slots[SlotDataReady])
	}
	b, frameErr, err := br.ReceiveByte()
	if err != nil {
		t.Fatalf("ReceiveByte 失败: %v", err)
	}
	if b != 0x5A || frameErr {
		t.Fatalf("读到 (0x%02X, frameErr=%v)，期望 (0x5A, false)", b, frameErr)
	}
	if engine.slots[SlotDataReady] != 0 || engine.slots[SlotAckFlag] != 0 {
		t.Fatalf("ACK 后应清 dataReady/ackFlag，实际 dataReady=%v ackFlag=%v",
			engine.slots[SlotDataReady], engine.slots[SlotAckFlag])
	}
}

// TestReceiveByteFrameError 断言帧错误经第三个返回值上报，而不是被吞掉。
func TestReceiveByteFrameError(t *testing.T) {
	engine := newFakeEngine()
	engine.rxQueue = []byte{0x99}
	engine.rxFrame = []bool{true}
	br := newTestBridge(t, engine)
	if _, err := engine.Advance(); err != nil {
		t.Fatalf("Advance 失败: %v", err)
	}
	b, frameErr, err := br.ReceiveByte()
	if err != nil {
		t.Fatalf("ReceiveByte 失败: %v", err)
	}
	if b != 0x99 || !frameErr {
		t.Fatalf("读到 (0x%02X, frameErr=%v)，期望 (0x99, true)", b, frameErr)
	}
}

// TestReceiveByteRejectsOutOfRangeSlot 断言槽值越界时返回 ErrSlotValue，且不产生 ACK 副作用。
func TestReceiveByteRejectsOutOfRangeSlot(t *testing.T) {
	engine := newFakeEngine()
	engine.slots[SlotDataReady] = 1
	engine.slots[SlotRxByte] = 300
	br := newTestBridge(t, engine)
	if _, _, err := br.ReceiveByte(); !errors.Is(err, ErrSlotValue) {
		t.Fatalf("rxByte=300 应返回 ErrSlotValue，实际 %v", err)
	}
	if len(engine.events) != 0 {
		t.Fatalf("越界时不得写 ACK 事件，实际 %v", engine.events)
	}
}

// TestSlotIoErrorsPropagate 断言底层读/写/推进的错误被原样返回（不 panic、不伪装成功）。
func TestSlotIoErrorsPropagate(t *testing.T) {
	br, err := NewBridge(brokenSlotIo{err: boom}, Config{Instance: "UART1"})
	if err != nil {
		t.Fatalf("NewBridge 失败: %v", err)
	}
	if _, _, rerr := br.ReceiveByte(); !errors.Is(rerr, boom) {
		t.Fatalf("ReadSlot 错误应被返回，实际 %v", rerr)
	}
	if serr := br.SendByte(0x01); !errors.Is(serr, boom) {
		t.Fatalf("WriteEvent 错误应被返回，实际 %v", serr)
	}
	if aerr := br.Advance(); !errors.Is(aerr, boom) {
		t.Fatalf("Advance 错误应被返回，实际 %v", aerr)
	}
}

// TestAdvanceFailureKeepsPending 断言推进失败时不得谎报"字节已被消费"。
func TestAdvanceFailureKeepsPending(t *testing.T) {
	br, err := NewBridge(advanceFailIo{inner: newFakeEngine()}, Config{Instance: "UART1"})
	if err != nil {
		t.Fatalf("NewBridge 失败: %v", err)
	}
	if err := br.SendByte(0x10); err != nil {
		t.Fatalf("SendByte 失败: %v", err)
	}
	if aerr := br.Advance(); !errors.Is(aerr, boom) {
		t.Fatalf("Advance 应返回注入错误，实际 %v", aerr)
	}
	if !br.Pending() {
		t.Fatal("推进失败后字节必须仍在途（不能因失败而解除在途标记）")
	}
	if err := br.SendByte(0x20); !errors.Is(err, ErrByteInFlight) {
		t.Fatalf("在途未确认时必须拒绝新字节，实际 %v", err)
	}
}
