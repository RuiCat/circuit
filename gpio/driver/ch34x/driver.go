package ch34x

import (
	"circuit/gpio/driver"
	"unsafe"
)

// baseDriver 基础驱动结构体，包含文件描述符
type baseDriver struct {
	fd int // 设备文件描述符
}

// Close 关闭设备
func (d *baseDriver) Close() error {
	return GlobalLib.CloseDevice(d.fd)
}

// SPIDriver SPI驱动实现
type SPIDriver struct {
	baseDriver
}

// I2CDriver I2C驱动实现
type I2CDriver struct {
	baseDriver
}

// JTAGDriver JTAG驱动实现
type JTAGDriver struct {
	baseDriver
}

// UARTDriver UART驱动实现
type UARTDriver struct {
	baseDriver
}

// GPIODriver GPIO驱动实现
type GPIODriver struct {
	baseDriver
}

// OpenSPI 打开SPI设备并返回SPI接口
func OpenSPI(path string) (driver.SPI, error) {
	fd, err := GlobalLib.OpenDevice(path)
	if err != nil {
		return nil, err
	}
	return &SPIDriver{baseDriver{fd: fd}}, nil
}

// OpenI2C 打开I2C设备并返回I2C接口
func OpenI2C(path string) (driver.I2C, error) {
	fd, err := GlobalLib.OpenDevice(path)
	if err != nil {
		return nil, err
	}
	return &I2CDriver{baseDriver{fd: fd}}, nil
}

// OpenJTAG 打开JTAG设备并返回JTAG接口
func OpenJTAG(path string) (driver.JTAG, error) {
	fd, err := GlobalLib.OpenDevice(path)
	if err != nil {
		return nil, err
	}
	return &JTAGDriver{baseDriver{fd: fd}}, nil
}

// OpenUART 打开UART设备并返回UART接口
func OpenUART(path string) (driver.UART, error) {
	fd, err := GlobalLib.UartOpen(path)
	if err != nil {
		return nil, err
	}
	return &UARTDriver{baseDriver{fd: fd}}, nil
}

// OpenGPIO 打开GPIO设备并返回GPIO接口
func OpenGPIO(path string) (driver.GPIO, error) {
	fd, err := GlobalLib.OpenDevice(path)
	if err != nil {
		return nil, err
	}
	return &GPIODriver{baseDriver{fd: fd}}, nil
}

// SPI接口实现

// SetFrequency 设置SPI频率
func (d *SPIDriver) SetFrequency(freqHz uint32) error {
	return GlobalLib.SPISetFrequency(d.fd, freqHz)
}

// Init 初始化SPI接口
func (d *SPIDriver) Init(cfg *driver.SPIConfig) error {
	return GlobalLib.SPIInit(d.fd, cfg)
}

// Write 写入SPI数据
func (d *SPIDriver) Write(ignoreCS bool, chipSelect uint8, data []byte) error {
	return GlobalLib.SPIWrite(d.fd, ignoreCS, chipSelect, data)
}

// Read 读取SPI数据
func (d *SPIDriver) Read(ignoreCS bool, chipSelect uint8, length int) ([]byte, error) {
	return GlobalLib.SPIRead(d.fd, ignoreCS, chipSelect, length)
}

// WriteRead 全双工SPI传输，同时写入和读取数据
func (d *SPIDriver) WriteRead(ignoreCS bool, chipSelect uint8, data []byte) ([]byte, error) {
	return GlobalLib.SPIWriteRead(d.fd, ignoreCS, chipSelect, data)
}

// SetAutoCS 设置SPI自动片选
func (d *SPIDriver) SetAutoCS(disable bool) error {
	return GlobalLib.SPISetAutoCS(d.fd, disable)
}

// SetDataBits 设置SPI数据位宽
func (d *SPIDriver) SetDataBits(dataBits uint8) error {
	return GlobalLib.SPISetDataBits(d.fd, dataBits)
}

// GetConfig 获取SPI配置
func (d *SPIDriver) GetConfig(cfg *driver.SPIConfig) error {
	return GlobalLib.SPIGetCfg(d.fd, unsafe.Pointer(cfg))
}

// ChangeCS 改变SPI片选状态
func (d *SPIDriver) ChangeCS(status uint8) error {
	return GlobalLib.SPIChangeCS(d.fd, status)
}

// GetHwStreamCfg 获取SPI硬件流配置
func (d *SPIDriver) GetHwStreamCfg(streamCfg unsafe.Pointer) error {
	return GlobalLib.SPIGetHwStreamCfg(d.fd, streamCfg)
}

// I2C接口实现

// Set 配置I2C接口模式
func (d *I2CDriver) Set(mode int) error {
	return GlobalLib.I2CSet(d.fd, mode)
}

// SetStretch 设置时钟拉伸使能
func (d *I2CDriver) SetStretch(enable bool) error {
	return GlobalLib.i2cSetStretch(d.fd, enable)
}

// SetDriveMode 设置驱动模式
func (d *I2CDriver) SetDriveMode(mode uint8) error {
	return GlobalLib.i2cSetDriveMode(d.fd, mode)
}

// SetIgnoreNack 设置忽略NACK
func (d *I2CDriver) SetIgnoreNack(mode uint8) error {
	return GlobalLib.i2cSetIgnoreNack(d.fd, mode)
}

// SetDelayMS 设置延迟时间（毫秒）
func (d *I2CDriver) SetDelayMS(delay int) error {
	return GlobalLib.i2cSetDelayMS(d.fd, delay)
}

// SetAckClkDelay 设置ACK时钟延迟（微秒）
func (d *I2CDriver) SetAckClkDelay(delay int) error {
	return GlobalLib.i2cSetAckClkDelay(d.fd, delay)
}

// Stream 执行I2C流式读写操作
func (d *I2CDriver) Stream(writeData []byte, readLength int) ([]byte, error) {
	return GlobalLib.StreamI2C(d.fd, writeData, readLength)
}

// StreamWithAck 执行I2C流式读写操作并返回ACK状态
func (d *I2CDriver) StreamWithAck(writeData []byte, readLength int) ([]byte, int, error) {
	return GlobalLib.i2cStreamWithAck(d.fd, writeData, readLength)
}

// JTAG接口实现

// Reset 复位JTAG TAP状态机
func (d *JTAGDriver) Reset() (int, error) {
	return GlobalLib.JtagReset(d.fd)
}

// Init 初始化JTAG接口
func (d *JTAGDriver) Init(clockRate uint8) error {
	return GlobalLib.JtagInit(d.fd, clockRate)
}

// SwitchTapState 切换JTAG TAP状态
func (d *JTAGDriver) SwitchTapState(tapState uint8) error {
	return GlobalLib.JtagSwitchTapState(d.fd, tapState)
}

// WriteRead 执行JTAG读写操作
func (d *JTAGDriver) WriteRead(isDR bool, writeData []byte) ([]byte, error) {
	return GlobalLib.JtagWriteRead(d.fd, isDR, writeData)
}

// ByteWriteDR 按字节写入DR数据
func (d *JTAGDriver) ByteWriteDR(data []byte) error {
	return GlobalLib.JtagByteWriteDR(d.fd, data)
}

// ByteReadDR 按字节读取DR数据
func (d *JTAGDriver) ByteReadDR(length int) ([]byte, error) {
	return GlobalLib.JtagByteReadDR(d.fd, length)
}

// ResetTrst 重置JTAG TRST信号
func (d *JTAGDriver) ResetTrst(trstLevel bool) error {
	return GlobalLib.JtagResetTrst(d.fd, trstLevel)
}

// GetConfig 获取JTAG配置
func (d *JTAGDriver) GetConfig(clockRate *uint8) error {
	return GlobalLib.JtagGetCfg(d.fd, clockRate)
}

// WriteReadFast 快速JTAG写/读操作
func (d *JTAGDriver) WriteReadFast(isDR bool, writeData []byte) ([]byte, error) {
	return GlobalLib.JtagWriteReadFast(d.fd, isDR, writeData)
}

// ByteWriteIR 以字节为单位写入JTAG IR数据
func (d *JTAGDriver) ByteWriteIR(data []byte) error {
	return GlobalLib.JtagByteWriteIR(d.fd, data)
}

// ByteReadIR 以字节为单位读取JTAG IR数据
func (d *JTAGDriver) ByteReadIR(length int) ([]byte, error) {
	return GlobalLib.JtagByteReadIR(d.fd, length)
}

// BitWriteDR 以位为单位写入JTAG DR数据
func (d *JTAGDriver) BitWriteDR(bitLength int, bitBuffer []byte) error {
	return GlobalLib.JtagBitWriteDR(d.fd, bitLength, bitBuffer)
}

// BitWriteIR 以位为单位写入JTAG IR数据
func (d *JTAGDriver) BitWriteIR(bitLength int, bitBuffer []byte) error {
	return GlobalLib.JtagBitWriteIR(d.fd, bitLength, bitBuffer)
}

// BitReadIR 以位为单位读取JTAG IR数据
func (d *JTAGDriver) BitReadIR(bitLength int) ([]byte, error) {
	return GlobalLib.JtagBitReadIR(d.fd, bitLength)
}

// BitReadDR 以位为单位读取JTAG DR数据
func (d *JTAGDriver) BitReadDR(bitLength int) ([]byte, error) {
	return GlobalLib.JtagBitReadDR(d.fd, bitLength)
}

// ClockTms 生成JTAG时钟TMS序列
func (d *JTAGDriver) ClockTms(bitBangPkt []byte, tms, bi uint32) uint32 {
	return GlobalLib.JtagClockTms(bitBangPkt, tms, bi)
}

// IdleClock 生成JTAG空闲时钟序列
func (d *JTAGDriver) IdleClock(bitBangPkt []byte, bi uint32) uint32 {
	return GlobalLib.JtagIdleClock(bitBangPkt, bi)
}

// TmsChange 改变JTAG TMS状态
func (d *JTAGDriver) TmsChange(tmsValue []byte, step, skip uint32) error {
	return GlobalLib.JtagTmsChange(d.fd, tmsValue, step, skip)
}

// IoScan 执行JTAG IO扫描
func (d *JTAGDriver) IoScan(dataBits []byte, dataBitsNb uint32, isRead bool) error {
	return GlobalLib.JtagIoScan(d.fd, dataBits, dataBitsNb, isRead)
}

// IoScanT 执行JTAG IO扫描（带结束包标志）
func (d *JTAGDriver) IoScanT(dataBits []byte, dataBitsNb uint32, isRead, isLastPkt bool) error {
	return GlobalLib.JtagIoScanT(d.fd, dataBits, dataBitsNb, isRead, isLastPkt)
}

// UART接口实现

// Close 关闭UART设备
func (d *UARTDriver) Close() error {
	return GlobalLib.UartClose(d.fd)
}

// Init 初始化UART配置
func (d *UARTDriver) Init(baudRate int, byteSize, parity, stopBits, byteTimeout uint8) error {
	return GlobalLib.UartInit(d.fd, baudRate, byteSize, parity, stopBits, byteTimeout)
}

// GetConfig 获取UART配置
func (d *UARTDriver) GetConfig() (*driver.UARTConfig, error) {
	baudRate, byteSize, parity, stopBits, byteTimeout, err := GlobalLib.UartGetCfg(d.fd)
	if err != nil {
		return nil, err
	}
	return &driver.UARTConfig{
		BaudRate:    baudRate,
		ByteSize:    byteSize,
		Parity:      parity,
		StopBits:    stopBits,
		ByteTimeout: byteTimeout,
	}, nil
}

// Read 从UART读取数据
func (d *UARTDriver) Read(length int) ([]byte, error) {
	return GlobalLib.UartRead(d.fd, length)
}

// Write 向UART写入数据
func (d *UARTDriver) Write(data []byte) error {
	return GlobalLib.UartWrite(d.fd, data)
}

// GPIO接口实现

// Get 获取GPIO方向和电平状态
func (d *GPIODriver) Get() (dir, data uint8, err error) {
	return GlobalLib.GPIOGet(d.fd)
}

// Set 设置GPIO方向和数据
func (d *GPIODriver) Set(enable, dirOut, dataOut uint8) error {
	return GlobalLib.GPIOSet(d.fd, enable, dirOut, dataOut)
}

// SetIRQ 设置GPIO中断
func (d *GPIODriver) SetIRQ(gpioIndex uint8, enable bool, irqType uint8, handler any) error {
	// 将handler转换为unsafe.Pointer
	var handlerPtr unsafe.Pointer
	if handler != nil {
		// 这里假设handler已经是函数指针，需要根据实际类型转换
		// 简化处理，传入nil，实际使用时需要根据接口文档处理
		handlerPtr = nil
	}
	return GlobalLib.GPIOIRQSet(d.fd, gpioIndex, enable, irqType, handlerPtr)
}
