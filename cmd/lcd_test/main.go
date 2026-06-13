package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"time"

	"circuit/gpio/driver/ch34x"
	"circuit/gpio/gui"
	"circuit/gpio/modules/ST7789"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

func main() {
	devicePath := flag.String("device", "/dev/ch34x_pis0", "CH34x 设备路径")
	libFlag := flag.String("lib", "", "CH34x 共享库路径（提示：运行前请设置 CH34X_LIB_PATH 环境变量）")
	flag.Parse()

	if *libFlag != "" {
		fmt.Printf("注意：-lib 标志仅供参考。\n")
		fmt.Printf("      CH34x 库在启动时通过 init() 加载。\n")
		fmt.Printf("      要使用自定义库路径，请在运行前设置 CH34X_LIB_PATH=%s。\n", *libFlag)
	}

	const (
		screenWidth  = 240
		screenHeight = 280
	)
	dtype := ST7789.Lcd1in69

	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║   ST7789 LCD 驱动测试套件             ║")
	fmt.Printf("║   屏幕：1.69\" %dx%d (Lcd1in69)        ║\n", screenWidth, screenHeight)
	fmt.Printf("║   设备：%-27s ║\n", *devicePath)
	fmt.Println("╚══════════════════════════════════════╝")

	fmt.Println("\n[初始化] 正在打开 CH34x SPI/GPIO，设备：", *devicePath)

	if _, err := os.Stat(*devicePath); os.IsNotExist(err) {
		fmt.Printf("       设备 %s 未找到。\n", *devicePath)
		fmt.Println("       CH34x 硬件是否已连接？")
		if *libFlag == "" {
			fmt.Println("       如果找不到共享库，请设置 CH34X_LIB_PATH")
			fmt.Println("       环境变量，或从项目根目录运行。")
		}
		log.Fatalf("设备未找到：%s", *devicePath)
	}

	spi, err := ch34x.OpenSPI(*devicePath)
	if err != nil {
		if *libFlag == "" {
			fmt.Println("       如果找不到共享库，请设置 CH34X_LIB_PATH")
			fmt.Println("       环境变量，或从项目根目录运行。")
		}
		log.Fatalf("在 %s 上打开 SPI 失败：%v", *devicePath, err)
	}

	gpio, err := ch34x.OpenGPIO(*devicePath)
	if err != nil {
		spi.Close()
		if *libFlag == "" {
			fmt.Println("       如果找不到共享库，请设置 CH34X_LIB_PATH")
			fmt.Println("       环境变量，或从项目根目录运行。")
		}
		log.Fatalf("在 %s 上打开 GPIO 失败：%v", *devicePath, err)
	}

	disp := ST7789.NewDriver(spi, gpio, dtype)

	fmt.Println("[初始化] 正在初始化 ST7789 LCD 驱动...")
	if err := disp.Init(); err != nil {
		gpio.Close()
		spi.Close()
		log.Fatalf("初始化失败：%v", err)
	}

	disp.Clear(uint16(gui.White))
	fmt.Println("[初始化] 显示屏已初始化并清屏为白色。")

	// ═══════════════════════════════════════════════
	// 测试 1：绘制基本图形
	// ═══════════════════════════════════════════════
	fmt.Println("\n┌─ 测试 1：绘制基本图形 ───────────────┐")

	paint := gui.NewPaint(screenWidth, screenHeight, gui.Rotate0, gui.White)
	if err := paint.SetImage(make([]gui.Color, screenWidth*screenHeight)); err != nil {
		log.Fatalf("SetImage 失败：%v", err)
	}
	paint.Clear(gui.White)

	fmt.Println("│  正在绘制点...")
	paint.DrawPoint(25, 10, gui.Black, gui.DotPixel1x1, gui.DotFillAround)
	paint.DrawPoint(25, 25, gui.Black, gui.DotPixel2x2, gui.DotFillAround)
	paint.DrawPoint(25, 40, gui.Black, gui.DotPixel3x3, gui.DotFillAround)
	paint.DrawPoint(25, 55, gui.Black, gui.DotPixel4x4, gui.DotFillAround)

	fmt.Println("│  正在绘制矩形...")
	paint.DrawRectangle(40, 10, 90, 60, gui.Blue, gui.DotPixel1x1, gui.DrawFillEmpty)
	paint.DrawRectangle(105, 10, 150, 60, gui.Blue, gui.DotPixel1x1, gui.DrawFillFull)

	fmt.Println("│  正在绘制线条...")
	paint.DrawLine(40, 10, 90, 60, gui.Red, gui.DotPixel1x1, gui.LineStyleSolid)
	paint.DrawLine(90, 10, 40, 60, gui.Red, gui.DotPixel1x1, gui.LineStyleSolid)
	paint.DrawLine(130, 65, 130, 115, gui.Red, gui.DotPixel1x1, gui.LineStyleSolid)
	paint.DrawLine(105, 90, 155, 90, gui.Red, gui.DotPixel1x1, gui.LineStyleSolid)

	fmt.Println("│  正在绘制圆形...")
	paint.DrawCircle(130, 90, 25, gui.Green, gui.DotPixel1x1, gui.DrawFillEmpty)
	paint.DrawCircle(65, 90, 25, gui.Green, gui.DotPixel1x1, gui.DrawFillFull)

	fmt.Println("│  正在绘制文字...")
	fontFace := basicfont.Face7x13

	paint.DrawRectangle(20, 120, 160, 153, gui.Blue, gui.DotPixel1x1, gui.DrawFillFull)
	paint.DrawString(25, 129, "Hello world", fontFace, gui.Blue, gui.Red)

	paint.DrawRectangle(20, 155, 192, 195, gui.Red, gui.DotPixel1x1, gui.DrawFillFull)
	paint.DrawString(21, 164, "WaveShare", fontFace, gui.Red, gui.White)

	paint.DrawString(25, 190, "1234567890", fontFace, gui.White, gui.Green)
	paint.DrawString(25, 215, "Weixue Electronics", fontFace, gui.White, gui.Blue)

	if err := disp.ShowPaint(paint); err != nil {
		log.Fatalf("ShowPaint 失败：%v", err)
	}
	time.Sleep(2 * time.Second)
	fmt.Println("└──────────────────────────────────────┘")

	// ═══════════════════════════════════════════════
	// 测试 2：诗词显示
	// ═══════════════════════════════════════════════
	fmt.Println("\n┌─ 测试 2：诗词显示 ───────────────────┐")

	paint2 := gui.NewPaint(screenWidth, screenHeight, gui.Rotate0, gui.White)
	if err := paint2.SetImage(make([]gui.Color, screenWidth*screenHeight)); err != nil {
		log.Fatalf("SetImage 失败：%v", err)
	}
	paint2.Clear(gui.White)

	paint2.DrawString(60, 30, "Ti Long Yang Xian Qing Cao Hu", fontFace, gui.White, gui.Green)
	paint2.DrawString(100, 50, "by Tang Gong (Yuan Dynasty)", fontFace, gui.White, gui.Gray)

	poemLines := []struct {
		text  string
		color gui.Color
	}{
		{"Xi feng chui lao Dongting bo,", gui.Blue},
		{"Yi ye Xiangjun baifa duo.", gui.Red},
		{"Zui hou buzhi tian zai shui,", gui.Green},
		{"Man chuan qingmeng ya xinghe.", gui.Black},
	}

	y := 90
	for _, line := range poemLines {
		paint2.DrawString(50, y, line.text, fontFace, gui.White, line.color)
		y += 20
	}

	paint2.DrawString(30, 200, "(中文需要 CJK TTF 字体)", fontFace, gui.White, gui.Gray)
	paint2.DrawString(30, 220, "放入 .ttf 文件，例如 Font02.ttf", fontFace, gui.White, gui.Gray)
	paint2.DrawString(30, 240, "并使用 golang.org/x/image/font/sfnt", fontFace, gui.White, gui.Gray)

	if err := disp.ShowPaint(paint2); err != nil {
		log.Fatalf("ShowPaint 失败：%v", err)
	}
	time.Sleep(2 * time.Second)
	fmt.Println("└──────────────────────────────────────┘")

	// ═══════════════════════════════════════════════
	// 测试 3：图像显示
	// ═══════════════════════════════════════════════
	fmt.Println("\n┌─ 测试 3：图像显示 ───────────────────┐")

	testImg := createTestPattern(screenWidth, screenHeight)
	if err := disp.ShowImage(testImg); err != nil {
		log.Fatalf("ShowImage 失败：%v", err)
	}
	time.Sleep(2 * time.Second)

	gradImg := createGradient(screenWidth, screenHeight)
	if err := disp.ShowImage(gradImg); err != nil {
		log.Fatalf("ShowImage 失败：%v", err)
	}
	time.Sleep(2 * time.Second)

	imgPaths := findTestImages()
	for _, imgPath := range imgPaths {
		fmt.Printf("│  正在加载：%s\n", imgPath)
		img, err := loadImage(imgPath)
		if err != nil {
			fmt.Printf("│  警告：%v\n", err)
			continue
		}
		if err := disp.ShowImage(img); err != nil {
			fmt.Printf("│  警告：ShowImage 失败：%v\n", err)
			continue
		}
		time.Sleep(2 * time.Second)
	}
	if len(imgPaths) == 0 {
		fmt.Println("│  (未找到外部图像文件)")
	}
	fmt.Println("└──────────────────────────────────────┘")

	// ═══════════════════════════════════════════════
	// 测试 4：RGB888 到 RGB565 转换
	// ═══════════════════════════════════════════════
	fmt.Println("\n┌─ 测试 4：RGB888 转 RGB565 ───────────┐")
	testRGBConversion()
	fmt.Println("└──────────────────────────────────────┘")

	// 清理
	disp.Clear(uint16(gui.Black))
	time.Sleep(500 * time.Millisecond)
	disp.Close()
	spi.Close()
	gpio.Close()

	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║   所有测试已完成！                     ║")
	fmt.Println("╚══════════════════════════════════════╝")
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("无法打开：%w", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func findTestImages() []string {
	candidates := []string{
		"test.png",
		"test.jpg",
		"../pic/LCD_1inch69_4.jpg",
		"../pic/LCD_1inch69_5.jpg",
		"../pic/LCD_1inch69_6.jpg",
		"../../pic/LCD_1inch69_4.jpg",
	}
	var result []string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			result = append(result, p)
		}
	}
	return result
}

func createTestPattern(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	barH := h / 4
	colors := []color.RGBA{
		{255, 0, 0, 255},
		{0, 255, 0, 255},
		{0, 0, 255, 255},
		{255, 255, 255, 255},
	}
	for i, c := range colors {
		yStart := i * barH
		yEnd := (i + 1) * barH
		if i == len(colors)-1 {
			yEnd = h
		}
		for y := yStart; y < yEnd; y++ {
			for x := 0; x < w; x++ {
				img.Set(x, y, c)
			}
		}
	}
	return img
}

func createGradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := uint8(float64(x) / float64(w-1) * 255)
			g := uint8(float64(y) / float64(h-1) * 255)
			b := uint8(128)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

func testRGBConversion() {
	tests := []struct {
		r, g, b  uint8
		expected uint16
		name     string
	}{
		{255, 255, 255, 0xFFFF, "白色"},
		{0, 0, 0, 0x0000, "黑色"},
		{255, 0, 0, 0xF800, "红色"},
		{0, 255, 0, 0x07E0, "绿色"},
		{0, 0, 255, 0x001F, "蓝色"},
		{255, 255, 0, 0xFFE0, "黄色"},
		{0, 255, 255, 0x07FF, "青色"},
		{255, 0, 255, 0xF81F, "品红"},
		{128, 128, 128, 0x8410, "灰色 ~50%"},
	}

	passed := 0
	for _, tt := range tests {
		result := ST7789.RGB888ToRGB565(tt.r, tt.g, tt.b)
		if result == tt.expected {
			fmt.Printf("│  通过  %-12s RGB(%3d,%3d,%3d) -> 0x%04X\n", tt.name, tt.r, tt.g, tt.b, result)
			passed++
		} else {
			fmt.Printf("│  失败  %-12s RGB(%3d,%3d,%3d) -> 0x%04X (期望 0x%04X)\n", tt.name, tt.r, tt.g, tt.b, result, tt.expected)
		}
	}
	fmt.Printf("│  %d/%d 测试通过\n", passed, len(tests))
}

var _ font.Face = basicfont.Face7x13
