// pty.go 用 golang.org/x/sys/unix 自建 Linux pty 主/从设备，并把从设备配成原始模式，
// 使 0x00/0x0D/0x11/0x13 等控制字节逐字节透传（不依赖 socat，也不装外部依赖）。
package serialbridge

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// ptyMasterPath 是 Linux 的 pty 主设备复用节点。
const ptyMasterPath = "/dev/ptmx"

// OpenPty 打开一个新的 pty 主设备，解锁后返回主设备文件与从设备路径（/dev/pts/N）。
// 返回的从设备已置为原始模式；调用方负责关闭主设备（关闭主设备会使从设备侧读到 EIO）。
func OpenPty() (*os.File, string, error) {
	fd, err := unix.Open(ptyMasterPath, unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", fmt.Errorf("serialbridge: 打开 %s 失败: %w", ptyMasterPath, err)
	}
	// 后续任一步失败都必须关掉 fd，否则每次失败泄漏一个 pty。
	ptn, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		unix.Close(fd)
		return nil, "", fmt.Errorf("serialbridge: TIOCGPTN 失败: %w", err)
	}
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		unix.Close(fd)
		return nil, "", fmt.Errorf("serialbridge: TIOCSPTLCK 解锁失败: %w", err)
	}
	master := os.NewFile(uintptr(fd), ptyMasterPath)
	return master, ptySlavePath(ptn), nil
}

// ptySlavePath 由主设备号拼出从设备路径（独立成函数便于单测拼接规则）。
func ptySlavePath(ptn int) string {
	return "/dev/pts/" + strconv.Itoa(ptn)
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
	term, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return fmt.Errorf("serialbridge: 读取 termios 失败: %w", err)
	}
	term.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	term.Oflag &^= unix.OPOST
	term.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	term.Cflag &^= unix.CSIZE | unix.PARENB
	term.Cflag |= unix.CS8
	term.Cc[unix.VMIN] = 1
	term.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, term); err != nil {
		return fmt.Errorf("serialbridge: 写入 termios 失败: %w", err)
	}
	return nil
}
