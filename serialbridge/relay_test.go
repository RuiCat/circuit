// relay_test.go 覆盖两个搬运方向的字节数量/顺序一致（C13 口径：只断言字节序列），
// 以及超时、步数预算、部分搬运等边界不 panic。
package serialbridge

import (
	"bytes"
	"errors"
	"testing"
)

// payload16 含 NUL/CR/LF/XON/XOFF/高位字节：行规程若没关原始模式，这些字节会被改写或吞掉。
var payload16 = []byte{
	0x00, 0x01, 0x0D, 0x0A, 0x11, 0x13, 0x1A, 0x7F,
	0x41, 0x42, 0x80, 0xFE, 0xFF, 0x55, 0xAA, 0x0F,
}

// TestIngest16BytesInOrder 是 C15 的连写判据：一次搬运 16 字节，收侧数量与顺序逐字节一致。
func TestIngest16BytesInOrder(t *testing.T) {
	engine := newFakeEngine()
	br := newTestBridge(t, engine)
	link := &fakeLink{in: append([]byte(nil), payload16...)}

	got, err := br.Ingest(link, len(payload16))
	if err != nil {
		t.Fatalf("Ingest 失败: %v", err)
	}
	if got != len(payload16) {
		t.Fatalf("搬运字节数 = %d，期望 %d", got, len(payload16))
	}
	if len(engine.txLog) != len(payload16) {
		t.Fatalf("收侧字节数 = %d，期望 %d", len(engine.txLog), len(payload16))
	}
	if !bytes.Equal(engine.txLog, payload16) {
		t.Fatalf("收侧字节序列不一致:\n got=%v\nwant=%v", engine.txLog, payload16)
	}
	if engine.overwrites != 0 {
		t.Fatalf("一次一字节 + 确认路径下不得出现同名事件覆盖，实际 %d 次", engine.overwrites)
	}
}

// TestDrain16BytesInOrder 断言出站方向同样只按字节序列判定：数量与顺序逐字节一致。
func TestDrain16BytesInOrder(t *testing.T) {
	engine := newFakeEngine()
	engine.rxQueue = append([]byte(nil), payload16...)
	engine.rxFrame = make([]bool, len(payload16))
	br := newTestBridge(t, engine)
	link := &fakeLink{}

	sent, frameErrs, err := br.Drain(link, len(payload16))
	if err != nil {
		t.Fatalf("Drain 失败: %v", err)
	}
	if sent != len(payload16) || frameErrs != 0 {
		t.Fatalf("Drain 返回 (sent=%d, frameErrs=%d)，期望 (%d, 0)", sent, frameErrs, len(payload16))
	}
	if !bytes.Equal(link.out, payload16) {
		t.Fatalf("串口侧字节序列不一致:\n got=%v\nwant=%v", link.out, payload16)
	}
}

// TestDrainStopsAtStepBudget 断言元件不出字节时以 ErrStepBudget 收场（不挂起、不 panic）。
func TestDrainStopsAtStepBudget(t *testing.T) {
	br, err := NewBridge(newFakeEngine(), Config{Instance: "UART1", StepBudget: 8})
	if err != nil {
		t.Fatalf("NewBridge 失败: %v", err)
	}
	sent, frameErrs, derr := br.Drain(&fakeLink{}, 4)
	if !errors.Is(derr, ErrStepBudget) {
		t.Fatalf("无出站数据时应返回 ErrStepBudget，实际 %v", derr)
	}
	if sent != 0 || frameErrs != 0 {
		t.Fatalf("不应写出任何字节，实际 (sent=%d, frameErrs=%d)", sent, frameErrs)
	}
}

// TestIngestPropagatesLinkErrors 断言串口侧超时/断开被原样上报，而不是被当作"无数据"吞掉。
func TestIngestPropagatesLinkErrors(t *testing.T) {
	for _, want := range []error{ErrTimeout, ErrLinkClosed} {
		br := newTestBridge(t, newFakeEngine())
		got, err := br.Ingest(&fakeLink{err: want}, 4)
		if !errors.Is(err, want) {
			t.Fatalf("err = %v，期望 %v", err, want)
		}
		if got != 0 {
			t.Fatalf("未搬运任何字节时应返回 0，实际 %d", got)
		}
	}
}

// TestIngestPartialThenTimeout 断言"已搬运 2 字节后对端不再给数据"返回部分计数 + 超时错误。
func TestIngestPartialThenTimeout(t *testing.T) {
	engine := newFakeEngine()
	br := newTestBridge(t, engine)
	link := &fakeLink{in: []byte{0xAA, 0xBB}}

	got, err := br.Ingest(link, 4)
	if got != 2 || !errors.Is(err, ErrTimeout) {
		t.Fatalf("返回 (got=%d, err=%v)，期望 (2, ErrTimeout)", got, err)
	}
	if !bytes.Equal(engine.txLog, []byte{0xAA, 0xBB}) {
		t.Fatalf("已搬运字节应完整到达收侧，实际 %v", engine.txLog)
	}
}
