//go:build windows

// portWindows.go 是 OpenPort 的 Windows 实现：打开 COM 端口并配置成 8N1、
// 无流控、读写都有期限的字节通道（期限由 COMMTIMEOUTS 提供，不依赖 pty）。
package serialbridge

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// defaultBaud 是未给 Baud 时的默认波特率（与常见串口设备一致）。
	defaultBaud = 9600
	// dcbBinary 是 DCB.Flags 的 fBinary 位：必须置 1，否则 0x11/0x13 会被当流控处理。
	dcbBinary = 0x0001
	// commQueueSize 是驱动收发队列的建议大小（字节）。
	commQueueSize = 4096
)

// OpenPort 打开一个 Windows COM 端口并返回带期限的 Link。
// Name 为空返回 ErrPortName；端口不存在或打不开返回 ErrPortNotFound。
func OpenPort(cfg PortConfig) (Link, error) {
	if cfg.Name == "" {
		return nil, ErrPortName
	}
	name, err := windows.UTF16PtrFromString(windowsPortPath(cfg.Name))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPortName, err)
	}
	handle, err := syscall.CreateFile(name,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrPortNotFound, cfg.Name, err)
	}
	port := windows.Handle(handle)
	if err := configureWindowsPort(port, cfg); err != nil {
		windows.CloseHandle(port)
		return nil, err
	}
	return &WindowsLink{handle: port}, nil
}

// windowsPortPath 把 COM 名归一成 Win32 设备路径（COM10 及以上必须带 \\.\ 前缀）。
func windowsPortPath(name string) string {
	if strings.HasPrefix(name, `\\.\`) {
		return name
	}
	return `\\.\` + name
}

// portTimeout 把配置超时归一成有效值。
func portTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return DefaultReadTimeout
	}
	return timeout
}

// timeoutMillis 把期限换算成 COMMTIMEOUTS 的毫秒数（向上取整，至少 1ms）。
func timeoutMillis(timeout time.Duration) uint32 {
	ms := (timeout + time.Millisecond - 1) / time.Millisecond
	if ms <= 0 {
		ms = 1
	}
	return uint32(ms)
}

// configureWindowsPort 配置 8N1、无流控、读写期限。
func configureWindowsPort(port windows.Handle, cfg PortConfig) error {
	if err := windows.SetupComm(port, commQueueSize, commQueueSize); err != nil {
		return fmt.Errorf("serialbridge: SetupComm 失败: %w", err)
	}
	var dcb windows.DCB
	dcb.DCBlength = uint32(unsafe.Sizeof(dcb))
	if err := windows.GetCommState(port, &dcb); err != nil {
		return fmt.Errorf("serialbridge: GetCommState 失败: %w", err)
	}
	baud := cfg.Baud
	if baud <= 0 {
		baud = defaultBaud
	}
	dcb.BaudRate = uint32(baud)
	dcb.ByteSize = 8
	dcb.Parity = windows.NOPARITY
	dcb.StopBits = windows.ONESTOPBIT
	// 清掉 GetCommState 可能带回的流控/自动 RTS/DTR 位，只留 fBinary。
	dcb.Flags = dcbBinary
	if err := windows.SetCommState(port, &dcb); err != nil {
		return fmt.Errorf("serialbridge: SetCommState 失败: %w", err)
	}
	ms := timeoutMillis(portTimeout(cfg.Timeout))
	timeouts := windows.CommTimeouts{
		ReadTotalTimeoutConstant:    ms,
		WriteTotalTimeoutConstant:   ms,
		ReadIntervalTimeout:         0,
		ReadTotalTimeoutMultiplier:  0,
		WriteTotalTimeoutMultiplier: 0,
	}
	if err := windows.SetCommTimeouts(port, &timeouts); err != nil {
		return fmt.Errorf("serialbridge: SetCommTimeouts 失败: %w", err)
	}
	return nil
}

// WindowsLink 是 Windows COM 端口的 Link 实现：同步句柄 + COMMTIMEOUTS 期限。
// 并发约定：ReadByte/WriteBytes/Close 由调用方串行调用（桥的搬运是单线程的）。
type WindowsLink struct {
	handle windows.Handle
}

// ReadByte 读 1 字节：期限内无数据 → ErrTimeout；句柄失效 → ErrLinkClosed。
func (l *WindowsLink) ReadByte() (byte, error) {
	if l.handle == 0 {
		return 0, ErrLinkClosed
	}
	var buf [1]byte
	var done uint32
	if err := windows.ReadFile(l.handle, buf[:], &done, nil); err != nil {
		if isWindowsTimeout(err) {
			return 0, ErrTimeout
		}
		return 0, fmt.Errorf("%w: %v", ErrLinkClosed, err)
	}
	if done == 0 {
		return 0, ErrTimeout
	}
	return buf[0], nil
}

// WriteBytes 全量写出 p：期限内写不出去 → ErrTimeout；写失败 → ErrLinkClosed。
func (l *WindowsLink) WriteBytes(p []byte) (int, error) {
	if l.handle == 0 {
		return 0, ErrLinkClosed
	}
	written := 0
	for written < len(p) {
		var done uint32
		if err := windows.WriteFile(l.handle, p[written:], &done, nil); err != nil {
			if isWindowsTimeout(err) {
				return written, ErrTimeout
			}
			return written, fmt.Errorf("%w: %v", ErrLinkClosed, err)
		}
		if done == 0 {
			return written, ErrTimeout
		}
		written += int(done)
	}
	return written, nil
}

// Close 关闭 COM 端口句柄；重复关闭返回 ErrLinkClosed（不 panic）。
func (l *WindowsLink) Close() error {
	if l.handle == 0 {
		return ErrLinkClosed
	}
	err := windows.CloseHandle(l.handle)
	l.handle = 0
	return err
}

// isWindowsTimeout 把 Windows 的超时错误码归一成可判定的超时。
func isWindowsTimeout(err error) bool {
	return errors.Is(err, windows.ERROR_SEM_TIMEOUT) || errors.Is(err, windows.ERROR_TIMEOUT)
}

// 静态断言：WindowsLink 是 Link 的平台实现。
var _ Link = (*WindowsLink)(nil)
