// Package serial 提供跨平台的串行端口访问功能。
//
// 该包支持 Windows、Linux 和 POSIX 系统（如 macOS），提供统一的 API 接口。
// 支持基本的串口配置，包括波特率、数据位、停止位、校验位和超时设置。
//
// 主要功能：
//   - 打开和关闭串行端口
//   - 读取和写入数据
//   - 支持阻塞和非阻塞操作
//   - 实现 driver.UART 接口，可与 gogio/driver 系统集成
//   - 支持 RS232、RS422、RS485 等串行接口
//
// 示例用法：
//
//	config := &driver.UARTConfig{
//	    BaudRate: 115200,
//	    ByteSize: 8,
//	    Parity: 0,
//	    StopBits: 0,
//	    ByteTimeout: 10,
//	}
//	port, err := serial.OpenPort("COM5", config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer port.Close()
//	n, err := port.Write([]byte("test"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	buf := make([]byte, 128)
//	n, err = port.Read(buf)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Received: %q\n", buf[:n])
//
// 高级用法（使用 driver.UART 接口）：
//
//	uart, err := serial.NewUART(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer uart.Close()
//	err = uart.Init(9600, 8, 0, 0, 10) // 动态重配置
//	if err != nil {
//	    log.Fatal(err)
//	}
//	err = uart.Write([]byte("Hello"))
//	if err != nil {
//	    log.Fatal(err)
//	}
package serial

import (
	"circuit/gpio/driver"
	"errors"
	"sync"
)

// DefaultSize 是 Config.Size 字段的默认值。
// 当 Config.Size 为 0 时，将使用此值（8 个数据位）。
const DefaultSize = 8

// StopBits 表示串行通信中使用的停止位数量。
// 停止位是每个字符传输结束时发送的位，用于表示字符结束。
type StopBits byte

// Parity 表示串行通信中使用的奇偶校验类型。
// 奇偶校验是一种简单的错误检测方法，通过添加一个校验位来确保数据位中1的个数为奇数或偶数。
type Parity byte

// 停止位常量定义
const (
	// Stop1 表示使用 1 个停止位
	Stop1 StopBits = 1
	// Stop1Half 表示使用 1.5 个停止位（某些系统支持）
	Stop1Half StopBits = 15
	// Stop2 表示使用 2 个停止位
	Stop2 StopBits = 2
)

// 奇偶校验常量定义
const (
	// ParityNone 表示不使用奇偶校验
	ParityNone Parity = 'N'
	// ParityOdd 表示使用奇校验
	ParityOdd Parity = 'O'
	// ParityEven 表示使用偶校验
	ParityEven Parity = 'E'
	// ParityMark 表示校验位始终为 1（标记校验）
	ParityMark Parity = 'M'
	// ParitySpace 表示校验位始终为 0（空格校验）
	ParitySpace Parity = 'S'
)

// Config 包含打开串行端口所需的信息。
//
// 该结构体基于 driver.UARTConfig，提供完整的串口配置选项。
// 使用 driver.UART 接口可以动态修改配置。
//
// 例如：
//
//	uart, err := serial.NewUART("COM5", &driver.UARTConfig{
//	    BaudRate: 9600,
//	    ByteSize: 8,
//	    Parity: 0,
//	    StopBits: 0,
//	    ByteTimeout: 10,
//	})

// ErrBadSize 在 Size 不受支持时返回。
var ErrBadSize error = errors.New("unsupported serial data size")

// ErrBadStopBits 在指定的 StopBits 设置不受支持时返回。
var ErrBadStopBits error = errors.New("unsupported stop bit setting")

// ErrBadParity 在校验位不受支持时返回。
var ErrBadParity error = errors.New("unsupported parity setting")

// OpenPort 使用指定的配置打开一个串行端口
func OpenPort(name string, config *driver.UARTConfig) (*Port, error) {
	if config == nil {
		return nil, errors.New("serial: config cannot be nil")
	}
	return openPort(name, config)
}

// uartDriver 是 driver.UART 接口的实现，包装了底层的 serial.Port。
// 它提供了线程安全的串口操作，支持动态配置和状态管理。
//
// 字段说明：
//   - mu: 互斥锁，确保并发访问的安全性
//   - port: 底层的串口端口实例
//   - portName: 串口设备名称
//   - config: 当前端口配置，使用 driver.UARTConfig 格式
type uartDriver struct {
	mu       sync.Mutex
	port     *Port
	portName string
	config   *driver.UARTConfig
}

// NewUART 创建一个新的 UART 接口实例。
//
// 参数：
//   - portName: 串口设备名称，如 "COM5" (Windows) 或 "/dev/ttyUSB0" (Linux/POSIX)
//   - config: UART配置，包含波特率、数据位、停止位、校验位等设置
//
// 返回值：
//   - driver.UART: 符合 driver.UART 接口的串口实例
//   - error: 如果打开端口失败，返回错误信息
//
// 注意事项：
//   - 如果配置中的 ByteSize 为 0，将使用 DefaultSize (8)
//   - 如果配置中的 Parity 为 0，将使用无校验
//   - 如果配置中的 StopBits 为 0，将使用 1 个停止位
//   - 创建成功后，可以调用 Init 方法动态修改配置
//   - 使用完成后必须调用 Close 方法释放资源
func NewUART(portName string, config *driver.UARTConfig) (driver.UART, error) {
	if config == nil {
		return nil, errors.New("serial: config cannot be nil")
	}

	// 验证必要的配置参数
	if portName == "" {
		return nil, errors.New("serial: port name is required")
	}
	if config.BaudRate == 0 {
		return nil, errors.New("serial: baud rate must be positive")
	}

	port, err := OpenPort(portName, config)
	if err != nil {
		return nil, err
	}
	return &uartDriver{port: port, config: config, portName: portName}, nil
}

// Close 关闭 UART 设备
func (u *uartDriver) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.port == nil {
		return errors.New("serial: port already closed")
	}
	err := u.port.Close()
	u.port = nil
	u.config = nil
	return err
}

// Init 初始化 UART 配置
func (u *uartDriver) Init(baudRate int, byteSize, parity, stopBits, byteTimeout uint8) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	// 创建新的配置
	newConfig := &driver.UARTConfig{
		BaudRate:    uint32(baudRate),
		ByteSize:    byteSize,
		Parity:      parity,
		StopBits:    stopBits,
		ByteTimeout: byteTimeout,
	}

	// 检查配置是否与当前相同
	if u.config != nil {
		if u.config.BaudRate == newConfig.BaudRate &&
			u.config.ByteSize == newConfig.ByteSize &&
			u.config.Parity == newConfig.Parity &&
			u.config.StopBits == newConfig.StopBits &&
			u.config.ByteTimeout == newConfig.ByteTimeout {
			// 配置相同，无需更改
			return nil
		}
	}

	// 尝试用新配置打开端口
	newPort, err := OpenPort(u.portName, newConfig)
	if err != nil {
		return err
	}

	// 成功打开新端口，关闭旧端口
	oldPort := u.port
	u.port = newPort
	u.config = newConfig

	// 关闭旧端口（忽略错误，因为新端口已打开）
	if oldPort != nil {
		oldPort.Close()
	}
	return nil
}

// GetConfig 获取 UART 配置
func (u *uartDriver) GetConfig() (*driver.UARTConfig, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.config == nil {
		return nil, errors.New("serial: port not configured")
	}

	// 创建配置的副本以避免外部修改影响内部状态
	configCopy := *u.config
	return &configCopy, nil
}

// Read 从 UART 读取数据
func (u *uartDriver) Read(length int) ([]byte, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.port == nil {
		return nil, errors.New("serial: port closed")
	}
	buf := make([]byte, length)
	n, err := u.port.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// Write 向 UART 写入数据
func (u *uartDriver) Write(data []byte) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.port == nil {
		return errors.New("serial: port closed")
	}
	_, err := u.port.Write(data)
	return err
}
