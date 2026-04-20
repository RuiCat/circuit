// Package gui 提供LCD显示屏的绘图基本功能
package gui

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Color 表示16位RGB565颜色。
type Color uint16

// 预定义颜色（RGB565格式）
const (
	White      Color = 0xFFFF
	Black      Color = 0x0000
	Blue       Color = 0x001F
	BRed       Color = 0xF81F
	GRed       Color = 0xFFE0
	GBlue      Color = 0x07FF
	Red        Color = 0xF800
	Magenta    Color = 0xF81F
	Green      Color = 0x07E0
	Cyan       Color = 0x7FFF
	Yellow     Color = 0xFFE0
	Brown      Color = 0xBC40
	BRRed      Color = 0xFC07
	Gray       Color = 0x8430
	DarkBlue   Color = 0x01CF
	LightBlue  Color = 0x7D7C
	GrayBlue   Color = 0x5458
	LightGreen Color = 0x841F
	LGray      Color = 0xC618
	LGrayBlue  Color = 0xA651
	LBBlue     Color = 0x2B12
)

// ImageBackground 默认背景颜色
const ImageBackground = White

// FontForeground 默认字体前景色
const FontForeground = Black

// FontBackground 默认字体背景色
const FontBackground = White

// Rotate 表示显示旋转。
type Rotate int

const (
	Rotate0   Rotate = 0
	Rotate90  Rotate = 90
	Rotate180 Rotate = 180
	Rotate270 Rotate = 270
)

// Mirror 表示图像镜像。
type Mirror int

const (
	MirrorNone       Mirror = 0x00
	MirrorHorizontal Mirror = 0x01
	MirrorVertical   Mirror = 0x02
	MirrorOrigin     Mirror = 0x03
)

// DotPixel 表示点的大小。
type DotPixel int

const (
	DotPixel1x1 DotPixel = 1
	DotPixel2x2 DotPixel = 2
	DotPixel3x3 DotPixel = 3
	DotPixel4x4 DotPixel = 4
	DotPixel5x5 DotPixel = 5
	DotPixel6x6 DotPixel = 6
	DotPixel7x7 DotPixel = 7
	DotPixel8x8 DotPixel = 8
)

// DotStyle 表示点的填充样式。
type DotStyle int

const (
	DotFillAround  DotStyle = 1 // 点像素围绕中心扩展
	DotFillRightUp DotStyle = 2 // 点像素向右和向上扩展
)

// LineStyle 表示线条样式。
type LineStyle int

const (
	LineStyleSolid  LineStyle = 0
	LineStyleDotted LineStyle = 1
)

// DrawFill 表示形状的填充样式。
type DrawFill int

const (
	DrawFillEmpty DrawFill = 0
	DrawFillFull  DrawFill = 1
)

// Paint 表示绘图上下文。
type Paint struct {
	// 图像缓冲区（16位颜色切片）
	Image []Color
	// 可见区域的宽度（旋转后）
	Width int
	// 可见区域的高度（旋转后）
	Height int
	// 内存缓冲区的宽度（原始方向）
	WidthMemory int
	// 内存缓冲区的高度（原始方向）
	HeightMemory int
	// 默认颜色
	Color Color
	// 当前旋转
	Rotate Rotate
	// 当前镜像
	Mirror Mirror
	// 宽度字节数（用于兼容性，Go版本中未使用）
	WidthByte int
	// 高度字节数（用于兼容性，Go版本中未使用）
	HeightByte int
	// 回调函数
	clearFunc   func(Color)
	displayFunc func(int, int, Color)
}

// PaintTime 表示用于绘制时间的时间结构。
type PaintTime struct {
	Year  uint16
	Month uint8
	Day   uint8
	Hour  uint8
	Min   uint8
	Sec   uint8
}

// NewPaint 使用给定的尺寸和旋转创建一个新的Paint上下文。
// 图像缓冲区必须通过SetImage单独提供。
func NewPaint(width, height int, rotate Rotate, color Color) *Paint {
	p := &Paint{
		WidthMemory:  width,
		HeightMemory: height,
		Color:        color,
		Rotate:       rotate,
		Mirror:       MirrorNone,
		WidthByte:    width,
		HeightByte:   height,
	}
	if rotate == Rotate0 || rotate == Rotate180 {
		p.Width = width
		p.Height = height
	} else {
		p.Width = height
		p.Height = width
	}
	return p
}

// SetImage 设置用于绘图的图像缓冲区。
// 缓冲区必须至少有width*height个元素。
func (p *Paint) SetImage(image []Color) error {
	if len(image) < p.WidthMemory*p.HeightMemory {
		return errors.New("image buffer too small")
	}
	p.Image = image
	return nil
}

// SetClearFunc 设置清除整个显示的函数。
func (p *Paint) SetClearFunc(clear func(Color)) {
	p.clearFunc = clear
}

// SetDisplayFunc 设置显示单个像素的函数。
func (p *Paint) SetDisplayFunc(display func(int, int, Color)) {
	p.displayFunc = display
}

// SetRotate 设置显示的旋转。
func (p *Paint) SetRotate(rotate Rotate) error {
	switch rotate {
	case Rotate0, Rotate90, Rotate180, Rotate270:
		p.Rotate = rotate
		// 根据旋转更新宽度和高度
		if rotate == Rotate0 || rotate == Rotate180 {
			p.Width = p.WidthMemory
			p.Height = p.HeightMemory
		} else {
			p.Width = p.HeightMemory
			p.Height = p.WidthMemory
		}
		return nil
	default:
		return errors.New("rotate must be 0, 90, 180, or 270")
	}
}

// transform 应用旋转和镜像到坐标。
// 返回内存缓冲区空间中的转换后坐标。
func (p *Paint) transform(x, y int) (int, int) {
	// 应用旋转
	var xr, yr int
	switch p.Rotate {
	case Rotate0:
		xr = x
		yr = y
	case Rotate90:
		xr = p.WidthMemory - y - 1
		yr = x
	case Rotate180:
		xr = p.WidthMemory - x - 1
		yr = p.HeightMemory - y - 1
	case Rotate270:
		xr = y
		yr = p.HeightMemory - x - 1
	default:
		return x, y
	}
	// 应用镜像
	switch p.Mirror {
	case MirrorNone:
		// 无操作
	case MirrorHorizontal:
		xr = p.WidthMemory - xr - 1
	case MirrorVertical:
		yr = p.HeightMemory - yr - 1
	case MirrorOrigin:
		xr = p.WidthMemory - xr - 1
		yr = p.HeightMemory - yr - 1
	}
	return xr, yr
}

// SetPixel 使用给定颜色在(x, y)位置设置一个像素。
// 坐标在可见区域空间中（旋转后）。
func (p *Paint) SetPixel(x, y int, color Color) error {
	if x < 0 || x >= p.Width || y < 0 || y >= p.Height {
		return errors.New("coordinates out of bounds")
	}
	// 转换到内存缓冲区坐标
	xm, ym := p.transform(x, y)
	if xm < 0 || xm >= p.WidthMemory || ym < 0 || ym >= p.HeightMemory {
		return errors.New("transformed coordinates out of bounds")
	}
	// 如果有显示回调函数，则使用它
	if p.displayFunc != nil {
		p.displayFunc(xm, ym, color)
	}
	// 如果有图像缓冲区，则更新它
	if p.Image != nil {
		index := ym*p.WidthMemory + xm
		if index < len(p.Image) {
			p.Image[index] = color
		}
	}
	return nil
}

// Clear 使用给定颜色清除整个显示。
func (p *Paint) Clear(color Color) {
	if p.clearFunc != nil {
		p.clearFunc(color)
	}
	// 如果存在图像缓冲区，也清除它
	if p.Image != nil {
		for i := range p.Image {
			p.Image[i] = color
		}
	}
}

// ClearWindow 使用给定颜色清除一个矩形窗口。
func (p *Paint) ClearWindow(xStart, yStart, xEnd, yEnd int, color Color) {
	for y := yStart; y < yEnd; y++ {
		for x := xStart; x < xEnd; x++ {
			p.SetPixel(x, y, color)
		}
	}
}

// DrawPoint 使用给定颜色、点大小和填充样式在(x, y)位置绘制一个点。
func (p *Paint) DrawPoint(x, y int, color Color, dotPixel DotPixel, dotStyle DotStyle) {
	if x < 0 || x >= p.Width || y < 0 || y >= p.Height {
		// 超出可见区域
		return
	}
	if dotStyle == DotFillAround {
		// 围绕中心点绘制，对于dotPixel=1，只绘制中心点
		offset := int(dotPixel) - 1
		for xd := 0; xd < 2*int(dotPixel)-1; xd++ {
			for yd := 0; yd < 2*int(dotPixel)-1; yd++ {
				px := x + xd - offset
				py := y + yd - offset
				if px < 0 || py < 0 {
					continue
				}
				p.SetPixel(px, py, color)
			}
		}
	} else { // DotFillRightUp 填充样式
		// 向右和向上扩展，对于dotPixel=1，绘制在(x,y)
		for xd := 0; xd < int(dotPixel); xd++ {
			for yd := 0; yd < int(dotPixel); yd++ {
				px := x + xd
				py := y + yd
				p.SetPixel(px, py, color)
			}
		}
	}
}

// DrawLine 使用给定颜色、线宽和线条样式从(xStart, yStart)到(xEnd, yEnd)绘制一条线。
func (p *Paint) DrawLine(xStart, yStart, xEnd, yEnd int, color Color, lineWidth DotPixel, lineStyle LineStyle) {
	if xStart < 0 || xStart >= p.Width || yStart < 0 || yStart >= p.Height ||
		xEnd < 0 || xEnd >= p.Width || yEnd < 0 || yEnd >= p.Height {
		// 超出可见区域
		return
	}
	x := xStart
	y := yStart
	dx := xEnd - xStart
	if dx < 0 {
		dx = -dx
	}
	dy := yEnd - yStart
	if dy < 0 {
		dy = -dy
	}
	// 增量方向
	xAdd := 1
	if xStart > xEnd {
		xAdd = -1
	}
	yAdd := 1
	if yStart > yEnd {
		yAdd = -1
	}
	// 累积误差
	esp := dx + dy
	dottedLen := 0
	for {
		dottedLen++
		// 绘制虚线：虚线样式中每第3个点为背景色
		if lineStyle == LineStyleDotted && dottedLen%3 == 0 {
			p.DrawPoint(x, y, ImageBackground, lineWidth, DotFillAround)
			dottedLen = 0
		} else {
			p.DrawPoint(x, y, color, lineWidth, DotFillAround)
		}
		if 2*esp >= dy {
			if x == xEnd {
				break
			}
			esp += dy
			x += xAdd
		}
		if 2*esp <= dx {
			if y == yEnd {
				break
			}
			esp += dx
			y += yAdd
		}
	}
}

// DrawRectangle 使用给定颜色、线宽和填充样式从(xStart, yStart)到(xEnd, yEnd)绘制一个矩形。
func (p *Paint) DrawRectangle(xStart, yStart, xEnd, yEnd int, color Color, lineWidth DotPixel, filled DrawFill) {
	if xStart < 0 || xStart >= p.Width || yStart < 0 || yStart >= p.Height ||
		xEnd < 0 || xEnd >= p.Width || yEnd < 0 || yEnd >= p.Height {
		// 超出可见区域
		return
	}
	if filled == DrawFillFull {
		for y := yStart; y < yEnd; y++ {
			p.DrawLine(xStart, y, xEnd, y, color, lineWidth, LineStyleSolid)
		}
	} else {
		p.DrawLine(xStart, yStart, xEnd, yStart, color, lineWidth, LineStyleSolid)
		p.DrawLine(xStart, yStart, xStart, yEnd, color, lineWidth, LineStyleSolid)
		p.DrawLine(xEnd, yEnd, xEnd, yStart, color, lineWidth, LineStyleSolid)
		p.DrawLine(xEnd, yEnd, xStart, yEnd, color, lineWidth, LineStyleSolid)
	}
}

// DrawCircle 使用给定半径、颜色、线宽和填充样式以(xCenter, yCenter)为中心绘制一个圆。
func (p *Paint) DrawCircle(xCenter, yCenter, radius int, color Color, lineWidth DotPixel, fill DrawFill) {
	if xCenter < 0 || xCenter >= p.Width || yCenter < 0 || yCenter >= p.Height {
		// 超出可见区域
		return
	}
	// 从(0, radius)开始
	xCurrent := 0
	yCurrent := radius
	// 累积误差
	esp := 3 - (radius << 1)
	if fill == DrawFillFull {
		for xCurrent <= yCurrent {
			for sCountY := xCurrent; sCountY <= yCurrent; sCountY++ {
				p.DrawPoint(xCenter+xCurrent, yCenter+sCountY, color, DotPixel1x1, DotFillAround)
				p.DrawPoint(xCenter-xCurrent, yCenter+sCountY, color, DotPixel1x1, DotFillAround)
				p.DrawPoint(xCenter-sCountY, yCenter+xCurrent, color, DotPixel1x1, DotFillAround)
				p.DrawPoint(xCenter-sCountY, yCenter-xCurrent, color, DotPixel1x1, DotFillAround)
				p.DrawPoint(xCenter-xCurrent, yCenter-sCountY, color, DotPixel1x1, DotFillAround)
				p.DrawPoint(xCenter+xCurrent, yCenter-sCountY, color, DotPixel1x1, DotFillAround)
				p.DrawPoint(xCenter+sCountY, yCenter-xCurrent, color, DotPixel1x1, DotFillAround)
				p.DrawPoint(xCenter+sCountY, yCenter+xCurrent, color, DotPixel1x1, DotFillAround)
			}
			if esp < 0 {
				esp += 4*xCurrent + 6
			} else {
				esp += 10 + 4*(xCurrent-yCurrent)
				yCurrent--
			}
			xCurrent++
		}
	} else { // 空心圆
		for xCurrent <= yCurrent {
			p.DrawPoint(xCenter+xCurrent, yCenter+yCurrent, color, lineWidth, DotFillAround)
			p.DrawPoint(xCenter-xCurrent, yCenter+yCurrent, color, lineWidth, DotFillAround)
			p.DrawPoint(xCenter-yCurrent, yCenter+xCurrent, color, lineWidth, DotFillAround)
			p.DrawPoint(xCenter-yCurrent, yCenter-xCurrent, color, lineWidth, DotFillAround)
			p.DrawPoint(xCenter-xCurrent, yCenter-yCurrent, color, lineWidth, DotFillAround)
			p.DrawPoint(xCenter+xCurrent, yCenter-yCurrent, color, lineWidth, DotFillAround)
			p.DrawPoint(xCenter+yCurrent, yCenter-xCurrent, color, lineWidth, DotFillAround)
			p.DrawPoint(xCenter+yCurrent, yCenter+xCurrent, color, lineWidth, DotFillAround)
			if esp < 0 {
				esp += 4*xCurrent + 6
			} else {
				esp += 10 + 4*(xCurrent-yCurrent)
				yCurrent--
			}
			xCurrent++
		}
	}
}

// DrawImage 在(x, y)位置绘制指定宽度和高度的位图图像。
// 图像数据是按行主序排列的Color值切片。
func (p *Paint) DrawImage(x, y int, image []Color, imgWidth, imgHeight int) {
	if x < 0 || x >= p.Width || y < 0 || y >= p.Height {
		return
	}
	if len(image) < imgWidth*imgHeight {
		return
	}
	for j := range imgHeight {
		for i := range imgWidth {
			px := x + i
			py := y + j
			if px >= p.Width || py >= p.Height {
				continue
			}
			color := image[j*imgWidth+i]
			p.SetPixel(px, py, color)
		}
	}
}

// DrawFloatNum 在(x, y)位置绘制指定小数位数的浮点数。
func (p *Paint) DrawFloatNum(x, y int, num float64, decimalPlaces int, face font.Face, bgColor, fgColor Color) {
	if x < 0 || x >= p.Width || y < 0 || y >= p.Height {
		return
	}
	// 格式化，使用所需的小数位数，额外多留几位用于修剪
	format := fmt.Sprintf("%%.%df", decimalPlaces+2)
	str := fmt.Sprintf(format, num)
	// 移除末尾的零以及可能的小数点
	// 类似于C版本：如果最后两个字符是".0"，则移除它们
	// 实际上C版本进行更复杂的修剪，我们进行了简化。
	// 移除小数点后的末尾零
	for len(str) > 0 && str[len(str)-1] == '0' {
		str = str[:len(str)-1]
	}
	if len(str) > 0 && str[len(str)-1] == '.' {
		str = str[:len(str)-1]
	}
	p.DrawString(x, y, str, face, bgColor, fgColor)
}

// DrawTime 使用给定的字体和颜色在(x, y)位置绘制时间结构。
func (p *Paint) DrawTime(x, y int, pt *PaintTime, face font.Face, bgColor, fgColor Color) {
	if x < 0 || x >= p.Width || y < 0 || y >= p.Height {
		return
	}
	digits := []rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
	// 获取字符宽度估计值（使用'0'字符的宽度）
	advance, _ := face.GlyphAdvance('0')
	dx := int((advance + 31) / 64) // 转换为像素，四舍五入
	p.DrawCharRune(x, y, digits[pt.Hour/10], face, bgColor, fgColor)
	p.DrawCharRune(x+dx, y, digits[pt.Hour%10], face, bgColor, fgColor)
	p.DrawCharRune(x+dx+dx/4+dx/2, y, ':', face, bgColor, fgColor)
	p.DrawCharRune(x+dx*2+dx/2, y, digits[pt.Min/10], face, bgColor, fgColor)
	p.DrawCharRune(x+dx*3+dx/2, y, digits[pt.Min%10], face, bgColor, fgColor)
	p.DrawCharRune(x+dx*4+dx/2-dx/4, y, ':', face, bgColor, fgColor)
	p.DrawCharRune(x+dx*5, y, digits[pt.Sec/10], face, bgColor, fgColor)
	p.DrawCharRune(x+dx*6, y, digits[pt.Sec%10], face, bgColor, fgColor)
}

// DrawNum 使用给定的字体和颜色在(x, y)位置绘制整数。
func (p *Paint) DrawNum(x, y int, num int32, face font.Face, bgColor, fgColor Color) {
	if x < 0 || x >= p.Width || y < 0 || y >= p.Height {
		return
	}
	// 将数字转换为字符串
	str := fmt.Sprintf("%d", num)
	p.DrawString(x, y, str, face, bgColor, fgColor)
}

// DrawString 使用DrawCharRune绘制字符串
func (p *Paint) DrawString(x, y int, str string, face font.Face, bgColor, fgColor Color) (int, int) {
	if x < 0 || x >= p.Width || y < 0 || y >= p.Height {
		return x, y
	}
	xPos := x
	yPos := y
	var dx, dy int
	for _, ch := range str {
		if ch == '\n' {
			xPos = x
			yPos += dy
		} else {
			dx, dy = p.DrawCharRune(xPos, yPos, ch, face, bgColor, fgColor)
			xPos += dx
		}
	}
	return xPos, yPos
}

// DrawCharRune 渲染字符
func (p *Paint) DrawCharRune(x, y int, ch rune, face font.Face, bgColor, fgColor Color) (int, int) {
	// 获取字体度量
	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	// 计算基线位置
	baseY := ascent
	// 获取字形
	dot := fixed.P(0, baseY)
	dr, mask, maskp, _, ok := face.Glyph(dot, ch)
	if !ok {
		return 0, 0
	}
	// 遍历字形掩码的像素
	for my := 0; my < dr.Dy(); my++ {
		for mx := 0; mx < dr.Dx(); mx++ {
			px := x + dr.Min.X + mx
			py := y + baseY + dr.Min.Y + my
			if px < 0 || px >= p.Width || py < 0 || py >= p.Height {
				continue
			}
			// 获取掩码像素的alpha值
			_, _, _, a := mask.At(maskp.X+mx, maskp.Y+my).RGBA()
			if a > 0 {
				p.SetPixel(px, py, fgColor)
			} else if bgColor != FontBackground {
				p.SetPixel(px, py, bgColor)
			}
		}
	}
	return dr.Dx(), dr.Dy()
}

// DrawRoundRect 绘制圆角矩形
func (p *Paint) DrawRoundRect(xStart, yStart, xEnd, yEnd, radius int, color Color, lineWidth DotPixel, filled DrawFill) {
	if xStart < 0 || xStart >= p.Width || yStart < 0 || yStart >= p.Height ||
		xEnd < 0 || xEnd >= p.Width || yEnd < 0 || yEnd >= p.Height {
		return
	}
	// 确保起点在终点之前
	if xStart > xEnd {
		xStart, xEnd = xEnd, xStart
	}
	if yStart > yEnd {
		yStart, yEnd = yEnd, yStart
	}
	width := xEnd - xStart
	height := yEnd - yStart
	// 限制半径大小
	maxRadius := min(height/2, width/2)
	if radius > maxRadius {
		radius = maxRadius
	}
	if radius < 0 {
		radius = 0
	}
	if filled == DrawFillFull {
		// 填充整个圆角矩形
		// 填充中心矩形区域
		for y := yStart + radius; y < yEnd-radius; y++ {
			p.DrawLine(xStart, y, xEnd, y, color, lineWidth, LineStyleSolid)
		}
		// 填充四个边角区域（使用半圆填充）
		// 左上角
		for dy := 0; dy < radius; dy++ {
			dx := int(math.Sqrt(float64(radius*radius - dy*dy)))
			p.DrawLine(xStart+radius-dx, yStart+dy, xEnd-radius+dx, yStart+dy, color, lineWidth, LineStyleSolid)
		}
		// 左下角
		for dy := 0; dy < radius; dy++ {
			dx := int(math.Sqrt(float64(radius*radius - dy*dy)))
			p.DrawLine(xStart+radius-dx, yEnd-radius+dy, xEnd-radius+dx, yEnd-radius+dy, color, lineWidth, LineStyleSolid)
		}
	} else {
		// 绘制圆角矩形的边框
		// 绘制四条直线边
		p.DrawLine(xStart+radius, yStart, xEnd-radius, yStart, color, lineWidth, LineStyleSolid) // 上边
		p.DrawLine(xStart+radius, yEnd, xEnd-radius, yEnd, color, lineWidth, LineStyleSolid)     // 下边
		p.DrawLine(xStart, yStart+radius, xStart, yEnd-radius, color, lineWidth, LineStyleSolid) // 左边
		p.DrawLine(xEnd, yStart+radius, xEnd, yEnd-radius, color, lineWidth, LineStyleSolid)     // 右边
		// 绘制四个圆角
		if radius > 0 {
			// 使用中点圆算法绘制圆角
			xc := xStart + radius
			yc := yStart + radius
			p.drawRoundCorner(xc, yc, radius, 180, 270, color, lineWidth) // 左上角
			xc = xEnd - radius
			yc = yStart + radius
			p.drawRoundCorner(xc, yc, radius, 270, 360, color, lineWidth) // 右上角
			xc = xEnd - radius
			yc = yEnd - radius
			p.drawRoundCorner(xc, yc, radius, 0, 90, color, lineWidth) // 右下角
			xc = xStart + radius
			yc = yEnd - radius
			p.drawRoundCorner(xc, yc, radius, 90, 180, color, lineWidth) // 左下角
		}
	}
}

// drawRoundCorner 绘制圆角（辅助函数）
func (p *Paint) drawRoundCorner(xCenter, yCenter, radius, startAngle, endAngle int, color Color, lineWidth DotPixel) {
	if radius <= 0 {
		return
	}
	// 使用中点圆算法绘制指定角度的弧段
	xCurrent := 0
	yCurrent := radius
	esp := 3 - (radius << 1)
	for xCurrent <= yCurrent {
		// 检查八个对称点是否在角度范围内
		points := []struct {
			x, y  int
			angle float64
		}{
			{xCenter + xCurrent, yCenter + yCurrent, math.Atan2(float64(yCurrent), float64(xCurrent))},
			{xCenter - xCurrent, yCenter + yCurrent, math.Atan2(float64(yCurrent), float64(-xCurrent))},
			{xCenter + yCurrent, yCenter + xCurrent, math.Atan2(float64(xCurrent), float64(yCurrent))},
			{xCenter - yCurrent, yCenter + xCurrent, math.Atan2(float64(xCurrent), float64(-yCurrent))},
			{xCenter - xCurrent, yCenter - yCurrent, math.Atan2(float64(-yCurrent), float64(-xCurrent))},
			{xCenter + xCurrent, yCenter - yCurrent, math.Atan2(float64(-yCurrent), float64(xCurrent))},
			{xCenter - yCurrent, yCenter - xCurrent, math.Atan2(float64(-xCurrent), float64(-yCurrent))},
			{xCenter + yCurrent, yCenter - xCurrent, math.Atan2(float64(-xCurrent), float64(yCurrent))},
		}
		for _, pt := range points {
			// 计算角度（度）
			angle := pt.angle * 180.0 / math.Pi
			if angle < 0 {
				angle += 360.0
			}
			// 检查角度是否在指定范围内
			if startAngle <= endAngle {
				if angle >= float64(startAngle) && angle <= float64(endAngle) {
					p.DrawPoint(pt.x, pt.y, color, lineWidth, DotFillAround)
				}
			} else {
				// 处理跨越0度的情况
				if angle >= float64(startAngle) || angle <= float64(endAngle) {
					p.DrawPoint(pt.x, pt.y, color, lineWidth, DotFillAround)
				}
			}
		}
		if esp < 0 {
			esp += 4*xCurrent + 6
		} else {
			esp += 10 + 4*(xCurrent-yCurrent)
			yCurrent--
		}
		xCurrent++
	}
}

// DrawInterpolatedLine 绘制插值动画线条
// 从起点到终点逐步绘制，模拟动画效果
// steps: 插值步数，越大动画越平滑
// delayFunc: 可选的延迟函数，用于控制动画速度
func (p *Paint) DrawInterpolatedLine(xStart, yStart, xEnd, yEnd int, color Color, lineWidth DotPixel, steps int) {
	if steps <= 0 {
		steps = 1
	}
	// 计算每一步的增量
	dx := float64(xEnd-xStart) / float64(steps)
	dy := float64(yEnd-yStart) / float64(steps)
	// 绘制每一步
	for i := 0; i <= steps; i++ {
		x := xStart + int(float64(i)*dx+0.5)
		y := yStart + int(float64(i)*dy+0.5)
		// 绘制点
		p.DrawPoint(x, y, color, lineWidth, DotFillAround)
	}
}

// DrawInterpolatedCircle 绘制插值动画圆
// 从0度到360度逐步绘制圆
func (p *Paint) DrawInterpolatedCircle(xCenter, yCenter, radius int, color Color, lineWidth DotPixel, steps int) {
	if steps <= 0 {
		steps = 36 // 默认36步，每步10度
	}
	// 绘制每一步
	for i := 0; i <= steps; i++ {
		angle := 2.0 * math.Pi * float64(i) / float64(steps)
		x := xCenter + int(float64(radius)*math.Cos(angle)+0.5)
		y := yCenter + int(float64(radius)*math.Sin(angle)+0.5)
		// 绘制点
		p.DrawPoint(x, y, color, lineWidth, DotFillAround)
	}
}

// DrawInterpolatedRect 绘制插值动画矩形
// 顺时针逐步绘制矩形的四条边
func (p *Paint) DrawInterpolatedRect(xStart, yStart, xEnd, yEnd int, color Color, lineWidth DotPixel, stepsPerSide int) {
	if xStart > xEnd {
		xStart, xEnd = xEnd, xStart
	}
	if yStart > yEnd {
		yStart, yEnd = yEnd, yStart
	}
	if stepsPerSide <= 0 {
		stepsPerSide = 10
	}
	// 绘制上边
	for i := 0; i <= stepsPerSide; i++ {
		x := xStart + (xEnd-xStart)*i/stepsPerSide
		y := yStart
		p.DrawPoint(x, y, color, lineWidth, DotFillAround)
	}
	// 绘制右边
	for i := 0; i <= stepsPerSide; i++ {
		x := xEnd
		y := yStart + (yEnd-yStart)*i/stepsPerSide
		p.DrawPoint(x, y, color, lineWidth, DotFillAround)
	}
	// 绘制下边（从右到左）
	for i := 0; i <= stepsPerSide; i++ {
		x := xEnd - (xEnd-xStart)*i/stepsPerSide
		y := yEnd
		p.DrawPoint(x, y, color, lineWidth, DotFillAround)
	}
	// 绘制左边（从下到上）
	for i := 0; i <= stepsPerSide; i++ {
		x := xStart
		y := yEnd - (yEnd-yStart)*i/stepsPerSide
		p.DrawPoint(x, y, color, lineWidth, DotFillAround)
	}
}

// DrawTriangle 绘制三角形
// x1,y1,x2,y2,x3,y3 为三个顶点的坐标
// color 颜色
// lineWidth 线宽（仅当filled为DrawFillEmpty时有效）
// filled 填充样式
func (p *Paint) DrawTriangle(x1, y1, x2, y2, x3, y3 int, color Color, lineWidth DotPixel, filled DrawFill) {
	if filled == DrawFillEmpty {
		// 绘制三条边
		p.DrawLine(x1, y1, x2, y2, color, lineWidth, LineStyleSolid)
		p.DrawLine(x2, y2, x3, y3, color, lineWidth, LineStyleSolid)
		p.DrawLine(x3, y3, x1, y1, color, lineWidth, LineStyleSolid)
	} else {
		// 填充三角形
		p.fillTriangle(x1, y1, x2, y2, x3, y3, color)
	}
}

// fillTriangle 使用扫描线算法填充三角形
func (p *Paint) fillTriangle(x1, y1, x2, y2, x3, y3 int, color Color) {
	// 将三个顶点按y坐标排序
	vertices := []struct{ x, y int }{{x1, y1}, {x2, y2}, {x3, y3}}
	sort.Slice(vertices, func(i, j int) bool {
		return vertices[i].y < vertices[j].y
	})
	v0, v1, v2 := vertices[0], vertices[1], vertices[2]
	// 整个三角形的高度
	totalHeight := v2.y - v0.y
	if totalHeight == 0 {
		// 退化三角形，所有顶点在同一水平线上
		// 绘制水平线
		minX := min(v2.x, min(v1.x, v0.x))
		maxX := max(v2.x, max(v1.x, v0.x))
		for x := minX; x <= maxX; x++ {
			p.SetPixel(x, v0.y, color)
		}
		return
	}
	// 扫描上半部分（v0到v1）
	for y := v0.y; y <= v1.y; y++ {
		segmentHeight := v1.y - v0.y
		if segmentHeight == 0 {
			continue
		}
		alpha := float64(y-v0.y) / float64(totalHeight)
		beta := float64(y-v0.y) / float64(segmentHeight)
		ax := float64(v0.x) + alpha*float64(v2.x-v0.x)
		bx := float64(v0.x) + beta*float64(v1.x-v0.x)
		startX := int(ax + 0.5)
		endX := int(bx + 0.5)
		if startX > endX {
			startX, endX = endX, startX
		}
		for x := startX; x <= endX; x++ {
			p.SetPixel(x, y, color)
		}
	}
	// 扫描下半部分（v1到v2）
	for y := v1.y; y <= v2.y; y++ {
		segmentHeight := v2.y - v1.y
		if segmentHeight == 0 {
			continue
		}
		alpha := float64(y-v0.y) / float64(totalHeight)
		beta := float64(y-v1.y) / float64(segmentHeight)
		ax := float64(v0.x) + alpha*float64(v2.x-v0.x)
		bx := float64(v1.x) + beta*float64(v2.x-v1.x)
		startX := int(ax + 0.5)
		endX := int(bx + 0.5)
		if startX > endX {
			startX, endX = endX, startX
		}
		for x := startX; x <= endX; x++ {
			p.SetPixel(x, y, color)
		}
	}
}

// DrawPolygon 绘制多边形
// points 是顶点切片，每个顶点为{x, y}
// color 颜色
// lineWidth 线宽（仅当filled为DrawFillEmpty时有效）
// filled 填充样式
func (p *Paint) DrawPolygon(points [][2]int, color Color, lineWidth DotPixel, filled DrawFill) {
	if len(points) < 3 {
		// 至少需要三个顶点才能构成多边形
		return
	}
	if filled == DrawFillEmpty {
		// 绘制多边形边框
		for i := 0; i < len(points); i++ {
			j := (i + 1) % len(points)
			p.DrawLine(points[i][0], points[i][1], points[j][0], points[j][1], color, lineWidth, LineStyleSolid)
		}
	} else {
		// 填充多边形
		p.fillPolygon(points, color)
	}
}

// edge 表示扫描线算法中的边
type edge struct {
	yMax int     // 边的最大y坐标
	x    float64 // 当前扫描线与边交点的x坐标
	dx   float64 // 斜率倒数（Δx/Δy）
}

// fillPolygon 使用扫描线算法填充多边形
func (p *Paint) fillPolygon(points [][2]int, color Color) {
	// 找到多边形的y范围
	minY := points[0][1]
	maxY := points[0][1]
	for _, pt := range points {
		if pt[1] < minY {
			minY = pt[1]
		}
		if pt[1] > maxY {
			maxY = pt[1]
		}
	}
	// 边表（ET）：按边的较小y坐标索引
	et := make([][]edge, maxY-minY+1)
	// 构建边表
	for i := range points {
		j := (i + 1) % len(points)
		x1, y1 := points[i][0], points[i][1]
		x2, y2 := points[j][0], points[j][1]
		// 忽略水平线
		if y1 == y2 {
			continue
		}
		// 确保y1 < y2
		if y1 > y2 {
			x1, x2 = x2, x1
			y1, y2 = y2, y1
		}
		dx := float64(x2-x1) / float64(y2-y1)
		et[y1-minY] = append(et[y1-minY], edge{
			yMax: y2,
			x:    float64(x1),
			dx:   dx,
		})
	}
	// 活动边表（AET）
	var aet []edge
	// 扫描每条扫描线
	for y := minY; y <= maxY; y++ {
		// 将边表中起始y等于当前y的边添加到AET
		if y-minY < len(et) {
			aet = append(aet, et[y-minY]...)
		}
		// 从AET中移除yMax等于当前y的边
		newAet := make([]edge, 0, len(aet))
		for _, e := range aet {
			if e.yMax > y {
				newAet = append(newAet, e)
			}
		}
		aet = newAet
		// 按当前x排序
		sort.Slice(aet, func(i, j int) bool {
			return aet[i].x < aet[j].x
		})
		// 填充扫描线
		for i := 0; i < len(aet); i += 2 {
			if i+1 >= len(aet) {
				break
			}
			startX := int(aet[i].x + 0.5)
			endX := int(aet[i+1].x + 0.5)
			if startX > endX {
				startX, endX = endX, startX
			}
			for x := startX; x <= endX; x++ {
				p.SetPixel(x, y, color)
			}
		}
		// 更新AET中边的x值
		for i := range aet {
			aet[i].x += aet[i].dx
		}
	}
}
