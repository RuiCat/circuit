// Package modbus 提供基于 driver.UART 接口的 Modbus RTU 协议实现。
//
// 该包包含两个主要组件：
// 1. Master (主站) - 用于主动发起 Modbus 请求
// 2. Server (从站/服务器) - 用于响应 Modbus 请求
//
// Master 支持的功能码：
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
//	// 创建Modbus主站
//	master := modbus.NewMaster(uart)
//	master.SetSlaveID(1) // 设置从站地址
//	master.SetTimeout(100 * time.Millisecond) // 设置超时
//
//	// 读取保持寄存器
//	registers, err := master.ReadHoldingRegisters(0, 10)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Registers: %v\n", registers)
package modbus

import (
	"circuit/gpio/driver"
	"errors"
	"fmt"
	"time"
)

// Exception 表示 Modbus 异常响应
// 当从站返回异常响应时，会返回此类型的错误
type Exception struct {
	Code byte // 异常码，参见 Modbus 协议规范
}

// Error 实现 error 接口，返回异常信息的字符串表示
func (e *Exception) Error() string {
	return fmt.Sprintf("modbus exception code: 0x%02X", e.Code)
}

// Master 表示一个 Modbus RTU 主站（客户端）
// 用于向 Modbus 从站发送请求并接收响应
type Master struct {
	uart    driver.UART   // 底层的 UART 接口，用于串口通信
	slaveID uint8         // 目标从站地址，范围 1-247
	timeout time.Duration // 读写操作超时时间
}

// NewMaster 创建一个新的 Modbus 主站
// 参数:
//   - uart: 实现 driver.UART 接口的串口设备，用于底层通信
//
// 返回值:
//   - *Master: 新创建的 Modbus 主站实例
//
// 注意: 默认从站地址为 1，超时时间为 100ms
func NewMaster(uart driver.UART) *Master {
	return &Master{
		uart:    uart,
		slaveID: 1, // 默认从站地址
		timeout: 100 * time.Millisecond,
	}
}

// SetSlaveID 设置 Modbus 从站地址
// 参数:
//   - id: 从站地址，范围 1-247
//
// 返回值:
//   - error: 如果地址无效则返回错误
//
// 注意: Modbus 协议规定从站地址范围为 1-247，0 为广播地址
func (c *Master) SetSlaveID(id uint8) error {
	if id == 0 || id > 247 {
		return errors.New("modbus: slave ID must be between 1 and 247")
	}
	c.slaveID = id
	return nil
}

// SetTimeout 设置读写超时时间
// 参数:
//   - timeout: 超时时间，影响 sendRequest 中等待响应的时长
//
// 注意: 超时时间用于控制 UART 读取操作的等待时间
func (c *Master) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// 内部辅助函数

// calculateCRC 计算 Modbus RTU 的 CRC16 校验
// 参数:
//   - data: 需要计算 CRC 的字节切片
//
// 返回值:
//   - uint16: 计算得到的 CRC16 校验值（小端字节序）
//
// 算法: 使用 Modbus RTU 标准的 CRC-16-IBM 多项式 0x8005（反向为 0xA001）
func calculateCRC(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc >>= 1
				crc ^= 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// sendRequest 发送请求并接收响应
// 这是内部方法，所有公共的读写方法都通过此方法发送请求
// 参数:
//   - pdu: Protocol Data Unit，即功能码和数据部分
//
// 返回值:
//   - []byte: 响应中的 PDU 部分（去掉从站地址和 CRC）
//   - error: 发送或接收过程中出现的错误，包括异常响应
//
// 处理流程:
//  1. 构建 RTU 帧: 从站地址 + PDU + CRC
//  2. 发送帧到串口
//  3. 接收响应数据
//  4. 验证响应格式、CRC 和从站地址
//  5. 检查异常响应，如有异常则返回 Exception 错误
func (c *Master) sendRequest(pdu []byte) ([]byte, error) {
	// 构建 RTU 帧: 从站地址 + PDU + CRC
	frame := make([]byte, 1+len(pdu)+2)
	frame[0] = c.slaveID
	copy(frame[1:], pdu)
	crc := calculateCRC(frame[:len(frame)-2])
	frame[len(frame)-2] = byte(crc & 0xFF)
	frame[len(frame)-1] = byte(crc >> 8)

	// 发送请求
	if err := c.uart.Write(frame); err != nil {
		return nil, err
	}

	// 接收响应
	// 简单实现: 读取足够大的缓冲区，解析响应
	// 实际实现应考虑超时和帧间隔检测
	readBuf, err := c.uart.Read(256)
	if err != nil {
		return nil, err
	}

	// 验证响应
	if len(readBuf) < 5 {
		return nil, errors.New("modbus: response too short")
	}

	// 检查从站地址
	if readBuf[0] != c.slaveID {
		return nil, errors.New("modbus: slave ID mismatch")
	}

	// 检查 CRC
	receivedCRC := uint16(readBuf[len(readBuf)-2]) | uint16(readBuf[len(readBuf)-1])<<8
	expectedCRC := calculateCRC(readBuf[:len(readBuf)-2])
	if receivedCRC != expectedCRC {
		return nil, errors.New("modbus: CRC error")
	}

	// 检查异常响应
	if readBuf[1]&0x80 != 0 {
		// 异常响应
		if len(readBuf) < 5 {
			return nil, errors.New("modbus: malformed exception response")
		}
		exceptionCode := readBuf[2]
		return nil, &Exception{Code: exceptionCode}
	}

	// 返回 PDU (去掉从站地址和 CRC)
	return readBuf[1 : len(readBuf)-2], nil
}

// 功能码常量定义
// 参考 Modbus 协议规范，支持以下标准功能码:
const (
	FuncCodeReadCoils              = 0x01 // 读线圈（可读写布尔量）
	FuncCodeReadDiscreteInputs     = 0x02 // 读离散输入（只读布尔量）
	FuncCodeReadHoldingRegisters   = 0x03 // 读保持寄存器（可读写16位寄存器）
	FuncCodeReadInputRegisters     = 0x04 // 读输入寄存器（只读16位寄存器）
	FuncCodeWriteSingleCoil        = 0x05 // 写单个线圈
	FuncCodeWriteSingleRegister    = 0x06 // 写单个保持寄存器
	FuncCodeWriteMultipleCoils     = 0x0F // 写多个线圈
	FuncCodeWriteMultipleRegisters = 0x10 // 写多个保持寄存器
)

// ReadCoils 读取线圈状态 (功能码 0x01)
// 线圈是 Modbus 中的可读写布尔量，通常表示继电器输出状态
// 参数:
//   - address: 起始线圈地址，0-based
//   - quantity: 要读取的线圈数量，范围 1-2000
//
// 返回值:
//   - []bool: 线圈状态切片，true 表示 ON/1，false 表示 OFF/0
//   - error: 通信错误或协议错误
//
// 注意:
//   - 线圈地址范围取决于从站设备
//   - 读取数量受 Modbus 协议限制（最大 2000 个线圈）
func (c *Master) ReadCoils(address, quantity uint16) ([]bool, error) {
	if quantity < 1 || quantity > 2000 {
		return nil, errors.New("modbus: quantity must be between 1 and 2000")
	}

	pdu := make([]byte, 5)
	pdu[0] = FuncCodeReadCoils
	pdu[1] = byte(address >> 8)
	pdu[2] = byte(address)
	pdu[3] = byte(quantity >> 8)
	pdu[4] = byte(quantity)

	response, err := c.sendRequest(pdu)
	if err != nil {
		return nil, err
	}

	if len(response) < 2 {
		return nil, errors.New("modbus: invalid response length")
	}
	byteCount := int(response[1])
	if len(response) != 2+byteCount {
		return nil, errors.New("modbus: response data length mismatch")
	}

	// 解析线圈状态
	coils := make([]bool, quantity)
	for i := uint16(0); i < quantity; i++ {
		byteIndex := 2 + int(i/8)
		bitIndex := i % 8
		coils[i] = (response[byteIndex]>>bitIndex)&1 != 0
	}
	return coils, nil
}

// ReadDiscreteInputs 读取离散输入状态 (功能码 0x02)
// 离散输入是 Modbus 中的只读布尔量，通常表示开关输入状态
// 参数:
//   - address: 起始离散输入地址，0-based
//   - quantity: 要读取的离散输入数量，范围 1-2000
//
// 返回值:
//   - []bool: 离散输入状态切片，true 表示 ON/1，false 表示 OFF/0
//   - error: 通信错误或协议错误
//
// 注意:
//   - 离散输入是只读的，不能通过 Modbus 写入
//   - 读取数量受 Modbus 协议限制（最大 2000 个输入）
func (c *Master) ReadDiscreteInputs(address, quantity uint16) ([]bool, error) {
	if quantity < 1 || quantity > 2000 {
		return nil, errors.New("modbus: quantity must be between 1 and 2000")
	}

	pdu := make([]byte, 5)
	pdu[0] = FuncCodeReadDiscreteInputs
	pdu[1] = byte(address >> 8)
	pdu[2] = byte(address)
	pdu[3] = byte(quantity >> 8)
	pdu[4] = byte(quantity)

	response, err := c.sendRequest(pdu)
	if err != nil {
		return nil, err
	}

	if len(response) < 2 {
		return nil, errors.New("modbus: invalid response length")
	}
	byteCount := int(response[1])
	if len(response) != 2+byteCount {
		return nil, errors.New("modbus: response data length mismatch")
	}

	// 解析离散输入状态
	inputs := make([]bool, quantity)
	for i := uint16(0); i < quantity; i++ {
		byteIndex := 2 + int(i/8)
		bitIndex := i % 8
		inputs[i] = (response[byteIndex]>>bitIndex)&1 != 0
	}
	return inputs, nil
}

// ReadHoldingRegisters 读取保持寄存器 (功能码 0x03)
// 保持寄存器是 Modbus 中的可读写16位寄存器，通常用于存储配置参数或测量值
// 参数:
//   - address: 起始保持寄存器地址，0-based
//   - quantity: 要读取的寄存器数量，范围 1-125
//
// 返回值:
//   - []uint16: 寄存器值切片，每个元素为16位无符号整数
//   - error: 通信错误或协议错误
//
// 注意:
//   - 保持寄存器是可读写的，常用于存储设备配置或过程数据
//   - 读取数量受 Modbus 协议限制（最大 125 个寄存器）
//   - 寄存器值以大端字节序传输
func (c *Master) ReadHoldingRegisters(address, quantity uint16) ([]uint16, error) {
	if quantity < 1 || quantity > 125 {
		return nil, errors.New("modbus: quantity must be between 1 and 125")
	}

	pdu := make([]byte, 5)
	pdu[0] = FuncCodeReadHoldingRegisters
	pdu[1] = byte(address >> 8)
	pdu[2] = byte(address)
	pdu[3] = byte(quantity >> 8)
	pdu[4] = byte(quantity)

	response, err := c.sendRequest(pdu)
	if err != nil {
		return nil, err
	}

	if len(response) < 2 {
		return nil, errors.New("modbus: invalid response length")
	}
	byteCount := int(response[1])
	if len(response) != 2+byteCount || byteCount != int(quantity)*2 {
		return nil, errors.New("modbus: response data length mismatch")
	}

	// 解析寄存器值
	registers := make([]uint16, quantity)
	for i := uint16(0); i < quantity; i++ {
		offset := 2 + int(i)*2
		registers[i] = uint16(response[offset])<<8 | uint16(response[offset+1])
	}
	return registers, nil
}

// ReadInputRegisters 读取输入寄存器 (功能码 0x04)
// 输入寄存器是 Modbus 中的只读16位寄存器，通常用于存储模拟量输入或只读数据
// 参数:
//   - address: 起始输入寄存器地址，0-based
//   - quantity: 要读取的寄存器数量，范围 1-125
//
// 返回值:
//   - []uint16: 寄存器值切片，每个元素为16位无符号整数
//   - error: 通信错误或协议错误
//
// 注意:
//   - 输入寄存器是只读的，不能通过 Modbus 写入
//   - 常用于存储传感器读数、模拟量输入等
//   - 读取数量受 Modbus 协议限制（最大 125 个寄存器）
func (c *Master) ReadInputRegisters(address, quantity uint16) ([]uint16, error) {
	if quantity < 1 || quantity > 125 {
		return nil, errors.New("modbus: quantity must be between 1 and 125")
	}

	pdu := make([]byte, 5)
	pdu[0] = FuncCodeReadInputRegisters
	pdu[1] = byte(address >> 8)
	pdu[2] = byte(address)
	pdu[3] = byte(quantity >> 8)
	pdu[4] = byte(quantity)

	response, err := c.sendRequest(pdu)
	if err != nil {
		return nil, err
	}

	if len(response) < 2 {
		return nil, errors.New("modbus: invalid response length")
	}
	byteCount := int(response[1])
	if len(response) != 2+byteCount || byteCount != int(quantity)*2 {
		return nil, errors.New("modbus: response data length mismatch")
	}

	// 解析寄存器值
	registers := make([]uint16, quantity)
	for i := uint16(0); i < quantity; i++ {
		offset := 2 + int(i)*2
		registers[i] = uint16(response[offset])<<8 | uint16(response[offset+1])
	}
	return registers, nil
}

// WriteSingleCoil 写入单个线圈 (功能码 0x05)
// 向指定地址的线圈写入单个布尔值
// 参数:
//   - address: 线圈地址，0-based
//   - value: 要写入的值，true 表示 ON/1，false 表示 OFF/0
//
// 返回值:
//   - error: 通信错误或协议错误
//
// 注意:
//   - Modbus 协议规定，写入线圈时，0xFF00 表示 ON，0x0000 表示 OFF
//   - 成功的响应应回显请求的完整 PDU
//   - 只能写入线圈，不能写入离散输入
func (c *Master) WriteSingleCoil(address uint16, value bool) error {
	pdu := make([]byte, 5)
	pdu[0] = FuncCodeWriteSingleCoil
	pdu[1] = byte(address >> 8)
	pdu[2] = byte(address)
	if value {
		pdu[3] = 0xFF
		pdu[4] = 0x00
	} else {
		pdu[3] = 0x00
		pdu[4] = 0x00
	}

	response, err := c.sendRequest(pdu)
	if err != nil {
		return err
	}

	// 响应应该回显请求
	if len(response) != 5 {
		return errors.New("modbus: invalid response length")
	}
	for i := 0; i < 5; i++ {
		if response[i] != pdu[i] {
			return errors.New("modbus: response does not match request")
		}
	}
	return nil
}

// WriteSingleRegister 写入单个寄存器 (功能码 0x06)
// 向指定地址的保持寄存器写入单个16位值
// 参数:
//   - address: 保持寄存器地址，0-based
//   - value: 要写入的16位无符号整数值
//
// 返回值:
//   - error: 通信错误或协议错误
//
// 注意:
//   - 只能写入保持寄存器，不能写入输入寄存器
//   - 成功的响应应回显请求的完整 PDU
//   - 寄存器值以大端字节序传输
func (c *Master) WriteSingleRegister(address uint16, value uint16) error {
	pdu := make([]byte, 5)
	pdu[0] = FuncCodeWriteSingleRegister
	pdu[1] = byte(address >> 8)
	pdu[2] = byte(address)
	pdu[3] = byte(value >> 8)
	pdu[4] = byte(value)

	response, err := c.sendRequest(pdu)
	if err != nil {
		return err
	}

	// 响应应该回显请求
	if len(response) != 5 {
		return errors.New("modbus: invalid response length")
	}
	for i := 0; i < 5; i++ {
		if response[i] != pdu[i] {
			return errors.New("modbus: response does not match request")
		}
	}
	return nil
}

// WriteMultipleCoils 写入多个线圈 (功能码 0x0F)
func (c *Master) WriteMultipleCoils(address uint16, values []bool) error {
	quantity := uint16(len(values))
	if quantity < 1 || quantity > 1968 {
		return errors.New("modbus: quantity must be between 1 and 1968")
	}

	byteCount := (quantity + 7) / 8
	pdu := make([]byte, 6+byteCount)
	pdu[0] = FuncCodeWriteMultipleCoils
	pdu[1] = byte(address >> 8)
	pdu[2] = byte(address)
	pdu[3] = byte(quantity >> 8)
	pdu[4] = byte(quantity)
	pdu[5] = byte(byteCount)

	// 打包线圈状态
	for i := uint16(0); i < quantity; i++ {
		byteIndex := 6 + int(i/8)
		bitIndex := i % 8
		if values[i] {
			pdu[byteIndex] |= 1 << bitIndex
		}
	}

	response, err := c.sendRequest(pdu)
	if err != nil {
		return err
	}

	// 响应应该是地址和数量
	if len(response) != 5 {
		return errors.New("modbus: invalid response length")
	}
	if response[0] != FuncCodeWriteMultipleCoils ||
		response[1] != pdu[1] || response[2] != pdu[2] ||
		response[3] != pdu[3] || response[4] != pdu[4] {
		return errors.New("modbus: response does not match request")
	}
	return nil
}

// WriteMultipleRegisters 写入多个寄存器 (功能码 0x10)
func (c *Master) WriteMultipleRegisters(address uint16, values []uint16) error {
	quantity := uint16(len(values))
	if quantity < 1 || quantity > 123 {
		return errors.New("modbus: quantity must be between 1 and 123")
	}

	byteCount := quantity * 2
	pdu := make([]byte, 6+int(byteCount))
	pdu[0] = FuncCodeWriteMultipleRegisters
	pdu[1] = byte(address >> 8)
	pdu[2] = byte(address)
	pdu[3] = byte(quantity >> 8)
	pdu[4] = byte(quantity)
	pdu[5] = byte(byteCount)

	// 打包寄存器值
	for i, value := range values {
		offset := 6 + i*2
		pdu[offset] = byte(value >> 8)
		pdu[offset+1] = byte(value)
	}

	response, err := c.sendRequest(pdu)
	if err != nil {
		return err
	}

	// 响应应该是地址和数量
	if len(response) != 5 {
		return errors.New("modbus: invalid response length")
	}
	if response[0] != FuncCodeWriteMultipleRegisters ||
		response[1] != pdu[1] || response[2] != pdu[2] ||
		response[3] != pdu[3] || response[4] != pdu[4] {
		return errors.New("modbus: response does not match request")
	}
	return nil
}
