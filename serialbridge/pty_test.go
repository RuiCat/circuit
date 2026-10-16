// pty_test.go 用自建 pty（golang.org/x/sys/unix，无 socat）验证真实字节流双向一致，
// 并把桥接到 pty 上跑通 16 字节双向端到端；同时覆盖读超时与对端关闭。
// 判定口径按 C13：只断言字节序列的数量/顺序/内容，不断言真实波特率时序。
package serialbridge

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"
)

// ptyReadTimeout 是测试侧的读超时（真实 pty 只做字节流判定，不测时序）。
const ptyReadTimeout = 2 * time.Second

// openTestPty 自建 pty 对，返回主设备与从设备文件；清理由 t.Cleanup 负责。
func openTestPty(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, slavePath, err := OpenPty()
	if err != nil {
		t.Fatalf("自建 pty 失败: %v", err)
	}
	slave, err := OpenSlave(slavePath)
	if err != nil {
		master.Close()
		t.Fatalf("打开从设备 %s 失败: %v", slavePath, err)
	}
	t.Cleanup(func() {
		slave.Close()
		master.Close()
	})
	t.Logf("pty 自建成功: master=%s slave=%s", master.Name(), slavePath)
	return master, slave
}

// readExactly 用 PtyLink 在超时内读满 n 字节；不足即失败（读超时同样判失败，不重试）。
func readExactly(t *testing.T, link Link, n int) []byte {
	t.Helper()
	buf := make([]byte, 0, n)
	for len(buf) < n {
		b, err := link.ReadByte()
		if err != nil {
			t.Fatalf("读第 %d 个字节失败（已读 %d 字节）: %v", len(buf)+1, len(buf), err)
		}
		buf = append(buf, b)
	}
	return buf
}

// TestPtySelfHostedByteStream 断言本机可用 x/sys 自建 pty，且双向都能按序收到 16 字节。
func TestPtySelfHostedByteStream(t *testing.T) {
	master, slave := openTestPty(t)
	masterLink := NewPtyLink(master, ptyReadTimeout)
	slaveLink := NewPtyLink(slave, ptyReadTimeout)

	if _, err := slave.Write(payload16); err != nil {
		t.Fatalf("从设备写入失败: %v", err)
	}
	got := readExactly(t, masterLink, len(payload16))
	if !bytes.Equal(got, payload16) {
		t.Fatalf("从设备→主设备 字节序列不一致:\n got=%v\nwant=%v", got, payload16)
	}

	if _, err := master.Write(payload16); err != nil {
		t.Fatalf("主设备写入失败: %v", err)
	}
	gotBack := readExactly(t, slaveLink, len(payload16))
	if !bytes.Equal(gotBack, payload16) {
		t.Fatalf("主设备→从设备 字节序列不一致:\n got=%v\nwant=%v", gotBack, payload16)
	}
}

// TestPtyLinkReadTimeout 断言无数据时 PtyLink 返回 ErrTimeout（不无限阻塞、不 panic）。
func TestPtyLinkReadTimeout(t *testing.T) {
	_, slave := openTestPty(t)
	link := NewPtyLink(slave, 100*time.Millisecond)
	if _, err := link.ReadByte(); !errors.Is(err, ErrTimeout) {
		t.Fatalf("无数据应返回 ErrTimeout，实际 %v", err)
	}
}

// TestPtyLinkReadAfterMasterClosed 断言对端关闭后返回 ErrLinkClosed（可判定，不 panic）。
func TestPtyLinkReadAfterMasterClosed(t *testing.T) {
	master, slave := openTestPty(t)
	link := NewPtyLink(slave, 200*time.Millisecond)
	if err := master.Close(); err != nil {
		t.Fatalf("关闭主设备失败: %v", err)
	}
	if _, err := link.ReadByte(); !errors.Is(err, ErrLinkClosed) {
		t.Fatalf("对端关闭应返回 ErrLinkClosed，实际 %v", err)
	}
}

// TestPtyBridgeEndToEnd 把桥的串口侧接到 pty 从设备，跑 16 字节双向端到端：
// 主设备写入 → 桥搬运进槽 3 事件 → 假元件按序收下；假元件排出 → 桥直读槽 11/12 → 主设备侧按序收到。
func TestPtyBridgeEndToEnd(t *testing.T) {
	master, slave := openTestPty(t)
	masterLink := NewPtyLink(master, ptyReadTimeout)
	engine := newFakeEngine()
	br := newTestBridge(t, engine)
	link := NewPtyLink(slave, 500*time.Millisecond)

	if _, err := master.Write(payload16); err != nil {
		t.Fatalf("主设备写入失败: %v", err)
	}
	got, err := br.Ingest(link, len(payload16))
	if err != nil {
		t.Fatalf("Ingest 失败（已搬运 %d 字节）: %v", got, err)
	}
	if got != len(payload16) {
		t.Fatalf("桥搬运字节数 = %d，期望 %d", got, len(payload16))
	}
	if !bytes.Equal(engine.txLog, payload16) {
		t.Fatalf("串口→仿真 字节序列不一致:\n got=%v\nwant=%v", engine.txLog, payload16)
	}

	engine.rxQueue = append([]byte(nil), payload16...)
	engine.rxFrame = make([]bool, len(payload16))
	sent, frameErrs, err := br.Drain(link, len(payload16))
	if err != nil {
		t.Fatalf("Drain 失败（已写出 %d 字节）: %v", sent, err)
	}
	if sent != len(payload16) || frameErrs != 0 {
		t.Fatalf("Drain 返回 (sent=%d, frameErrs=%d)，期望 (%d, 0)", sent, frameErrs, len(payload16))
	}
	back := readExactly(t, masterLink, len(payload16))
	if !bytes.Equal(back, payload16) {
		t.Fatalf("仿真→串口 字节序列不一致:\n got=%v\nwant=%v", back, payload16)
	}
	t.Logf("端到端双向各 %d 字节一致（pty 只证明字节序列，不证明波特率时序）", len(payload16))
}
