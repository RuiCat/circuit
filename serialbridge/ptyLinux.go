//go:build linux

// ptyLinux.go 是 ptyUnix.go 的 Linux 差异段：
// 用 /dev/ptmx + TIOCGPTN/TIOCSPTLCK 造 pty，termios 用 TCGETS/TCSETS。
package serialbridge

import (
	"fmt"
	"strconv"

	"golang.org/x/sys/unix"
)

// openPtyMaster 打开 Linux pty 主设备并解锁，返回 fd 与从设备路径（/dev/pts/N）。
func openPtyMaster() (int, string, error) {
	fd, err := unix.Open(ptyMasterPath, unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, "", fmt.Errorf("serialbridge: 打开 %s 失败: %w", ptyMasterPath, err)
	}
	// 后续任一步失败都必须关掉 fd，否则每次失败泄漏一个 pty。
	ptn, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		unix.Close(fd)
		return -1, "", fmt.Errorf("serialbridge: TIOCGPTN 失败: %w", err)
	}
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		unix.Close(fd)
		return -1, "", fmt.Errorf("serialbridge: TIOCSPTLCK 解锁失败: %w", err)
	}
	return fd, ptySlavePath(ptn), nil
}

// ptySlavePath 由主设备号拼出从设备路径（独立成函数便于单测拼接规则）。
func ptySlavePath(ptn int) string {
	return "/dev/pts/" + strconv.Itoa(ptn)
}

// readTermios 读取 Linux termios（TCGETS）。
func readTermios(fd int) (*unix.Termios, error) {
	return unix.IoctlGetTermios(fd, unix.TCGETS)
}

// writeTermios 写入 Linux termios（TCSETS）。
func writeTermios(fd int, term *unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TCSETS, term)
}
