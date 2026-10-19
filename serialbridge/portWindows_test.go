//go:build windows

// portWindows_test.go 验证 OpenPort 在 Windows 上的参数校验与"端口不存在"的可判定错误。
// 真实 COM 端口的收发需要硬件，不在自动测试范围内（编译验证另由 make cross 覆盖）。
package serialbridge

import (
	"errors"
	"testing"
)

// TestOpenPortEmptyName 断言空端点名返回 ErrPortName（不 panic）。
func TestOpenPortEmptyName(t *testing.T) {
	_, err := OpenPort(PortConfig{})
	if !errors.Is(err, ErrPortName) {
		t.Fatalf("空端点名应返回 ErrPortName，实际 %v", err)
	}
}

// TestOpenPortMissingPort 断言不存在的 COM 端口返回 ErrPortNotFound。
func TestOpenPortMissingPort(t *testing.T) {
	_, err := OpenPort(PortConfig{Name: "COM_NOPE_9999"})
	if !errors.Is(err, ErrPortNotFound) {
		t.Fatalf("不存在的端口应返回 ErrPortNotFound，实际 %v", err)
	}
}
