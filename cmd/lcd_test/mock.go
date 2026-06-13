package main

import (
	"circuit/gpio/driver"
	"fmt"
	"image"
	"image/color"
	"sync"
	"unsafe"

	"circuit/gpio/gui"
)

// MockGPIO 实现 driver.GPIO 接口，用于测试。
// 跟踪 DC 引脚状态 (GPIO6)，供模拟 SPI 判断命令/数据模式。
type MockGPIO struct {
	mu      sync.Mutex
	dcState bool // DC 引脚 (GPIO6)：true = 高电平（数据模式）
}

// NewMockGPIO 创建一个新的 MockGPIO。
func NewMockGPIO() *MockGPIO {
	return &MockGPIO{}
}

// Close 为空操作。
func (g *MockGPIO) Close() error { return nil }

// Get 返回当前 GPIO 状态（桩函数）。
func (g *MockGPIO) Get() (dir, data uint8, err error) { return 0, 0, nil }

// Set 更新 GPIO 状态。跟踪 DC 引脚 (GPIO6, 位 0x40)。
func (g *MockGPIO) Set(enable, dirOut, dataOut uint8) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if (enable & 0x40) != 0 { // GPIO6 = DC 引脚
		g.dcState = (dataOut & 0x40) != 0
	}
	return nil
}

// SetIRQ 为空操作。
func (g *MockGPIO) SetIRQ(gpioIndex uint8, enable bool, irqType uint8, handler any) error {
	return nil
}

// IsDCHigh 返回当前 DC 引脚状态（线程安全）。
func (g *MockGPIO) IsDCHigh() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.dcState
}

// MockSPI 实现 driver.SPI 接口，用于测试。
// 通过解码 ST7789 SPI 命令重建帧缓冲区：
//   - CASET (0x2A) / RASET (0x2B)：设置输出窗口
//   - RAMWR (0x2C)：写入 RGB565 格式像素数据
//   - MADCTL (0x36)：跟踪水平/垂直扫描模式
type MockSPI struct {
	mu   sync.Mutex
	gpio *MockGPIO

	// 显示参数（来自 ST7789 驱动类型）
	width         int
	height        int
	xOffset       int
	yOffset       int
	offsetScanDep bool

	// GRAM 坐标下的帧缓冲区
	bufCols int
	bufRows int
	buffer  []uint16 // RGB565 帧缓冲区，按 [row*bufCols+col] 索引

	// SPI 命令状态机
	lastCmd byte
	cmdBuf  []byte // 累积多字节命令的数据字节（CASET/RASET）

	// 当前显示窗口（来自 CASET/RASET）
	colStart, colEnd int
	rowStart, rowEnd int

	// 当前窗口内的像素写入位置（用于 RAMWR）
	pixelX, pixelY int

	// 扫描方向（来自 MADCTL 命令 0x36）
	horizontal bool // true = 水平扫描（数据 0x70）
}

// NewMockSPI 创建一个新的 MockSPI，其帧缓冲区足够容纳 GRAM 寻址。
func NewMockSPI(gpio *MockGPIO, width, height, xOffset, yOffset int, offsetScanDep bool) *MockSPI {
	// 为各种 ST7789 型号预留充足空间
	bufCols := width + 2*xOffset + 20
	bufRows := height + 2*yOffset + 20
	if bufCols < 320 {
		bufCols = 320
	}
	if bufRows < 320 {
		bufRows = 320
	}

	return &MockSPI{
		gpio:           gpio,
		width:          width,
		height:         height,
		xOffset:        xOffset,
		yOffset:        yOffset,
		offsetScanDep:  offsetScanDep,
		bufCols:        bufCols,
		bufRows:        bufRows,
		buffer:         make([]uint16, bufCols*bufRows),
	}
}

// Close 为空操作。
func (s *MockSPI) Close() error { return nil }

// SetFrequency 存储频率设置。
func (s *MockSPI) SetFrequency(freqHz uint32) error { return nil }

// Init 为空操作。
func (s *MockSPI) Init(cfg *driver.SPIConfig) error { return nil }

// Read 返回空数据（桩函数）。
func (s *MockSPI) Read(ignoreCS bool, chipSelect uint8, length int) ([]byte, error) {
	return make([]byte, length), nil
}

// WriteRead 写入数据后返回空读取（桩函数）。
func (s *MockSPI) WriteRead(ignoreCS bool, chipSelect uint8, data []byte) ([]byte, error) {
	_ = s.Write(ignoreCS, chipSelect, data)
	return make([]byte, len(data)), nil
}

// SetAutoCS 为空操作。
func (s *MockSPI) SetAutoCS(disable bool) error { return nil }

// SetDataBits 为空操作。
func (s *MockSPI) SetDataBits(dataBits uint8) error { return nil }

// GetConfig 返回默认配置（桩函数）。
func (s *MockSPI) GetConfig(cfg *driver.SPIConfig) error {
	cfg.Mode = 0
	cfg.Clock = 3 // 7.5MHz
	return nil
}

// ChangeCS 为空操作。
func (s *MockSPI) ChangeCS(status uint8) error { return nil }

// GetHwStreamCfg 返回未实现错误。
func (s *MockSPI) GetHwStreamCfg(streamCfg unsafe.Pointer) error {
	return fmt.Errorf("未实现")
}

// Write 处理 SPI 写入操作。解码 ST7789 协议
// 并从像素数据重建帧缓冲区。
// 当 DC 为低电平（命令模式）：跟踪命令字节。
// 当 DC 为高电平（数据模式）：根据上一个命令处理数据。
func (s *MockSPI) Write(ignoreCS bool, chipSelect uint8, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.gpio.IsDCHigh() {
		// 命令模式
		if len(data) > 0 {
			s.lastCmd = data[0]
			s.cmdBuf = nil
			if s.lastCmd == 0x2C {
				// RAMWR：重置像素写入位置，准备新的内存写入
				s.pixelX = 0
				s.pixelY = 0
			}
		}
	} else {
		// 数据模式 — 根据上一个命令处理
		switch s.lastCmd {
		case 0x36: // MADCTL：内存数据访问控制
			if len(data) > 0 {
				// 0x70 = 水平扫描 (MY=1, MX=1, MV=1)
				// 其他常见值：0x00 = 垂直，0x08 = 垂直带 BGR
				s.horizontal = (data[0] & 0x70) == 0x70
			}
		case 0x2A: // CASET：列地址设置 (4 字节)
			s.cmdBuf = append(s.cmdBuf, data...)
			if len(s.cmdBuf) >= 4 {
				s.colStart = int(s.cmdBuf[0])<<8 | int(s.cmdBuf[1])
				s.colEnd = int(s.cmdBuf[2])<<8 | int(s.cmdBuf[3])
				s.cmdBuf = nil
			}
		case 0x2B: // RASET：行地址设置 (4 字节)
			s.cmdBuf = append(s.cmdBuf, data...)
			if len(s.cmdBuf) >= 4 {
				s.rowStart = int(s.cmdBuf[0])<<8 | int(s.cmdBuf[1])
				s.rowEnd = int(s.cmdBuf[2])<<8 | int(s.cmdBuf[3])
				s.cmdBuf = nil
			}
		case 0x2C: // RAMWR：内存写入 — 像素数据紧随其后
			s.writePixels(data)
		}
		// 其他命令：无需特殊处理
	}
	return nil
}

// writePixels 将 RGB565 像素数据写入帧缓冲区当前窗口位置。
// 当 pixelX 超过窗口宽度时处理行换行。
func (s *MockSPI) writePixels(data []byte) {
	windowWidth := s.colEnd - s.colStart
	for i := 0; i+1 < len(data); i += 2 {
		color565 := uint16(data[i])<<8 | uint16(data[i+1])
		col := s.colStart + s.pixelX
		row := s.rowStart + s.pixelY

		if col >= 0 && col < s.bufCols && row >= 0 && row < s.bufRows {
			s.buffer[row*s.bufCols+col] = color565
		}

		s.pixelX++
		if s.pixelX > windowWidth {
			s.pixelX = 0
			s.pixelY++
		}
	}
}

// ToImage 将捕获的帧缓冲区（GRAM 坐标）转换为可见面板的 image.RGBA。
// 根据扫描方向应用反向偏移映射。
// 返回大小为 width×height 的图像（可见面板区域）。
func (s *MockSPI) ToImage() *image.RGBA {
	s.mu.Lock()
	defer s.mu.Unlock()

	img := image.NewRGBA(image.Rect(0, 0, s.width, s.height))
	for y := 0; y < s.height; y++ {
		for x := 0; x < s.width; x++ {
			var gramCol, gramRow int
			if s.offsetScanDep {
				if s.horizontal {
					gramCol = x + s.xOffset
					gramRow = y
				} else {
					gramCol = x
					gramRow = y + s.yOffset
				}
			} else {
				gramCol = x + s.xOffset
				gramRow = y + s.yOffset
			}

			if gramCol >= 0 && gramCol < s.bufCols && gramRow >= 0 && gramRow < s.bufRows {
				c := s.buffer[gramRow*s.bufCols+gramCol]
				// 将 RGB565 转换为 RGB888
				r := uint8((c >> 11) & 0x1F) << 3
				g := uint8((c >> 5) & 0x3F) << 2
				b := uint8(c & 0x1F) << 3
				img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
	}
	return img
}

// PaintToImage 将 gui.Paint 帧缓冲区转换为 image.RGBA。
// 假设 Rotate0 和 MirrorNone，实现直接的内存到可见区域的映射。
func PaintToImage(p *gui.Paint) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, p.Width, p.Height))
	for y := 0; y < p.Height; y++ {
		for x := 0; x < p.Width; x++ {
			idx := y*p.WidthMemory + x
			if idx < len(p.Image) {
				c := p.Image[idx]
				r := uint8((c >> 11) & 0x1F) << 3
				g := uint8((c >> 5) & 0x3F) << 2
				b := uint8(c & 0x1F) << 3
				img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
	}
	return img
}
