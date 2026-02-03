package app

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/io/event"   // 事件处理
	"gioui.org/io/pointer" // 指针事件
	"gioui.org/layout"     // 布局系统
	"gioui.org/op"         // 操作栈
	"gioui.org/op/clip"    // 裁剪区域
	"gioui.org/op/paint"   // 绘制操作
	"gioui.org/unit"       // 单位转换
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Handle 控制点类型
type Handle int

const (
	HandleNone Handle = iota
	HandleN
	HandleS
	HandleE
	HandleW
	HandleNW
	HandleNE
	HandleSW
	HandleSE
)

type InteractionMode int

const (
	ModeNone     InteractionMode = iota
	ModePanning                  // 平移背景
	ModeMoving                   // 移动组件
	ModeResizing                 // 缩放组件
)

type ChildItem struct {
	ID        string
	Pos       image.Point // 虚拟空间位置
	Size      image.Point // 组件大小
	Widget    layout.Widget
	Movable   bool
	Resizable bool
}

type GridComponent struct {
	VirtualSize image.Point
	Children    []*ChildItem

	// 滚动状态
	scroll f32.Point

	// 缩放状态
	scale float32 // 缩放比例，1.0 表示原始大小

	// 交互状态
	mode          InteractionMode
	activeItem    *ChildItem
	activeHandle  Handle
	dragStartPos  f32.Point // 鼠标按下时的屏幕位置
	itemStartPos  image.Point
	itemStartSize image.Point

	// 配置
	GridLineSize int
	GridColor    color.NRGBA

	// 网格缓存优化
	lastSize  image.Point // 上次绘制时的控件大小
	lastScale float32     // 上次绘制时的缩放比例
	gridCache op.Ops      // 网格绘制指令缓存
	gridCall  op.CallOp   // 缓存的绘制调用

	// 滚动条
	Theme     *material.Theme
	scrollbar [2]*widget.Scrollbar // 垂直水平滚动条
}

// NewGridComponent 创建新的网格组件实例
// 返回一个初始化的 GridComponent，包含默认的虚拟空间大小和网格配置
func NewGridComponent() *GridComponent {
	g := &GridComponent{
		Theme:        material.NewTheme(),
		VirtualSize:  image.Pt(5000, 5000),                        // 5000x5000 像素的虚拟空间
		GridLineSize: 20,                                          // 网格线间距 20 像素
		GridColor:    color.NRGBA{R: 220, G: 220, B: 220, A: 255}, // 浅灰色网格
		scale:        1.0,                                         // 初始缩放比例为 1.0
	}
	// 初始化滚动条
	g.scrollbar[0] = &widget.Scrollbar{} // 垂直滚动条
	g.scrollbar[1] = &widget.Scrollbar{} // 水平滚动条
	return g
}

// Layout 实现 Gio 的布局接口，负责组件的渲染和事件处理
// 采用四层堆叠布局：背景层（网格+事件捕获）、内容层（子组件）、装饰层（选中框和控制点）、滚动条层
func (g *GridComponent) Layout(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	// 处理所有输入事件 (统一入口)
	g.handleEvents(gtx)
	// 渲染
	return layout.Stack{}.Layout(gtx,
		// 背景层：网格 + 事件捕获
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
			event.Op(gtx.Ops, g)
			g.drawGrid(gtx, size)
			return layout.Dimensions{Size: size}
		}),
		// 内容层
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			g.drawChildren(gtx, size)
			return layout.Dimensions{Size: size}
		}),
		// 装饰层：选中框和控制点 (始终在最顶层)
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if g.activeItem != nil {
				g.drawSelectionHelpers(gtx, size)
			}
			return layout.Dimensions{Size: size}
		}),
		// 滚动条层：垂直和水平滚动条
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			// 计算视口范围（归一化到[0,1]）
			viewportStartX := float32(g.scroll.X) / float32(g.VirtualSize.X)
			viewportEndX := float32(g.scroll.X+float32(size.X)) / float32(g.VirtualSize.X)
			viewportStartY := float32(g.scroll.Y) / float32(g.VirtualSize.Y)
			viewportEndY := float32(g.scroll.Y+float32(size.Y)) / float32(g.VirtualSize.Y)
			// 限制范围在[0,1]之间
			viewportStartX = max(0, min(1, viewportStartX))
			viewportEndX = max(0, min(1, viewportEndX))
			viewportStartY = max(0, min(1, viewportStartY))
			viewportEndY = max(0, min(1, viewportEndY))
			// 更新垂直滚动条
			if g.scrollbar[0] != nil {
				g.scrollbar[0].Update(gtx, layout.Vertical, viewportStartY, viewportEndY)
				// 处理垂直滚动条拖动
				if g.scrollbar[0].Dragging() {
					g.scroll.Y += g.scrollbar[0].ScrollDistance() * float32(g.VirtualSize.Y)
				}
			}
			// 更新水平滚动条
			if g.scrollbar[1] != nil {
				g.scrollbar[1].Update(gtx, layout.Horizontal, viewportStartX, viewportEndX)
				// 处理水平滚动条拖动
				if g.scrollbar[1].Dragging() {
					g.scroll.X += g.scrollbar[1].ScrollDistance() * float32(g.VirtualSize.X)
				}
			}
			// 位置限制
			g.scroll.X = max(0, min(g.scroll.X, float32(g.VirtualSize.X-size.X)))
			g.scroll.Y = max(0, min(g.scroll.Y, float32(g.VirtualSize.Y-size.Y)))
			return layout.Stack{}.Layout(gtx,
				// 垂直滚动条（右侧）
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					if g.scrollbar[0] != nil {
						scrollbarWidth := gtx.Dp(unit.Dp(12))
						// 设置裁剪区域
						scrollbarRect := image.Rect(size.X-scrollbarWidth, 0, size.X, size.Y)
						defer clip.Rect(scrollbarRect).Push(gtx.Ops).Pop()
						// 绘制滚动条
						gtx.Constraints = layout.Exact(scrollbarRect.Size())
						trans := op.Offset(scrollbarRect.Min).Push(gtx.Ops)
						dims := material.Scrollbar(g.Theme, g.scrollbar[0]).Layout(gtx, layout.Vertical, viewportStartY, viewportEndY)
						trans.Pop()
						return dims
					}
					return layout.Dimensions{}
				}),
				// 水平滚动条（底部）
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					if g.scrollbar[1] != nil {
						scrollbarHeight := gtx.Dp(unit.Dp(12))
						// 设置裁剪区域
						scrollbarRect := image.Rect(0, size.Y-scrollbarHeight, size.X, size.Y)
						defer clip.Rect(scrollbarRect).Push(gtx.Ops).Pop()
						gtx.Constraints = layout.Exact(scrollbarRect.Size())
						trans := op.Offset(scrollbarRect.Min).Push(gtx.Ops)
						dims := material.Scrollbar(g.Theme, g.scrollbar[1]).Layout(gtx, layout.Horizontal, viewportStartX, viewportEndX)
						trans.Pop()
						return dims
					}
					return layout.Dimensions{}
				}),
			)
		}),
	)
}

// handleEvents 统一处理所有交互逻辑
func (g *GridComponent) handleEvents(gtx layout.Context) {
	size := gtx.Constraints.Max
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  g,
			Kinds:   pointer.Press | pointer.Drag | pointer.Release | pointer.Scroll,
			ScrollY: pointer.ScrollRange{Min: -1, Max: 1},
		})
		if !ok {
			break
		}
		e, _ := ev.(pointer.Event)
		// 检查事件是否发生在滚动条区域
		scrollbarWidth := gtx.Dp(unit.Dp(12))
		scrollbarHeight := gtx.Dp(unit.Dp(12))
		// 垂直滚动条区域（右侧）
		verticalScrollbarRect := image.Rect(size.X-scrollbarWidth, 0, size.X, size.Y)
		// 水平滚动条区域（底部）
		horizontalScrollbarRect := image.Rect(0, size.Y-scrollbarHeight, size.X, size.Y)
		// 如果事件发生在滚动条区域，跳过处理（让滚动条自己处理）
		if pointInRect(e.Position, verticalScrollbarRect) || pointInRect(e.Position, horizontalScrollbarRect) {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			g.dragStartPos = e.Position
			// 1. 检查是否点击了控制点
			if g.activeItem != nil {
				if h := g.hitTestHandles(e.Position); h != HandleNone {
					g.mode = ModeResizing
					g.activeHandle = h
					g.itemStartPos = g.activeItem.Pos
					g.itemStartSize = g.activeItem.Size
					break
				}
			}
			// 检查是否点击了子组件
			found := false
			for i := len(g.Children) - 1; i >= 0; i-- {
				child := g.Children[i]
				rect := g.worldToScreenRect(child.Pos, child.Size)
				if pointInRect(e.Position, rect) {
					g.activeItem = child
					if child.Movable {
						g.mode = ModeMoving
						g.itemStartPos = child.Pos
					}
					found = true
					break
				}
			}
			// 点击空白区域
			if !found {
				g.activeItem = nil
				g.mode = ModePanning
			}
		case pointer.Drag:
			diff := e.Position.Sub(g.dragStartPos)
			switch g.mode {
			case ModePanning:
				if x := g.scroll.X - diff.X; x > 0 {
					g.scroll.X = x
				}
				if y := g.scroll.Y - diff.Y; y > 0 {
					g.scroll.Y = y
				}
				g.dragStartPos = e.Position
			case ModeMoving:
				if g.activeItem != nil {
					// 将屏幕坐标差值转换为世界坐标差值
					worldDiffX := diff.X / g.scale
					worldDiffY := diff.Y / g.scale
					g.activeItem.Pos = g.itemStartPos.Add(image.Pt(int(worldDiffX), int(worldDiffY)))
				}
			case ModeResizing:
				if g.activeItem != nil {
					g.updateGeometry(diff)
				}
			}
		case pointer.Release:
			g.mode = ModeNone
			g.activeHandle = HandleNone
		case pointer.Scroll:
			g.handleZoom(e.Position, e.Scroll.Y)
		}
	}
}

// handleZoom 处理鼠标滚轮缩放
func (g *GridComponent) handleZoom(mousePos f32.Point, scrollY float32) {
	// 保存缩放前的鼠标位置对应的世界坐标
	// 注意：世界坐标 = (屏幕坐标 + 滚动偏移) / 缩放比例
	worldXBefore := (g.scroll.X + mousePos.X) / g.scale
	worldYBefore := (g.scroll.Y + mousePos.Y) / g.scale
	// 计算缩放因子
	zoomFactor := float32(1.1) // 每次缩放10%
	if scrollY > 0 {
		// 向下滚动，缩小
		zoomFactor = 1.0 / zoomFactor
		if float32(g.GridLineSize)*g.scale < 10 {
			return
		}
	}
	// 应用缩放，限制在合理范围内
	newScale := g.scale * zoomFactor
	if newScale > 5.0 {
		newScale = 5.0
	}
	// 更新缩放比例
	g.scale = newScale
	// 计算缩放后，为了保持鼠标指向的世界坐标不变，需要调整滚动位置
	// 新的滚动位置 = 世界坐标 * 新缩放比例 - 鼠标屏幕坐标
	g.scroll.X = worldXBefore*newScale - mousePos.X
	g.scroll.Y = worldYBefore*newScale - mousePos.Y
	// 确保滚动位置在有效范围内
	g.scroll.X = max(0, min(g.scroll.X, float32(g.VirtualSize.X)*newScale-float32(g.lastSize.X)))
	g.scroll.Y = max(0, min(g.scroll.Y, float32(g.VirtualSize.Y)*newScale-float32(g.lastSize.Y)))
}

// updateGeometry 处理 8 个方向的缩放逻辑
func (g *GridComponent) updateGeometry(diff f32.Point) {
	// 将屏幕坐标差值转换为世界坐标差值
	worldDiffX := diff.X / g.scale
	worldDiffY := diff.Y / g.scale
	dx, dy := int(worldDiffX), int(worldDiffY)
	newPos := g.itemStartPos
	newSize := g.itemStartSize
	// 垂直方向
	switch g.activeHandle {
	case HandleNW, HandleN, HandleNE:
		newPos.Y += dy
		newSize.Y -= dy
	case HandleSW, HandleS, HandleSE:
		newSize.Y += dy
	}
	// 水平方向
	switch g.activeHandle {
	case HandleNW, HandleW, HandleSW:
		newPos.X += dx
		newSize.X -= dx
	case HandleNE, HandleE, HandleSE:
		newSize.X += dx
	}
	// 最小尺寸限制
	minSize := 20
	if newSize.X < minSize {
		if g.activeHandle == HandleNW || g.activeHandle == HandleW || g.activeHandle == HandleSW {
			newPos.X -= (minSize - newSize.X)
		}
		newSize.X = minSize
	}
	if newSize.Y < minSize {
		if g.activeHandle == HandleNW || g.activeHandle == HandleN || g.activeHandle == HandleNE {
			newPos.Y -= (minSize - newSize.Y)
		}
		newSize.Y = minSize
	}
	g.activeItem.Pos = newPos
	g.activeItem.Size = newSize
}

func (g *GridComponent) drawChildren(gtx layout.Context, viewSize image.Point) {
	// 创建缩放后的视口矩形
	viewRect := image.Rectangle{
		Min: image.Pt(int(g.scroll.X), int(g.scroll.Y)),
		Max: image.Pt(int(g.scroll.X)+viewSize.X, int(g.scroll.Y)+viewSize.Y),
	}
	for _, child := range g.Children {
		// 计算缩放后的子组件矩形
		scaledPosX := float32(child.Pos.X) * g.scale
		scaledPosY := float32(child.Pos.Y) * g.scale
		scaledSizeX := float32(child.Size.X) * g.scale
		scaledSizeY := float32(child.Size.Y) * g.scale
		childRect := image.Rectangle{
			Min: image.Pt(int(scaledPosX), int(scaledPosY)),
			Max: image.Pt(int(scaledPosX+scaledSizeX), int(scaledPosY+scaledSizeY)),
		}
		// 视口剔除优化
		if !viewRect.Overlaps(childRect) {
			continue
		}
		// 计算屏幕坐标
		screenPos := image.Pt(
			int(scaledPosX)-int(g.scroll.X),
			int(scaledPosY)-int(g.scroll.Y),
		)
		// 开启独立操作栈
		stack := op.Offset(screenPos).Push(gtx.Ops)
		// 应用缩放变换
		scaleOp := op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(g.scale, g.scale))).Push(gtx.Ops)
		// 强制物理剪裁，确保组件内容不溢出其原始 Size
		clipStack := clip.Rect{Max: child.Size}.Push(gtx.Ops)
		cgtx := gtx
		cgtx.Constraints = layout.Exact(child.Size)
		child.Widget(cgtx)
		clipStack.Pop()
		scaleOp.Pop()
		stack.Pop()
	}
}

func (g *GridComponent) drawSelectionHelpers(gtx layout.Context, _ image.Point) {
	child := g.activeItem
	rect := g.worldToScreenRect(child.Pos, child.Size)
	// 绘制包围框
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 120, B: 215, A: 255},
		clip.Stroke{Path: clip.Rect(f32RectToImage(rect)).Path(), Width: 2}.Op())
	// 绘制 8 个控制点
	if child.Resizable {
		handleSize := float32(gtx.Dp(unit.Dp(8)))
		for h := HandleN; h <= HandleSE; h++ {
			p := g.getHandlePos(rect, h)
			handleRect := image.Rect(p.X-int(handleSize/2), p.Y-int(handleSize/2), p.X+int(handleSize/2), p.Y+int(handleSize/2))
			paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, clip.Rect(f32RectToImage(handleRect)).Op())
			paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 120, B: 215, A: 255},
				clip.Stroke{Path: clip.Rect(f32RectToImage(handleRect)).Path(), Width: 1}.Op())
		}
	}
}

// 辅助函数：坐标转换
func (g *GridComponent) worldToScreenRect(pos, size image.Point) image.Rectangle {
	// 应用缩放
	scaledPosX := float32(pos.X) * g.scale
	scaledPosY := float32(pos.Y) * g.scale
	scaledSizeX := float32(size.X) * g.scale
	scaledSizeY := float32(size.Y) * g.scale
	return image.Rect(
		int(scaledPosX)-int(g.scroll.X),
		int(scaledPosY)-int(g.scroll.Y),
		int(scaledPosX+scaledSizeX)-int(g.scroll.X),
		int(scaledPosY+scaledSizeY)-int(g.scroll.Y),
	)
}

func (g *GridComponent) getHandlePos(r image.Rectangle, h Handle) image.Point {
	switch h {
	case HandleN:
		return image.Pt((r.Min.X+r.Max.X)/2, r.Min.Y)
	case HandleS:
		return image.Pt((r.Min.X+r.Max.X)/2, r.Max.Y)
	case HandleE:
		return image.Pt(r.Max.X, (r.Min.Y+r.Max.Y)/2)
	case HandleW:
		return image.Pt(r.Min.X, (r.Min.Y+r.Max.Y)/2)
	case HandleNW:
		return r.Min
	case HandleNE:
		return image.Pt(r.Max.X, r.Min.Y)
	case HandleSW:
		return image.Pt(r.Min.X, r.Max.Y)
	case HandleSE:
		return r.Max
	}
	return image.Point{}
}

func (g *GridComponent) hitTestHandles(mousePos f32.Point) Handle {
	if g.activeItem == nil || !g.activeItem.Resizable {
		return HandleNone
	}
	rect := g.worldToScreenRect(g.activeItem.Pos, g.activeItem.Size)
	threshold := float32(10.0)
	for h := HandleN; h <= HandleSE; h++ {
		hp := g.getHandlePos(rect, h)
		dist := float32(math.Hypot(float64(mousePos.X-float32(hp.X)), float64(mousePos.Y-float32(hp.Y))))
		if dist < threshold {
			return h
		}
	}
	return HandleNone
}

// drawGrid 绘制网格背景，使用缓存优化性能。
// 当控件大小改变或缩放比例改变时重建网格缓存，否则重用缓存的绘制指令。
func (g *GridComponent) drawGrid(gtx layout.Context, size image.Point) {
	// 检查大小或缩放比例是否改变，如果改变则重建缓存
	if size != g.lastSize || g.scale != g.lastScale {
		g.lastSize = size
		g.lastScale = g.scale
		g.gridCache.Reset() // 清空旧的缓存
		macro := op.Record(&g.gridCache)
		var p clip.Path
		p.Begin(&g.gridCache)
		// 计算缩放后的网格线间距
		scaledGridLineSize := float32(g.GridLineSize) * g.scale
		if scaledGridLineSize < 5 {
			// 如果网格线太密集，不绘制网格
			paint.FillShape(&g.gridCache, g.GridColor, clip.Rect{Max: size}.Op())
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
			paint.FillShape(&g.gridCache, g.GridColor, clip.Stroke{
				Path:  p.End(),
				Width: 1,
			}.Op())
		}
		g.gridCall = macro.Stop()
	}
	// 计算取模后的偏移量 (实现无限滚动视觉效果)
	// 使用负偏移，模拟背景随着滚动向反方向移动
	scaledGridLineSize := int(float32(g.GridLineSize) * g.scale)
	if scaledGridLineSize < 1 {
		scaledGridLineSize = 1
	}
	offX := -int(g.scroll.X) % scaledGridLineSize
	offY := -int(g.scroll.Y) % scaledGridLineSize
	// 应用偏移并调用缓存的指令
	trans := op.Offset(image.Point{X: offX, Y: offY}).Push(gtx.Ops)
	g.gridCall.Add(gtx.Ops)
	trans.Pop()
}

func f32RectToImage(r image.Rectangle) image.Rectangle {
	return image.Rect(int(r.Min.X), int(r.Min.Y), int(r.Max.X), int(r.Max.Y))
}

// pointInRect 检查点是否在矩形内
func pointInRect(p f32.Point, r image.Rectangle) bool {
	return float32(r.Min.X) <= p.X && p.X < float32(r.Max.X) &&
		float32(r.Min.Y) <= p.Y && p.Y < float32(r.Max.Y)
}
