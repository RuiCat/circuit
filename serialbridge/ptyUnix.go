//go:build unix

// ptyUnix.go 在 Unix（Linux / macOS）上自建 pty 主/从设备并置原始模式。
// 平台差异只在 ioctl 常量与从设备命名：openPtyMaster/readTermios/writeTermios
// 由 ptyLinux.go 与 ptyDarwin.go 各自提供，本文件只放共享流程。
package serialbridge

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// ptyMasterPath 是 Unix 的 pty 主设备复用节点。
const ptyMasterPath = "/dev/ptmx"

// OpenPty 打开一个新的 pty 主设备，解锁后返回主设备文件与从设备路径。
// 返回的从设备尚未打开；调用方通常再调 OpenSlave 打开并置原始模式。
// 调用方负责关闭主设备（关闭主设备会使从设备侧读到 EIO）。
func OpenPty() (*os.File, string, error) {
	fd, slavePath, err := openPtyMaster()
	if err != nil {
		return nil, "", err
	}
	return os.NewFile(uintptr(fd), ptyMasterPath), slavePath, nil
}

// OpenSlave 打开 pty 从设备路径，并把它置为原始模式（关回显/规范模式/信号与流控改写）。
func OpenSlave(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("serialbridge: 打开从设备 %s 失败: %w", path, err)
	}
	if err := SetRaw(file); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

// SetRaw 把 tty 置为原始模式（cfmakeraw 的显式等价写法）。
// 为什么必须关 IXON/OPOST/ICRNL：不关就会把 0x11/0x13 当流控吞掉、把 LF 改成 CRLF，
// 于是"字节流逐字节一致"的判据会被行规程改写而不是被桥改写，证据就无效了。
func SetRaw(file *os.File) error {
	fd := int(file.Fd())
	term, err := readTermios(fd)
	if err != nil {
		return fmt.Errorf("serialbridge: 读取 termios 失败: %w", err)
	}
	applyRaw(term)
	if err := writeTermios(fd, term); err != nil {
		return fmt.Errorf("serialbridge: 写入 termios 失败: %w", err)
	}
	return nil
}

// applyRaw 修改 termios 字段（Linux 与 macOS 字段名一致，常量在两平台都有）。
func applyRaw(term *unix.Termios) {
	term.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	term.Oflag &^= unix.OPOST
	term.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	term.Cflag &^= unix.CSIZE | unix.PARENB
	term.Cflag |= unix.CS8
	term.Cc[unix.VMIN] = 1
	term.Cc[unix.VTIME] = 0
}
