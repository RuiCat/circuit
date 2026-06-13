package ch34x

/*
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

// ===== 函数指针类型定义 =====
// 每个类型对应一个 C 函数签名

// --- 基础设备操作 ---
typedef int           (*t_CH347OpenDevice)(const char*);
typedef bool          (*t_CH347CloseDevice)(int);
typedef bool          (*t_CH34xSetTimeout)(int, uint32_t, uint32_t);
typedef bool          (*t_CH34x_GetDriverVersion)(int, unsigned char*);
typedef bool          (*t_CH34x_GetChipVersion)(int, unsigned char*);
typedef bool          (*t_CH34x_GetChipType)(int, void*);
typedef bool          (*t_CH34X_GetDeviceID)(int, uint32_t*);
typedef bool          (*t_CH347_OE_Enable)(int);

// --- SPI ---
typedef bool          (*t_CH347SPI_GetHwStreamCfg)(int, void*);
typedef bool          (*t_CH347SPI_SetFrequency)(int, uint32_t);
typedef bool          (*t_CH347SPI_SetAutoCS)(int, bool);
typedef bool          (*t_CH347SPI_SetDataBits)(int, uint8_t);
typedef bool          (*t_CH347SPI_Init)(int, void*);
typedef bool          (*t_CH347SPI_GetCfg)(int, void*);
typedef bool          (*t_CH347SPI_ChangeCS)(int, uint8_t);
typedef bool          (*t_CH347SPI_Write)(int, bool, uint8_t, int, int, void*);
typedef bool          (*t_CH347SPI_Read)(int, bool, uint8_t, int, uint32_t*, void*);
typedef bool          (*t_CH347SPI_WriteRead)(int, bool, uint8_t, int, void*);

// --- JTAG ---
typedef int           (*t_CH347Jtag_Reset)(int);
typedef bool          (*t_CH347Jtag_ResetTrst)(int, bool);
typedef bool          (*t_CH347Jtag_INIT)(int, uint8_t);
typedef bool          (*t_CH347Jtag_GetCfg)(int, uint8_t*);
typedef uint32_t      (*t_CH347Jtag_ClockTms)(uint8_t*, uint32_t, uint32_t);
typedef uint32_t      (*t_CH347Jtag_IdleClock)(uint8_t*, uint32_t);
typedef bool          (*t_CH347Jtag_TmsChange)(int, uint8_t*, uint32_t, uint32_t);
typedef bool          (*t_CH347Jtag_IoScan)(int, uint8_t*, uint32_t, bool);
typedef bool          (*t_CH347Jtag_IoScanT)(int, uint8_t*, uint32_t, bool, bool);
typedef bool          (*t_CH347Jtag_WriteRead)(int, bool, int, void*, uint32_t*, void*);
typedef bool          (*t_CH347Jtag_WriteRead_Fast)(int, bool, int, void*, uint32_t*, void*);
typedef bool          (*t_CH347Jtag_SwitchTapState)(int, uint8_t);
typedef bool          (*t_CH347Jtag_ByteWriteDR)(int, int, void*);
typedef bool          (*t_CH347Jtag_ByteReadDR)(int, uint32_t*, void*);
typedef bool          (*t_CH347Jtag_ByteWriteIR)(int, int, void*);
typedef bool          (*t_CH347Jtag_ByteReadIR)(int, uint32_t*, void*);
typedef bool          (*t_CH347Jtag_BitWriteDR)(int, int, void*);
typedef bool          (*t_CH347Jtag_BitWriteIR)(int, int, void*);
typedef bool          (*t_CH347Jtag_BitReadIR)(int, uint32_t*, void*);
typedef bool          (*t_CH347Jtag_BitReadDR)(int, uint32_t*, void*);

// --- GPIO ---
typedef bool          (*t_CH347GPIO_Get)(int, uint8_t*, uint8_t*);
typedef bool          (*t_CH347GPIO_Set)(int, uint8_t, uint8_t, uint8_t);
typedef bool          (*t_CH347GPIO_IRQ_Set)(int, uint8_t, bool, uint8_t, void*);

// --- UART ---
typedef int           (*t_CH347Uart_Open)(const char*);
typedef bool          (*t_CH347Uart_Close)(int);
typedef bool          (*t_CH347Uart_GetCfg)(int, uint32_t*, uint8_t*, uint8_t*, uint8_t*, uint8_t*);
typedef bool          (*t_CH347Uart_Init)(int, int, uint8_t, uint8_t, uint8_t, uint8_t);
typedef bool          (*t_CH347Uart_Read)(int, void*, uint32_t*);
typedef bool          (*t_CH347Uart_Write)(int, void*, uint32_t*);

// --- I2C ---
typedef bool          (*t_CH347I2C_Set)(int, int);
typedef bool          (*t_CH347I2C_SetStretch)(int, bool);
typedef bool          (*t_CH347I2C_SetDriveMode)(int, uint8_t);
typedef bool          (*t_CH347I2C_SetIgnoreNack)(int, uint8_t);
typedef bool          (*t_CH347I2C_SetDelaymS)(int, int);
typedef bool          (*t_CH347I2C_SetAckClk_DelayuS)(int, int);
typedef bool          (*t_CH347StreamI2C)(int, int, void*, int, void*);
typedef bool          (*t_CH347StreamI2C_RetAck)(int, int, void*, int, void*, int*);

// --- EEPROM ---
typedef bool          (*t_CH347ReadEEPROM)(int, int, int, int, uint8_t*);
typedef bool          (*t_CH347WriteEEPROM)(int, int, int, int, uint8_t*);

// --- Lib Info ---
typedef const char*   (*t_CH347GetLibInfo)(void);

// ===== 静态函数指针变量 =====
// 内部使用 _fp_ 前缀命名，Go 通过同名包装函数 pXXX 访问

// 基础设备操作
static t_CH347OpenDevice          _fp_OpenDevice;
static t_CH347CloseDevice         _fp_CloseDevice;
static t_CH34xSetTimeout          _fp_SetTimeout;
static t_CH34x_GetDriverVersion   _fp_GetDriverVersion;
static t_CH34x_GetChipVersion     _fp_GetChipVersion;
static t_CH34x_GetChipType        _fp_GetChipType;
static t_CH34X_GetDeviceID        _fp_GetDeviceID;
static t_CH347_OE_Enable          _fp_OEEnable;
// SPI
static t_CH347SPI_GetHwStreamCfg  _fp_SPIGetHwStreamCfg;
static t_CH347SPI_SetFrequency    _fp_SPISetFrequency;
static t_CH347SPI_SetAutoCS       _fp_SPISetAutoCS;
static t_CH347SPI_SetDataBits     _fp_SPISetDataBits;
static t_CH347SPI_Init            _fp_SPIInit;
static t_CH347SPI_GetCfg          _fp_SPIGetCfg;
static t_CH347SPI_ChangeCS        _fp_SPIChangeCS;
static t_CH347SPI_Write           _fp_SPIWrite;
static t_CH347SPI_Read            _fp_SPIRead;
static t_CH347SPI_WriteRead       _fp_SPIWriteRead;
// JTAG
static t_CH347Jtag_Reset          _fp_JtagReset;
static t_CH347Jtag_ResetTrst      _fp_JtagResetTrst;
static t_CH347Jtag_INIT           _fp_JtagInit;
static t_CH347Jtag_GetCfg         _fp_JtagGetCfg;
static t_CH347Jtag_ClockTms       _fp_JtagClockTms;
static t_CH347Jtag_IdleClock      _fp_JtagIdleClock;
static t_CH347Jtag_TmsChange      _fp_JtagTmsChange;
static t_CH347Jtag_IoScan         _fp_JtagIoScan;
static t_CH347Jtag_IoScanT        _fp_JtagIoScanT;
static t_CH347Jtag_WriteRead      _fp_JtagWriteRead;
static t_CH347Jtag_WriteRead_Fast _fp_JtagWriteReadFast;
static t_CH347Jtag_SwitchTapState _fp_JtagSwitchTapState;
static t_CH347Jtag_ByteWriteDR    _fp_JtagByteWriteDR;
static t_CH347Jtag_ByteReadDR     _fp_JtagByteReadDR;
static t_CH347Jtag_ByteWriteIR    _fp_JtagByteWriteIR;
static t_CH347Jtag_ByteReadIR     _fp_JtagByteReadIR;
static t_CH347Jtag_BitWriteDR     _fp_JtagBitWriteDR;
static t_CH347Jtag_BitWriteIR     _fp_JtagBitWriteIR;
static t_CH347Jtag_BitReadIR      _fp_JtagBitReadIR;
static t_CH347Jtag_BitReadDR      _fp_JtagBitReadDR;
// GPIO
static t_CH347GPIO_Get            _fp_GPIOGet;
static t_CH347GPIO_Set            _fp_GPIOSet;
static t_CH347GPIO_IRQ_Set        _fp_GPIOIRQSet;
// UART
static t_CH347Uart_Open           _fp_UartOpen;
static t_CH347Uart_Close          _fp_UartClose;
static t_CH347Uart_GetCfg         _fp_UartGetCfg;
static t_CH347Uart_Init           _fp_UartInit;
static t_CH347Uart_Read           _fp_UartRead;
static t_CH347Uart_Write          _fp_UartWrite;
// I2C
static t_CH347I2C_Set             _fp_I2CSet;
static t_CH347I2C_SetStretch      _fp_I2CSetStretch;
static t_CH347I2C_SetDriveMode    _fp_I2CSetDriveMode;
static t_CH347I2C_SetIgnoreNack   _fp_I2CSetIgnoreNack;
static t_CH347I2C_SetDelaymS      _fp_I2CSetDelaymS;
static t_CH347I2C_SetAckClk_DelayuS _fp_I2CSetAckClkDelayuS;
static t_CH347StreamI2C           _fp_StreamI2C;
static t_CH347StreamI2C_RetAck    _fp_StreamI2CRetAck;
// EEPROM
static t_CH347ReadEEPROM          _fp_ReadEEPROM;
static t_CH347WriteEEPROM         _fp_WriteEEPROM;
// Lib Info
static t_CH347GetLibInfo          _fp_GetLibInfo;

// ===== Setter 函数：供 Go 层通过 dynlib 设置函数指针 =====
#define SETTER(name, type) \
	static void set_##name(void* ptr) { _fp_##name = (type)ptr; }

SETTER(OpenDevice, t_CH347OpenDevice)
SETTER(CloseDevice, t_CH347CloseDevice)
SETTER(SetTimeout, t_CH34xSetTimeout)
SETTER(GetDriverVersion, t_CH34x_GetDriverVersion)
SETTER(GetChipVersion, t_CH34x_GetChipVersion)
SETTER(GetChipType, t_CH34x_GetChipType)
SETTER(GetDeviceID, t_CH34X_GetDeviceID)
SETTER(OEEnable, t_CH347_OE_Enable)
SETTER(SPIGetHwStreamCfg, t_CH347SPI_GetHwStreamCfg)
SETTER(SPISetFrequency, t_CH347SPI_SetFrequency)
SETTER(SPISetAutoCS, t_CH347SPI_SetAutoCS)
SETTER(SPISetDataBits, t_CH347SPI_SetDataBits)
SETTER(SPIInit, t_CH347SPI_Init)
SETTER(SPIGetCfg, t_CH347SPI_GetCfg)
SETTER(SPIChangeCS, t_CH347SPI_ChangeCS)
SETTER(SPIWrite, t_CH347SPI_Write)
SETTER(SPIRead, t_CH347SPI_Read)
SETTER(SPIWriteRead, t_CH347SPI_WriteRead)
SETTER(JtagReset, t_CH347Jtag_Reset)
SETTER(JtagResetTrst, t_CH347Jtag_ResetTrst)
SETTER(JtagInit, t_CH347Jtag_INIT)
SETTER(JtagGetCfg, t_CH347Jtag_GetCfg)
SETTER(JtagClockTms, t_CH347Jtag_ClockTms)
SETTER(JtagIdleClock, t_CH347Jtag_IdleClock)
SETTER(JtagTmsChange, t_CH347Jtag_TmsChange)
SETTER(JtagIoScan, t_CH347Jtag_IoScan)
SETTER(JtagIoScanT, t_CH347Jtag_IoScanT)
SETTER(JtagWriteRead, t_CH347Jtag_WriteRead)
SETTER(JtagWriteReadFast, t_CH347Jtag_WriteRead_Fast)
SETTER(JtagSwitchTapState, t_CH347Jtag_SwitchTapState)
SETTER(JtagByteWriteDR, t_CH347Jtag_ByteWriteDR)
SETTER(JtagByteReadDR, t_CH347Jtag_ByteReadDR)
SETTER(JtagByteWriteIR, t_CH347Jtag_ByteWriteIR)
SETTER(JtagByteReadIR, t_CH347Jtag_ByteReadIR)
SETTER(JtagBitWriteDR, t_CH347Jtag_BitWriteDR)
SETTER(JtagBitWriteIR, t_CH347Jtag_BitWriteIR)
SETTER(JtagBitReadIR, t_CH347Jtag_BitReadIR)
SETTER(JtagBitReadDR, t_CH347Jtag_BitReadDR)
SETTER(GPIOGet, t_CH347GPIO_Get)
SETTER(GPIOSet, t_CH347GPIO_Set)
SETTER(GPIOIRQSet, t_CH347GPIO_IRQ_Set)
SETTER(UartOpen, t_CH347Uart_Open)
SETTER(UartClose, t_CH347Uart_Close)
SETTER(UartGetCfg, t_CH347Uart_GetCfg)
SETTER(UartInit, t_CH347Uart_Init)
SETTER(UartRead, t_CH347Uart_Read)
SETTER(UartWrite, t_CH347Uart_Write)
SETTER(I2CSet, t_CH347I2C_Set)
SETTER(I2CSetStretch, t_CH347I2C_SetStretch)
SETTER(I2CSetDriveMode, t_CH347I2C_SetDriveMode)
SETTER(I2CSetIgnoreNack, t_CH347I2C_SetIgnoreNack)
SETTER(I2CSetDelaymS, t_CH347I2C_SetDelaymS)
SETTER(I2CSetAckClkDelayuS, t_CH347I2C_SetAckClk_DelayuS)
SETTER(StreamI2C, t_CH347StreamI2C)
SETTER(StreamI2CRetAck, t_CH347StreamI2C_RetAck)
SETTER(ReadEEPROM, t_CH347ReadEEPROM)
SETTER(WriteEEPROM, t_CH347WriteEEPROM)
SETTER(GetLibInfo, t_CH347GetLibInfo)

#undef SETTER

// ===== 包装函数：Go 通过 C.pXXX 调用这些函数 =====

// 基础设备操作
static int           pOpenDevice(const char* a)          { return _fp_OpenDevice(a); }
static bool          pCloseDevice(int a)                  { return _fp_CloseDevice(a); }
static bool          pSetTimeout(int a, uint32_t b, uint32_t c)  { return _fp_SetTimeout(a,b,c); }
static bool          pGetDriverVersion(int a, unsigned char* b)  { return _fp_GetDriverVersion(a,b); }
static bool          pGetChipVersion(int a, unsigned char* b)    { return _fp_GetChipVersion(a,b); }
static bool          pGetChipType(int a, void* b)         { return _fp_GetChipType(a,b); }
static bool          pGetDeviceID(int a, uint32_t* b)     { return _fp_GetDeviceID(a,b); }
static bool          pOEEnable(int a)                     { return _fp_OEEnable(a); }

// SPI
static bool          pSPIGetHwStreamCfg(int a, void* b)   { return _fp_SPIGetHwStreamCfg(a,b); }
static bool          pSPISetFrequency(int a, uint32_t b)   { return _fp_SPISetFrequency(a,b); }
static bool          pSPISetAutoCS(int a, bool b)          { return _fp_SPISetAutoCS(a,b); }
static bool          pSPISetDataBits(int a, uint8_t b)     { return _fp_SPISetDataBits(a,b); }
static bool          pSPIInit(int a, void* b)              { return _fp_SPIInit(a,b); }
static bool          pSPIGetCfg(int a, void* b)            { return _fp_SPIGetCfg(a,b); }
static bool          pSPIChangeCS(int a, uint8_t b)        { return _fp_SPIChangeCS(a,b); }
static bool          pSPIWrite(int a, bool b, uint8_t c, int d, int e, void* f) { return _fp_SPIWrite(a,b,c,d,e,f); }
static bool          pSPIRead(int a, bool b, uint8_t c, int d, uint32_t* e, void* f) { return _fp_SPIRead(a,b,c,d,e,f); }
static bool          pSPIWriteRead(int a, bool b, uint8_t c, int d, void* e) { return _fp_SPIWriteRead(a,b,c,d,e); }

// JTAG
static int           pJtagReset(int a)                    { return _fp_JtagReset(a); }
static bool          pJtagResetTrst(int a, bool b)        { return _fp_JtagResetTrst(a,b); }
static bool          pJtagInit(int a, uint8_t b)          { return _fp_JtagInit(a,b); }
static bool          pJtagGetCfg(int a, uint8_t* b)       { return _fp_JtagGetCfg(a,b); }
static uint32_t      pJtagClockTms(uint8_t* a, uint32_t b, uint32_t c) { return _fp_JtagClockTms(a,b,c); }
static uint32_t      pJtagIdleClock(uint8_t* a, uint32_t b) { return _fp_JtagIdleClock(a,b); }
static bool          pJtagTmsChange(int a, uint8_t* b, uint32_t c, uint32_t d) { return _fp_JtagTmsChange(a,b,c,d); }
static bool          pJtagIoScan(int a, uint8_t* b, uint32_t c, bool d) { return _fp_JtagIoScan(a,b,c,d); }
static bool          pJtagIoScanT(int a, uint8_t* b, uint32_t c, bool d, bool e) { return _fp_JtagIoScanT(a,b,c,d,e); }
static bool          pJtagWriteRead(int a, bool b, int c, void* d, uint32_t* e, void* f) { return _fp_JtagWriteRead(a,b,c,d,e,f); }
static bool          pJtagWriteReadFast(int a, bool b, int c, void* d, uint32_t* e, void* f) { return _fp_JtagWriteReadFast(a,b,c,d,e,f); }
static bool          pJtagSwitchTapState(int a, uint8_t b) { return _fp_JtagSwitchTapState(a,b); }
static bool          pJtagByteWriteDR(int a, int b, void* c) { return _fp_JtagByteWriteDR(a,b,c); }
static bool          pJtagByteReadDR(int a, uint32_t* b, void* c) { return _fp_JtagByteReadDR(a,b,c); }
static bool          pJtagByteWriteIR(int a, int b, void* c) { return _fp_JtagByteWriteIR(a,b,c); }
static bool          pJtagByteReadIR(int a, uint32_t* b, void* c) { return _fp_JtagByteReadIR(a,b,c); }
static bool          pJtagBitWriteDR(int a, int b, void* c) { return _fp_JtagBitWriteDR(a,b,c); }
static bool          pJtagBitWriteIR(int a, int b, void* c) { return _fp_JtagBitWriteIR(a,b,c); }
static bool          pJtagBitReadIR(int a, uint32_t* b, void* c) { return _fp_JtagBitReadIR(a,b,c); }
static bool          pJtagBitReadDR(int a, uint32_t* b, void* c) { return _fp_JtagBitReadDR(a,b,c); }

// GPIO
static bool          pGPIOGet(int a, uint8_t* b, uint8_t* c) { return _fp_GPIOGet(a,b,c); }
static bool          pGPIOSet(int a, uint8_t b, uint8_t c, uint8_t d) { return _fp_GPIOSet(a,b,c,d); }
static bool          pGPIOIRQSet(int a, uint8_t b, bool c, uint8_t d, void* e) { return _fp_GPIOIRQSet(a,b,c,d,e); }

// UART
static int           pUartOpen(const char* a)             { return _fp_UartOpen(a); }
static bool          pUartClose(int a)                     { return _fp_UartClose(a); }
static bool          pUartGetCfg(int a, uint32_t* b, uint8_t* c, uint8_t* d, uint8_t* e, uint8_t* f) { return _fp_UartGetCfg(a,b,c,d,e,f); }
static bool          pUartInit(int a, int b, uint8_t c, uint8_t d, uint8_t e, uint8_t f) { return _fp_UartInit(a,b,c,d,e,f); }
static bool          pUartRead(int a, void* b, uint32_t* c) { return _fp_UartRead(a,b,c); }
static bool          pUartWrite(int a, void* b, uint32_t* c) { return _fp_UartWrite(a,b,c); }

// I2C
static bool          pI2CSet(int a, int b)                { return _fp_I2CSet(a,b); }
static bool          pI2CSetStretch(int a, bool b)         { return _fp_I2CSetStretch(a,b); }
static bool          pI2CSetDriveMode(int a, uint8_t b)    { return _fp_I2CSetDriveMode(a,b); }
static bool          pI2CSetIgnoreNack(int a, uint8_t b)   { return _fp_I2CSetIgnoreNack(a,b); }
static bool          pI2CSetDelaymS(int a, int b)          { return _fp_I2CSetDelaymS(a,b); }
static bool          pI2CSetAckClkDelayuS(int a, int b)    { return _fp_I2CSetAckClkDelayuS(a,b); }
static bool          pStreamI2C(int a, int b, void* c, int d, void* e) { return _fp_StreamI2C(a,b,c,d,e); }
static bool          pStreamI2CRetAck(int a, int b, void* c, int d, void* e, int* f) { return _fp_StreamI2CRetAck(a,b,c,d,e,f); }

// EEPROM
static bool          pReadEEPROM(int a, int b, int c, int d, uint8_t* e) { return _fp_ReadEEPROM(a,b,c,d,e); }
static bool          pWriteEEPROM(int a, int b, int c, int d, uint8_t* e) { return _fp_WriteEEPROM(a,b,c,d,e); }

// Lib Info
static const char*   pGetLibInfo(void)                    { return _fp_GetLibInfo(); }

*/
import "C"
import (
	"circuit/gpio/driver"
	"circuit/gpio/driver/ch34x/lib"
	"circuit/gpio/driver/dynlib"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"
)

const (
	ErrInvalid = -1
	ErrRange   = -2
	ErrIoctl   = -3

	CH347SPIMaxFreq = 60000000
	CH347SPIMinFreq = 218750

	IRQTypeNone        = 0
	IRQTypeEdgeRising  = 1
	IRQTypeEdgeFalling = 2
	IRQTypeEdgeBoth    = 3

	TypeTTY = 0
	TypeHID = 1
	TypeVCP = 2

	ChipMode0 = 0
	ChipMode1 = 1
	ChipMode2 = 2
	ChipMode3 = 3
)

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

type ChipType int

const (
	ChipCH341 ChipType = iota
	ChipCH347T
	ChipCH347F
	ChipCH339W
	ChipCH346C
)

type FuncType int

const (
	FuncTypeTTY FuncType = iota
	FuncTypeHID
	FuncTypeVCP
)

type library struct {
	handle unsafe.Pointer
	mu     sync.RWMutex
	closed bool // 防止 dlclose 后调用函数指针导致 SIGSEGV
}

var (
	GlobalLib *library
)

// loadSymbols 使用 dynlib 加载 CH34x 动态库的全部函数指针符号。
// 返回加载失败的符号数量（0 = 全部成功）。
func loadSymbols(h dynlib.Handle) int {
	missing := 0
	load := func(name string, setter func(unsafe.Pointer)) {
		ptr, err := dynlib.Lookup(h, name)
		if err != nil {
			missing++
			return
		}
		setter(ptr)
	}

	// 基础设备操作
	load("CH347OpenDevice", func(p unsafe.Pointer) { C.set_OpenDevice(p) })
	load("CH347CloseDevice", func(p unsafe.Pointer) { C.set_CloseDevice(p) })
	load("CH34xSetTimeout", func(p unsafe.Pointer) { C.set_SetTimeout(p) })
	load("CH34x_GetDriverVersion", func(p unsafe.Pointer) { C.set_GetDriverVersion(p) })
	load("CH34x_GetChipVersion", func(p unsafe.Pointer) { C.set_GetChipVersion(p) })
	load("CH34x_GetChipType", func(p unsafe.Pointer) { C.set_GetChipType(p) })
	load("CH34X_GetDeviceID", func(p unsafe.Pointer) { C.set_GetDeviceID(p) })
	load("CH347_OE_Enable", func(p unsafe.Pointer) { C.set_OEEnable(p) })

	// SPI
	load("CH347SPI_GetHwStreamCfg", func(p unsafe.Pointer) { C.set_SPIGetHwStreamCfg(p) })
	load("CH347SPI_SetFrequency", func(p unsafe.Pointer) { C.set_SPISetFrequency(p) })
	load("CH347SPI_SetAutoCS", func(p unsafe.Pointer) { C.set_SPISetAutoCS(p) })
	load("CH347SPI_SetDataBits", func(p unsafe.Pointer) { C.set_SPISetDataBits(p) })
	load("CH347SPI_Init", func(p unsafe.Pointer) { C.set_SPIInit(p) })
	load("CH347SPI_GetCfg", func(p unsafe.Pointer) { C.set_SPIGetCfg(p) })
	load("CH347SPI_ChangeCS", func(p unsafe.Pointer) { C.set_SPIChangeCS(p) })
	load("CH347SPI_Write", func(p unsafe.Pointer) { C.set_SPIWrite(p) })
	load("CH347SPI_Read", func(p unsafe.Pointer) { C.set_SPIRead(p) })
	load("CH347SPI_WriteRead", func(p unsafe.Pointer) { C.set_SPIWriteRead(p) })

	// JTAG
	load("CH347Jtag_Reset", func(p unsafe.Pointer) { C.set_JtagReset(p) })
	load("CH347Jtag_ResetTrst", func(p unsafe.Pointer) { C.set_JtagResetTrst(p) })
	load("CH347Jtag_INIT", func(p unsafe.Pointer) { C.set_JtagInit(p) })
	load("CH347Jtag_GetCfg", func(p unsafe.Pointer) { C.set_JtagGetCfg(p) })
	load("CH347Jtag_ClockTms", func(p unsafe.Pointer) { C.set_JtagClockTms(p) })
	load("CH347Jtag_IdleClock", func(p unsafe.Pointer) { C.set_JtagIdleClock(p) })
	load("CH347Jtag_TmsChange", func(p unsafe.Pointer) { C.set_JtagTmsChange(p) })
	load("CH347Jtag_IoScan", func(p unsafe.Pointer) { C.set_JtagIoScan(p) })
	load("CH347Jtag_IoScanT", func(p unsafe.Pointer) { C.set_JtagIoScanT(p) })
	load("CH347Jtag_WriteRead", func(p unsafe.Pointer) { C.set_JtagWriteRead(p) })
	load("CH347Jtag_WriteRead_Fast", func(p unsafe.Pointer) { C.set_JtagWriteReadFast(p) })
	load("CH347Jtag_SwitchTapState", func(p unsafe.Pointer) { C.set_JtagSwitchTapState(p) })
	load("CH347Jtag_ByteWriteDR", func(p unsafe.Pointer) { C.set_JtagByteWriteDR(p) })
	load("CH347Jtag_ByteReadDR", func(p unsafe.Pointer) { C.set_JtagByteReadDR(p) })
	load("CH347Jtag_ByteWriteIR", func(p unsafe.Pointer) { C.set_JtagByteWriteIR(p) })
	load("CH347Jtag_ByteReadIR", func(p unsafe.Pointer) { C.set_JtagByteReadIR(p) })
	load("CH347Jtag_BitWriteDR", func(p unsafe.Pointer) { C.set_JtagBitWriteDR(p) })
	load("CH347Jtag_BitWriteIR", func(p unsafe.Pointer) { C.set_JtagBitWriteIR(p) })
	load("CH347Jtag_BitReadIR", func(p unsafe.Pointer) { C.set_JtagBitReadIR(p) })
	load("CH347Jtag_BitReadDR", func(p unsafe.Pointer) { C.set_JtagBitReadDR(p) })

	// GPIO
	load("CH347GPIO_Get", func(p unsafe.Pointer) { C.set_GPIOGet(p) })
	load("CH347GPIO_Set", func(p unsafe.Pointer) { C.set_GPIOSet(p) })
	load("CH347GPIO_IRQ_Set", func(p unsafe.Pointer) { C.set_GPIOIRQSet(p) })

	// UART
	load("CH347Uart_Open", func(p unsafe.Pointer) { C.set_UartOpen(p) })
	load("CH347Uart_Close", func(p unsafe.Pointer) { C.set_UartClose(p) })
	load("CH347Uart_GetCfg", func(p unsafe.Pointer) { C.set_UartGetCfg(p) })
	load("CH347Uart_Init", func(p unsafe.Pointer) { C.set_UartInit(p) })
	load("CH347Uart_Read", func(p unsafe.Pointer) { C.set_UartRead(p) })
	load("CH347Uart_Write", func(p unsafe.Pointer) { C.set_UartWrite(p) })

	// I2C
	load("CH347I2C_Set", func(p unsafe.Pointer) { C.set_I2CSet(p) })
	load("CH347I2C_SetStretch", func(p unsafe.Pointer) { C.set_I2CSetStretch(p) })
	load("CH347I2C_SetDriveMode", func(p unsafe.Pointer) { C.set_I2CSetDriveMode(p) })
	load("CH347I2C_SetIgnoreNack", func(p unsafe.Pointer) { C.set_I2CSetIgnoreNack(p) })
	load("CH347I2C_SetDelaymS", func(p unsafe.Pointer) { C.set_I2CSetDelaymS(p) })
	load("CH347I2C_SetAckClk_DelayuS", func(p unsafe.Pointer) { C.set_I2CSetAckClkDelayuS(p) })
	load("CH347StreamI2C", func(p unsafe.Pointer) { C.set_StreamI2C(p) })
	load("CH347StreamI2C_RetAck", func(p unsafe.Pointer) { C.set_StreamI2CRetAck(p) })

	// EEPROM
	load("CH347ReadEEPROM", func(p unsafe.Pointer) { C.set_ReadEEPROM(p) })
	load("CH347WriteEEPROM", func(p unsafe.Pointer) { C.set_WriteEEPROM(p) })

	// Lib Info
	load("CH347GetLibInfo", func(p unsafe.Pointer) { C.set_GetLibInfo(p) })

	return missing
}

func init() {
	libPath := findLibPath()
	h, err := dynlib.Load(libPath)
	if err != nil {
		panic("ch34x: " + err.Error())
	}

	// Go 层使用 dynlib 加载所有函数指针符号
	missing := loadSymbols(h)
	if missing != 0 {
		dynlib.Close(h)
		panic(fmt.Sprintf("ch34x: 未能解析 CH34x 库 %s 中的 %d 个符号", libPath, int(missing)))
	}

	GlobalLib = &library{handle: unsafe.Pointer(h)}
}

type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("ch34x error %d: %s", e.Code, e.Message)
}

// Close 释放动态库句柄。调用后 GlobalLib 不可再使用。
// 幂等关闭：重复调用安全。
func (lib *library) Close() error {
	lib.mu.Lock()
	defer lib.mu.Unlock()
	if lib.closed {
		return nil
	}
	lib.closed = true
	if lib.handle != nil {
		if err := dynlib.Close(dynlib.Handle(lib.handle)); err != nil {
			return fmt.Errorf("ch34x: %w", err)
		}
		lib.handle = nil
	}
	return nil
}

func (lib *library) GetLibInfo() string {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	cstr := C.pGetLibInfo()
	if cstr == nil {
		return ""
	}
	return C.GoString(cstr)
}

func (lib *library) OpenDevice(path string) (int, error) {
	lib.mu.RLock()
	// 防止在已卸载的共享库上调用函数指针导致段错误
	defer lib.mu.RUnlock()
	if lib.closed {
		return 0, errors.New("ch34x: library is closed")
	}
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	fd := C.pOpenDevice(cpath)
	if fd < 0 {
		return int(fd), &Error{Code: int(fd), Message: "failed to open device"}
	}
	return int(fd), nil
}

func (lib *library) CloseDevice(fd int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pCloseDevice(C.int(fd))
	if !success {
		return errors.New("failed to close device")
	}
	return nil
}

func (lib *library) SetTimeout(fd int, writeTimeout, readTimeout uint32) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pSetTimeout(C.int(fd), C.uint32_t(writeTimeout), C.uint32_t(readTimeout))
	if !success {
		return errors.New("failed to set timeout")
	}
	return nil
}

func (lib *library) GetDriverVersion(fd int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var version [256]byte
	success := C.pGetDriverVersion(C.int(fd), (*C.uchar)(unsafe.Pointer(&version[0])))
	if !success {
		return nil, errors.New("failed to get driver version")
	}
	for i := 0; i < len(version); i++ {
		if version[i] == 0 {
			return version[:i], nil
		}
	}
	return version[:], nil
}

func (lib *library) GetChipVersion(fd int) (byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var version byte
	success := C.pGetChipVersion(C.int(fd), (*C.uchar)(unsafe.Pointer(&version)))
	if !success {
		return 0, errors.New("failed to get chip version")
	}
	return version, nil
}

func (lib *library) GetChipType(fd int) (ChipType, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var chipType int
	success := C.pGetChipType(C.int(fd), unsafe.Pointer(&chipType))
	if !success {
		return ChipCH341, errors.New("failed to get chip type")
	}
	return ChipType(chipType), nil
}

func (lib *library) GetDeviceID(fd int) (uint32, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var id uint32
	success := C.pGetDeviceID(C.int(fd), (*C.uint32_t)(unsafe.Pointer(&id)))
	if !success {
		return 0, errors.New("failed to get device ID")
	}
	return id, nil
}

func (lib *library) SPISetFrequency(fd int, freqHz uint32) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pSPISetFrequency(C.int(fd), C.uint32_t(freqHz))
	if !success {
		return errors.New("failed to set SPI frequency")
	}
	return nil
}

func (lib *library) SPIInit(fd int, cfg *driver.SPIConfig) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if lib.closed {
		return errors.New("ch34x: library is closed")
	}
	success := C.pSPIInit(C.int(fd), unsafe.Pointer(cfg))
	if !success {
		return errors.New("failed to initialize SPI")
	}
	return nil
}

func (lib *library) SPIWrite(fd int, ignoreCS bool, chipSelect uint8, data []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil
	}
	success := C.pSPIWrite(
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

func (lib *library) SPIRead(fd int, ignoreCS bool, chipSelect uint8, length int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if length <= 0 {
		return nil, errors.New("invalid read length")
	}
	buffer := make([]byte, length)
	var outLength uint32
	success := C.pSPIRead(
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

func (lib *library) SPIWriteRead(fd int, ignoreCS bool, chipSelect uint8, data []byte) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil, nil
	}
	buffer := make([]byte, len(data))
	copy(buffer, data)
	success := C.pSPIWriteRead(
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

func (lib *library) GPIOGet(fd int) (dir, data uint8, err error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var cDir, cData uint8
	success := C.pGPIOGet(C.int(fd), (*C.uchar)(unsafe.Pointer(&cDir)), (*C.uchar)(unsafe.Pointer(&cData)))
	if !success {
		return 0, 0, errors.New("failed to get GPIO status")
	}
	return cDir, cData, nil
}

func (lib *library) GPIOSet(fd int, enable, dirOut, dataOut uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pGPIOSet(C.int(fd), C.uchar(enable), C.uchar(dirOut), C.uchar(dataOut))
	if !success {
		return errors.New("failed to set GPIO")
	}
	return nil
}

func (lib *library) UartOpen(path string) (int, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	fd := C.pUartOpen(cpath)
	if fd < 0 {
		return int(fd), &Error{Code: int(fd), Message: "failed to open UART device"}
	}
	return int(fd), nil
}

func (lib *library) UartClose(fd int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pUartClose(C.int(fd))
	if !success {
		return errors.New("failed to close UART device")
	}
	return nil
}

func (lib *library) UartInit(fd, baudRate int, byteSize, parity, stopBits, byteTimeout uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if lib.closed {
		return errors.New("ch34x: library is closed")
	}
	success := C.pUartInit(C.int(fd), C.int(baudRate), C.uchar(byteSize), C.uchar(parity), C.uchar(stopBits), C.uchar(byteTimeout))
	if !success {
		return errors.New("failed to initialize UART")
	}
	return nil
}

func (lib *library) UartRead(fd int, length int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if length <= 0 {
		return nil, errors.New("invalid read length")
	}
	buffer := make([]byte, length)
	var outLength uint32 = uint32(length)
	success := C.pUartRead(C.int(fd), unsafe.Pointer(&buffer[0]), (*C.uint32_t)(unsafe.Pointer(&outLength)))
	if !success {
		return nil, errors.New("failed to read from UART")
	}
	if int(outLength) < length {
		buffer = buffer[:outLength]
	}
	return buffer, nil
}

func (lib *library) UartWrite(fd int, data []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil
	}
	var outLength uint32 = uint32(len(data))
	success := C.pUartWrite(C.int(fd), unsafe.Pointer(&data[0]), (*C.uint32_t)(unsafe.Pointer(&outLength)))
	if !success || int(outLength) != len(data) {
		return errors.New("failed to write to UART")
	}
	return nil
}

func (lib *library) I2CSet(fd int, mode int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pI2CSet(C.int(fd), C.int(mode))
	if !success {
		return errors.New("failed to configure I2C")
	}
	return nil
}

func (lib *library) i2cSetStretch(fd int, enable bool) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pI2CSetStretch(C.int(fd), C.bool(enable))
	if !success {
		return errors.New("failed to set I2C stretch")
	}
	return nil
}

func (lib *library) i2cSetDriveMode(fd int, mode uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pI2CSetDriveMode(C.int(fd), C.uchar(mode))
	if !success {
		return errors.New("failed to set I2C drive mode")
	}
	return nil
}

func (lib *library) i2cSetIgnoreNack(fd int, mode uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pI2CSetIgnoreNack(C.int(fd), C.uchar(mode))
	if !success {
		return errors.New("failed to set I2C ignore NACK")
	}
	return nil
}

func (lib *library) i2cSetDelayMS(fd int, delay int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pI2CSetDelaymS(C.int(fd), C.int(delay))
	if !success {
		return errors.New("failed to set I2C delay")
	}
	return nil
}

func (lib *library) i2cSetAckClkDelay(fd int, delay int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pI2CSetAckClkDelayuS(C.int(fd), C.int(delay))
	if !success {
		return errors.New("failed to set I2C ACK clock delay")
	}
	return nil
}

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
	success := C.pStreamI2CRetAck(C.int(fd), C.int(writeLen), writePtr, C.int(readLength), readPtr, (*C.int)(unsafe.Pointer(&retAck)))
	if !success {
		return nil, 0, errors.New("failed to perform stream I2C operation with ACK")
	}
	return readBuffer, retAck, nil
}

func (lib *library) StreamI2C(fd int, writeData []byte, readLength int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if lib.closed {
		return nil, errors.New("ch34x: library is closed")
	}
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
	success := C.pStreamI2C(C.int(fd), C.int(writeLen), writePtr, C.int(readLength), readPtr)
	if !success {
		return nil, errors.New("failed to perform stream I2C operation")
	}
	return readBuffer, nil
}

func (lib *library) JtagReset(fd int) (int, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	result := C.pJtagReset(C.int(fd))
	if result < 0 {
		return int(result), &Error{Code: int(result), Message: "failed to reset JTAG"}
	}
	return int(result), nil
}

func (lib *library) JtagInit(fd int, clockRate uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pJtagInit(C.int(fd), C.uchar(clockRate))
	if !success {
		return errors.New("failed to initialize JTAG")
	}
	return nil
}

func (lib *library) JtagSwitchTapState(fd int, tapState uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pJtagSwitchTapState(C.int(fd), C.uchar(tapState))
	if !success {
		return errors.New("failed to switch JTAG tap state")
	}
	return nil
}

func (lib *library) JtagWriteRead(fd int, isDR bool, writeData []byte) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(writeData) == 0 {
		return nil, nil
	}
	var readLength uint32
	readBuffer := make([]byte, len(writeData))
	success := C.pJtagWriteRead(
		C.int(fd),
		C.bool(isDR),
		C.int(len(writeData)*8),
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

func (lib *library) JtagByteWriteDR(fd int, data []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil
	}
	success := C.pJtagByteWriteDR(C.int(fd), C.int(len(data)), unsafe.Pointer(&data[0]))
	if !success {
		return errors.New("failed to write JTAG DR data")
	}
	return nil
}

func (lib *library) JtagByteReadDR(fd int, length int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if length <= 0 {
		return nil, errors.New("invalid read length")
	}
	buffer := make([]byte, length)
	var readLength uint32
	success := C.pJtagByteReadDR(C.int(fd), (*C.uint32_t)(unsafe.Pointer(&readLength)), unsafe.Pointer(&buffer[0]))
	if !success {
		return nil, errors.New("failed to read JTAG DR data")
	}
	if int(readLength) < length {
		buffer = buffer[:readLength]
	}
	return buffer, nil
}

func (lib *library) ReadEEPROM(fd int, eepromType EEPROMType, addr, length int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if length <= 0 {
		return nil, errors.New("invalid read length")
	}
	buffer := make([]byte, length)
	success := C.pReadEEPROM(C.int(fd), C.int(eepromType), C.int(addr), C.int(length), (*C.uchar)(unsafe.Pointer(&buffer[0])))
	if !success {
		return nil, errors.New("failed to read EEPROM")
	}
	return buffer, nil
}

func (lib *library) WriteEEPROM(fd int, eepromType EEPROMType, addr int, data []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil
	}
	success := C.pWriteEEPROM(C.int(fd), C.int(eepromType), C.int(addr), C.int(len(data)), (*C.uchar)(unsafe.Pointer(&data[0])))
	if !success {
		return errors.New("failed to write EEPROM")
	}
	return nil
}

func (lib *library) OEEnable(fd int) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pOEEnable(C.int(fd))
	if !success {
		return errors.New("failed to enable output enable")
	}
	return nil
}

func (lib *library) SPIGetHwStreamCfg(fd int, streamCfg unsafe.Pointer) error {
	if streamCfg == nil {
		return errors.New("streamCfg cannot be nil")
	}
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pSPIGetHwStreamCfg(C.int(fd), streamCfg)
	if !success {
		return errors.New("failed to get SPI hardware stream configuration")
	}
	return nil
}

func (lib *library) SPISetAutoCS(fd int, disable bool) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pSPISetAutoCS(C.int(fd), C.bool(disable))
	if !success {
		return errors.New("failed to set SPI auto chip select")
	}
	return nil
}

func (lib *library) SPISetDataBits(fd int, dataBits uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pSPISetDataBits(C.int(fd), C.uchar(dataBits))
	if !success {
		return errors.New("failed to set SPI data bits")
	}
	return nil
}

func (lib *library) SPIGetCfg(fd int, spiCfg unsafe.Pointer) error {
	if spiCfg == nil {
		return errors.New("spiCfg cannot be nil")
	}
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pSPIGetCfg(C.int(fd), spiCfg)
	if !success {
		return errors.New("failed to get SPI configuration")
	}
	return nil
}

func (lib *library) SPIChangeCS(fd int, status uint8) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pSPIChangeCS(C.int(fd), C.uchar(status))
	if !success {
		return errors.New("failed to change SPI chip select")
	}
	return nil
}

func (lib *library) JtagResetTrst(fd int, trstLevel bool) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pJtagResetTrst(C.int(fd), C.bool(trstLevel))
	if !success {
		return errors.New("failed to reset JTAG TRST")
	}
	return nil
}

func (lib *library) JtagGetCfg(fd int, clockRate *uint8) error {
	if clockRate == nil {
		return errors.New("clockRate cannot be nil")
	}
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pJtagGetCfg(C.int(fd), (*C.uchar)(unsafe.Pointer(clockRate)))
	if !success {
		return errors.New("failed to get JTAG configuration")
	}
	return nil
}

func (lib *library) JtagClockTms(bitBangPkt []byte, tms, bi uint32) uint32 {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var pktPtr *C.uchar
	if len(bitBangPkt) > 0 {
		pktPtr = (*C.uchar)(unsafe.Pointer(&bitBangPkt[0]))
	}
	return uint32(C.pJtagClockTms(pktPtr, C.uint32_t(tms), C.uint32_t(bi)))
}

func (lib *library) JtagIdleClock(bitBangPkt []byte, bi uint32) uint32 {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var pktPtr *C.uchar
	if len(bitBangPkt) > 0 {
		pktPtr = (*C.uchar)(unsafe.Pointer(&bitBangPkt[0]))
	}
	return uint32(C.pJtagIdleClock(pktPtr, C.uint32_t(bi)))
}

func (lib *library) JtagTmsChange(fd int, tmsValue []byte, step, skip uint32) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var tmsPtr *C.uchar
	if len(tmsValue) > 0 {
		tmsPtr = (*C.uchar)(unsafe.Pointer(&tmsValue[0]))
	}
	success := C.pJtagTmsChange(C.int(fd), tmsPtr, C.uint32_t(step), C.uint32_t(skip))
	if !success {
		return errors.New("failed to change JTAG TMS")
	}
	return nil
}

func (lib *library) JtagIoScan(fd int, dataBits []byte, dataBitsNb uint32, isRead bool) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var dataPtr *C.uchar
	if len(dataBits) > 0 {
		dataPtr = (*C.uchar)(unsafe.Pointer(&dataBits[0]))
	}
	success := C.pJtagIoScan(C.int(fd), dataPtr, C.uint32_t(dataBitsNb), C.bool(isRead))
	if !success {
		return errors.New("failed to perform JTAG IO scan")
	}
	return nil
}

func (lib *library) JtagIoScanT(fd int, dataBits []byte, dataBitsNb uint32, isRead, isLastPkt bool) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var dataPtr *C.uchar
	if len(dataBits) > 0 {
		dataPtr = (*C.uchar)(unsafe.Pointer(&dataBits[0]))
	}
	success := C.pJtagIoScanT(C.int(fd), dataPtr, C.uint32_t(dataBitsNb), C.bool(isRead), C.bool(isLastPkt))
	if !success {
		return errors.New("failed to perform JTAG IO scan with packet flag")
	}
	return nil
}

func (lib *library) JtagWriteReadFast(fd int, isDR bool, writeData []byte) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(writeData) == 0 {
		return nil, nil
	}
	var readLength uint32
	readBuffer := make([]byte, len(writeData))
	success := C.pJtagWriteReadFast(
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

func (lib *library) JtagByteWriteIR(fd int, data []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if len(data) == 0 {
		return nil
	}
	success := C.pJtagByteWriteIR(C.int(fd), C.int(len(data)), unsafe.Pointer(&data[0]))
	if !success {
		return errors.New("failed to write JTAG IR data")
	}
	return nil
}

func (lib *library) JtagByteReadIR(fd int, length int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if length <= 0 {
		return nil, errors.New("invalid read length")
	}
	buffer := make([]byte, length)
	var readLength uint32
	success := C.pJtagByteReadIR(C.int(fd), (*C.uint32_t)(unsafe.Pointer(&readLength)), unsafe.Pointer(&buffer[0]))
	if !success {
		return nil, errors.New("failed to read JTAG IR data")
	}
	if int(readLength) < length {
		buffer = buffer[:readLength]
	}
	return buffer, nil
}

func (lib *library) JtagBitWriteDR(fd int, bitLength int, bitBuffer []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if bitLength <= 0 || len(bitBuffer) == 0 {
		return nil
	}
	success := C.pJtagBitWriteDR(C.int(fd), C.int(bitLength), unsafe.Pointer(&bitBuffer[0]))
	if !success {
		return errors.New("failed to write JTAG DR bit data")
	}
	return nil
}

func (lib *library) JtagBitWriteIR(fd int, bitLength int, bitBuffer []byte) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if bitLength <= 0 || len(bitBuffer) == 0 {
		return nil
	}
	success := C.pJtagBitWriteIR(C.int(fd), C.int(bitLength), unsafe.Pointer(&bitBuffer[0]))
	if !success {
		return errors.New("failed to write JTAG IR bit data")
	}
	return nil
}

func (lib *library) JtagBitReadIR(fd int, bitLength int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if bitLength <= 0 {
		return nil, errors.New("invalid bit length")
	}
	byteLength := (bitLength + 7) / 8
	buffer := make([]byte, byteLength)
	var readBitLength uint32
	success := C.pJtagBitReadIR(C.int(fd), (*C.uint32_t)(unsafe.Pointer(&readBitLength)), unsafe.Pointer(&buffer[0]))
	if !success {
		return nil, errors.New("failed to read JTAG IR bit data")
	}
	actualBytes := (readBitLength + 7) / 8
	if int(actualBytes) < byteLength {
		buffer = buffer[:actualBytes]
	}
	return buffer, nil
}

func (lib *library) JtagBitReadDR(fd int, bitLength int) ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	if bitLength <= 0 {
		return nil, errors.New("invalid bit length")
	}
	byteLength := (bitLength + 7) / 8
	buffer := make([]byte, byteLength)
	var readBitLength uint32
	success := C.pJtagBitReadDR(C.int(fd), (*C.uint32_t)(unsafe.Pointer(&readBitLength)), unsafe.Pointer(&buffer[0]))
	if !success {
		return nil, errors.New("failed to read JTAG DR bit data")
	}
	actualBytes := (readBitLength + 7) / 8
	if int(actualBytes) < byteLength {
		buffer = buffer[:actualBytes]
	}
	return buffer, nil
}

func (lib *library) GPIOIRQSet(fd int, gpioIndex uint8, enable bool, irqType uint8, isrHandler unsafe.Pointer) error {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	success := C.pGPIOIRQSet(C.int(fd), C.uchar(gpioIndex), C.bool(enable), C.uchar(irqType), isrHandler)
	if !success {
		return errors.New("failed to set GPIO interrupt")
	}
	return nil
}

func (lib *library) UartGetCfg(fd int) (baudRate uint32, byteSize, parity, stopBits, byteTimeout uint8, err error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	var cBaudRate C.uint32_t
	var cByteSize, cParity, cStopBits, cByteTimeout C.uchar
	success := C.pUartGetCfg(C.int(fd), &cBaudRate, &cByteSize, &cParity, &cStopBits, &cByteTimeout)
	if !success {
		return 0, 0, 0, 0, 0, errors.New("failed to get UART configuration")
	}
	return uint32(cBaudRate), uint8(cByteSize), uint8(cParity), uint8(cStopBits), uint8(cByteTimeout), nil
}

// findLibPath 定位 CH34x 动态库文件 (.so / .dll)。
// 查找优先级：
//  1. 环境变量 CH34X_LIB_PATH（推荐生产部署使用绝对路径）
//  2. 相对于工作目录的 lib/<arch>/libch347.so
//  3. 系统库路径 /usr/lib/ 和 /usr/local/lib/
//  4. 从二进制内嵌库中提取到临时目录（通过 go:embed 编译时嵌入）
func findLibPath() string {

	// 1. 环境变量优先
	if p := os.Getenv("CH34X_LIB_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	baseDir := archDirName()
	libName := libFileName()

	// 2-3. 工作目录相对路径 及 系统路径
	candidates := []string{
		filepath.Join(baseDir, libName),
		filepath.Join("/usr/lib", libName),
		filepath.Join("/usr/local/lib", libName),
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// 4. 从二进制内嵌库中提取（跨平台自动选择对应架构的 .so）
	if libPath := extractEmbeddedLib(); libPath != "" {
		return libPath
	}

	// 均失败：返回默认路径（让 dlopen 报清晰错误）
	return filepath.Join(baseDir, libName)
}

// extractEmbeddedLib 将编译时内嵌的动态库提取到临时目录并返回路径。
// 若当前平台无内嵌库则返回空字符串。
func extractEmbeddedLib() string {
	tmpDir := filepath.Join(os.TempDir(), "circuit_ch34x")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return ""
	}
	p, err := lib.Extract(tmpDir)
	if err != nil {
		return ""
	}
	return p
}

func archDirName() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return "lib/x64"
	case "linux/386":
		return "lib/x86"
	case "linux/arm64":
		return "lib/aarch64"
	case "linux/arm":
		// ARM 32-bit: 尝试硬浮点, 软浮点需通过 CH34X_LIB_PATH 覆盖
		return "lib/arm-gnueabihf"
	case "windows/amd64":
		return "lib/win"
	case "windows/386":
		return "lib/win"
	default:
		// mips/sw64/openwrt 等非标准平台需通过 CH34X_LIB_PATH 指定路径
		return "lib/x64"
	}
}

func libFileName() string {
	if runtime.GOOS == "windows" {
		if runtime.GOARCH == "amd64" {
			return "CH347DLLA64.dll"
		}
		return "CH347DLL.dll"
	}
	return "libch347.so"
}
