//go:build !windows && !linux && cgo
// +build !windows,!linux,cgo

// 本文件包含 serial 包在 POSIX 系统（如 macOS、BSD）下的实现。
// 使用 cgo 调用 termios.h 中的函数进行串口配置。
//
// 注意：此实现仅在不支持 Windows 和 Linux 的系统上使用。
package serial

// #include <termios.h>
// #include <unistd.h>
import "C"

// TODO: 也许改为使用 syscall 包 + ioctl 而不是 cgo

import (
	"circuit/gpio/driver"
	"errors"
	"fmt"
	"os"
	"syscall"
)

// openPort 在 POSIX 系统上打开一个串行端口。
//
// 参数：
//   - name: 串口设备路径，例如 "/dev/tty.usbserial"
//   - baud: 波特率，支持 50 到 115200 的标准值
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
//   - 通过 cgo 调用 tcgetattr/tcsetattr 等函数
//   - 支持原始模式（raw mode）和超时设置
//   - 自动检测是否为 TTY 设备
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

	f, err := os.OpenFile(name, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0666)
	if err != nil {
		return
	}
	fd := C.int(f.Fd())
	if C.isatty(fd) != 1 {
		f.Close()
		return nil, errors.New("File is not a tty")
	}
	var st C.struct_termios
	_, err = C.tcgetattr(fd, &st)
	if err != nil {
		f.Close()
		return nil, err
	}
	var speed C.speed_t
	// 将标准波特率转换为 termios 速度常量。
	switch baud {
	case 115200:
		speed = C.B115200
	case 57600:
		speed = C.B57600
	case 38400:
		speed = C.B38400
	case 19200:
		speed = C.B19200
	case 9600:
		speed = C.B9600
	case 4800:
		speed = C.B4800
	case 2400:
		speed = C.B2400
	case 1200:
		speed = C.B1200
	case 600:
		speed = C.B600
	case 300:
		speed = C.B300
	case 200:
		speed = C.B200
	case 150:
		speed = C.B150
	case 134:
		speed = C.B134
	case 110:
		speed = C.B110
	case 75:
		speed = C.B75
	case 50:
		speed = C.B50
	default:
		f.Close()
		return nil, fmt.Errorf("Unknown baud rate %v", baud)
	}
	_, err = C.cfsetispeed(&st, speed)
	if err != nil {
		f.Close()
		return nil, err
	}
	_, err = C.cfsetospeed(&st, speed)
	if err != nil {
		f.Close()
		return nil, err
	}
	// 关闭中断、CR->NL 转换、奇偶校验、剥离和 IXON
	st.c_iflag &= ^C.tcflag_t(C.BRKINT | C.ICRNL | C.INPCK | C.ISTRIP | C.IXOFF | C.IXON | C.PARMRK)
	// 选择本地模式，关闭奇偶校验，设置为 8 位数据
	st.c_cflag &= ^C.tcflag_t(C.CSIZE | C.PARENB)
	st.c_cflag |= (C.CLOCAL | C.CREAD)
	// 数据位
	switch databits {
	case 5:
		st.c_cflag |= C.CS5
	case 6:
		st.c_cflag |= C.CS6
	case 7:
		st.c_cflag |= C.CS7
	case 8:
		st.c_cflag |= C.CS8
	default:
		return nil, ErrBadSize
	}
	// 奇偶校验设置
	switch parityVal {
	case ParityNone:
		// 默认为无校验
	case ParityOdd:
		st.c_cflag |= C.PARENB
		st.c_cflag |= C.PARODD
	case ParityEven:
		st.c_cflag |= C.PARENB
		st.c_cflag &= ^C.tcflag_t(C.PARODD)
	default:
		return nil, ErrBadParity
	}
	// 停止位设置
	switch stopbitsVal {
	case Stop1:
		// 保持原样，默认为 1 位
	case Stop2:
		st.c_cflag |= C.CSTOPB
	default:
		return nil, ErrBadStopBits
	}
	// 选择原始模式
	st.c_lflag &= ^C.tcflag_t(C.ICANON | C.ECHO | C.ECHOE | C.ISIG)
	st.c_oflag &= ^C.tcflag_t(C.OPOST)
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
	st.c_cc[C.VMIN] = C.cc_t(vmin)
	st.c_cc[C.VTIME] = C.cc_t(vtime)
	_, err = C.tcsetattr(fd, C.TCSANOW, &st)
	if err != nil {
		f.Close()
		return nil, err
	}
	r1, _, e := syscall.Syscall(syscall.SYS_FCNTL,
		uintptr(f.Fd()),
		uintptr(syscall.F_SETFL),
		uintptr(0))
	if e != 0 || r1 != 0 {
		s := fmt.Sprint("Clearing NONBLOCK syscall error:", e, r1)
		f.Close()
		return nil, errors.New(s)
	}
	return &Port{f: f}, nil
}

// Port 代表一个打开的 POSIX 串行端口。
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
// 使用 tcflush 系统调用丢弃所有未读和未写的数据。
// 常用于恢复通信状态或清除垃圾数据。
func (p *Port) Flush() error {
	_, err := C.tcflush(C.int(p.f.Fd()), C.TCIOFLUSH)
	return err
}

// Close 关闭串行端口并释放底层文件描述符。
// 关闭后，任何未完成的读写操作将返回错误。
// 重复关闭是安全的，不会导致 panic。
func (p *Port) Close() (err error) {
	return p.f.Close()
}
