package ch34x

/*
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/lib/x64 -lch347
#cgo linux,386 LDFLAGS: -L${SRCDIR}/lib/x86 -lch347
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/lib/aarch64 -lch347
#cgo linux,arm LDFLAGS: -L${SRCDIR}/lib/arm-gnueabihf -lch347
#cgo windows,amd64 LDFLAGS: -L${SRCDIR}/lib/win -lCH347DLL
#cgo windows,386 LDFLAGS: -L${SRCDIR}/lib/win -lCH347DLL
#cgo !linux,!windows LDFLAGS: -lch347

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

// 动态加载的C函数的前向声明
// 这些函数在CH347动态库中定义

const char* CH347GetLibInfo(void);
int CH347OpenDevice(const char *pathname);
bool CH347CloseDevice(int fd);
bool CH34xSetTimeout(int fd, uint32_t iWriteTimeout, uint32_t iReadTimeout);
bool CH34x_GetDriverVersion(int fd, unsigned char *Drv_Version);
bool CH34x_GetChipVersion(int fd, unsigned char *Version);
bool CH34x_GetChipType(int fd, void *ChipType);
bool CH34X_GetDeviceID(int fd, uint32_t *id);
bool CH347_OE_Enable(int fd);
bool CH347SPI_GetHwStreamCfg(int fd, void *StreamCfg);
bool CH347SPI_SetFrequency(int fd, uint32_t iSpiSpeedHz);
bool CH347SPI_SetAutoCS(int fd, bool disable);
bool CH347SPI_SetDataBits(int fd, uint8_t iDataBits);
bool CH347SPI_Init(int fd, void *SpiCfg);
bool CH347SPI_GetCfg(int fd, void *SpiCfg);
bool CH347SPI_ChangeCS(int fd, uint8_t iStatus);
bool CH347SPI_Write(int fd, bool ignoreCS, uint8_t iChipSelect, int iLength, int iWriteStep, void *ioBuffer);
bool CH347SPI_Read(int fd, bool ignoreCS, uint8_t iChipSelect, int iLength, uint32_t *oLength, void *ioBuffer);
bool CH347SPI_WriteRead(int fd, bool ignoreCS, uint8_t iChipSelect, int iLength, void *ioBuffer);
int CH347Jtag_Reset(int fd);
bool CH347Jtag_ResetTrst(int fd, bool TRSTLevel);
bool CH347Jtag_INIT(int fd, uint8_t iClockRate);
bool CH347Jtag_GetCfg(int fd, uint8_t *ClockRate);
uint32_t CH347Jtag_ClockTms(uint8_t *BitBangPkt, uint32_t Tms, uint32_t BI);
uint32_t CH347Jtag_IdleClock(uint8_t *BitBangPkt, uint32_t BI);
bool CH347Jtag_TmsChange(int fd, uint8_t *tmsValue, uint32_t Step, uint32_t Skip);
bool CH347Jtag_IoScan(int fd, uint8_t *DataBits, uint32_t DataBitsNb, bool IsRead);
bool CH347Jtag_IoScanT(int fd, uint8_t *DataBits, uint32_t DataBitsNb, bool IsRead, bool IsLastPkt);
bool CH347Jtag_WriteRead(int fd, bool IsDR, int iWriteBitLength, void *iWriteBitBuffer, uint32_t *oReadBitLength, void *oReadBitBuffer);
bool CH347Jtag_WriteRead_Fast(int fd, bool IsDR, int iWriteLength, void *iWriteBuffer, uint32_t *oReadLength, void *oReadBuffer);
bool CH347Jtag_SwitchTapState(int fd, uint8_t TapState);
bool CH347Jtag_ByteWriteDR(int fd, int iWriteLength, void *iWriteBuffer);
bool CH347Jtag_ByteReadDR(int fd, uint32_t *oReadLength, void *oReadBuffer);
bool CH347Jtag_ByteWriteIR(int fd, int iWriteLength, void *iWriteBuffer);
bool CH347Jtag_ByteReadIR(int fd, uint32_t *oReadLength, void *oReadBuffer);
bool CH347Jtag_BitWriteDR(int fd, int iWriteBitLength, void *iWriteBitBuffer);
bool CH347Jtag_BitWriteIR(int fd, int iWriteBitLength, void *iWriteBitBuffer);
bool CH347Jtag_BitReadIR(int fd, uint32_t *oReadBitLength, void *oReadBitBuffer);
bool CH347Jtag_BitReadDR(int fd, uint32_t *oReadBitLength, void *oReadBitBuffer);
bool CH347GPIO_Get(int fd, uint8_t *iDir, uint8_t *iData);
bool CH347GPIO_Set(int fd, uint8_t iEnable, uint8_t iSetDirOut, uint8_t iSetDataOut);
bool CH347GPIO_IRQ_Set(int fd, uint8_t gpioindex, bool enable, uint8_t irqtype, void *isr_handler);
int CH347Uart_Open(const char *pathname);
bool CH347Uart_Close(int fd);
bool CH347Uart_GetCfg(int fd, uint32_t *BaudRate, uint8_t *ByteSize, uint8_t *Parity, uint8_t *StopBits, uint8_t *ByteTimeout);
bool CH347Uart_Init(int fd, int BaudRate, uint8_t ByteSize, uint8_t Parity, uint8_t StopBits, uint8_t ByteTimeout);
bool CH347Uart_Read(int fd, void *oBuffer, uint32_t *ioLength);
bool CH347Uart_Write(int fd, void *iBuffer, uint32_t *ioLength);
bool CH347I2C_Set(int fd, int iMode);
bool CH347I2C_SetStretch(int fd, bool enable);
bool CH347I2C_SetDriveMode(int fd, uint8_t mode);
bool CH347I2C_SetIgnoreNack(int fd, uint8_t mode);
bool CH347I2C_SetDelaymS(int fd, int iDelay);
bool CH347I2C_SetAckClk_DelayuS(int fd, int iDelay);
bool CH347StreamI2C(int fd, int iWriteLength, void *iWriteBuffer, int iReadLength, void *oReadBuffer);
bool CH347StreamI2C_RetAck(int fd, int iWriteLength, void *iWriteBuffer, int iReadLength, void *oReadBuffer, int *retAck);
bool CH347ReadEEPROM(int fd, int iEepromID, int iAddr, int iLength, uint8_t *oBuffer);
bool CH347WriteEEPROM(int fd, int iEepromID, int iAddr, int iLength, uint8_t *iBuffer);
*/
import "C"
import (
	"circuit/gpio/driver"
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

// CH347库中的常量
const (
	ErrInvalid = -1
	ErrRange   = -2
	ErrIoctl   = -3

	CH347SPIMaxFreq = 60000000 // 60 MHz
	CH347SPIMinFreq = 218750   // 218.75 kHz

	IRQTypeNone        = 0
	IRQTypeEdgeRising  = 1
	IRQTypeEdgeFalling = 2
	IRQTypeEdgeBoth    = 3

	// FuncType 枚举
	TypeTTY = 0
	TypeHID = 1
	TypeVCP = 2

	// ChipMode 枚举
	ChipMode0 = 0 // Mode0(UART0/UART1)
	ChipMode1 = 1 // Mode1(UART1+SPI+I2C)
	ChipMode2 = 2 // Mode2(HID UART1+SPI+I2C)
	ChipMode3 = 3 // Mode3(UART1+JTAG+I2C)
)

// EEPROMType 表示CH347支持的EEPROM类型
type EEPROMType int

const (
	ID24C01 EEPROMType = iota
	ID24C02
	ID24C04
	ID24C08
	ID24C16
	ID24C32
	ID24C64
	ID24C128
	ID24C256
	ID24C512
	ID24C1024
	ID24C2048
	ID24C4096
)

// ChipType 表示CH34x芯片类型
type ChipType int

const (
	ChipCH341 ChipType = iota
	ChipCH347T
	ChipCH347F
	ChipCH339W
	ChipCH346C
)

// FuncType 表示功能类型(TTY, HID, VCP)
type FuncType int

const (
	FuncTypeTTY FuncType = iota
	FuncTypeHID
	FuncTypeVCP
)

// library represents a loaded CH347 dynamic library
type library struct {
	handle unsafe.Pointer
	mu     sync.RWMutex
}

var (
	GlobalLib *library
)

func init() {
	GlobalLib = &library{
		handle: unsafe.Pointer(uintptr(1)), // Dummy handle
	}
}

// Error represents a CH34x library error
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("ch34x error %d: %s", e.Code, e.Message)
}

// GetLibInfo 获取CH347库信息
func (lib *library) GetLibInfo() string {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	cstr := C.CH347GetLibInfo()
	if cstr == nil {
		return ""
	}
	return C.GoString(cstr)
}

// OpenDevice 打开CH34x设备
func (lib *library) OpenDevice(path string) (int, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	fd := C.CH347OpenDevice(cpath)
	if fd < 0 {
		return int(fd), &Error{Code: int(fd), Message: "failed to open device"}
	}
	return int(fd), nil
}

// CloseDevice 关闭CH34x设备
func (lib *library) CloseDevice(fd int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347CloseDevice(C.int(fd))
	if !success {
		return errors.New("failed to close device")
	}
	return nil
}

// SetTimeout 设置USB数据读写超时
func (lib *library) SetTimeout(fd int, writeTimeout, readTimeout uint32) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH34xSetTimeout(C.int(fd), C.uint32_t(writeTimeout), C.uint32_t(readTimeout))
	if !success {
		return errors.New("failed to set timeout")
	}
	return nil
}

// GetDriverVersion 获取厂商驱动版本
func (lib *library) GetDriverVersion(fd int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var version [256]byte
	success := C.CH34x_GetDriverVersion(C.int(fd), (*C.uchar)(unsafe.Pointer(&version[0])))
	if !success {
		return nil, errors.New("failed to get driver version")
	}
	// Find null terminator
	for i := 0; i < len(version); i++ {
		if version[i] == 0 {
			return version[:i], nil
		}
	}
	return version[:], nil
}

// GetChipVersion 获取芯片版本
func (lib *library) GetChipVersion(fd int) (byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var version byte
	success := C.CH34x_GetChipVersion(C.int(fd), (*C.uchar)(unsafe.Pointer(&version)))
	if !success {
		return 0, errors.New("failed to get chip version")
	}
	return version, nil
}

// GetChipType 获取芯片类型
func (lib *library) GetChipType(fd int) (ChipType, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var chipType int
	success := C.CH34x_GetChipType(C.int(fd), unsafe.Pointer(&chipType))
	if !success {
		return ChipCH341, errors.New("failed to get chip type")
	}
	return ChipType(chipType), nil
}

// GetDeviceID 获取设备VID和PID
func (lib *library) GetDeviceID(fd int) (uint32, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var id uint32
	success := C.CH34X_GetDeviceID(C.int(fd), (*C.uint32_t)(unsafe.Pointer(&id)))
	if !success {
		return 0, errors.New("failed to get device ID")
	}
	return id, nil
}

// SPISetFrequency 设置SPI频率
func (lib *library) SPISetFrequency(fd int, freqHz uint32) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347SPI_SetFrequency(C.int(fd), C.uint32_t(freqHz))
	if !success {
		return errors.New("failed to set SPI frequency")
	}
	return nil
}

// SPIInit 初始化SPI接口
func (lib *library) SPIInit(fd int, cfg *driver.SPIConfig) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347SPI_Init(C.int(fd), unsafe.Pointer(cfg))
	if !success {
		return errors.New("failed to initialize SPI")
	}
	return nil
}

// SPIWrite 写入SPI数据
func (lib *library) SPIWrite(fd int, ignoreCS bool, chipSelect uint8, data []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil
	}
	success := C.CH347SPI_Write(
		C.int(fd),
		C.bool(ignoreCS),
		C.uchar(chipSelect),
		C.int(len(data)),
		C.int(len(data)),
		unsafe.Pointer(&data[0]),
	)
	if !success {
		return errors.New("failed to write SPI data")
	}
	return nil
}

// SPIRead 读取SPI数据
func (lib *library) SPIRead(fd int, ignoreCS bool, chipSelect uint8, length int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if length <= 0 {
		return nil, errors.New("invalid read length")
	}
	buffer := make([]byte, length)
	var outLength uint32
	success := C.CH347SPI_Read(
		C.int(fd),
		C.bool(ignoreCS),
		C.uchar(chipSelect),
		C.int(length),
		(*C.uint32_t)(unsafe.Pointer(&outLength)),
		unsafe.Pointer(&buffer[0]),
	)
	if !success {
		return nil, errors.New("failed to read SPI data")
	}
	if int(outLength) < length {
		buffer = buffer[:outLength]
	}
	return buffer, nil
}

// SPIWriteRead 在全双工模式下写入和读取SPI数据
func (lib *library) SPIWriteRead(fd int, ignoreCS bool, chipSelect uint8, data []byte) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil, nil
	}
	// Create a copy for in/out buffer
	buffer := make([]byte, len(data))
	copy(buffer, data)
	success := C.CH347SPI_WriteRead(
		C.int(fd),
		C.bool(ignoreCS),
		C.uchar(chipSelect),
		C.int(len(buffer)),
		unsafe.Pointer(&buffer[0]),
	)
	if !success {
		return nil, errors.New("failed to write/read SPI data")
	}
	return buffer, nil
}

// GPIOGet 获取GPIO状态
func (lib *library) GPIOGet(fd int) (dir, data uint8, err error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var cDir, cData uint8
	success := C.CH347GPIO_Get(C.int(fd), (*C.uchar)(unsafe.Pointer(&cDir)), (*C.uchar)(unsafe.Pointer(&cData)))
	if !success {
		return 0, 0, errors.New("failed to get GPIO status")
	}
	return cDir, cData, nil
}

// GPIOSet 设置GPIO配置
func (lib *library) GPIOSet(fd int, enable, dirOut, dataOut uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347GPIO_Set(C.int(fd), C.uchar(enable), C.uchar(dirOut), C.uchar(dataOut))
	if !success {
		return errors.New("failed to set GPIO")
	}
	return nil
}

// UartOpen 打开UART设备
func (lib *library) UartOpen(path string) (int, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	fd := C.CH347Uart_Open(cpath)
	if fd < 0 {
		return int(fd), &Error{Code: int(fd), Message: "failed to open UART device"}
	}
	return int(fd), nil
}

// UartClose 关闭UART设备
func (lib *library) UartClose(fd int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347Uart_Close(C.int(fd))
	if !success {
		return errors.New("failed to close UART device")
	}
	return nil
}

// UartInit 初始化UART设置
func (lib *library) UartInit(fd, baudRate int, byteSize, parity, stopBits, byteTimeout uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347Uart_Init(C.int(fd), C.int(baudRate), C.uchar(byteSize), C.uchar(parity), C.uchar(stopBits), C.uchar(byteTimeout))
	if !success {
		return errors.New("failed to initialize UART")
	}
	return nil
}

// UartRead 从UART读取数据
func (lib *library) UartRead(fd int, length int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if length <= 0 {
		return nil, errors.New("invalid read length")
	}
	buffer := make([]byte, length)
	var outLength uint32 = uint32(length)
	success := C.CH347Uart_Read(C.int(fd), unsafe.Pointer(&buffer[0]), (*C.uint32_t)(unsafe.Pointer(&outLength)))
	if !success {
		return nil, errors.New("failed to read from UART")
	}
	if int(outLength) < length {
		buffer = buffer[:outLength]
	}
	return buffer, nil
}

// UartWrite 向UART写入数据
func (lib *library) UartWrite(fd int, data []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil
	}
	var outLength uint32 = uint32(len(data))
	success := C.CH347Uart_Write(C.int(fd), unsafe.Pointer(&data[0]), (*C.uint32_t)(unsafe.Pointer(&outLength)))
	if !success || int(outLength) != len(data) {
		return errors.New("failed to write to UART")
	}
	return nil
}

// I2CSet 配置I2C接口
func (lib *library) I2CSet(fd int, mode int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347I2C_Set(C.int(fd), C.int(mode))
	if !success {
		return errors.New("failed to configure I2C")
	}
	return nil
}

// i2cSetStretch 设置I2C时钟拉伸使能
func (lib *library) i2cSetStretch(fd int, enable bool) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347I2C_SetStretch(C.int(fd), C.bool(enable))
	if !success {
		return errors.New("failed to set I2C stretch")
	}
	return nil
}

// i2cSetDriveMode 设置I2C驱动模式
func (lib *library) i2cSetDriveMode(fd int, mode uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347I2C_SetDriveMode(C.int(fd), C.uchar(mode))
	if !success {
		return errors.New("failed to set I2C drive mode")
	}
	return nil
}

// i2cSetIgnoreNack 设置I2C忽略NACK模式
func (lib *library) i2cSetIgnoreNack(fd int, mode uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347I2C_SetIgnoreNack(C.int(fd), C.uchar(mode))
	if !success {
		return errors.New("failed to set I2C ignore NACK")
	}
	return nil
}

// i2cSetDelayMS 设置I2C延迟（毫秒）
func (lib *library) i2cSetDelayMS(fd int, delay int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347I2C_SetDelaymS(C.int(fd), C.int(delay))
	if !success {
		return errors.New("failed to set I2C delay")
	}
	return nil
}

// i2cSetAckClkDelay 设置I2C ACK时钟延迟（微秒）
func (lib *library) i2cSetAckClkDelay(fd int, delay int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347I2C_SetAckClk_DelayuS(C.int(fd), C.int(delay))
	if !success {
		return errors.New("failed to set I2C ACK clock delay")
	}
	return nil
}

// i2cStreamWithAck 执行带ACK返回的I2C流操作
func (lib *library) i2cStreamWithAck(fd int, writeData []byte, readLength int) ([]byte, int, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var readBuffer []byte
	if readLength > 0 {
		readBuffer = make([]byte, readLength)
	}
	var writePtr unsafe.Pointer
	var writeLen int
	if len(writeData) > 0 {
		writePtr = unsafe.Pointer(&writeData[0])
		writeLen = len(writeData)
	}
	var readPtr unsafe.Pointer
	if readLength > 0 {
		readPtr = unsafe.Pointer(&readBuffer[0])
	}
	var retAck int
	success := C.CH347StreamI2C_RetAck(C.int(fd), C.int(writeLen), writePtr, C.int(readLength), readPtr, (*C.int)(unsafe.Pointer(&retAck)))
	if !success {
		return nil, 0, errors.New("failed to perform stream I2C operation with ACK")
	}
	return readBuffer, retAck, nil
}

// StreamI2C 在流模式下执行I2C写/读操作
func (lib *library) StreamI2C(fd int, writeData []byte, readLength int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var readBuffer []byte
	if readLength > 0 {
		readBuffer = make([]byte, readLength)
	}
	var writePtr unsafe.Pointer
	var writeLen int
	if len(writeData) > 0 {
		writePtr = unsafe.Pointer(&writeData[0])
		writeLen = len(writeData)
	}
	var readPtr unsafe.Pointer
	if readLength > 0 {
		readPtr = unsafe.Pointer(&readBuffer[0])
	}
	success := C.CH347StreamI2C(C.int(fd), C.int(writeLen), writePtr, C.int(readLength), readPtr)
	if !success {
		return nil, errors.New("failed to perform stream I2C operation")
	}
	return readBuffer, nil
}

// JtagReset 重置JTAG TAP状态
func (lib *library) JtagReset(fd int) (int, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	result := C.CH347Jtag_Reset(C.int(fd))
	if result < 0 {
		return int(result), &Error{Code: int(result), Message: "failed to reset JTAG"}
	}
	return int(result), nil
}

// JtagInit 初始化JTAG接口
func (lib *library) JtagInit(fd int, clockRate uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347Jtag_INIT(C.int(fd), C.uchar(clockRate))
	if !success {
		return errors.New("failed to initialize JTAG")
	}
	return nil
}

// JtagSwitchTapState 切换JTAG状态机
func (lib *library) JtagSwitchTapState(fd int, tapState uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347Jtag_SwitchTapState(C.int(fd), C.uchar(tapState))
	if !success {
		return errors.New("failed to switch JTAG tap state")
	}
	return nil
}

// JtagWriteRead 执行JTAG写/读操作
func (lib *library) JtagWriteRead(fd int, isDR bool, writeData []byte) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(writeData) == 0 {
		return nil, nil
	}
	var readLength uint32
	readBuffer := make([]byte, len(writeData))
	success := C.CH347Jtag_WriteRead(
		C.int(fd),
		C.bool(isDR),
		C.int(len(writeData)*8), // bit length
		unsafe.Pointer(&writeData[0]),
		(*C.uint32_t)(unsafe.Pointer(&readLength)),
		unsafe.Pointer(&readBuffer[0]),
	)
	if !success {
		return nil, errors.New("failed to perform JTAG write/read")
	}
	bytesRead := (readLength + 7) / 8
	if int(bytesRead) < len(readBuffer) {
		readBuffer = readBuffer[:bytesRead]
	}
	return readBuffer, nil
}

// JtagByteWriteDR 以字节为单位写入JTAG DR数据
func (lib *library) JtagByteWriteDR(fd int, data []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil
	}
	success := C.CH347Jtag_ByteWriteDR(C.int(fd), C.int(len(data)), unsafe.Pointer(&data[0]))
	if !success {
		return errors.New("failed to write JTAG DR data")
	}
	return nil
}

// JtagByteReadDR 以字节为单位读取JTAG DR数据
func (lib *library) JtagByteReadDR(fd int, length int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if length <= 0 {
		return nil, errors.New("invalid read length")
	}
	buffer := make([]byte, length)
	var readLength uint32
	success := C.CH347Jtag_ByteReadDR(C.int(fd), (*C.uint32_t)(unsafe.Pointer(&readLength)), unsafe.Pointer(&buffer[0]))
	if !success {
		return nil, errors.New("failed to read JTAG DR data")
	}
	if int(readLength) < length {
		buffer = buffer[:readLength]
	}
	return buffer, nil
}

// ReadEEPROM 从EEPROM读取数据
func (lib *library) ReadEEPROM(fd int, eepromType EEPROMType, addr, length int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if length <= 0 {
		return nil, errors.New("invalid read length")
	}
	buffer := make([]byte, length)
	success := C.CH347ReadEEPROM(C.int(fd), C.int(eepromType), C.int(addr), C.int(length), (*C.uchar)(unsafe.Pointer(&buffer[0])))
	if !success {
		return nil, errors.New("failed to read EEPROM")
	}
	return buffer, nil
}

// WriteEEPROM 向EEPROM写入数据
func (lib *library) WriteEEPROM(fd int, eepromType EEPROMType, addr int, data []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil
	}
	success := C.CH347WriteEEPROM(C.int(fd), C.int(eepromType), C.int(addr), C.int(len(data)), (*C.uchar)(unsafe.Pointer(&data[0])))
	if !success {
		return errors.New("failed to write EEPROM")
	}
	return nil
}

// OEEnable 使能输出使能
func (lib *library) OEEnable(fd int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347_OE_Enable(C.int(fd))
	if !success {
		return errors.New("failed to enable output enable")
	}
	return nil
}

// SPIGetHwStreamCfg 获取SPI硬件流配置
func (lib *library) SPIGetHwStreamCfg(fd int, streamCfg unsafe.Pointer) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347SPI_GetHwStreamCfg(C.int(fd), streamCfg)
	if !success {
		return errors.New("failed to get SPI hardware stream configuration")
	}
	return nil
}

// SPISetAutoCS 设置SPI自动片选
func (lib *library) SPISetAutoCS(fd int, disable bool) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347SPI_SetAutoCS(C.int(fd), C.bool(disable))
	if !success {
		return errors.New("failed to set SPI auto chip select")
	}
	return nil
}

// SPISetDataBits 设置SPI数据位宽
func (lib *library) SPISetDataBits(fd int, dataBits uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347SPI_SetDataBits(C.int(fd), C.uchar(dataBits))
	if !success {
		return errors.New("failed to set SPI data bits")
	}
	return nil
}

// SPIGetCfg 获取SPI配置
func (lib *library) SPIGetCfg(fd int, spiCfg unsafe.Pointer) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347SPI_GetCfg(C.int(fd), spiCfg)
	if !success {
		return errors.New("failed to get SPI configuration")
	}
	return nil
}

// SPIChangeCS 改变SPI片选状态
func (lib *library) SPIChangeCS(fd int, status uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347SPI_ChangeCS(C.int(fd), C.uchar(status))
	if !success {
		return errors.New("failed to change SPI chip select")
	}
	return nil
}

// JtagResetTrst 重置JTAG TRST信号
func (lib *library) JtagResetTrst(fd int, trstLevel bool) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347Jtag_ResetTrst(C.int(fd), C.bool(trstLevel))
	if !success {
		return errors.New("failed to reset JTAG TRST")
	}
	return nil
}

// JtagGetCfg 获取JTAG配置
func (lib *library) JtagGetCfg(fd int, clockRate *uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347Jtag_GetCfg(C.int(fd), (*C.uchar)(unsafe.Pointer(clockRate)))
	if !success {
		return errors.New("failed to get JTAG configuration")
	}
	return nil
}

// JtagClockTms 生成JTAG时钟TMS序列
func (lib *library) JtagClockTms(bitBangPkt []byte, tms, bi uint32) uint32 {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var pktPtr *C.uchar
	if len(bitBangPkt) > 0 {
		pktPtr = (*C.uchar)(unsafe.Pointer(&bitBangPkt[0]))
	}
	return uint32(C.CH347Jtag_ClockTms(pktPtr, C.uint32_t(tms), C.uint32_t(bi)))
}

// JtagIdleClock 生成JTAG空闲时钟序列
func (lib *library) JtagIdleClock(bitBangPkt []byte, bi uint32) uint32 {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var pktPtr *C.uchar
	if len(bitBangPkt) > 0 {
		pktPtr = (*C.uchar)(unsafe.Pointer(&bitBangPkt[0]))
	}
	return uint32(C.CH347Jtag_IdleClock(pktPtr, C.uint32_t(bi)))
}

// JtagTmsChange 改变JTAG TMS状态
func (lib *library) JtagTmsChange(fd int, tmsValue []byte, step, skip uint32) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var tmsPtr *C.uchar
	if len(tmsValue) > 0 {
		tmsPtr = (*C.uchar)(unsafe.Pointer(&tmsValue[0]))
	}
	success := C.CH347Jtag_TmsChange(C.int(fd), tmsPtr, C.uint32_t(step), C.uint32_t(skip))
	if !success {
		return errors.New("failed to change JTAG TMS")
	}
	return nil
}

// JtagIoScan 执行JTAG IO扫描
func (lib *library) JtagIoScan(fd int, dataBits []byte, dataBitsNb uint32, isRead bool) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var dataPtr *C.uchar
	if len(dataBits) > 0 {
		dataPtr = (*C.uchar)(unsafe.Pointer(&dataBits[0]))
	}
	success := C.CH347Jtag_IoScan(C.int(fd), dataPtr, C.uint32_t(dataBitsNb), C.bool(isRead))
	if !success {
		return errors.New("failed to perform JTAG IO scan")
	}
	return nil
}

// JtagIoScanT 执行JTAG IO扫描（带结束包标志）
func (lib *library) JtagIoScanT(fd int, dataBits []byte, dataBitsNb uint32, isRead, isLastPkt bool) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var dataPtr *C.uchar
	if len(dataBits) > 0 {
		dataPtr = (*C.uchar)(unsafe.Pointer(&dataBits[0]))
	}
	success := C.CH347Jtag_IoScanT(C.int(fd), dataPtr, C.uint32_t(dataBitsNb), C.bool(isRead), C.bool(isLastPkt))
	if !success {
		return errors.New("failed to perform JTAG IO scan with packet flag")
	}
	return nil
}

// JtagWriteReadFast 快速JTAG写/读操作
func (lib *library) JtagWriteReadFast(fd int, isDR bool, writeData []byte) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(writeData) == 0 {
		return nil, nil
	}
	var readLength uint32
	readBuffer := make([]byte, len(writeData))
	success := C.CH347Jtag_WriteRead_Fast(
		C.int(fd),
		C.bool(isDR),
		C.int(len(writeData)),
		unsafe.Pointer(&writeData[0]),
		(*C.uint32_t)(unsafe.Pointer(&readLength)),
		unsafe.Pointer(&readBuffer[0]),
	)
	if !success {
		return nil, errors.New("failed to perform fast JTAG write/read")
	}
	if int(readLength) < len(readBuffer) {
		readBuffer = readBuffer[:readLength]
	}
	return readBuffer, nil
}

// JtagByteWriteIR 以字节为单位写入JTAG IR数据
func (lib *library) JtagByteWriteIR(fd int, data []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil
	}
	success := C.CH347Jtag_ByteWriteIR(C.int(fd), C.int(len(data)), unsafe.Pointer(&data[0]))
	if !success {
		return errors.New("failed to write JTAG IR data")
	}
	return nil
}

// JtagByteReadIR 以字节为单位读取JTAG IR数据
func (lib *library) JtagByteReadIR(fd int, length int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if length <= 0 {
		return nil, errors.New("invalid read length")
	}
	buffer := make([]byte, length)
	var readLength uint32
	success := C.CH347Jtag_ByteReadIR(C.int(fd), (*C.uint32_t)(unsafe.Pointer(&readLength)), unsafe.Pointer(&buffer[0]))
	if !success {
		return nil, errors.New("failed to read JTAG IR data")
	}
	if int(readLength) < length {
		buffer = buffer[:readLength]
	}
	return buffer, nil
}

// JtagBitWriteDR 以位为单位写入JTAG DR数据
func (lib *library) JtagBitWriteDR(fd int, bitLength int, bitBuffer []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if bitLength <= 0 || len(bitBuffer) == 0 {
		return nil
	}
	success := C.CH347Jtag_BitWriteDR(C.int(fd), C.int(bitLength), unsafe.Pointer(&bitBuffer[0]))
	if !success {
		return errors.New("failed to write JTAG DR bit data")
	}
	return nil
}

// JtagBitWriteIR 以位为单位写入JTAG IR数据
func (lib *library) JtagBitWriteIR(fd int, bitLength int, bitBuffer []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if bitLength <= 0 || len(bitBuffer) == 0 {
		return nil
	}
	success := C.CH347Jtag_BitWriteIR(C.int(fd), C.int(bitLength), unsafe.Pointer(&bitBuffer[0]))
	if !success {
		return errors.New("failed to write JTAG IR bit data")
	}
	return nil
}

// JtagBitReadIR 以位为单位读取JTAG IR数据
func (lib *library) JtagBitReadIR(fd int, bitLength int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if bitLength <= 0 {
		return nil, errors.New("invalid bit length")
	}
	byteLength := (bitLength + 7) / 8
	buffer := make([]byte, byteLength)
	var readBitLength uint32
	success := C.CH347Jtag_BitReadIR(C.int(fd), (*C.uint32_t)(unsafe.Pointer(&readBitLength)), unsafe.Pointer(&buffer[0]))
	if !success {
		return nil, errors.New("failed to read JTAG IR bit data")
	}
	actualBytes := (readBitLength + 7) / 8
	if int(actualBytes) < byteLength {
		buffer = buffer[:actualBytes]
	}
	return buffer, nil
}

// JtagBitReadDR 以位为单位读取JTAG DR数据
func (lib *library) JtagBitReadDR(fd int, bitLength int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if bitLength <= 0 {
		return nil, errors.New("invalid bit length")
	}
	byteLength := (bitLength + 7) / 8
	buffer := make([]byte, byteLength)
	var readBitLength uint32
	success := C.CH347Jtag_BitReadDR(C.int(fd), (*C.uint32_t)(unsafe.Pointer(&readBitLength)), unsafe.Pointer(&buffer[0]))
	if !success {
		return nil, errors.New("failed to read JTAG DR bit data")
	}
	actualBytes := (readBitLength + 7) / 8
	if int(actualBytes) < byteLength {
		buffer = buffer[:actualBytes]
	}
	return buffer, nil
}

// GPIOIRQSet 设置GPIO中断
func (lib *library) GPIOIRQSet(fd int, gpioIndex uint8, enable bool, irqType uint8, isrHandler unsafe.Pointer) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.CH347GPIO_IRQ_Set(C.int(fd), C.uchar(gpioIndex), C.bool(enable), C.uchar(irqType), isrHandler)
	if !success {
		return errors.New("failed to set GPIO interrupt")
	}
	return nil
}

// UartGetCfg 获取UART配置
func (lib *library) UartGetCfg(fd int) (baudRate uint32, byteSize, parity, stopBits, byteTimeout uint8, err error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var cBaudRate C.uint32_t
	var cByteSize, cParity, cStopBits, cByteTimeout C.uchar
	success := C.CH347Uart_GetCfg(C.int(fd), &cBaudRate, &cByteSize, &cParity, &cStopBits, &cByteTimeout)
	if !success {
		return 0, 0, 0, 0, 0, errors.New("failed to get UART configuration")
	}
	return uint32(cBaudRate), uint8(cByteSize), uint8(cParity), uint8(cStopBits), uint8(cByteTimeout), nil
}
