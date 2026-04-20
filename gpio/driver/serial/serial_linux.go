//go:build linux
// +build linux

// 本文件包含 serial 包在 Linux 系统下的实现。
// 提供了基于 termios 的串口操作。
//
// 该实现使用 golang.org/x/sys/unix 包进行系统调用。
package serial

import (
	"circuit/gpio/driver"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// openPort 在 Linux 系统上打开一个串行端口。
//
// 参数：
//   - name: 串口设备路径，例如 "/dev/ttyUSB0"
//   - baud: 波特率，支持 50 到 4000000 的标准值
//   - databits: 数据位大小，支持 5、6、7、8
//   - parity: 奇偶校验类型，使用 Parity 常量
//   - stopbits: 停止位类型，使用 StopBits 常量
//   - readTimeout: 读取超时时间，影响 VMIN 和 VTIME 设置
//
// 返回值：
//   - *Port: 打开的串口端口实例
//   - error: 如果打开或配置失败，返回错误信息
//
// 实现细节：
//   - 使用 termios 结构进行串口配置
//   - 支持阻塞和非阻塞模式（通过超时设置）
//   - 自动处理文件描述符和资源清理
func openPort(name string, config *driver.UARTConfig) (p *Port, err error) {
	// 从配置中提取参数并设置默认值
	baud := int(config.BaudRate)
	databits := config.ByteSize
	par := Parity(config.Parity)
	stop := StopBits(config.StopBits)

	// 设置默认值
	if databits == 0 {
		databits = DefaultSize
	}
	if par == 0 {
		par = ParityNone
	}
	if stop == 0 {
		stop = Stop1
	}

	// 转换校验位常量为 Parity 类型
	var parityVal Parity
	switch config.Parity {
	case 0:
		parityVal = ParityNone
	case 1:
		parityVal = ParityOdd
	case 2:
		parityVal = ParityEven
	default:
		parityVal = ParityNone
	}

	// 转换停止位常量为 StopBits 类型
	var stopbitsVal StopBits
	switch config.StopBits {
	case 0:
		stopbitsVal = Stop1
	case 1:
		stopbitsVal = Stop1Half
	case 2:
		stopbitsVal = Stop2
	default:
		stopbitsVal = Stop1
	}

	// bauds 映射将标准波特率值转换为 termios 常量。
	var bauds = map[int]uint32{
		50:      unix.B50,
		75:      unix.B75,
		110:     unix.B110,
		134:     unix.B134,
		150:     unix.B150,
		200:     unix.B200,
		300:     unix.B300,
		600:     unix.B600,
		1200:    unix.B1200,
		1800:    unix.B1800,
		2400:    unix.B2400,
		4800:    unix.B4800,
		9600:    unix.B9600,
		19200:   unix.B19200,
		38400:   unix.B38400,
		57600:   unix.B57600,
		115200:  unix.B115200,
		230400:  unix.B230400,
		460800:  unix.B460800,
		500000:  unix.B500000,
		576000:  unix.B576000,
		921600:  unix.B921600,
		1000000: unix.B1000000,
		1152000: unix.B1152000,
		1500000: unix.B1500000,
		2000000: unix.B2000000,
		2500000: unix.B2500000,
		3000000: unix.B3000000,
		3500000: unix.B3500000,
		4000000: unix.B4000000,
	}
	rate, ok := bauds[baud]
	if !ok {
		return nil, fmt.Errorf("Unrecognized baud rate")
	}
	f, err := os.OpenFile(name, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0666)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil && f != nil {
			f.Close()
		}
	}()
	// 基础设置
	cflagToUse := unix.CREAD | unix.CLOCAL | rate
	switch databits {
	case 5:
		cflagToUse |= unix.CS5
	case 6:
		cflagToUse |= unix.CS6
	case 7:
		cflagToUse |= unix.CS7
	case 8:
		cflagToUse |= unix.CS8
	default:
		return nil, ErrBadSize
	}
	// 停止位设置
	switch stopbitsVal {
	case Stop1:
		// 默认为 1 个停止位
	case Stop2:
		cflagToUse |= unix.CSTOPB
	default:
		// 不知道如何设置 1.5 停止位
		return nil, ErrBadStopBits
	}
	// 奇偶校验设置
	switch parityVal {
	case ParityNone:
		// 默认为无校验
	case ParityOdd:
		cflagToUse |= unix.PARENB
		cflagToUse |= unix.PARODD
	case ParityEven:
		cflagToUse |= unix.PARENB
	default:
		return nil, ErrBadParity
	}
	fd := f.Fd()
	// 设置阻塞/非阻塞读取
	var vmin, vtime uint8
	if config.ByteTimeout == 0 {
		// 阻塞读取，无超时
		vmin = 1
		vtime = 0
	} else {
		// 非阻塞读取，字符间超时
		vmin = 0
		// 将毫秒超时转换为十分之一秒（VTIME单位）
		vtime = config.ByteTimeout / 100
		if vtime == 0 {
			// 小于100毫秒的超时设为最小单位（0.1秒）
			vtime = 1
		}
	}
	t := unix.Termios{
		Iflag:  unix.IGNPAR,
		Cflag:  cflagToUse,
		Ispeed: rate,
		Ospeed: rate,
	}
	t.Cc[unix.VMIN] = vmin
	t.Cc[unix.VTIME] = vtime
	if _, _, errno := unix.Syscall6(
		unix.SYS_IOCTL,
		uintptr(fd),
		uintptr(unix.TCSETS),
		uintptr(unsafe.Pointer(&t)),
		0,
		0,
		0,
	); errno != 0 {
		return nil, errno
	}
	if err = unix.SetNonblock(int(fd), false); err != nil {
		return
	}
	return &Port{f: f}, nil
}

// Port 代表一个打开的 Linux 串行端口。
// 它封装了底层的 os.File 并提供基本的读写操作。
type Port struct {
	f *os.File
}

// Read 从串口读取数据到字节切片中。
// 实现 io.Reader 接口，读取的字节数可能小于缓冲区长度。
// 返回读取的字节数和可能的错误（如超时或连接断开）。
func (p *Port) Read(b []byte) (n int, err error) {
	return p.f.Read(b)
}

// Write 将字节切片中的数据写入串口。
// 实现 io.Writer 接口，返回写入的字节数和可能的错误。
// 如果写入部分数据后发生错误，返回的 n 可能小于 len(b)。
func (p *Port) Write(b []byte) (n int, err error) {
	return p.f.Write(b)
}

// Flush 清空串口的输入和输出缓冲区。
// 使用 TCFLSH ioctl 命令丢弃所有未读和未写的数据。
// 常用于恢复通信状态或清除垃圾数据。
func (p *Port) Flush() error {
	// TCFLSH 是 Linux 终端刷新操作的 ioctl 命令码。
	// 该常量在 asm-generic/ioctls.h 中定义，用于控制终端缓冲区的刷新行为。
	const TCFLSH = 0x540B
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(p.f.Fd()),
		uintptr(TCFLSH),
		uintptr(unix.TCIOFLUSH),
	)
	if errno == 0 {
		return nil
	}
	return errno
}

// Close 关闭串行端口并释放底层文件描述符。
// 关闭后，任何未完成的读写操作将返回错误。
// 重复关闭是安全的，不会导致 panic。
func (p *Port) Close() (err error) {
	return p.f.Close()
}
