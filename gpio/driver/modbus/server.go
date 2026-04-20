// Package modbus 提供基于 driver.UART 接口的 Modbus RTU 协议实现。
//
// 该文件实现了 Modbus RTU 服务器（从站）功能。
// 有关主站功能，请参见 master.go。
//
// Server 支持的功能码与 Master 对应：
//   - 读线圈 (0x01)
//   - 读离散输入 (0x02)
//   - 读保持寄存器 (0x03)
//   - 读输入寄存器 (0x04)
//   - 写单个线圈 (0x05)
//   - 写单个寄存器 (0x06)
//   - 写多个线圈 (0x0F)
//   - 写多个寄存器 (0x10)
//
// 示例用法：
//
//	// 打开UART设备
//	uart, err := ch34x.OpenUART("/dev/ch34x_pis0")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer uart.Close()
//
//	// 创建Modbus服务器（从站地址为1）
//	server := modbus.NewServer(uart, 1)
//
//	// 可选：设置自定义处理器（如果不设置，使用默认内存存储）
//	// customHandler := &MyCustomHandler{}
//	// server.SetHandler(customHandler)
//
//	// 启动服务器开始监听请求
//	if err := server.Start(); err != nil {
//	    log.Fatal(err)
//	}
//	defer server.Stop()
//
//	// 服务器现在在后台运行，处理来自主站的请求
//	// 可以通过自定义处理器实现业务逻辑
package modbus

import (
	"circuit/gpio/driver"
	"errors"
	"sync"
	"time"
)

// Server 表示一个 Modbus RTU 服务器（从站）
// 用于监听并响应来自 Modbus 主站的请求
// 支持 Modbus RTU 协议标准功能码
type Server struct {
	uart             driver.UART    // 底层的 UART 接口，用于串口通信
	slaveID          uint8          // 本从站地址，范围 1-247，0 为广播地址
	timeout          time.Duration  // 读取超时时间，影响帧间隔检测
	stopChan         chan struct{}  // 停止信号通道，用于优雅停止服务器
	handler          RequestHandler // 请求处理器，处理具体的读写操作
	running          bool           // 服务器运行状态标志
	mu               sync.RWMutex   // 读写锁，保护服务器状态和共享数据
	coils            []bool         // 内部线圈存储（如果使用默认处理器）
	discreteInputs   []bool         // 内部离散输入存储（如果使用默认处理器）
	holdingRegisters []uint16       // 内部保持寄存器存储（如果使用默认处理器）
	inputRegisters   []uint16       // 内部输入寄存器存储（如果使用默认处理器）
}

// RequestHandler 定义请求处理器接口
type RequestHandler interface {
	// HandleReadCoils 处理读线圈请求
	HandleReadCoils(address, quantity uint16) ([]bool, error)
	// HandleReadDiscreteInputs 处理读离散输入请求
	HandleReadDiscreteInputs(address, quantity uint16) ([]bool, error)
	// HandleReadHoldingRegisters 处理读保持寄存器请求
	HandleReadHoldingRegisters(address, quantity uint16) ([]uint16, error)
	// HandleReadInputRegisters 处理读输入寄存器请求
	HandleReadInputRegisters(address, quantity uint16) ([]uint16, error)
	// HandleWriteSingleCoil 处理写单个线圈请求
	HandleWriteSingleCoil(address uint16, value bool) error
	// HandleWriteSingleRegister 处理写单个寄存器请求
	HandleWriteSingleRegister(address uint16, value uint16) error
	// HandleWriteMultipleCoils 处理写多个线圈请求
	HandleWriteMultipleCoils(address uint16, values []bool) error
	// HandleWriteMultipleRegisters 处理写多个寄存器请求
	HandleWriteMultipleRegisters(address uint16, values []uint16) error
}

// DefaultHandler 默认的内存存储处理器
type DefaultHandler struct {
	coils            []bool
	discreteInputs   []bool
	holdingRegisters []uint16
	inputRegisters   []uint16
	maxCoils         int
	maxInputs        int
	maxHoldingRegs   int
	maxInputRegs     int
	mu               sync.RWMutex
}

// NewDefaultHandler 创建默认处理器
func NewDefaultHandler(maxCoils, maxInputs, maxHoldingRegs, maxInputRegs int) *DefaultHandler {
	return &DefaultHandler{
		coils:            make([]bool, maxCoils),
		discreteInputs:   make([]bool, maxInputs),
		holdingRegisters: make([]uint16, maxHoldingRegs),
		inputRegisters:   make([]uint16, maxInputRegs),
		maxCoils:         maxCoils,
		maxInputs:        maxInputs,
		maxHoldingRegs:   maxHoldingRegs,
		maxInputRegs:     maxInputRegs,
	}
}

// HandleReadCoils 处理读线圈请求
func (h *DefaultHandler) HandleReadCoils(address, quantity uint16) ([]bool, error) {
	if int(address)+int(quantity) > h.maxCoils {
		return nil, &Exception{Code: ExceptionIllegalDataAddress}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]bool, quantity)
	copy(result, h.coils[address:address+quantity])
	return result, nil
}

// HandleReadDiscreteInputs 处理读离散输入请求
func (h *DefaultHandler) HandleReadDiscreteInputs(address, quantity uint16) ([]bool, error) {
	if int(address)+int(quantity) > h.maxInputs {
		return nil, &Exception{Code: ExceptionIllegalDataAddress}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]bool, quantity)
	copy(result, h.discreteInputs[address:address+quantity])
	return result, nil
}

// HandleReadHoldingRegisters 处理读保持寄存器请求
func (h *DefaultHandler) HandleReadHoldingRegisters(address, quantity uint16) ([]uint16, error) {
	if int(address)+int(quantity) > h.maxHoldingRegs {
		return nil, &Exception{Code: ExceptionIllegalDataAddress}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]uint16, quantity)
	copy(result, h.holdingRegisters[address:address+quantity])
	return result, nil
}

// HandleReadInputRegisters 处理读输入寄存器请求
func (h *DefaultHandler) HandleReadInputRegisters(address, quantity uint16) ([]uint16, error) {
	if int(address)+int(quantity) > h.maxInputRegs {
		return nil, &Exception{Code: ExceptionIllegalDataAddress}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]uint16, quantity)
	copy(result, h.inputRegisters[address:address+quantity])
	return result, nil
}

// HandleWriteSingleCoil 处理写单个线圈请求
func (h *DefaultHandler) HandleWriteSingleCoil(address uint16, value bool) error {
	if int(address) >= h.maxCoils {
		return &Exception{Code: ExceptionIllegalDataAddress}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.coils[address] = value
	return nil
}

// HandleWriteSingleRegister 处理写单个寄存器请求
func (h *DefaultHandler) HandleWriteSingleRegister(address uint16, value uint16) error {
	if int(address) >= h.maxHoldingRegs {
		return &Exception{Code: ExceptionIllegalDataAddress}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.holdingRegisters[address] = value
	return nil
}

// HandleWriteMultipleCoils 处理写多个线圈请求
func (h *DefaultHandler) HandleWriteMultipleCoils(address uint16, values []bool) error {
	quantity := uint16(len(values))
	if int(address)+int(quantity) > h.maxCoils {
		return &Exception{Code: ExceptionIllegalDataAddress}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	copy(h.coils[address:address+quantity], values)
	return nil
}

// HandleWriteMultipleRegisters 处理写多个寄存器请求
func (h *DefaultHandler) HandleWriteMultipleRegisters(address uint16, values []uint16) error {
	quantity := uint16(len(values))
	if int(address)+int(quantity) > h.maxHoldingRegs {
		return &Exception{Code: ExceptionIllegalDataAddress}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	copy(h.holdingRegisters[address:address+quantity], values)
	return nil
}

// 异常码常量
const (
	ExceptionIllegalFunction        = 0x01
	ExceptionIllegalDataAddress     = 0x02
	ExceptionIllegalDataValue       = 0x03
	ExceptionServerDeviceFailure    = 0x04
	ExceptionAcknowledge            = 0x05
	ExceptionServerDeviceBusy       = 0x06
	ExceptionMemoryParityError      = 0x08
	ExceptionGatewayPathUnavailable = 0x0A
	ExceptionGatewayTargetDevice    = 0x0B
)

// NewServer 创建一个新的 Modbus 服务器
func NewServer(uart driver.UART, slaveID uint8) *Server {
	return &Server{
		uart:     uart,
		slaveID:  slaveID,
		timeout:  100 * time.Millisecond,
		stopChan: make(chan struct{}),
		handler:  NewDefaultHandler(2000, 2000, 125, 125),
	}
}

// SetHandler 设置请求处理器
func (s *Server) SetHandler(handler RequestHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// SetTimeout 设置超时时间
func (s *Server) SetTimeout(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timeout = timeout
}

// Start 启动服务器，开始监听请求
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("server already running")
	}
	s.running = true
	s.mu.Unlock()

	go s.listenLoop()
	return nil
}

// Stop 停止服务器
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	close(s.stopChan)
	s.running = false
}

// listenLoop 监听循环，处理接收到的请求
func (s *Server) listenLoop() {
	for {
		select {
		case <-s.stopChan:
			return
		default:
			// 读取数据
			data, err := s.uart.Read(256)
			if err != nil {
				continue
			}
			if len(data) == 0 {
				time.Sleep(s.timeout / 10)
				continue
			}

			// 处理接收到的数据
			s.processFrame(data)
		}
	}
}

// processFrame 处理接收到的 Modbus 帧
func (s *Server) processFrame(frame []byte) {
	if len(frame) < 5 {
		return // 帧太短
	}

	// 检查从站地址
	if frame[0] != s.slaveID && frame[0] != 0 {
		return // 不是发给本从站的广播或单播
	}

	// 检查 CRC
	receivedCRC := uint16(frame[len(frame)-2]) | uint16(frame[len(frame)-1])<<8
	expectedCRC := calculateCRC(frame[:len(frame)-2])
	if receivedCRC != expectedCRC {
		return // CRC 错误
	}

	// 解析 PDU
	pdu := frame[1 : len(frame)-2]
	if len(pdu) == 0 {
		return
	}

	// 处理请求并生成响应
	responsePDU := s.handleRequest(pdu)
	if responsePDU == nil {
		return // 不需要响应（广播）
	}

	// 构建响应帧
	responseFrame := make([]byte, 1+len(responsePDU)+2)
	responseFrame[0] = s.slaveID
	copy(responseFrame[1:], responsePDU)
	crc := calculateCRC(responseFrame[:len(responseFrame)-2])
	responseFrame[len(responseFrame)-2] = byte(crc & 0xFF)
	responseFrame[len(responseFrame)-1] = byte(crc >> 8)

	// 发送响应
	s.uart.Write(responseFrame)
}

// handleRequest 处理 Modbus 请求并返回响应 PDU
func (s *Server) handleRequest(pdu []byte) []byte {
	if len(pdu) == 0 {
		return nil
	}

	functionCode := pdu[0]
	var response []byte

	switch functionCode {
	case FuncCodeReadCoils:
		response = s.handleReadCoils(pdu)
	case FuncCodeReadDiscreteInputs:
		response = s.handleReadDiscreteInputs(pdu)
	case FuncCodeReadHoldingRegisters:
		response = s.handleReadHoldingRegisters(pdu)
	case FuncCodeReadInputRegisters:
		response = s.handleReadInputRegisters(pdu)
	case FuncCodeWriteSingleCoil:
		response = s.handleWriteSingleCoil(pdu)
	case FuncCodeWriteSingleRegister:
		response = s.handleWriteSingleRegister(pdu)
	case FuncCodeWriteMultipleCoils:
		response = s.handleWriteMultipleCoils(pdu)
	case FuncCodeWriteMultipleRegisters:
		response = s.handleWriteMultipleRegisters(pdu)
	default:
		// 非法功能码
		response = []byte{functionCode | 0x80, ExceptionIllegalFunction}
	}

	return response
}

// handleReadCoils 处理读线圈请求
func (s *Server) handleReadCoils(pdu []byte) []byte {
	if len(pdu) != 5 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	address := uint16(pdu[1])<<8 | uint16(pdu[2])
	quantity := uint16(pdu[3])<<8 | uint16(pdu[4])

	if quantity < 1 || quantity > 2000 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()

	coils, err := handler.HandleReadCoils(address, quantity)
	if err != nil {
		if e, ok := err.(*Exception); ok {
			return []byte{pdu[0] | 0x80, e.Code}
		}
		return []byte{pdu[0] | 0x80, ExceptionServerDeviceFailure}
	}

	// 构建响应
	byteCount := (quantity + 7) / 8
	response := make([]byte, 2+byteCount)
	response[0] = FuncCodeReadCoils
	response[1] = byte(byteCount)

	// 打包线圈状态
	for i, coil := range coils {
		if coil {
			byteIndex := 2 + i/8
			bitIndex := i % 8
			response[byteIndex] |= 1 << bitIndex
		}
	}

	return response
}

// handleReadDiscreteInputs 处理读离散输入请求
func (s *Server) handleReadDiscreteInputs(pdu []byte) []byte {
	if len(pdu) != 5 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	address := uint16(pdu[1])<<8 | uint16(pdu[2])
	quantity := uint16(pdu[3])<<8 | uint16(pdu[4])

	if quantity < 1 || quantity > 2000 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()

	inputs, err := handler.HandleReadDiscreteInputs(address, quantity)
	if err != nil {
		if e, ok := err.(*Exception); ok {
			return []byte{pdu[0] | 0x80, e.Code}
		}
		return []byte{pdu[0] | 0x80, ExceptionServerDeviceFailure}
	}

	// 构建响应
	byteCount := (quantity + 7) / 8
	response := make([]byte, 2+byteCount)
	response[0] = FuncCodeReadDiscreteInputs
	response[1] = byte(byteCount)

	// 打包离散输入状态
	for i, input := range inputs {
		if input {
			byteIndex := 2 + i/8
			bitIndex := i % 8
			response[byteIndex] |= 1 << bitIndex
		}
	}

	return response
}

// handleReadHoldingRegisters 处理读保持寄存器请求
func (s *Server) handleReadHoldingRegisters(pdu []byte) []byte {
	if len(pdu) != 5 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	address := uint16(pdu[1])<<8 | uint16(pdu[2])
	quantity := uint16(pdu[3])<<8 | uint16(pdu[4])

	if quantity < 1 || quantity > 125 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()

	registers, err := handler.HandleReadHoldingRegisters(address, quantity)
	if err != nil {
		if e, ok := err.(*Exception); ok {
			return []byte{pdu[0] | 0x80, e.Code}
		}
		return []byte{pdu[0] | 0x80, ExceptionServerDeviceFailure}
	}

	// 构建响应
	byteCount := quantity * 2
	response := make([]byte, 2+byteCount)
	response[0] = FuncCodeReadHoldingRegisters
	response[1] = byte(byteCount)

	// 打包寄存器值
	for i, value := range registers {
		offset := 2 + i*2
		response[offset] = byte(value >> 8)
		response[offset+1] = byte(value)
	}

	return response
}

// handleReadInputRegisters 处理读输入寄存器请求
func (s *Server) handleReadInputRegisters(pdu []byte) []byte {
	if len(pdu) != 5 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	address := uint16(pdu[1])<<8 | uint16(pdu[2])
	quantity := uint16(pdu[3])<<8 | uint16(pdu[4])

	if quantity < 1 || quantity > 125 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()

	registers, err := handler.HandleReadInputRegisters(address, quantity)
	if err != nil {
		if e, ok := err.(*Exception); ok {
			return []byte{pdu[0] | 0x80, e.Code}
		}
		return []byte{pdu[0] | 0x80, ExceptionServerDeviceFailure}
	}

	// 构建响应
	byteCount := quantity * 2
	response := make([]byte, 2+byteCount)
	response[0] = FuncCodeReadInputRegisters
	response[1] = byte(byteCount)

	// 打包寄存器值
	for i, value := range registers {
		offset := 2 + i*2
		response[offset] = byte(value >> 8)
		response[offset+1] = byte(value)
	}

	return response
}

// handleWriteSingleCoil 处理写单个线圈请求
func (s *Server) handleWriteSingleCoil(pdu []byte) []byte {
	if len(pdu) != 5 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	address := uint16(pdu[1])<<8 | uint16(pdu[2])
	value := pdu[3] == 0xFF && pdu[4] == 0x00

	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()

	err := handler.HandleWriteSingleCoil(address, value)
	if err != nil {
		if e, ok := err.(*Exception); ok {
			return []byte{pdu[0] | 0x80, e.Code}
		}
		return []byte{pdu[0] | 0x80, ExceptionServerDeviceFailure}
	}

	// 回显请求
	return pdu
}

// handleWriteSingleRegister 处理写单个寄存器请求
func (s *Server) handleWriteSingleRegister(pdu []byte) []byte {
	if len(pdu) != 5 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	address := uint16(pdu[1])<<8 | uint16(pdu[2])
	value := uint16(pdu[3])<<8 | uint16(pdu[4])

	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()

	err := handler.HandleWriteSingleRegister(address, value)
	if err != nil {
		if e, ok := err.(*Exception); ok {
			return []byte{pdu[0] | 0x80, e.Code}
		}
		return []byte{pdu[0] | 0x80, ExceptionServerDeviceFailure}
	}

	// 回显请求
	return pdu
}

// handleWriteMultipleCoils 处理写多个线圈请求
func (s *Server) handleWriteMultipleCoils(pdu []byte) []byte {
	if len(pdu) < 6 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	address := uint16(pdu[1])<<8 | uint16(pdu[2])
	quantity := uint16(pdu[3])<<8 | uint16(pdu[4])
	byteCount := int(pdu[5])

	if quantity < 1 || quantity > 1968 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	expectedLength := 6 + byteCount
	if len(pdu) < expectedLength {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	// 解析线圈值
	values := make([]bool, quantity)
	for i := uint16(0); i < quantity; i++ {
		byteIndex := 6 + int(i/8)
		bitIndex := i % 8
		values[i] = (pdu[byteIndex]>>bitIndex)&1 != 0
	}

	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()

	err := handler.HandleWriteMultipleCoils(address, values)
	if err != nil {
		if e, ok := err.(*Exception); ok {
			return []byte{pdu[0] | 0x80, e.Code}
		}
		return []byte{pdu[0] | 0x80, ExceptionServerDeviceFailure}
	}

	// 返回地址和数量
	return []byte{FuncCodeWriteMultipleCoils, pdu[1], pdu[2], pdu[3], pdu[4]}
}

// handleWriteMultipleRegisters 处理写多个寄存器请求
func (s *Server) handleWriteMultipleRegisters(pdu []byte) []byte {
	if len(pdu) < 6 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	address := uint16(pdu[1])<<8 | uint16(pdu[2])
	quantity := uint16(pdu[3])<<8 | uint16(pdu[4])
	byteCount := int(pdu[5])

	if quantity < 1 || quantity > 123 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	if byteCount != int(quantity)*2 {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	expectedLength := 6 + byteCount
	if len(pdu) < expectedLength {
		return []byte{pdu[0] | 0x80, ExceptionIllegalDataValue}
	}

	// 解析寄存器值
	values := make([]uint16, quantity)
	for i := uint16(0); i < quantity; i++ {
		offset := 6 + int(i)*2
		values[i] = uint16(pdu[offset])<<8 | uint16(pdu[offset+1])
	}

	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()

	err := handler.HandleWriteMultipleRegisters(address, values)
	if err != nil {
		if e, ok := err.(*Exception); ok {
			return []byte{pdu[0] | 0x80, e.Code}
		}
		return []byte{pdu[0] | 0x80, ExceptionServerDeviceFailure}
	}

	// 返回地址和数量
	return []byte{FuncCodeWriteMultipleRegisters, pdu[1], pdu[2], pdu[3], pdu[4]}
}
