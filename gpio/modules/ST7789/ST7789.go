// Package ST7789 提供使用CH347 SPI接口的1.69英寸LCD显示屏驱动。
package ST7789

import (
	"fmt"
	"image"
	"time"

	"circuit/gpio/driver"
	"circuit/gpio/gui"
)

// GPIO引脚定义
const (
	OLED_DC_Enable_x     = 0x40 // GPIO6
	OLED_DC_SetDirOut    = 0x40 // GPIO6输出
	OLED_DC_OUT_H        = 0x40 // GPIO6高电平
	OLED_DC_OUT_L        = 0x00 // GPIO6低电平
	OLED_Reset_Enable_x  = 0x80 // GPIO7
	OLED_Reset_SetDirOut = 0x80 // GPIO7输出
	OLED_Reset_OUT_H     = 0x80 // GPIO7高电平
	OLED_Reset_OUT_L     = 0x00 // GPIO7低电平
	CS1                  = 0x80 // SPI片选CS1
)

// Driver 显示屏的LCD驱动
type Driver struct {
	*driverTypes
	spi  driver.SPI
	gpio driver.GPIO
}

// DriverType 屏幕类型
type DriverType uint8

// 支持的屏幕类型
const (
	Lcd0in96 DriverType = iota
	Lcd1in14
	Lcd1in28
	Lcd1in3
	Lcd1in47
	Lcd1in54
	Lcd1in69
	Lcd1in8
	Lcd1in9
	Lcd2inch4
	Lcd2inch
)

type driverTypes struct {
	Width               int
	Height              int
	XOffset             int  // X方向偏移量
	YOffset             int  // Y方向偏移量
	OffsetScanDependent bool // 偏移量是否依赖扫描方向（true: horizontal时X加偏移，vertical时Y加偏移；false: 总是加偏移）
	Commands            []struct {
		Cmd  byte
		Data []byte
	}
}

var driverType = map[DriverType]*driverTypes{
	Lcd0in96: {
		Width:               160,
		Height:              80,
		XOffset:             1,
		YOffset:             26,
		OffsetScanDependent: false,
		Commands: []struct {
			Cmd  byte
			Data []byte
		}{
			{0x11, nil},
			{0x21, nil},
			{0x21, nil},
			{0xB1, []byte{0x05, 0x3A, 0x3A}},
			{0xB2, []byte{0x05, 0x3A, 0x3A}},
			{0xB3, []byte{0x05, 0x3A, 0x3A, 0x05, 0x3A, 0x3A}},
			{0xB4, []byte{0x03}},
			{0xC0, []byte{0x62, 0x02, 0x04}},
			{0xC1, []byte{0xC0}},
			{0xC2, []byte{0x0D, 0x00}},
			{0xC3, []byte{0x8D, 0x6A}},
			{0xC4, []byte{0x8D, 0xEE}},
			{0xC5, []byte{0x0E}},
			{0xE0, []byte{0x10, 0x0E, 0x02, 0x03, 0x0E, 0x07, 0x02, 0x07, 0x0A, 0x12, 0x27, 0x37, 0x00, 0x0D, 0x0E, 0x10}},
			{0xE1, []byte{0x10, 0x0E, 0x03, 0x03, 0x0F, 0x06, 0x02, 0x08, 0x0A, 0x13, 0x26, 0x36, 0x00, 0x0D, 0x0E, 0x10}},
			{0x3A, []byte{0x05}},
			{0x36, []byte{0xA8}},
			{0x29, nil},
		},
	},
	Lcd1in14: {
		Width:               135,
		Height:              240,
		XOffset:             40,
		YOffset:             53,
		OffsetScanDependent: true,
		Commands: []struct {
			Cmd  byte
			Data []byte
		}{
			{0x3A, []byte{0x05}},
			{0xB2, []byte{0x0C, 0x0C, 0x00, 0x33, 0x33}},
			{0xB7, []byte{0x35}},
			{0xBB, []byte{0x19}},
			{0xC0, []byte{0x2C}},
			{0xC2, []byte{0x01}},
			{0xC3, []byte{0x12}},
			{0xC4, []byte{0x20}},
			{0xC6, []byte{0x0F}},
			{0xD0, []byte{0xA4, 0xA1}},
			{0xE0, []byte{0xD0, 0x04, 0x0D, 0x11, 0x13, 0x2B, 0x3F, 0x54, 0x4C, 0x18, 0x0D, 0x0B, 0x1F, 0x23}},
			{0xE1, []byte{0xD0, 0x04, 0x0C, 0x11, 0x13, 0x2C, 0x3F, 0x44, 0x51, 0x2F, 0x1F, 0x1F, 0x20, 0x23}},
			{0x21, nil},
			{0x11, nil},
			{0x29, nil},
		},
	},
	Lcd1in28: {
		Width:               240,
		Height:              240,
		XOffset:             0,
		YOffset:             0,
		OffsetScanDependent: true,
		Commands: []struct {
			Cmd  byte
			Data []byte
		}{
			{0xEF, nil},
			{0xEB, []byte{0x14}},
			{0xFE, nil},
			{0xEF, nil},
			{0xEB, []byte{0x14}},
			{0x84, []byte{0x40}},
			{0x85, []byte{0xFF}},
			{0x86, []byte{0xFF}},
			{0x87, []byte{0xFF}},
			{0x88, []byte{0x0A}},
			{0x89, []byte{0x21}},
			{0x8A, []byte{0x00}},
			{0x8B, []byte{0x80}},
			{0x8C, []byte{0x01}},
			{0x8D, []byte{0x01}},
			{0x8E, []byte{0xFF}},
			{0x8F, []byte{0xFF}},
			{0xB6, []byte{0x00, 0x20}},
			{0x36, []byte{0x08}},
			{0x3A, []byte{0x05}},
			{0x90, []byte{0x08, 0x08, 0x08, 0x08}},
			{0xBD, []byte{0x06}},
			{0xBC, []byte{0x00}},
			{0xFF, []byte{0x60, 0x01, 0x04}},
			{0xC3, []byte{0x13}},
			{0xC4, []byte{0x13}},
			{0xC9, []byte{0x22}},
			{0xBE, []byte{0x11}},
			{0xE1, []byte{0x10, 0x0E}},
			{0xDF, []byte{0x21, 0x0C, 0x02}},
			{0xF0, []byte{0x45, 0x09, 0x08, 0x08, 0x26, 0x2A}},
			{0xF1, []byte{0x43, 0x70, 0x72, 0x36, 0x37, 0x6F}},
			{0xF2, []byte{0x45, 0x09, 0x08, 0x08, 0x26, 0x2A}},
			{0xF3, []byte{0x43, 0x70, 0x72, 0x36, 0x37, 0x6F}},
			{0xED, []byte{0x1B, 0x0B}},
			{0xAE, []byte{0x77}},
			{0xCD, []byte{0x63}},
			{0x70, []byte{0x07, 0x07, 0x04, 0x0E, 0x0F, 0x09, 0x07, 0x08, 0x03}},
			{0xE8, []byte{0x34}},
			{0x62, []byte{0x18, 0x0D, 0x71, 0xED, 0x70, 0x70, 0x18, 0x0F, 0x71, 0xEF, 0x70, 0x70}},
			{0x63, []byte{0x18, 0x11, 0x71, 0xF1, 0x70, 0x70, 0x18, 0x13, 0x71, 0xF3, 0x70, 0x70}},
			{0x64, []byte{0x28, 0x29, 0xF1, 0x01, 0xF1, 0x00, 0x07}},
			{0x66, []byte{0x3C, 0x00, 0xCD, 0x67, 0x45, 0x45, 0x10, 0x00, 0x00, 0x00}},
			{0x67, []byte{0x00, 0x3C, 0x00, 0x00, 0x00, 0x01, 0x54, 0x10, 0x32, 0x98}},
			{0x74, []byte{0x10, 0x85, 0x80, 0x00, 0x00, 0x4E, 0x00}},
			{0x98, []byte{0x3E, 0x07}},
			{0x35, nil},
			{0x21, nil},
			{0x11, nil},
			{0x29, nil},
		},
	},
	Lcd1in3: {
		Width:               240,
		Height:              240,
		XOffset:             0,
		YOffset:             0,
		OffsetScanDependent: true,
		Commands: []struct {
			Cmd  byte
			Data []byte
		}{
			{0x3A, []byte{0x05}},
			{0xB2, []byte{0x0C, 0x0C, 0x00, 0x33, 0x33}},
			{0xB7, []byte{0x35}},
			{0xBB, []byte{0x19}},
			{0xC0, []byte{0x2C}},
			{0xC2, []byte{0x01}},
			{0xC3, []byte{0x12}},
			{0xC4, []byte{0x20}},
			{0xC6, []byte{0x0F}},
			{0xD0, []byte{0xA4, 0xA1}},
			{0xE0, []byte{0xD0, 0x04, 0x0D, 0x11, 0x13, 0x2B, 0x3F, 0x54, 0x4C, 0x18, 0x0D, 0x0B, 0x1F, 0x23}},
			{0xE1, []byte{0xD0, 0x04, 0x0C, 0x11, 0x13, 0x2C, 0x3F, 0x44, 0x51, 0x2F, 0x1F, 0x1F, 0x20, 0x23}},
			{0x21, nil},
			{0x11, nil},
			{0x29, nil},
		},
	},
	Lcd1in47: {
		Width:               320,
		Height:              172,
		XOffset:             0x22,
		YOffset:             0x22,
		OffsetScanDependent: true,
		Commands: []struct {
			Cmd  byte
			Data []byte
		}{
			{0x11, nil},
			{0x3A, []byte{0x05}},
			{0xB2, []byte{0x0C, 0x0C, 0x00, 0x33, 0x33}},
			{0xB7, []byte{0x35}},
			{0xBB, []byte{0x35}},
			{0xC0, []byte{0x2C}},
			{0xC2, []byte{0x01}},
			{0xC3, []byte{0x13}},
			{0xC4, []byte{0x20}},
			{0xC6, []byte{0x0F}},
			{0xD0, []byte{0xA4, 0xA1}},
			{0xD6, []byte{0xA1}},
			{0xE0, []byte{0xF0, 0x00, 0x04, 0x04, 0x04, 0x05, 0x29, 0x33, 0x3E, 0x38, 0x12, 0x12, 0x28, 0x30}},
			{0xE1, []byte{0xF0, 0x07, 0x0A, 0x0D, 0x0B, 0x07, 0x28, 0x33, 0x3E, 0x36, 0x14, 0x14, 0x29, 0x32}},
			{0x21, nil},
			{0x11, nil},
			{0x29, nil},
		},
	},
	Lcd1in54: {
		Width:               240,
		Height:              240,
		XOffset:             0,
		YOffset:             0,
		OffsetScanDependent: true,
		Commands: []struct {
			Cmd  byte
			Data []byte
		}{
			{0x3A, []byte{0x05}},
			{0xB2, []byte{0x0C, 0x0C, 0x00, 0x33, 0x33}},
			{0xB7, []byte{0x35}},
			{0xBB, []byte{0x19}},
			{0xC0, []byte{0x2C}},
			{0xC2, []byte{0x01}},
			{0xC3, []byte{0x12}},
			{0xC4, []byte{0x20}},
			{0xC6, []byte{0x0F}},
			{0xD0, []byte{0xA4, 0xA1}},
			{0xE0, []byte{0xD0, 0x04, 0x0D, 0x11, 0x13, 0x2B, 0x3F, 0x54, 0x4C, 0x18, 0x0D, 0x0B, 0x1F, 0x23}},
			{0xE1, []byte{0xD0, 0x04, 0x0C, 0x11, 0x13, 0x2C, 0x3F, 0x44, 0x51, 0x2F, 0x1F, 0x1F, 0x20, 0x23}},
			{0x21, nil},
			{0x11, nil},
			{0x29, nil},
		},
	},
	Lcd1in69: {
		Width:               240,
		Height:              280,
		XOffset:             20,
		YOffset:             20,
		OffsetScanDependent: true,
		Commands: []struct {
			Cmd  byte
			Data []byte
		}{
			{0x36, []byte{0x00}},
			{0x3A, []byte{0x05}},
			{0xB2, []byte{0x0B, 0x0B, 0x00, 0x33, 0x35}},
			{0xB7, []byte{0x11}},
			{0xBB, []byte{0x35}},
			{0xC0, []byte{0x2C}},
			{0xC2, []byte{0x01}},
			{0xC3, []byte{0x0D}},
			{0xC4, []byte{0x20}},
			{0xC6, []byte{0x13}},
			{0xD0, []byte{0xA4, 0xA1}},
			{0xD6, []byte{0xA1}},
			{0xE0, []byte{0xF0, 0x06, 0x0B, 0x0A, 0x09, 0x26, 0x29, 0x33, 0x41, 0x18, 0x16, 0x15, 0x29, 0x2D}},
			{0xE1, []byte{0xF0, 0x04, 0x08, 0x08, 0x07, 0x03, 0x28, 0x32, 0x40, 0x3B, 0x19, 0x18, 0x2A, 0x2E}},
			{0xE4, []byte{0x25, 0x00, 0x00}},
			{0x21, nil},
			{0x11, nil},
			{0x29, nil},
		},
	},
	Lcd1in8: {
		Width:               160,
		Height:              128,
		XOffset:             2,
		YOffset:             1,
		OffsetScanDependent: true,
		Commands: []struct {
			Cmd  byte
			Data []byte
		}{
			{0xB1, []byte{0x01, 0x2C, 0x2D}},
			{0xB2, []byte{0x01, 0x2C, 0x2D}},
			{0xB3, []byte{0x01, 0x2C, 0x2D, 0x01, 0x2C, 0x2D}},
			{0xB4, []byte{0x07}},
			{0xC0, []byte{0xA2, 0x02, 0x84}},
			{0xC1, []byte{0xC5}},
			{0xC2, []byte{0x0A, 0x00}},
			{0xC3, []byte{0x8A, 0x2A}},
			{0xC4, []byte{0x8A, 0xEE}},
			{0xC5, []byte{0x0E}},
			{0xE0, []byte{0x0F, 0x1A, 0x0F, 0x18, 0x2F, 0x28, 0x20, 0x22, 0x1F, 0x1B, 0x23, 0x37, 0x00, 0x07, 0x02, 0x10}},
			{0xE1, []byte{0x0F, 0x1B, 0x0F, 0x17, 0x33, 0x2C, 0x29, 0x2E, 0x30, 0x30, 0x39, 0x3F, 0x00, 0x07, 0x03, 0x10}},
			{0xF0, []byte{0x01}},
			{0xF6, []byte{0x00}},
			{0x3A, []byte{0x05}},
			{0x11, nil},
			{0x29, nil},
		},
	},
	Lcd1in9: {
		Width:               320,
		Height:              170,
		XOffset:             35,
		YOffset:             35,
		OffsetScanDependent: true,
		Commands: []struct {
			Cmd  byte
			Data []byte
		}{
			{0x3A, []byte{0x55}},
			{0xB2, []byte{0x0C, 0x0C, 0x00, 0x33, 0x33}},
			{0xB7, []byte{0x35}},
			{0xBB, []byte{0x13}},
			{0xC0, []byte{0x2C}},
			{0xC2, []byte{0x01}},
			{0xC3, []byte{0x0B}},
			{0xC4, []byte{0x20}},
			{0xC6, []byte{0x0F}},
			{0xD0, []byte{0xA4, 0xA1}},
			{0xD6, []byte{0xA1}},
			{0xE0, []byte{0x00, 0x03, 0x07, 0x08, 0x07, 0x15, 0x2A, 0x44, 0x42, 0x0A, 0x17, 0x18, 0x25, 0x27}},
			{0xE1, []byte{0x00, 0x03, 0x08, 0x07, 0x07, 0x23, 0x2A, 0x43, 0x42, 0x09, 0x18, 0x17, 0x25, 0x27}},
			{0x21, nil},
			{0x11, nil},
			{0x29, nil},
		},
	},
	Lcd2inch: {
		Width:               240,
		Height:              320,
		XOffset:             0,
		YOffset:             0,
		OffsetScanDependent: true,
		Commands: []struct {
			Cmd  byte
			Data []byte
		}{
			{0x36, []byte{0x00}},
			{0x3A, []byte{0x05}},
			{0x21, nil},
			{0x2A, []byte{0x00, 0x00, 0x01, 0x3F}},
			{0x2B, []byte{0x00, 0x00, 0x00, 0xEF}},
			{0xB2, []byte{0x0C, 0x0C, 0x00, 0x33, 0x33}},
			{0xB7, []byte{0x35}},
			{0xBB, []byte{0x1F}},
			{0xC0, []byte{0x2C}},
			{0xC2, []byte{0x01}},
			{0xC3, []byte{0x12}},
			{0xC4, []byte{0x20}},
			{0xC6, []byte{0x0F}},
			{0xD0, []byte{0xA4, 0xA1}},
			{0xE0, []byte{0xD0, 0x08, 0x11, 0x08, 0x0C, 0x15, 0x39, 0x33, 0x50, 0x36, 0x13, 0x14, 0x29, 0x2D}},
			{0xE1, []byte{0xD0, 0x08, 0x10, 0x08, 0x06, 0x06, 0x39, 0x44, 0x51, 0x0B, 0x16, 0x14, 0x2F, 0x31}},
			{0x21, nil},
			{0x11, nil},
			{0x29, nil},
		},
	},
	Lcd2inch4: {
		Width:               240,
		Height:              320,
		XOffset:             0,
		YOffset:             0,
		OffsetScanDependent: true,
		Commands: []struct {
			Cmd  byte
			Data []byte
		}{
			{0x11, nil},
			{0xCF, []byte{0x00, 0xC1, 0x30}},
			{0xED, []byte{0x64, 0x03, 0x12, 0x81}},
			{0xE8, []byte{0x85, 0x00, 0x79}},
			{0xCB, []byte{0x39, 0x2C, 0x00, 0x34, 0x02}},
			{0xF7, []byte{0x20}},
			{0xEA, []byte{0x00, 0x00}},
			{0xC0, []byte{0x1D}},
			{0xC1, []byte{0x12}},
			{0xC5, []byte{0x33, 0x3F}},
			{0xC7, []byte{0x92}},
			{0x3A, []byte{0x55}},
			{0x36, []byte{0x08}},
			{0xB1, []byte{0x00, 0x12}},
			{0xB6, []byte{0x0A, 0xA2}},
			{0x44, []byte{0x02}},
			{0xF2, []byte{0x00}},
			{0x26, []byte{0x01}},
			{0xE0, []byte{0x0F, 0x22, 0x1C, 0x1B, 0x08, 0x0F, 0x48, 0xB8, 0x34, 0x05, 0x0C, 0x09, 0x0F, 0x07, 0x00}},
			{0xE1, []byte{0x00, 0x23, 0x24, 0x07, 0x10, 0x07, 0x38, 0x47, 0x4B, 0x0A, 0x13, 0x06, 0x30, 0x38, 0x0F}},
			{0x29, nil},
		},
	},
}

// NewDriver 创建新的LCD驱动实例
// spi和gpio是SPI和GPIO操作的驱动接口
func NewDriver(spi driver.SPI, gpio driver.GPIO, dtype DriverType) *Driver {
	return &Driver{
		spi:         spi,
		gpio:        gpio,
		driverTypes: driverType[dtype],
	}
}

// Close 关闭LCD驱动。
func (lcd *Driver) Close() error {
	lcd.Reset()
	// 发送休眠指令
	return nil
}

// SPIInit 初始化SPI接口
func (lcd *Driver) SPIInit() error {
	// 设置SPI频率为7.5MHz (Python代码中使用0x00表示60MHz，但这里用7.5MHz更稳定)
	err := lcd.spi.SetFrequency(7500000)
	if err != nil {
		return fmt.Errorf("设置SPI频率失败: %v", err)
	}
	return nil
}

// DCHigh 设置DC引脚高电平（数据模式）
func (lcd *Driver) DCHigh() error {
	return lcd.gpio.Set(OLED_DC_Enable_x, OLED_DC_SetDirOut, OLED_DC_OUT_H)
}

// DCLow 设置DC引脚低电平（命令模式）
func (lcd *Driver) DCLow() error {
	return lcd.gpio.Set(OLED_DC_Enable_x, OLED_DC_SetDirOut, OLED_DC_OUT_L)
}

// ResetHigh 设置复位引脚高电平
func (lcd *Driver) ResetHigh() error {
	return lcd.gpio.Set(OLED_Reset_Enable_x, OLED_Reset_SetDirOut, OLED_Reset_OUT_H)
}

// ResetLow 设置复位引脚低电平
func (lcd *Driver) ResetLow() error {
	return lcd.gpio.Set(OLED_Reset_Enable_x, OLED_Reset_SetDirOut, OLED_Reset_OUT_L)
}

// Reset LCD复位
func (lcd *Driver) Reset() {
	lcd.ResetHigh()
	time.Sleep(100 * time.Millisecond)
	lcd.ResetLow()
	time.Sleep(100 * time.Millisecond)
	lcd.ResetHigh()
	time.Sleep(100 * time.Millisecond)
}

// SendCommand 发送命令到LCD
func (lcd *Driver) SendCommand(cmd byte) error {
	lcd.DCLow()
	return lcd.spi.Write(false, CS1, []byte{cmd})
}

// SendData 发送数据到LCD
func (lcd *Driver) SendData(data []byte) error {
	lcd.DCHigh()
	return lcd.spi.Write(false, CS1, data)
}

// SendDataByte 发送单个数据字节到LCD
func (lcd *Driver) SendDataByte(data byte) error {
	return lcd.SendData([]byte{data})
}

// Init 初始化LCD屏幕
func (lcd *Driver) Init() error {
	if err := lcd.SPIInit(); err != nil {
		return err
	}
	// 发送指令
	for _, cmd := range lcd.Commands {
		if err := lcd.SendCommand(cmd.Cmd); err != nil {
			return fmt.Errorf("发送命令0x%02X失败: %v", cmd.Cmd, err)
		}
		if cmd.Data != nil {
			if err := lcd.SendData(cmd.Data); err != nil {
				return fmt.Errorf("发送命令0x%02X数据失败: %v", cmd.Cmd, err)
			}
		}
		// 如果命令是退出睡眠模式(0x11)，则等待120ms
		if cmd.Cmd == 0x11 {
			time.Sleep(120 * time.Millisecond)
		}
	}
	return nil
}

// SetWindow 设置显示窗口
func (lcd *Driver) SetWindow(xStart, yStart, xEnd, yEnd int, horizontal bool) error {
	xOffset := lcd.XOffset
	yOffset := lcd.YOffset
	if lcd.OffsetScanDependent {
		// 偏移量依赖扫描方向
		if horizontal {
			// 水平扫描：X坐标加偏移，Y坐标不加
			lcd.SendCommand(0x2A)
			lcd.SendData([]byte{
				byte((xStart + xOffset) >> 8),
				byte((xStart + xOffset) & 0xFF),
				byte((xEnd + xOffset - 1) >> 8),
				byte((xEnd + xOffset - 1) & 0xFF),
			})
			lcd.SendCommand(0x2B)
			lcd.SendData([]byte{
				byte(yStart >> 8),
				byte(yStart & 0xFF),
				byte((yEnd - 1) >> 8),
				byte((yEnd - 1) & 0xFF),
			})
		} else {
			// 垂直扫描：Y坐标加偏移，X坐标不加
			lcd.SendCommand(0x2A)
			lcd.SendData([]byte{
				byte(xStart >> 8),
				byte(xStart & 0xFF),
				byte((xEnd - 1) >> 8),
				byte((xEnd - 1) & 0xFF),
			})
			lcd.SendCommand(0x2B)
			lcd.SendData([]byte{
				byte((yStart + yOffset) >> 8),
				byte((yStart + yOffset) & 0xFF),
				byte((yEnd + yOffset - 1) >> 8),
				byte((yEnd + yOffset - 1) & 0xFF),
			})
		}
	} else {
		// 偏移量与扫描方向无关，总是同时添加X和Y偏移
		lcd.SendCommand(0x2A)
		lcd.SendData([]byte{
			byte((xStart + xOffset) >> 8),
			byte((xStart + xOffset) & 0xFF),
			byte((xEnd + xOffset - 1) >> 8),
			byte((xEnd + xOffset - 1) & 0xFF),
		})
		lcd.SendCommand(0x2B)
		lcd.SendData([]byte{
			byte((yStart + yOffset) >> 8),
			byte((yStart + yOffset) & 0xFF),
			byte((yEnd + yOffset - 1) >> 8),
			byte((yEnd + yOffset - 1) & 0xFF),
		})
	}
	// 开始内存写入
	return lcd.SendCommand(0x2C)
}

// Clear 使用指定颜色清屏
func (lcd *Driver) Clear(colorRGB uint16) {
	// 设置全屏窗口
	lcd.SetWindow(0, 0, lcd.Width, lcd.Height, false)
	// 准备颜色数据 (RGB565)
	colorHigh := byte(colorRGB >> 8)
	colorLow := byte(colorRGB & 0xFF)
	// 计算像素总数
	totalPixels := lcd.Width * lcd.Height
	bufferSize := totalPixels * 2 // 每个像素2字节
	// 分段发送数据（每次最多1024字节，即512个像素）
	chunkSize := 1024
	for i := 0; i < bufferSize; i += chunkSize {
		chunkEnd := i + chunkSize
		if chunkEnd > bufferSize {
			chunkEnd = bufferSize
		}
		// 创建数据块
		chunk := make([]byte, chunkEnd-i)
		for j := 0; j < len(chunk); j += 2 {
			chunk[j] = colorHigh
			chunk[j+1] = colorLow
		}
		// 发送数据
		lcd.DCHigh()
		lcd.spi.Write(false, CS1, chunk)
	}
}

// RGB888ToRGB565 将RGB888转换为RGB565格式
func RGB888ToRGB565(r, g, b uint8) uint16 {
	// 取高5位红色，高6位绿色，高5位蓝色
	r5 := uint16(r) >> 3
	g6 := uint16(g) >> 2
	b5 := uint16(b) >> 3
	return (r5 << 11) | (g6 << 5) | b5
}

// DrawRect 绘制填充矩形
func (lcd *Driver) DrawRect(x, y, width, height int, fillColor, borderColor uint16) {
	// 简单实现：绘制填充矩形
	lcd.SetWindow(x, y, x+width, y+height, false)
	totalPixels := width * height
	bufferSize := totalPixels * 2
	// 准备颜色数据
	colorHigh := byte(fillColor >> 8)
	colorLow := byte(fillColor & 0xFF)
	// 分段发送
	chunkSize := 1024
	for i := 0; i < bufferSize; i += chunkSize {
		chunkEnd := i + chunkSize
		if chunkEnd > bufferSize {
			chunkEnd = bufferSize
		}
		chunk := make([]byte, chunkEnd-i)
		for j := 0; j < len(chunk); j += 2 {
			chunk[j] = colorHigh
			chunk[j+1] = colorLow
		}
		lcd.DCHigh()
		lcd.spi.Write(false, CS1, chunk)
	}
}

// ShowImage 在LCD上显示图像
func (lcd *Driver) ShowImage(img image.Image) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	// 设置窗口
	var horizontal bool
	if width == lcd.Height && height == lcd.Width {
		horizontal = true
		lcd.SendCommand(0x36)
		lcd.SendDataByte(0x70)
		lcd.SetWindow(0, 0, lcd.Height, lcd.Width, horizontal)
	} else {
		horizontal = false
		lcd.SendCommand(0x36)
		lcd.SendDataByte(0x00)
		lcd.SetWindow(0, 0, lcd.Width, lcd.Height, horizontal)
	}
	// 将图像转换为RGB565数据
	totalPixels := width * height
	pixelData := make([]byte, totalPixels*2)
	idx := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// 转换为8位
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			// 转换为RGB565
			color565 := RGB888ToRGB565(r8, g8, b8)
			pixelData[idx] = byte(color565 >> 8)
			pixelData[idx+1] = byte(color565 & 0xFF)
			idx += 2
		}
	}
	// 分段发送数据
	chunkSize := 1024
	for i := 0; i < len(pixelData); i += chunkSize {
		chunkEnd := i + chunkSize
		if chunkEnd > len(pixelData) {
			chunkEnd = len(pixelData)
		}
		lcd.DCHigh()
		if err := lcd.spi.Write(false, CS1, pixelData[i:chunkEnd]); err != nil {
			return err
		}
	}
	return nil
}

// ShowPaint 在LCD上显示绘制
func (lcd *Driver) ShowPaint(paint *gui.Paint) error {
	if paint == nil || paint.Image == nil {
		return fmt.Errorf("paint or image is nil")
	}
	// 设置窗口覆盖整个物理屏幕
	// 使用默认水平扫描方向（false），因为我们将按可见坐标顺序发送像素
	horizontal := false
	lcd.SetWindow(0, 0, lcd.Width, lcd.Height, horizontal)
	// 准备像素数据缓冲区：每个像素2字节
	totalPixels := paint.Width * paint.Height
	pixelData := make([]byte, totalPixels*2)
	idx := 0
	for y := 0; y < paint.Height; y++ {
		for x := 0; x < paint.Width; x++ {
			// 应用旋转和镜像变换到内存缓冲区坐标（仿照 gui.Paint.transform）
			xr, yr := x, y
			switch paint.Rotate {
			case gui.Rotate0:
				// 无操作
			case gui.Rotate90:
				xr = paint.WidthMemory - y - 1
				yr = x
			case gui.Rotate180:
				xr = paint.WidthMemory - x - 1
				yr = paint.HeightMemory - y - 1
			case gui.Rotate270:
				xr = y
				yr = paint.HeightMemory - x - 1
			}
			// 应用镜像
			switch paint.Mirror {
			case gui.MirrorNone:
				// 无操作
			case gui.MirrorHorizontal:
				xr = paint.WidthMemory - xr - 1
			case gui.MirrorVertical:
				yr = paint.HeightMemory - yr - 1
			case gui.MirrorOrigin:
				xr = paint.WidthMemory - xr - 1
				yr = paint.HeightMemory - yr - 1
			}
			// 检查边界
			if xr < 0 || xr >= paint.WidthMemory || yr < 0 || yr >= paint.HeightMemory {
				// 如果变换后的坐标越界，使用默认颜色（黑色）
				pixelData[idx] = 0x00
				pixelData[idx+1] = 0x00
			} else {
				color := paint.Image[yr*paint.WidthMemory+xr]
				pixelData[idx] = byte(color >> 8)
				pixelData[idx+1] = byte(color & 0xFF)
			}
			idx += 2
		}
	}
	// 分段发送数据
	chunkSize := 1024
	for i := 0; i < len(pixelData); i += chunkSize {
		chunkEnd := i + chunkSize
		if chunkEnd > len(pixelData) {
			chunkEnd = len(pixelData)
		}
		lcd.DCHigh()
		if err := lcd.spi.Write(false, CS1, pixelData[i:chunkEnd]); err != nil {
			return err
		}
	}
	return nil
}

// DrawPoint 绘制单个像素
func (lcd *Driver) DrawPoint(x, y int, color uint16) error {
	lcd.SetWindow(x, y, x, y, false)
	return lcd.SendData([]byte{byte(color >> 8), byte(color & 0xFF)})
}

// SetBackLight 设置背光亮度（空实现，需硬件支持）
func (lcd *Driver) SetBackLight(value uint16) error {
	// 背光控制需要硬件支持，此处为空实现
	return nil
}
