//go:build darwin
// +build darwin

// 本文件包含 serial 包在 macOS (darwin) 下的实现。
// 使用 golang.org/x/sys/unix 进行纯 Go 系统调用，无需 cgo。
// macOS 的 Termios 结构使用 uint64 字段，与 Linux/BSD 的 uint32 不同。
package serial

import (
	"circuit/gpio/driver"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// bauds 波特率映射表（macOS 支持的波特率）。
var bauds = map[int]uint64{
	50:     unix.B50,
	75:     unix.B75,
	110:    unix.B110,
	134:    unix.B134,
	150:    unix.B150,
	200:    unix.B200,
	300:    unix.B300,
	600:    unix.B600,
	1200:   unix.B1200,
	1800:   unix.B1800,
	2400:   unix.B2400,
	4800:   unix.B4800,
	7200:   unix.B7200,
	9600:   unix.B9600,
	14400:  unix.B14400,
	19200:  unix.B19200,
	28800:  unix.B28800,
	38400:  unix.B38400,
	57600:  unix.B57600,
	76800:  unix.B76800,
	115200: unix.B115200,
	230400: unix.B230400,
}

// openPort 在 macOS 上打开一个串行端口。
func openPort(name string, config *driver.UARTConfig) (p *Port, err error) {
	baud := int(config.BaudRate)
	databits := config.ByteSize
	par := Parity(config.Parity)
	stop := StopBits(config.StopBits)

	if databits == 0 {
		databits = DefaultSize
	}
	if par == 0 {
		par = ParityNone
	}
	if stop == 0 {
		stop = Stop1
	}

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

	rate, ok := bauds[baud]
	if !ok {
		return nil, fmt.Errorf("不支持的波特率: %d", baud)
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

	cflagToUse := uint64(unix.CREAD) | uint64(unix.CLOCAL) | rate
	switch databits {
	case 5:
		cflagToUse |= uint64(unix.CS5)
	case 6:
		cflagToUse |= uint64(unix.CS6)
	case 7:
		cflagToUse |= uint64(unix.CS7)
	case 8:
		cflagToUse |= uint64(unix.CS8)
	default:
		return nil, ErrBadSize
	}

	switch stopbitsVal {
	case Stop1:
		// 默认 1 位停止位
	case Stop2:
		cflagToUse |= uint64(unix.CSTOPB)
	default:
		return nil, ErrBadStopBits
	}

	switch parityVal {
	case ParityNone:
		// 默认无校验
	case ParityOdd:
		cflagToUse |= uint64(unix.PARENB)
		cflagToUse |= uint64(unix.PARODD)
	case ParityEven:
		cflagToUse |= uint64(unix.PARENB)
	default:
		return nil, ErrBadParity
	}

	fd := f.Fd()

	var vmin, vtime uint8
	if config.ByteTimeout == 0 {
		vmin = 1
		vtime = 0
	} else {
		vmin = 0
		vtime = (config.ByteTimeout + 99) / 100
	}

	t := unix.Termios{
		Iflag:  uint64(unix.IGNPAR),
		Cflag:  cflagToUse,
		Ispeed: rate,
		Ospeed: rate,
	}
	t.Cc[unix.VMIN] = vmin
	t.Cc[unix.VTIME] = vtime

	if _, _, errno := unix.Syscall6(
		unix.SYS_IOCTL,
		uintptr(fd),
		uintptr(unix.TIOCSETA),
		uintptr(unsafe.Pointer(&t)),
		0, 0, 0,
	); errno != 0 {
		return nil, errno
	}

	if err = unix.SetNonblock(int(fd), false); err != nil {
		return nil, err
	}

	return &Port{f: f}, nil
}

// Port 代表一个打开的串行端口。
type Port struct {
	f *os.File
}

// Read 从串口读取数据。
func (p *Port) Read(b []byte) (n int, err error) {
	return p.f.Read(b)
}

// Write 向串口写入数据。
func (p *Port) Write(b []byte) (n int, err error) {
	return p.f.Write(b)
}

// Flush 清空串口的输入输出缓冲区。
func (p *Port) Flush() error {
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(p.f.Fd()),
		uintptr(unix.TIOCFLUSH),
		uintptr(unix.TCIOFLUSH),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// Close 关闭串行端口。
func (p *Port) Close() (err error) {
	return p.f.Close()
}
