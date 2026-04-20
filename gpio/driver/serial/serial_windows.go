//go:build windows
// +build windows

// 本文件包含 serial 包在 Windows 系统下的实现。
// 使用 Windows API 进行串口操作，支持重叠 I/O 以提高性能。
//
// 注意：此实现仅在使用 Windows 构建标签时编译。
package serial

import (
	"circuit/gpio/driver"
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// Port 代表一个打开的 Windows 串行端口。
// 使用重叠 I/O 实现异步读写操作，提高并发性能。
//
// 字段说明：
//   - f: 底层的 os.File，用于关闭操作
//   - fd: 系统句柄，用于 Windows API 调用
//   - rl: 读操作互斥锁，确保读操作的线程安全
//   - wl: 写操作互斥锁，确保写操作的线程安全
//   - ro: 读操作的重叠结构，用于异步读取
//   - wo: 写操作的重叠结构，用于异步写入
type Port struct {
	f  *os.File
	fd syscall.Handle
	rl sync.Mutex
	wl sync.Mutex
	ro *syscall.Overlapped
	wo *syscall.Overlapped
}

// structDCB 是 Windows DCB (Device Control Block) 结构的 Go 表示。
// 用于配置串口通信参数，如波特率、数据位、停止位、校验位等。
type structDCB struct {
	DCBlength, BaudRate                            uint32
	flags                                          [4]byte
	wReserved, XonLim, XoffLim                     uint16
	ByteSize, Parity, StopBits                     byte
	XonChar, XoffChar, ErrorChar, EofChar, EvtChar byte
	wReserved1                                     uint16
}

// structTimeouts 是 Windows COMMTIMEOUTS 结构的 Go 表示。
// 用于设置串口读写超时参数，控制阻塞和非阻塞行为。
type structTimeouts struct {
	ReadIntervalTimeout         uint32
	ReadTotalTimeoutMultiplier  uint32
	ReadTotalTimeoutConstant    uint32
	WriteTotalTimeoutMultiplier uint32
	WriteTotalTimeoutConstant   uint32
}

// openPort 在 Windows 系统上打开一个串行端口。
//
// 参数：
//   - name: 串口设备名称，如 "COM5" 或 "\\\\.\\COM5"
//   - baud: 波特率，支持标准值
//   - databits: 数据位大小，支持 5、6、7、8
//   - parity: 奇偶校验类型，使用 Parity 常量
//   - stopbits: 停止位类型，使用 StopBits 常量
//   - readTimeout: 读取超时时间，影响阻塞行为
//
// 返回值：
//   - *Port: 打开的串口端口实例
//   - error: 如果打开或配置失败，返回错误信息
//
// 实现细节：
//   - 自动为 COM 端口添加 "\\\\.\\" 前缀以支持大于 COM9 的端口
//   - 使用重叠 I/O 模式打开文件（FILE_FLAG_OVERLAPPED）
//   - 配置 DCB、缓冲区大小、超时和事件掩码
//   - 创建读/写重叠结构用于异步操作
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

	if len(name) > 0 && name[0] != '\\' {
		name = "\\\\.\\" + name
	}
	namePtr, _ := syscall.UTF16PtrFromString(name)
	h, err := syscall.CreateFile(namePtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OVERLAPPED,
		0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), name)
	defer func() {
		if err != nil && f != nil {
			f.Close()
		}
	}()
	if err = setCommState(h, baud, databits, parityVal, stopbitsVal); err != nil {
		return nil, err
	}
	if err = setupComm(h, 64, 64); err != nil {
		return nil, err
	}
	if err = setCommTimeouts(h, config.ByteTimeout); err != nil {
		return nil, err
	}
	if err = setCommMask(h); err != nil {
		return nil, err
	}
	ro, err := newOverlapped()
	if err != nil {
		return nil, err
	}
	wo, err := newOverlapped()
	if err != nil {
		return nil, err
	}
	port := new(Port)
	port.f = f
	port.fd = h
	port.ro = ro
	port.wo = wo
	return port, nil
}

// Close 关闭串行端口并释放所有资源。
// 关闭后，任何未完成的读写操作将返回错误。
// 重复关闭是安全的，不会导致 panic。
func (p *Port) Close() error {
	return p.f.Close()
}

// Write 将字节切片中的数据写入串口。
// 使用重叠 I/O 实现异步写入，提高并发性能。
// 返回写入的字节数和可能的错误。
// 如果写入部分数据后发生错误，返回的 n 可能小于 len(buf)。
func (p *Port) Write(buf []byte) (int, error) {
	p.wl.Lock()
	defer p.wl.Unlock()
	if err := resetEvent(p.wo.HEvent); err != nil {
		return 0, err
	}
	var n uint32
	err := syscall.WriteFile(p.fd, buf, &n, p.wo)
	if err != nil && err != syscall.ERROR_IO_PENDING {
		return int(n), err
	}
	return getOverlappedResult(p.fd, p.wo)
}

// Read 从串口读取数据到字节切片中。
// 使用重叠 I/O 实现异步读取，提高并发性能。
// 返回读取的字节数和可能的错误（如超时或连接断开）。
func (p *Port) Read(buf []byte) (int, error) {
	if p == nil || p.f == nil {
		return 0, fmt.Errorf("Invalid port on read")
	}
	p.rl.Lock()
	defer p.rl.Unlock()
	if err := resetEvent(p.ro.HEvent); err != nil {
		return 0, err
	}
	var done uint32
	err := syscall.ReadFile(p.fd, buf, &done, p.ro)
	if err != nil && err != syscall.ERROR_IO_PENDING {
		return int(done), err
	}
	return getOverlappedResult(p.fd, p.ro)
}

// Flush 清空串口的输入和输出缓冲区。
// 使用 PurgeComm API 丢弃所有未读和未写的数据。
// 常用于恢复通信状态或清除垃圾数据。
func (p *Port) Flush() error {
	return purgeComm(p.fd)
}

// 以下变量存储 kernel32.dll 中相关函数的地址。
// 在 init 函数中动态加载，避免硬编码系统调用号。
var (
	nSetCommState,
	nSetCommTimeouts,
	nSetCommMask,
	nSetupComm,
	nGetOverlappedResult,
	nCreateEvent,
	nResetEvent,
	nPurgeComm,
	nFlushFileBuffers uintptr
)

// init 初始化 Windows API 函数地址。
// 在包加载时动态加载 kernel32.dll 并获取所需函数的地址。
// 如果加载失败，程序将 panic，因为串口功能无法正常工作。
func init() {
	k32, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		panic("LoadLibrary " + err.Error())
	}
	defer syscall.FreeLibrary(k32)
	nSetCommState = getProcAddr(k32, "SetCommState")
	nSetCommTimeouts = getProcAddr(k32, "SetCommTimeouts")
	nSetCommMask = getProcAddr(k32, "SetCommMask")
	nSetupComm = getProcAddr(k32, "SetupComm")
	nGetOverlappedResult = getProcAddr(k32, "GetOverlappedResult")
	nCreateEvent = getProcAddr(k32, "CreateEventW")
	nResetEvent = getProcAddr(k32, "ResetEvent")
	nPurgeComm = getProcAddr(k32, "PurgeComm")
	nFlushFileBuffers = getProcAddr(k32, "FlushFileBuffers")
}

// getProcAddr 从动态链接库中获取函数地址。
// 如果获取失败，程序将 panic，因为相应的串口操作无法执行。
func getProcAddr(lib syscall.Handle, name string) uintptr {
	addr, err := syscall.GetProcAddress(lib, name)
	if err != nil {
		panic(name + " " + err.Error())
	}
	return addr
}

// setCommState 设置串口通信状态
func setCommState(h syscall.Handle, baud int, databits byte, parity Parity, stopbits StopBits) error {
	var params structDCB
	params.DCBlength = uint32(unsafe.Sizeof(params))
	params.flags[0] = 0x01  // fBinary - 二进制模式
	params.flags[0] |= 0x10 // Assert DSR - 断言DSR信号
	params.BaudRate = uint32(baud)
	params.ByteSize = databits
	switch parity {
	case ParityNone:
		params.Parity = 0
	case ParityOdd:
		params.Parity = 1
	case ParityEven:
		params.Parity = 2
	case ParityMark:
		params.Parity = 3
	case ParitySpace:
		params.Parity = 4
	default:
		return ErrBadParity
	}
	switch stopbits {
	case Stop1:
		params.StopBits = 0
	case Stop1Half:
		params.StopBits = 1
	case Stop2:
		params.StopBits = 2
	default:
		return ErrBadStopBits
	}
	r, _, err := syscall.SyscallN(nSetCommState, 2, uintptr(h), uintptr(unsafe.Pointer(&params)), 0)
	if r == 0 {
		return err
	}
	return nil
}

// setCommTimeouts 设置串口超时参数
func setCommTimeouts(h syscall.Handle, byteTimeout uint8) error {
	var timeouts structTimeouts
	const MAXDWORD = 1<<32 - 1
	// 默认阻塞读取（无限等待）
	var timeoutMs uint32 = MAXDWORD - 1
	if byteTimeout > 0 {
		// 非阻塞读取，使用字节超时时间（毫秒）
		timeoutMs = uint32(byteTimeout)
	}
	timeouts.ReadIntervalTimeout = MAXDWORD
	timeouts.ReadTotalTimeoutMultiplier = MAXDWORD
	timeouts.ReadTotalTimeoutConstant = timeoutMs
	r, _, err := syscall.SyscallN(nSetCommTimeouts, 2, uintptr(h), uintptr(unsafe.Pointer(&timeouts)), 0)
	if r == 0 {
		return err
	}
	return nil
}

// setupComm 设置串口输入输出缓冲区大小
func setupComm(h syscall.Handle, in, out int) error {
	r, _, err := syscall.SyscallN(nSetupComm, 3, uintptr(h), uintptr(in), uintptr(out))
	if r == 0 {
		return err
	}
	return nil
}

// setCommMask 设置串口事件掩码
func setCommMask(h syscall.Handle) error {
	const EV_RXCHAR = 0x0001 // 接收字符事件
	r, _, err := syscall.SyscallN(nSetCommMask, 2, uintptr(h), EV_RXCHAR, 0)
	if r == 0 {
		return err
	}
	return nil
}

// resetEvent 重置事件对象
func resetEvent(h syscall.Handle) error {
	r, _, err := syscall.SyscallN(nResetEvent, 1, uintptr(h), 0, 0)
	if r == 0 {
		return err
	}
	return nil
}

// purgeComm 清除串口缓冲区
func purgeComm(h syscall.Handle) error {
	const PURGE_TXABORT = 0x0001 // 中止所有挂起的写入操作
	const PURGE_RXABORT = 0x0002 // 中止所有挂起的读取操作
	const PURGE_TXCLEAR = 0x0004 // 清除发送缓冲区
	const PURGE_RXCLEAR = 0x0008 // 清除接收缓冲区
	r, _, err := syscall.SyscallN(nPurgeComm, 2, uintptr(h),
		PURGE_TXABORT|PURGE_RXABORT|PURGE_TXCLEAR|PURGE_RXCLEAR, 0)
	if r == 0 {
		return err
	}
	return nil
}

// newOverlapped 创建新的重叠I/O结构
func newOverlapped() (*syscall.Overlapped, error) {
	var overlapped syscall.Overlapped
	r, _, err := syscall.SyscallN(nCreateEvent, 4, 0, 1, 0, 0, 0, 0)
	if r == 0 {
		return nil, err
	}
	overlapped.HEvent = syscall.Handle(r)
	return &overlapped, nil
}

// getOverlappedResult 获取重叠I/O操作结果
func getOverlappedResult(h syscall.Handle, overlapped *syscall.Overlapped) (int, error) {
	var n int
	r, _, err := syscall.SyscallN(nGetOverlappedResult, 4,
		uintptr(h),
		uintptr(unsafe.Pointer(overlapped)),
		uintptr(unsafe.Pointer(&n)), 1, 0, 0)
	if r == 0 {
		return n, err
	}
	return n, nil
}
