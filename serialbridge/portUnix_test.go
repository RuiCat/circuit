//go:build unix

// portUnix_test.go 验证跨平台入口 OpenPort 在 Unix 上的可判定错误与真实 tty 打开。
package serialbridge

import (
	"errors"
	"testing"
	"time"
)

// TestOpenPortEmptyName 断言空端点名返回 ErrPortName（不 panic）。
func TestOpenPortEmptyName(t *testing.T) {
	_, err := OpenPort(PortConfig{})
	if !errors.Is(err, ErrPortName) {
		t.Fatalf("空端点名应返回 ErrPortName，实际 %v", err)
	}
}

// TestOpenPortMissingPath 断言不存在的 tty 路径返回 ErrPortNotFound。
func TestOpenPortMissingPath(t *testing.T) {
	_, err := OpenPort(PortConfig{Name: "/dev/definitely-not-a-tty-9999"})
	if !errors.Is(err, ErrPortNotFound) {
		t.Fatalf("不存在的路径应返回 ErrPortNotFound，实际 %v", err)
	}
}

// TestOpenPortPtySlave 断言 OpenPort 能打开真实 pty 从设备，并保留"无数据 → ErrTimeout"的期限语义。
func TestOpenPortPtySlave(t *testing.T) {
	master, slavePath, err := OpenPty()
	if err != nil {
		t.Fatalf("自建 pty 失败: %v", err)
	}
	defer master.Close()

	link, err := OpenPort(PortConfig{Name: slavePath, Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("打开 pty 从设备 %s 失败: %v", slavePath, err)
	}
	defer link.Close()

	if _, err := link.ReadByte(); !errors.Is(err, ErrTimeout) {
		t.Fatalf("无数据应返回 ErrTimeout，实际 %v", err)
	}
}
