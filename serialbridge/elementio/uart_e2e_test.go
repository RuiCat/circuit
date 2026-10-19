//go:build unix

// uart_e2e_test.go：B 档端到端验收 —— 真实 pty ↔ 桥 ↔ SlotIo 适配器 ↔ UART 元件。
//
// 本文件由 Lead 在集成阶段补写：它闭合 m5 自陈的「未验证 1」（与 UART 元件的端到端往返）
// 与 C2/C14/C15 的验收。链路：pty 主设备写入字节 → 桥 Ingest（一次一字节 + 等 TX 空闲，D16）
// → UART 元件经 tx 脚发出 8N1 帧 → 网表把 tx/rx 接在同一节点（自环回）→ UART 解出同一字节
// → 桥 Drain 直读槽 11/12/13 写回 pty → 主设备读回。
package elementio_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "circuit/element/register" // R 等既有元件
	_ "circuit/element/serial"   // C7：blank import 触发 UART 的 init() 注册
	"circuit/load"
	"circuit/serialbridge"
	"circuit/serialbridge/elementio"
)

// loopbackNetlist 读取自环回网表（tx 与 rx 同节点）。
func loopbackNetlist(t *testing.T) []byte {
	t.Helper()
	for _, p := range []string{
		filepath.Join("..", "..", "cmd", "circuits", "68_ttl_uart_loopback.net"),
		filepath.Join("cmd", "circuits", "68_ttl_uart_loopback.net"),
	} {
		if b, err := os.ReadFile(p); err == nil {
			return b
		}
	}
	t.Fatalf("找不到网表 68_ttl_uart_loopback.net")
	return nil
}

// readWithTimeout 在期限内从 pty 主设备读满 buf（pty 上 SetReadDeadline 不可用，故用 goroutine + 计时器）。
func readWithTimeout(t *testing.T, f *os.File, buf []byte, d time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(f, buf)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("从 pty 读回失败: %v", err)
		}
	case <-time.After(d):
		t.Fatalf("从 pty 读回超时（%s）", d)
	}
}

// TestUartPtyStreamEndToEnd 16 字节经真实 pty 往返：顺序与内容逐字节一致。
func TestUartPtyStreamEndToEnd(t *testing.T) {
	con, err := load.LoadString(string(loopbackNetlist(t)))
	if err != nil {
		t.Fatalf("加载网表失败: %v", err)
	}
	// C1：时序元件必须显式给步长上界（9600 baud ⇒ T≈104.17µs，上界取 T/20）。
	ad, err := elementio.NewAdapter(con, "UART1", elementio.Options{
		MinStep: 1e-9, MaxStep: 5e-6, TargetTime: 1.0, StepTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("构造适配器失败: %v", err)
	}
	defer func() { _ = ad.Close() }()

	br, err := serialbridge.NewBridge(ad, serialbridge.Config{Instance: "UART1", StepBudget: 8192})
	if err != nil {
		t.Fatalf("构造桥失败: %v", err)
	}

	master, slavePath, err := serialbridge.OpenPty()
	if err != nil {
		t.Skipf("pty 自建不可用，跳过端到端: %v", err)
	}
	defer func() { _ = master.Close() }()
	link, err := serialbridge.OpenPort(serialbridge.PortConfig{Name: slavePath, Timeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("打开 pty 从设备失败: %v", err)
	}
	defer func() { _ = link.Close() }()

	payload := make([]byte, 16)
	for i := range payload {
		payload[i] = byte(0x40 + i) // 0x40..0x4F：避开控制字符对 pty 的干扰
	}
	if _, err := master.Write(payload); err != nil {
		t.Fatalf("向 pty 写入失败: %v", err)
	}

	// 逐字节握手：发 1 字节 → 元件发完（TX 空闲）→ 自环回解出 → Drain 写回 pty。
	// 为什么不能一次性 Ingest(16)：入站槽与 rxByte 槽都只有一个（元件只在帧间空闲取走），
	// 不握手连发会静默丢字节（D16 / C15）。
	for i := 0; i < len(payload); i++ {
		if n, err := br.Ingest(link, 1); err != nil || n != 1 {
			t.Fatalf("第 %d 字节入站失败: n=%d err=%v", i, n, err)
		}
		n, frameErrs, err := br.Drain(link, 1)
		if err != nil || n != 1 {
			t.Fatalf("第 %d 字节出站失败: n=%d frameErrs=%d err=%v", i, n, frameErrs, err)
		}
		if frameErrs != 0 {
			t.Fatalf("第 %d 字节报帧错误（自环回不应出现）", i)
		}
	}

	out := make([]byte, len(payload))
	readWithTimeout(t, master, out, 10*time.Second)
	if !bytes.Equal(out, payload) {
		t.Fatalf("端到端不一致:\n got % x\nwant % x", out, payload)
	}
}
