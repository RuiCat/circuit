package can

import (
	"bytes"
	"circuit/gpio/driver"
	"errors"
	"time"
)

// CANDriver 实现CAN接口的结构体
type CANDriver struct {
	uart driver.UART       // 底层UART接口
	cfg  *driver.CANConfig // CAN配置
}

// getProtocol 获取协议配置，如果为空则返回默认配置
func (d *CANDriver) getProtocol() *driver.CANProtocolConfig {
	if d.cfg.Protocol != nil {
		return d.cfg.Protocol
	}
	// 返回默认的二进制协议配置
	return CANProtocolBinary()
}

// replacePlaceholder 替换命令模板中的占位符
func replacePlaceholder(template []byte, placeholder string, value []byte) []byte {
	placeholderBytes := []byte(placeholder)
	if !bytes.Contains(template, placeholderBytes) {
		return template
	}
	return bytes.ReplaceAll(template, placeholderBytes, value)
}

// OpenCAN 打开CAN设备并返回CAN接口
func OpenCAN(uart driver.UART, cfg *driver.CANConfig) (driver.CAN, error) {
	if uart == nil {
		return nil, errors.New("UART interface cannot be nil")
	}
	if cfg == nil {
		return nil, errors.New("CAN config cannot be nil")
	}
	driver := &CANDriver{
		uart: uart,
		cfg:  cfg,
	}
	// 初始化CAN接口
	err := driver.initCAN()
	if err != nil {
		return nil, err
	}
	return driver, nil
}

// initCAN 内部初始化CAN接口
func (d *CANDriver) initCAN() error {
	protocol := d.getProtocol()
	if len(protocol.InitCmd) == 0 {
		// 不需要初始化命令
		return nil
	}
	return d.uart.Write(protocol.InitCmd)
}

// NewCANDriver 创建新的CAN驱动实例
func NewCANDriver(uart driver.UART, cfg *driver.CANConfig) (driver.CAN, error) {
	if uart == nil {
		return nil, errors.New("UART interface cannot be nil")
	}
	if cfg == nil {
		return nil, errors.New("CAN config cannot be nil")
	}
	return &CANDriver{
		uart: uart,
		cfg:  cfg,
	}, nil
}

// Close 关闭设备
func (d *CANDriver) Close() error {
	return d.uart.Close()
}

// Init 初始化UART配置
func (d *CANDriver) Init(baudRate int, byteSize, parity, stopBits, byteTimeout uint8) error {
	return d.uart.Init(baudRate, byteSize, parity, stopBits, byteTimeout)
}

// GetConfig 获取UART配置
func (d *CANDriver) GetConfig() (*driver.UARTConfig, error) {
	return d.uart.GetConfig()
}

// Read 从UART读取数据
func (d *CANDriver) Read(length int) ([]byte, error) {
	return d.uart.Read(length)
}

// Write 向UART写入数据
func (d *CANDriver) Write(data []byte) error {
	return d.uart.Write(data)
}

// InitCAN 初始化CAN接口
func (d *CANDriver) InitCAN(uart driver.UART, cfg *driver.CANConfig) error {
	if uart == nil {
		return errors.New("UART interface cannot be nil")
	}
	if cfg == nil {
		return errors.New("CAN config cannot be nil")
	}
	// 如果已经有UART实例，先关闭旧的
	if d.uart != nil {
		d.uart.Close()
	}
	d.uart = uart
	d.cfg = cfg
	// 使用协议配置中的初始化命令
	return d.initCAN()
}

// SendFrame 发送CAN帧
func (d *CANDriver) SendFrame(frame *driver.CANFrame) error {
	if frame == nil {
		return errors.New("CAN frame cannot be nil")
	}
	protocol := d.getProtocol()
	// 优先使用解析函数
	if protocol.ParseSendFrame != nil {
		cmd := protocol.ParseSendFrame(frame)
		return d.uart.Write(cmd)
	}
	// 回退到模板逻辑
	if len(protocol.SendFrameCmd) == 0 {
		// 使用默认二进制协议的发送命令
		protocol = CANProtocolBinary()
	}
	// 复制命令模板
	cmd := make([]byte, len(protocol.SendFrameCmd))
	copy(cmd, protocol.SendFrameCmd)
	// 替换帧ID占位符
	idBytes := []byte{
		byte(frame.ID >> 24),
		byte(frame.ID >> 16),
		byte(frame.ID >> 8),
		byte(frame.ID),
	}
	cmd = replacePlaceholder(cmd, "{ID}", idBytes)
	// 替换扩展帧标志
	extByte := []byte{0}
	if frame.Extended {
		extByte[0] = 1
	}
	cmd = replacePlaceholder(cmd, "{EXT}", extByte)
	// 替换远程帧标志
	rtrByte := []byte{0}
	if frame.Remote {
		rtrByte[0] = 1
	}
	cmd = replacePlaceholder(cmd, "{RTR}", rtrByte)
	// 替换属性字节（合并EXT和RTR标志）
	var attrByte byte
	if frame.Extended {
		attrByte |= 0x80
	}
	if frame.Remote {
		attrByte |= 0x40
	}
	attrByte |= frame.DLC & 0x0F
	cmd = replacePlaceholder(cmd, "{ATTR}", []byte{attrByte})
	// 替换数据长度
	dlcByte := []byte{frame.DLC & 0x0F}
	cmd = replacePlaceholder(cmd, "{DLC}", dlcByte)
	// 替换数据（如果有）
	if !frame.Remote && frame.DLC > 0 {
		cmd = replacePlaceholder(cmd, "{DATA}", frame.Data[:frame.DLC])
	}
	return d.uart.Write(cmd)
}

// ReceiveFrame 接收CAN帧
func (d *CANDriver) ReceiveFrame(timeout uint32) (*driver.CANFrame, error) {
	protocol := d.getProtocol()
	// 优先使用解析函数
	if protocol.ParseReceiveFrame != nil {
		// 读取原始数据
		data, err := d.uart.Read(256) // 假设最大帧长度
		if err != nil {
			return nil, err
		}
		// 使用解析函数处理原始数据
		return protocol.ParseReceiveFrame(data)
	}
	// 如果没有协议配置，使用默认解析
	if len(protocol.FrameStartMarker) == 0 {
		// 简单读取和解析
		data, err := d.uart.Read(256) // 假设最大帧长度
		if err != nil {
			return nil, err
		}
		return d.parseCANPacket(data)
	}
	// 基于协议配置的帧接收逻辑
	// 首先读取直到找到帧起始标记
	markerLen := len(protocol.FrameStartMarker)
	// 简化实现：读取足够的数据
	data, err := d.uart.Read(256)
	if err != nil {
		return nil, err
	}
	// 查找帧起始标记
	startIdx := bytes.Index(data, protocol.FrameStartMarker)
	if startIdx == -1 {
		return nil, errors.New("frame start marker not found")
	}
	// 提取帧数据（从起始标记开始）
	frameData := data[startIdx:]
	// 检查最小长度
	minLen := protocol.FrameMinLength
	if minLen <= 0 {
		minLen = 6 // 默认最小长度
	}
	if len(frameData) < minLen {
		// 读取更多数据
		moreData, err := d.uart.Read(minLen - len(frameData))
		if err != nil {
			return nil, err
		}
		frameData = append(frameData, moreData...)
	}
	// 解析帧数据（跳过起始标记）
	framePayload := frameData[markerLen:]
	return d.parseCANPacket(framePayload)
}

// SetFilter 设置CAN过滤器
func (d *CANDriver) SetFilter(filterID, filterMask uint32, enable bool) error {
	protocol := d.getProtocol()
	// 优先使用解析函数
	if protocol.ParseSetFilter != nil {
		cmd := protocol.ParseSetFilter(filterID, filterMask, enable)
		if len(cmd) == 0 {
			return errors.New("ParseSetFilter returned empty command")
		}
		return d.uart.Write(cmd)
	}
	// 回退到模板逻辑
	if len(protocol.SetFilterCmd) == 0 {
		return errors.New("SetFilter command not defined in protocol")
	}
	// 复制命令模板
	cmd := make([]byte, len(protocol.SetFilterCmd))
	copy(cmd, protocol.SetFilterCmd)
	// 替换占位符
	idBytes := []byte{
		byte(filterID >> 24),
		byte(filterID >> 16),
		byte(filterID >> 8),
		byte(filterID),
	}
	maskBytes := []byte{
		byte(filterMask >> 24),
		byte(filterMask >> 16),
		byte(filterMask >> 8),
		byte(filterMask),
	}
	cmd = replacePlaceholder(cmd, "{ID}", idBytes)
	cmd = replacePlaceholder(cmd, "{MASK}", maskBytes)
	return d.uart.Write(cmd)
}

// GetStatus 获取CAN状态
func (d *CANDriver) GetStatus() (uint32, error) {
	protocol := d.getProtocol()
	if len(protocol.GetStatusCmd) == 0 {
		return 0, errors.New("GetStatus command not defined in protocol")
	}
	err := d.uart.Write(protocol.GetStatusCmd)
	if err != nil {
		return 0, err
	}
	// 读取状态响应
	respLength := protocol.StatusRespLength
	if respLength <= 0 {
		respLength = 4 // 默认长度
	}
	resp, err := d.uart.Read(respLength)
	if err != nil {
		return 0, err
	}
	// 优先使用解析函数
	if protocol.ParseStatusResponse != nil {
		return protocol.ParseStatusResponse(resp)
	}
	// 默认解析
	if len(resp) >= 4 {
		status := uint32(resp[0])<<24 | uint32(resp[1])<<16 | uint32(resp[2])<<8 | uint32(resp[3])
		return status, nil
	}
	return 0, errors.New("invalid status response")
}

// SetMode 设置CAN工作模式
func (d *CANDriver) SetMode(mode uint8) error {
	if mode > 2 {
		return errors.New("invalid CAN mode")
	}
	protocol := d.getProtocol()
	// 优先使用解析函数
	if protocol.ParseSetMode != nil {
		cmd := protocol.ParseSetMode(mode)
		if len(cmd) == 0 {
			return errors.New("ParseSetMode returned empty command")
		}
		return d.uart.Write(cmd)
	}
	// 回退到模板逻辑
	if len(protocol.SetModeCmd) == 0 {
		return errors.New("SetMode command not defined in protocol")
	}
	// 复制命令模板
	cmd := make([]byte, len(protocol.SetModeCmd))
	copy(cmd, protocol.SetModeCmd)
	// 替换模式占位符
	modeByte := []byte{byte(mode)}
	cmd = replacePlaceholder(cmd, "{MODE}", modeByte)
	return d.uart.Write(cmd)
}

// ClearErrors 清除CAN错误计数器
func (d *CANDriver) ClearErrors() error {
	protocol := d.getProtocol()
	if len(protocol.ClearErrorsCmd) == 0 {
		return errors.New("ClearErrors command not defined in protocol")
	}
	return d.uart.Write(protocol.ClearErrorsCmd)
}

// GetErrorCounters 获取CAN错误计数器
func (d *CANDriver) GetErrorCounters() (txErr, rxErr uint32, err error) {
	protocol := d.getProtocol()
	if len(protocol.GetErrorCountersCmd) == 0 {
		return 0, 0, errors.New("GetErrorCounters command not defined in protocol")
	}
	err = d.uart.Write(protocol.GetErrorCountersCmd)
	if err != nil {
		return 0, 0, err
	}
	// 读取错误计数器响应
	respLength := protocol.ErrorCountersRespLength
	if respLength <= 0 {
		respLength = 8 // 默认长度
	}
	resp, err := d.uart.Read(respLength)
	if err != nil {
		return 0, 0, err
	}
	// 优先使用解析函数
	if protocol.ParseErrorCountersResponse != nil {
		return protocol.ParseErrorCountersResponse(resp)
	}
	// 默认解析
	if len(resp) >= 8 {
		txErr = uint32(resp[0])<<24 | uint32(resp[1])<<16 | uint32(resp[2])<<8 | uint32(resp[3])
		rxErr = uint32(resp[4])<<24 | uint32(resp[5])<<16 | uint32(resp[6])<<8 | uint32(resp[7])
		return txErr, rxErr, nil
	}
	return 0, 0, errors.New("invalid error counters response")
}

// SetBaudRate 设置CAN波特率
func (d *CANDriver) SetBaudRate(baudRate uint32) error {
	protocol := d.getProtocol()
	// 优先使用解析函数
	if protocol.ParseSetBaudRate != nil {
		cmd := protocol.ParseSetBaudRate(baudRate)
		if len(cmd) == 0 {
			return errors.New("ParseSetBaudRate returned empty command")
		}
		return d.uart.Write(cmd)
	}
	// 回退到模板逻辑
	if len(protocol.SetBaudRateCmd) == 0 {
		return errors.New("SetBaudRate command not defined in protocol")
	}
	// 复制命令模板
	cmd := make([]byte, len(protocol.SetBaudRateCmd))
	copy(cmd, protocol.SetBaudRateCmd)
	// 替换波特率占位符
	baudBytes := []byte{
		byte(baudRate >> 24),
		byte(baudRate >> 16),
		byte(baudRate >> 8),
		byte(baudRate),
	}
	cmd = replacePlaceholder(cmd, "{BAUDRATE}", baudBytes)
	return d.uart.Write(cmd)
}

// buildCANPacket 构建CAN帧数据包（使用默认二进制协议模板）
func (d *CANDriver) buildCANPacket(frame *driver.CANFrame) []byte {
	// 使用默认二进制协议的发送帧模板
	protocol := CANProtocolBinary()
	if len(protocol.SendFrameCmd) == 0 {
		// 如果默认协议也没有发送命令，返回空
		return nil
	}
	// 复制命令模板
	cmd := make([]byte, len(protocol.SendFrameCmd))
	copy(cmd, protocol.SendFrameCmd)
	// 替换帧ID占位符
	idBytes := []byte{
		byte(frame.ID >> 24),
		byte(frame.ID >> 16),
		byte(frame.ID >> 8),
		byte(frame.ID),
	}
	cmd = replacePlaceholder(cmd, "{ID}", idBytes)
	// 替换属性字节
	var attrByte byte
	if frame.Extended {
		attrByte |= 0x80
	}
	if frame.Remote {
		attrByte |= 0x40
	}
	attrByte |= frame.DLC & 0x0F
	cmd = replacePlaceholder(cmd, "{ATTR}", []byte{attrByte})
	// 替换数据长度
	dlcByte := []byte{frame.DLC & 0x0F}
	cmd = replacePlaceholder(cmd, "{DLC}", dlcByte)
	// 替换数据（如果有）
	if !frame.Remote && frame.DLC > 0 {
		cmd = replacePlaceholder(cmd, "{DATA}", frame.Data[:frame.DLC])
	}
	return cmd
}

// parseCANPacket 解析CAN帧数据包
func (d *CANDriver) parseCANPacket(data []byte) (*driver.CANFrame, error) {
	if len(data) < 6 {
		return nil, errors.New("invalid CAN packet length")
	}
	// 简单解析，实际需要根据协议
	frame := &driver.CANFrame{}
	// 解析帧ID
	if len(data) >= 4 {
		frame.ID = uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	}
	// 解析帧属性
	if len(data) >= 5 {
		flags := data[4]
		frame.Extended = (flags & 0x80) != 0
		frame.Remote = (flags & 0x40) != 0
		frame.DLC = flags & 0x0F
	}
	// 解析数据
	if len(data) >= 6 && !frame.Remote && frame.DLC > 0 {
		dataStart := 5
		dataEnd := dataStart + int(frame.DLC)
		if dataEnd <= len(data) {
			copy(frame.Data[:], data[dataStart:dataEnd])
		}
	}
	frame.Timestamp = uint64(time.Now().UnixNano())
	return frame, nil
}
