package driver

import "unsafe"

// SPIConfig 代表SPI配置结构
type SPIConfig struct {
	Mode                 uint8  // SPI模式: 0-3对应Mode0/1/2/3
	Clock                uint8  // 时钟分频: 0=60MHz, 1=30MHz, 2=15MHz, 3=7.5MHz, 4=3.75MHz, 5=1.875MHz, 6=937.5KHz, 7=468.75KHz
	ByteOrder            uint8  // 字节顺序: 0=LSB先传, 1=MSB先传
	SpiWriteReadInterval uint16 // SPI读写间隔，单位微秒(us)
	SpiOutDefaultData    uint8  // SPI读取时的默认输出数据
	ChipSelect           uint32 // SPI片选: BIT7=CS1控制, BIT15=CS2控制
	CS1Polarity          uint8  // CS1极性控制: 0=低电平有效, 1=高电平有效
	CS2Polarity          uint8  // CS2极性控制: 0=低电平有效, 1=高电平有效
	IsAutoDeactiveCS     uint16 // 操作完成后自动取消片选
	ActiveDelay          uint16 // 设置片选后的读写操作延迟，单位微秒(us)
	DelayDeactive        uint32 // 取消片选后的读写操作延迟，单位微秒(us)
}

// UARTConfig 代表UART配置结构
type UARTConfig struct {
	BaudRate    uint32 // 波特率
	ByteSize    uint8  // 数据位长度: 5,6,7,8
	Parity      uint8  // 校验位: 0=无校验, 1=奇校验, 2=偶校验
	StopBits    uint8  // 停止位: 0=1位, 1=1.5位, 2=2位
	ByteTimeout uint8  // 字节超时时间
}

// CANProtocolConfig 代表UART转CAN适配器的协议配置
type CANProtocolConfig struct {
	// 命令定义
	InitCmd             []byte // CAN初始化命令
	SetFilterCmd        []byte // 设置过滤器命令模板，包含{ID}和{MASK}占位符
	GetStatusCmd        []byte // 获取状态命令
	SetModeCmd          []byte // 设置模式命令模板，包含{MODE}占位符
	ClearErrorsCmd      []byte // 清除错误计数器命令
	GetErrorCountersCmd []byte // 获取错误计数器命令
	SetBaudRateCmd      []byte // 设置波特率命令模板，包含{BAUDRATE}占位符
	SendFrameCmd        []byte // 发送CAN帧命令模板，包含{ID}, {EXT}, {RTR}, {DLC}, {DATA}占位符

	// 响应解析配置
	StatusRespLength        int // 状态响应长度（字节）
	ErrorCountersRespLength int // 错误计数器响应长度（字节）

	// 帧解析配置
	FrameStartMarker []byte // CAN帧起始标记（可选）
	FrameMinLength   int    // 最小帧长度

	// 协议解析函数（可选，如果提供则优先使用函数，否则使用模板）
	ParseSendFrame             func(*CANFrame) []byte
	ParseReceiveFrame          func([]byte) (*CANFrame, error)
	ParseSetFilter             func(uint32, uint32, bool) []byte
	ParseSetMode               func(uint8) []byte
	ParseSetBaudRate           func(uint32) []byte
	ParseStatusResponse        func([]byte) (uint32, error)
	ParseErrorCountersResponse func([]byte) (uint32, uint32, error)
}

// CANConfig 代表CAN配置结构
type CANConfig struct {
	Protocol    *CANProtocolConfig // UART转CAN协议配置，如果为nil则使用默认配置
	Mode        uint8              // 模式: 0=正常模式, 1=监听模式, 2=回环模式
	FilterMode  uint8              // 过滤器模式: 0=关闭, 1=单过滤器, 2=双过滤器
	FilterID    uint32             // 过滤器ID
	FilterMask  uint32             // 过滤器掩码
	AutoRetrans bool               // 自动重传使能
	TxPriority  uint8              // 发送优先级
}

// CANFrame 代表CAN数据帧
type CANFrame struct {
	ID        uint32  // 帧ID
	Extended  bool    // 是否是扩展帧
	Remote    bool    // 是否是远程帧
	DLC       uint8   // 数据长度码 (0-8)
	Data      [8]byte // 数据
	Timestamp uint64  // 时间戳（可选）
}

// I2CConfig 代表I2C配置结构
type I2CConfig struct {
	Mode        uint8 // I2C模式: 0=低速(20KHz), 1=标准(100KHz), 2=快速(400KHz), 3=高速(750KHz)
	Stretch     bool  // 时钟拉伸使能
	DriveMode   uint8 // 驱动模式
	IgnoreNack  uint8 // 忽略NACK
	DelayMS     int   // 延迟时间，单位毫秒(ms)
	AckClkDelay int   // ACK时钟延迟，单位微秒(us)
}

// JTAGConfig 代表JTAG配置结构
type JTAGConfig struct {
	ClockRate uint8 // JTAG时钟速率
}

// GPIOConfig 代表GPIO配置结构
type GPIOConfig struct {
	Enable  uint8 // 使能位: 对应位0-7使能GPIO0-7
	DirOut  uint8 // 方向设置: 0=输入, 1=输出
	DataOut uint8 // 输出数据: 0=低电平, 1=高电平
}

// SPI 接口定义了SPI总线操作
type SPI interface {
	// Close 关闭设备
	Close() error
	// SetFrequency 设置SPI频率
	SetFrequency(freqHz uint32) error
	// Init 初始化SPI接口
	Init(cfg *SPIConfig) error
	// Write 写入SPI数据
	Write(ignoreCS bool, chipSelect uint8, data []byte) error
	// Read 读取SPI数据
	Read(ignoreCS bool, chipSelect uint8, length int) ([]byte, error)
	// WriteRead 全双工SPI传输，同时写入和读取数据
	WriteRead(ignoreCS bool, chipSelect uint8, data []byte) ([]byte, error)
	// SetAutoCS 设置SPI自动片选
	SetAutoCS(disable bool) error
	// SetDataBits 设置SPI数据位宽
	SetDataBits(dataBits uint8) error
	// GetConfig 获取SPI配置
	GetConfig(cfg *SPIConfig) error
	// ChangeCS 改变SPI片选状态
	ChangeCS(status uint8) error
	// GetHwStreamCfg 获取SPI硬件流配置
	GetHwStreamCfg(streamCfg unsafe.Pointer) error
}

// I2C 接口定义了I2C总线操作
type I2C interface {
	// Close 关闭设备
	Close() error
	// Set 配置I2C接口模式
	Set(mode int) error
	// SetStretch 设置时钟拉伸使能
	SetStretch(enable bool) error
	// SetDriveMode 设置驱动模式
	SetDriveMode(mode uint8) error
	// SetIgnoreNack 设置忽略NACK
	SetIgnoreNack(mode uint8) error
	// SetDelayMS 设置延迟时间（毫秒）
	SetDelayMS(delay int) error
	// SetAckClkDelay 设置ACK时钟延迟（微秒）
	SetAckClkDelay(delay int) error
	// Stream 执行I2C流式读写操作
	Stream(writeData []byte, readLength int) ([]byte, error)
	// StreamWithAck 执行I2C流式读写操作并返回ACK状态
	StreamWithAck(writeData []byte, readLength int) ([]byte, int, error)
}

// JTAG 接口定义了JTAG调试接口操作
type JTAG interface {
	// Close 关闭设备
	Close() error
	// Reset 复位JTAG TAP状态机
	Reset() (int, error)
	// Init 初始化JTAG接口
	Init(clockRate uint8) error
	// SwitchTapState 切换JTAG TAP状态
	SwitchTapState(tapState uint8) error
	// WriteRead 执行JTAG读写操作
	WriteRead(isDR bool, writeData []byte) ([]byte, error)
	// ByteWriteDR 按字节写入DR数据
	ByteWriteDR(data []byte) error
	// ByteReadDR 按字节读取DR数据
	ByteReadDR(length int) ([]byte, error)
	// ResetTrst 重置JTAG TRST信号
	ResetTrst(trstLevel bool) error
	// GetConfig 获取JTAG配置
	GetConfig(clockRate *uint8) error
	// WriteReadFast 快速JTAG写/读操作
	WriteReadFast(isDR bool, writeData []byte) ([]byte, error)
	// ByteWriteIR 以字节为单位写入JTAG IR数据
	ByteWriteIR(data []byte) error
	// ByteReadIR 以字节为单位读取JTAG IR数据
	ByteReadIR(length int) ([]byte, error)
	// BitWriteDR 以位为单位写入JTAG DR数据
	BitWriteDR(bitLength int, bitBuffer []byte) error
	// BitWriteIR 以位为单位写入JTAG IR数据
	BitWriteIR(bitLength int, bitBuffer []byte) error
	// BitReadIR 以位为单位读取JTAG IR数据
	BitReadIR(bitLength int) ([]byte, error)
	// BitReadDR 以位为单位读取JTAG DR数据
	BitReadDR(bitLength int) ([]byte, error)
	// ClockTms 生成JTAG时钟TMS序列
	ClockTms(bitBangPkt []byte, tms, bi uint32) uint32
	// IdleClock 生成JTAG空闲时钟序列
	IdleClock(bitBangPkt []byte, bi uint32) uint32
	// TmsChange 改变JTAG TMS状态
	TmsChange(tmsValue []byte, step, skip uint32) error
	// IoScan 执行JTAG IO扫描
	IoScan(dataBits []byte, dataBitsNb uint32, isRead bool) error
	// IoScanT 执行JTAG IO扫描（带结束包标志）
	IoScanT(dataBits []byte, dataBitsNb uint32, isRead, isLastPkt bool) error
}

// GPIO 接口定义了GPIO操作
type GPIO interface {
	// Close 关闭设备
	Close() error
	// Get 获取GPIO方向和电平状态
	Get() (dir, data uint8, err error)
	// Set 设置GPIO方向和数据
	Set(enable, dirOut, dataOut uint8) error
	// SetIRQ 设置GPIO中断
	SetIRQ(gpioIndex uint8, enable bool, irqType uint8, handler any) error
}

// UART 接口定义了UART串口操作
type UART interface {
	// Close 关闭设备
	Close() error
	// Init 初始化UART配置
	Init(baudRate int, byteSize, parity, stopBits, byteTimeout uint8) error
	// GetConfig 获取UART配置
	GetConfig() (*UARTConfig, error)
	// Read 从UART读取数据
	Read(length int) ([]byte, error)
	// Write 向UART写入数据
	Write(data []byte) error
}

// CAN 接口定义了CAN操作（继承UART）
type CAN interface {
	UART // 继承 UART 接口
	// InitCAN 初始化CAN接口
	InitCAN(uart UART, cfg *CANConfig) error
	// SendFrame 发送CAN帧
	SendFrame(frame *CANFrame) error
	// ReceiveFrame 接收CAN帧
	ReceiveFrame(timeout uint32) (*CANFrame, error)
	// SetFilter 设置CAN过滤器
	SetFilter(filterID, filterMask uint32, enable bool) error
	// GetStatus 获取CAN状态
	GetStatus() (uint32, error)
	// SetMode 设置CAN工作模式
	SetMode(mode uint8) error
	// ClearErrors 清除CAN错误计数器
	ClearErrors() error
	// GetErrorCounters 获取CAN错误计数器
	GetErrorCounters() (txErr, rxErr uint32, err error)
	// SetBaudRate 设置CAN波特率
	SetBaudRate(baudRate uint32) error
}
