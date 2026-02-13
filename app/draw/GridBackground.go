package draw

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// GridBackground 表示可缓存的网格背景
// 这个结构体封装了网格绘制逻辑，支持缓存优化
type GridBackground struct {
	// 配置
	GridLineSize int         // 网格线间距（像素）
	GridColor    color.NRGBA // 网格线颜色

	// 缓存状态
	lastSize  image.Point // 上次绘制时的控件大小
	lastScale float32     // 上次绘制时的缩放比例
	gridCache op.Ops      // 网格绘制指令缓存
	gridCall  op.CallOp   // 缓存的绘制调用
}

// NewGridBackground 创建新的网格背景
func NewGridBackground(gridLineSize int, gridColor color.NRGBA) *GridBackground {
	return &GridBackground{
		GridLineSize: gridLineSize,
		GridColor:    gridColor,
	}
}

// Draw 绘制网格背景
// 当控件大小改变或缩放比例改变时重建网格缓存，否则重用缓存的绘制指令
func (gb *GridBackground) Draw(d *Draw, size image.Point) {
	// 检查大小或缩放比例是否改变，如果改变则重建缓存
	if size != gb.lastSize || d.Scale != gb.lastScale {
		gb.lastSize = size
		gb.lastScale = d.Scale
		gb.gridCache.Reset() // 清空旧的缓存
		macro := op.Record(&gb.gridCache)
		var p clip.Path
		p.Begin(&gb.gridCache)
		// 计算缩放后的网格线间距
		scaledGridLineSize := float32(gb.GridLineSize) * d.Scale
		if scaledGridLineSize < 5 {
			// 如果网格线太密集，不绘制网格
			paint.FillShape(&gb.gridCache, gb.GridColor, clip.Rect{Max: size}.Op())
		} else {
			// 绘制范围需要比实际 size 多出一个单元格，以防偏移后露底
			drawW := float32(size.X) + scaledGridLineSize
			drawH := float32(size.Y) + scaledGridLineSize
			// 绘制垂直线
			for x := float32(0); x <= drawW; x += scaledGridLineSize {
				p.MoveTo(f32.Pt(x, 0))
				p.LineTo(f32.Pt(x, drawH))
			}
			// 绘制水平线
			for y := float32(0); y <= drawH; y += scaledGridLineSize {
				p.MoveTo(f32.Pt(0, y))
				p.LineTo(f32.Pt(drawW, y))
			}
			paint.FillShape(&gb.gridCache, gb.GridColor, clip.Stroke{
				Path:  p.End(),
				Width: 1,
			}.Op())
		}
		gb.gridCall = macro.Stop()
	}
	// 计算取模后的偏移量 (实现无限滚动视觉效果)
	// 使用负偏移，模拟背景随着滚动向反方向移动
	scaledGridLineSize := int(float32(gb.GridLineSize) * d.Scale)
	if scaledGridLineSize < 1 {
		scaledGridLineSize = 1
	}
	offX := -int(d.Scroll.X) % scaledGridLineSize
	offY := -int(d.Scroll.Y) % scaledGridLineSize
	// 应用偏移并调用缓存的指令
	trans := op.Offset(image.Point{X: offX, Y: offY}).Push(d.Ops)
	gb.gridCall.Add(d.Ops)
	trans.Pop()
}

// DefaultGridBackground 创建默认的网格背景
func DefaultGridBackground() *GridBackground {
	return NewGridBackground(
		20, // 默认网格线间距
		color.NRGBA{R: 230, G: 230, B: 230, A: 255}, // 浅灰色
	)
}
