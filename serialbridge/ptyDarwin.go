//go:build darwin

// ptyDarwin.go 是 ptyUnix.go 的 macOS 差异段：
// 用 /dev/ptmx + TIOCPTYGRANT/TIOCPTYUNLK/TIOCPTYGNAME 造 pty，termios 用 TIOCGETA/TIOCSETA。
package serialbridge

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ptyNameLen 是 TIOCPTYGNAME 的参数长度（缓冲区是固定 128 字节的字符数组）。
const ptyNameLen = 128

// openPtyMaster 打开 macOS pty 主设备、授权并解锁，返回 fd 与从设备路径（/dev/ttysN）。
func openPtyMaster() (int, string, error) {
	fd, err := unix.Open(ptyMasterPath, unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, "", fmt.Errorf("serialbridge: 打开 %s 失败: %w", ptyMasterPath, err)
	}
	// 后续任一步失败都必须关掉 fd，否则每次失败泄漏一个 pty。
	var name [ptyNameLen]byte
	if err := ioctlPtr(fd, uintptr(syscall.TIOCPTYGNAME), unsafe.Pointer(&name[0])); err != nil {
		unix.Close(fd)
		return -1, "", fmt.Errorf("serialbridge: TIOCPTYGNAME 失败: %w", err)
	}
	if err := ioctlPtr(fd, uintptr(syscall.TIOCPTYGRANT), nil); err != nil {
		unix.Close(fd)
		return -1, "", fmt.Errorf("serialbridge: TIOCPTYGRANT 失败: %w", err)
	}
	if err := ioctlPtr(fd, uintptr(syscall.TIOCPTYUNLK), nil); err != nil {
		unix.Close(fd)
		return -1, "", fmt.Errorf("serialbridge: TIOCPTYUNLK 解锁失败: %w", err)
	}
	for i, c := range name {
		if c == 0 {
			if i == 0 {
				break
			}
			return fd, string(name[:i]), nil
		}
	}
	unix.Close(fd)
	return -1, "", fmt.Errorf("serialbridge: TIOCPTYGNAME 未给出 NUL 结尾的从设备名")
}

// ioctlPtr 发一次带指针参数的 ioctl；arg 为 nil 时按 0 传递。
func ioctlPtr(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// readTermios 读取 macOS termios（TIOCGETA）。
func readTermios(fd int) (*unix.Termios, error) {
	return unix.IoctlGetTermios(fd, unix.TIOCGETA)
}

// writeTermios 写入 macOS termios（TIOCSETA）。
func writeTermios(fd int, term *unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, term)
}
