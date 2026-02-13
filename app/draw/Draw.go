package draw

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Draw 绘图
type Draw struct {
	layout.Context           // 默认上下文
	Ops            *op.Ops   // 绘图上下文
	Scale          float32   // 缩放
	Scroll         f32.Point // 滚动
}

// HandleEvents 统一处理所有交互逻辑
// 处理缩放（鼠标滚轮）和滚动（拖拽平移）事件
func (d *Draw) HandleEvents(gtx layout.Context) {
	// 处理指针事件（拖拽平移）
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Kinds:   pointer.Press | pointer.Drag | pointer.Release | pointer.Scroll,
			ScrollY: pointer.ScrollRange{Min: -1, Max: 1},
		})
		if !ok {
			break
		}
		e, _ := ev.(pointer.Event)
		d.HandlePointerEvent(e)
	}
}

// HandlePointerEvent 处理单个指针事件
// 这个方法可以被其他组件调用，将事件委托给 Draw 处理
func (d *Draw) HandlePointerEvent(e pointer.Event) {
	switch e.Kind {
	case pointer.Press:
		// 记录拖拽起始位置
		d.handlePress(e.Position)
	case pointer.Drag:
		// 处理拖拽平移
		d.handleDrag(e.Position)
	case pointer.Release:
		// 重置拖拽状态
		d.handleRelease()
	case pointer.Scroll:
		// 处理鼠标滚轮缩放
		d.handleZoom(e.Position, e.Scroll.Y)
	}
}

// WorldToScreenF32 坐标计算
func (d *Draw) WorldToScreenF32(wp image.Point) f32.Point {
	return f32.Pt(
		float32(wp.X)*d.Scale-d.Scroll.X,
		float32(wp.Y)*d.Scale-d.Scroll.Y,
	)
}

// DrawPoint 绘制一个点
// 点实际上是一个小圆形，半径由 size 参数控制
func (d *Draw) DrawPoint(pos f32.Point, size float32, color color.NRGBA) layout.Dimensions {
	radius := size / 2
	ellipse := clip.Ellipse{
		Min: image.Pt(int(pos.X-radius), int(pos.Y-radius)),
		Max: image.Pt(int(pos.X+radius), int(pos.Y+radius)),
	}
	paint.FillShape(d.Ops, color, ellipse.Op(d.Ops))
	return layout.Dimensions{Size: image.Pt(int(size), int(size))}
}

// DrawLine 绘制一条直线
func (d *Draw) DrawLine(start, end f32.Point, width float32, color color.NRGBA) layout.Dimensions {
	var path clip.Path
	path.Begin(d.Ops)
	path.MoveTo(start)
	path.LineTo(end)
	// 将线宽从 Dp 转换为像素
	lineWidth := float32(d.Dp(unit.Dp(width)))
	if lineWidth < 1.0 {
		lineWidth = 1.0
	}
	paint.FillShape(d.Ops, color, clip.Stroke{
		Path:  path.End(),
		Width: lineWidth,
	}.Op())
	// 返回线的边界框
	minX := min(start.X, end.X) - lineWidth/2
	maxX := max(start.X, end.X) + lineWidth/2
	minY := min(start.Y, end.Y) - lineWidth/2
	maxY := max(start.Y, end.Y) + lineWidth/2
	return layout.Dimensions{
		Size: image.Pt(int(maxX-minX), int(maxY-minY)),
	}
}

// DrawPolyline 绘制折线（不闭合）
func (d *Draw) DrawPolyline(points []f32.Point, width float32, color color.NRGBA) layout.Dimensions {
	if len(points) < 2 {
		return layout.Dimensions{}
	}
	var path clip.Path
	path.Begin(d.Ops)
	path.MoveTo(points[0])
	for i := 1; i < len(points); i++ {
		path.LineTo(points[i])
	}
	// 将线宽从 Dp 转换为像素
	lineWidth := float32(d.Dp(unit.Dp(width)))
	if lineWidth < 1.0 {
		lineWidth = 1.0
	}
	paint.FillShape(d.Ops, color, clip.Stroke{
		Path:  path.End(),
		Width: lineWidth,
	}.Op())
	// 计算边界框
	var minX, minY, maxX, maxY float32
	if len(points) > 0 {
		minX, maxX = points[0].X, points[0].X
		minY, maxY = points[0].Y, points[0].Y
	}
	for _, p := range points {
		minX = min(minX, p.X)
		maxX = max(maxX, p.X)
		minY = min(minY, p.Y)
		maxY = max(maxY, p.Y)
	}
	// 考虑线宽
	minX -= lineWidth / 2
	maxX += lineWidth / 2
	minY -= lineWidth / 2
	maxY += lineWidth / 2
	return layout.Dimensions{
		Size: image.Pt(int(maxX-minX), int(maxY-minY)),
	}
}

// DrawPolygon 绘制多边形（闭合）
func (d *Draw) DrawPolygon(points []f32.Point, width float32, fillColor, strokeColor color.NRGBA, filled bool) layout.Dimensions {
	if len(points) < 2 {
		return layout.Dimensions{}
	}
	var path clip.Path
	path.Begin(d.Ops)
	path.MoveTo(points[0])
	for i := 1; i < len(points); i++ {
		path.LineTo(points[i])
	}
	path.LineTo(points[0]) // 闭合多边形
	pathEnd := path.End()
	// 将线宽从 Dp 转换为像素
	lineWidth := float32(d.Dp(unit.Dp(width)))
	if lineWidth < 1.0 {
		lineWidth = 1.0
	}
	// 计算边界框
	var minX, minY, maxX, maxY float32
	if len(points) > 0 {
		minX, maxX = points[0].X, points[0].X
		minY, maxY = points[0].Y, points[0].Y
	}
	for _, p := range points {
		minX = min(minX, p.X)
		maxX = max(maxX, p.X)
		minY = min(minY, p.Y)
		maxY = max(maxY, p.Y)
	}
	// 考虑线宽
	minX -= lineWidth / 2
	maxX += lineWidth / 2
	minY -= lineWidth / 2
	maxY += lineWidth / 2
	// 填充
	if filled {
		paint.FillShape(d.Ops, fillColor, clip.Outline{Path: pathEnd}.Op())
	}
	// 描边
	if strokeColor.A > 0 {
		paint.FillShape(d.Ops, strokeColor, clip.Stroke{
			Path:  pathEnd,
			Width: lineWidth,
		}.Op())
	}
	return layout.Dimensions{
		Size: image.Pt(int(maxX-minX), int(maxY-minY)),
	}
}

// DrawRect 绘制矩形
func (d *Draw) DrawRect(rect image.Rectangle, width float32, fillColor, strokeColor color.NRGBA, filled bool) layout.Dimensions {
	clipRect := clip.Rect(rect)
	// 将线宽从 Dp 转换为像素
	lineWidth := float32(d.Dp(unit.Dp(width)))
	if lineWidth < 1.0 {
		lineWidth = 1.0
	}
	// 填充
	if filled {
		paint.FillShape(d.Ops, fillColor, clipRect.Op())
	}
	// 描边
	if strokeColor.A > 0 {
		paint.FillShape(d.Ops, strokeColor, clip.Stroke{
			Path:  clipRect.Path(),
			Width: lineWidth,
		}.Op())
	}
	return layout.Dimensions{
		Size: rect.Size(),
	}
}

// DrawCircle 绘制圆形
func (d *Draw) DrawCircle(center f32.Point, radius float32, width float32, fillColor, strokeColor color.NRGBA, filled bool) layout.Dimensions {
	ellipse := clip.Ellipse{
		Min: image.Pt(int(center.X-radius), int(center.Y-radius)),
		Max: image.Pt(int(center.X+radius), int(center.Y+radius)),
	}
	// 将线宽从 Dp 转换为像素
	lineWidth := float32(d.Dp(unit.Dp(width)))
	if lineWidth < 1.0 {
		lineWidth = 1.0
	}
	// 填充
	if filled {
		paint.FillShape(d.Ops, fillColor, ellipse.Op(d.Ops))
	}
	// 描边
	if strokeColor.A > 0 {
		paint.FillShape(d.Ops, strokeColor, clip.Stroke{
			Path:  ellipse.Path(d.Ops),
			Width: lineWidth,
		}.Op())
	}
	diameter := radius * 2
	return layout.Dimensions{
		Size: image.Pt(int(diameter), int(diameter)),
	}
}

// DrawQuadraticBezier 绘制二次贝塞尔曲线
func (d *Draw) DrawQuadraticBezier(start, control, end f32.Point, width float32, color color.NRGBA) layout.Dimensions {
	var path clip.Path
	path.Begin(d.Ops)
	path.MoveTo(start)
	path.QuadTo(control, end)
	// 将线宽从 Dp 转换为像素
	lineWidth := float32(d.Dp(unit.Dp(width)))
	if lineWidth < 1.0 {
		lineWidth = 1.0
	}
	paint.FillShape(d.Ops, color, clip.Stroke{
		Path:  path.End(),
		Width: lineWidth,
	}.Op())
	// 计算边界框（包含控制点）
	minX := min(start.X, min(control.X, end.X)) - lineWidth/2
	maxX := max(start.X, max(control.X, end.X)) + lineWidth/2
	minY := min(start.Y, min(control.Y, end.Y)) - lineWidth/2
	maxY := max(start.Y, max(control.Y, end.Y)) + lineWidth/2
	return layout.Dimensions{
		Size: image.Pt(int(maxX-minX), int(maxY-minY)),
	}
}

// DrawCubicBezier 绘制三次贝塞尔曲线
func (d *Draw) DrawCubicBezier(start, control1, control2, end f32.Point, width float32, color color.NRGBA) layout.Dimensions {
	var path clip.Path
	path.Begin(d.Ops)
	path.MoveTo(start)
	path.CubeTo(control1, control2, end)
	// 将线宽从 Dp 转换为像素
	lineWidth := float32(d.Dp(unit.Dp(width)))
	if lineWidth < 1.0 {
		lineWidth = 1.0
	}
	paint.FillShape(d.Ops, color, clip.Stroke{
		Path:  path.End(),
		Width: lineWidth,
	}.Op())
	// 计算边界框（包含控制点）
	minX := min(start.X, min(control1.X, min(control2.X, end.X))) - lineWidth/2
	maxX := max(start.X, max(control1.X, max(control2.X, end.X))) + lineWidth/2
	minY := min(start.Y, min(control1.Y, min(control2.Y, end.Y))) - lineWidth/2
	maxY := max(start.Y, max(control1.Y, max(control2.Y, end.Y))) + lineWidth/2
	return layout.Dimensions{
		Size: image.Pt(int(maxX-minX), int(maxY-minY)),
	}
}

// DrawCapsule 绘制胶囊形状（矩形加两个半圆形）
func (d *Draw) DrawCapsule(rect image.Rectangle, width float32, fillColor, strokeColor color.NRGBA, filled bool) layout.Dimensions {
	// 计算胶囊的圆角半径（使用矩形较短边的一半）
	widthPx := rect.Dx()
	heightPx := rect.Dy()
	radius := float32(min(widthPx, heightPx)) / 2
	// 将线宽从 Dp 转换为像素
	lineWidth := float32(d.Dp(unit.Dp(width)))
	if lineWidth < 1.0 {
		lineWidth = 1.0
	}
	// 创建胶囊路径
	var path clip.Path
	path.Begin(d.Ops)
	// 转换为浮点坐标
	left := float32(rect.Min.X)
	right := float32(rect.Max.X)
	top := float32(rect.Min.Y)
	bottom := float32(rect.Max.Y)
	// 从左边的中间开始（顶部半圆）
	startY := top + radius
	path.MoveTo(f32.Pt(left, startY))
	// 左边的半圆（逆时针）
	path.ArcTo(f32.Pt(left, top), f32.Pt(left+radius, top), radius)
	// 顶部的直线
	path.LineTo(f32.Pt(right-radius, top))
	// 右边的半圆
	path.ArcTo(f32.Pt(right, top), f32.Pt(right, top+radius), radius)
	// 右边的直线
	path.LineTo(f32.Pt(right, bottom-radius))
	// 右下角的半圆
	path.ArcTo(f32.Pt(right, bottom), f32.Pt(right-radius, bottom), radius)
	// 底部的直线
	path.LineTo(f32.Pt(left+radius, bottom))
	// 左下角的半圆
	path.ArcTo(f32.Pt(left, bottom), f32.Pt(left, bottom-radius), radius)
	// 闭合路径
	path.LineTo(f32.Pt(left, startY))
	pathEnd := path.End()
	// 填充
	if filled {
		paint.FillShape(d.Ops, fillColor, clip.Outline{Path: pathEnd}.Op())
	}
	// 描边
	if strokeColor.A > 0 {
		paint.FillShape(d.Ops, strokeColor, clip.Stroke{
			Path:  pathEnd,
			Width: lineWidth,
		}.Op())
	}
	return layout.Dimensions{
		Size: rect.Size(),
	}
}

// DrawRoundedRect 绘制圆角矩形
func (d *Draw) DrawRoundedRect(rect image.Rectangle, radius float32, width float32, fillColor, strokeColor color.NRGBA, filled bool) layout.Dimensions {
	// 将线宽从 Dp 转换为像素
	lineWidth := float32(d.Dp(unit.Dp(width)))
	if lineWidth < 1.0 {
		lineWidth = 1.0
	}
	// 创建圆角矩形路径
	var path clip.Path
	path.Begin(d.Ops)
	// 转换为浮点坐标
	left := float32(rect.Min.X)
	right := float32(rect.Max.X)
	top := float32(rect.Min.Y)
	bottom := float32(rect.Max.Y)
	// 限制半径不超过矩形尺寸的一半
	maxRadius := min(float32(rect.Dx()), float32(rect.Dy())) / 2
	if radius > maxRadius {
		radius = maxRadius
	}
	// 从左上角开始（左上角的右边）
	startX := left + radius
	path.MoveTo(f32.Pt(startX, top))
	// 顶边
	path.LineTo(f32.Pt(right-radius, top))
	// 右上角
	path.ArcTo(f32.Pt(right, top), f32.Pt(right, top+radius), radius)
	// 右边
	path.LineTo(f32.Pt(right, bottom-radius))
	// 右下角
	path.ArcTo(f32.Pt(right, bottom), f32.Pt(right-radius, bottom), radius)
	// 底边
	path.LineTo(f32.Pt(left+radius, bottom))
	// 左下角
	path.ArcTo(f32.Pt(left, bottom), f32.Pt(left, bottom-radius), radius)
	// 左边
	path.LineTo(f32.Pt(left, top+radius))
	// 左上角
	path.ArcTo(f32.Pt(left, top), f32.Pt(left+radius, top), radius)
	pathEnd := path.End()
	// 填充
	if filled {
		paint.FillShape(d.Ops, fillColor, clip.Outline{Path: pathEnd}.Op())
	}
	// 描边
	if strokeColor.A > 0 {
		paint.FillShape(d.Ops, strokeColor, clip.Stroke{
			Path:  pathEnd,
			Width: lineWidth,
		}.Op())
	}
	return layout.Dimensions{
		Size: rect.Size(),
	}
}

// DrawArc 绘制圆弧
func (d *Draw) DrawArc(center f32.Point, radius, startAngle, endAngle float32, width float32, color color.NRGBA) layout.Dimensions {
	var path clip.Path
	path.Begin(d.Ops)
	// 计算起点
	startX := center.X + radius*float32(math.Cos(float64(startAngle)))
	startY := center.Y + radius*float32(math.Sin(float64(startAngle)))
	path.MoveTo(f32.Pt(startX, startY))
	// 绘制圆弧 - Gio 的 Arc 函数只需要中心点和半径，角度通过其他方式指定
	// 这里我们使用简化的实现：绘制到终点的直线
	endX := center.X + radius*float32(math.Cos(float64(endAngle)))
	endY := center.Y + radius*float32(math.Sin(float64(endAngle)))
	path.LineTo(f32.Pt(endX, endY))
	// 将线宽从 Dp 转换为像素
	lineWidth := float32(d.Dp(unit.Dp(width)))
	if lineWidth < 1.0 {
		lineWidth = 1.0
	}
	paint.FillShape(d.Ops, color, clip.Stroke{
		Path:  path.End(),
		Width: lineWidth,
	}.Op())
	// 计算边界框
	// 简化计算：使用整个圆的边界框
	minX := center.X - radius - lineWidth/2
	maxX := center.X + radius + lineWidth/2
	minY := center.Y - radius - lineWidth/2
	maxY := center.Y + radius + lineWidth/2
	return layout.Dimensions{
		Size: image.Pt(int(maxX-minX), int(maxY-minY)),
	}
}

// DrawEllipse 绘制椭圆
func (d *Draw) DrawEllipse(center, radius f32.Point, width float32, fillColor, strokeColor color.NRGBA, filled bool) layout.Dimensions {
	ellipse := clip.Ellipse{
		Min: image.Pt(int(center.X-radius.X), int(center.Y-radius.Y)),
		Max: image.Pt(int(center.X+radius.X), int(center.Y+radius.Y)),
	}
	// 将线宽从 Dp 转换为像素
	lineWidth := float32(d.Dp(unit.Dp(width)))
	if lineWidth < 1.0 {
		lineWidth = 1.0
	}
	// 填充
	if filled {
		paint.FillShape(d.Ops, fillColor, ellipse.Op(d.Ops))
	}
	// 描边
	if strokeColor.A > 0 {
		paint.FillShape(d.Ops, strokeColor, clip.Stroke{
			Path:  ellipse.Path(d.Ops),
			Width: lineWidth,
		}.Op())
	}
	widthPx := radius.X * 2
	heightPx := radius.Y * 2
	return layout.Dimensions{
		Size: image.Pt(int(widthPx), int(heightPx)),
	}
}

// --- 交互处理辅助方法 ---

// dragState 拖拽状态
type dragState struct {
	isDragging bool
	startPos   f32.Point
}

var drag dragState

// handlePress 处理鼠标按下事件
func (d *Draw) handlePress(pos f32.Point) {
	drag.isDragging = true
	drag.startPos = pos
}

// handleDrag 处理拖拽事件
func (d *Draw) handleDrag(pos f32.Point) {
	if drag.isDragging {
		// 计算拖拽差值
		diff := pos.Sub(drag.startPos)
		// 更新滚动位置（反向移动，实现拖拽平移效果）
		d.Scroll.X -= diff.X
		d.Scroll.Y -= diff.Y
		// 更新起始位置
		drag.startPos = pos
		// 限制滚动范围（简单实现，可根据需要添加更复杂的限制）
		d.Scroll.X = max(0, d.Scroll.X)
		d.Scroll.Y = max(0, d.Scroll.Y)
	}
}

// handleRelease 处理鼠标释放事件
func (d *Draw) handleRelease() {
	drag.isDragging = false
}

// handleZoom 处理缩放事件
func (d *Draw) handleZoom(mousePos f32.Point, scrollY float32) {
	// 保存缩放前的鼠标位置对应的世界坐标
	// 世界坐标 = (屏幕坐标 + 滚动偏移) / 缩放比例
	worldXBefore := (d.Scroll.X + mousePos.X) / d.Scale
	worldYBefore := (d.Scroll.Y + mousePos.Y) / d.Scale
	// 计算缩放因子
	zoomFactor := float32(1.1) // 每次缩放10%
	if scrollY > 0 {
		// 向下滚动，缩小
		zoomFactor = 1.0 / zoomFactor
		// 限制最小缩放比例
		if d.Scale < 0.1 {
			return
		}
	}
	// 应用缩放，限制在合理范围内
	newScale := d.Scale * zoomFactor
	if newScale > 5.0 {
		newScale = 5.0
	} else if newScale < 0.5 {
		newScale = 0.5
	}
	// 更新缩放比例
	d.Scale = newScale
	// 计算缩放后，为了保持鼠标指向的世界坐标不变，需要调整滚动位置
	// 新的滚动位置 = 世界坐标 * 新缩放比例 - 鼠标屏幕坐标
	d.Scroll.X = worldXBefore*newScale - mousePos.X
	d.Scroll.Y = worldYBefore*newScale - mousePos.Y
	// 确保滚动位置非负
	d.Scroll.X = max(0, d.Scroll.X)
	d.Scroll.Y = max(0, d.Scroll.Y)
}
