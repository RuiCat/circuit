package app

import (
	"circuit/app/draw"
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Handle 表示调整大小的控制点位置。
// 用于标识用户正在拖拽哪个方向的控制点来调整组件大小。
type Handle int

const (
	HandleNone Handle = iota // 无控制点
	HandleN                  // 北（上）控制点
	HandleS                  // 南（下）控制点
	HandleE                  // 东（右）控制点
	HandleW                  // 西（左）控制点
	HandleNW                 // 西北（左上）控制点
	HandleNE                 // 东北（右上）控制点
	HandleSW                 // 西南（左下）控制点
	HandleSE                 // 东南（右下）控制点
)

// InteractionMode 表示当前的交互模式。
// 定义了用户与网格组件交互时的不同状态。
type InteractionMode int

const (
	ModeNone      InteractionMode = iota // 无交互
	ModePanning                          // 平移背景
	ModeMoving                           // 移动组件
	ModeResizing                         // 缩放组件
	ModeDrawing                          // 绘制连线模式
	ModeEditing                          // 编辑线段模式
	ModeSelecting                        // 选择线段模式
)

// GridMode 表示网格组件的交互模式
type GridMode int

const (
	GridModeView GridMode = iota // 查看状态：只能查看，不能编辑
	GridModeEdit                 // 编辑状态：可以编辑控制点、连接点
	GridModeDraw                 // 绘制状态：可以绘制新连线
)

// ChildItem 表示网格中的一个子组件。
// 每个子组件可以有自己的位置、大小、可移动性和可调整大小属性。
type ChildItem struct {
	ID        string        // 组件唯一标识符
	Pos       image.Point   // 虚拟空间位置（世界坐标）
	Size      image.Point   // 组件大小（世界坐标）
	Widget    layout.Widget // Gio 布局组件
	Movable   bool          // 是否可移动
	Resizable bool          // 是否可调整大小
}

// ChildrenManager 子组件管理器接口
// 定义了对子组件列表的操作，使得 GridComponent 不直接依赖具体的实现
type ChildrenManager interface {
	GetByID(id string) *ChildItem
	Len() int
	Iterate(fn func(child *ChildItem) bool)
	UpdatePosition(id string, pos image.Point)
	UpdateSize(id string, size image.Point)
	FindAtPosition(pos f32.Point, worldToScreen func(pos, size image.Point) image.Rectangle) *ChildItem
}

// Children 子组件列表的默认实现
type Children []*ChildItem // 子组件列表

// GetByID 根据ID获取子组件
func (c *Children) GetByID(id string) *ChildItem {
	for i := range *c {
		if (*c)[i].ID == id {
			return (*c)[i]
		}
	}
	return nil
}

// GetByIndex 根据索引获取子组件
func (c *Children) GetByIndex(index int) *ChildItem {
	if index < 0 || index >= len(*c) {
		return nil
	}
	return (*c)[index]
}

// Len 返回子组件数量
func (c *Children) Len() int {
	return len(*c)
}

// Iterate 遍历所有子组件
func (c *Children) Iterate(fn func(child *ChildItem) bool) {
	for i := range *c {
		if !fn((*c)[i]) {
			break
		}
	}
}

// UpdatePosition 更新子组件位置
func (c *Children) UpdatePosition(id string, pos image.Point) {
	if child := c.GetByID(id); child != nil {
		child.Pos = pos
	}
}

// UpdateSize 更新子组件大小
func (c *Children) UpdateSize(id string, size image.Point) {
	if child := c.GetByID(id); child != nil {
		child.Size = size
	}
}

// FindAtPosition 在指定位置查找子组件
func (c *Children) FindAtPosition(pos f32.Point, worldToScreen func(pos, size image.Point) image.Rectangle) *ChildItem {
	// 从后往前遍历，这样最后添加的组件在最上面
	for i := len(*c) - 1; i >= 0; i-- {
		child := (*c)[i]
		rect := worldToScreen(child.Pos, child.Size)
		if pointInRect(pos, rect) {
			return child
		}
	}
	return nil
}

// GridComponent 是可交互的网格组件
type GridComponent struct {
	VirtualSize image.Point     // 虚拟空间总大小
	Children    ChildrenManager // 子组件管理器

	// 事件拦截钩子
	EventHook func(gtx layout.Context, e pointer.Event, d *draw.Draw) bool

	// 滚动和缩放状态 (作为数据源)
	scroll f32.Point
	scale  float32

	// 交互状态
	mode          InteractionMode
	activeItemID  string
	activeHandle  draw.Handle // 使用 draw 包中的定义
	dragStartPos  f32.Point
	itemStartPos  image.Point
	itemStartSize image.Point

	// 网格组件状态
	gridMode GridMode

	// 绘图辅助
	Background *draw.GridBackground
	Theme      *material.Theme
	scrollbar  [2]*widget.Scrollbar
}

// NewGridComponent 创建新的网格组件实例
func NewGridComponent(theme *material.Theme) *GridComponent {
	return &GridComponent{
		Theme:       theme,
		VirtualSize: image.Pt(5000, 5000),
		scale:       1.0,
		scrollbar:   [2]*widget.Scrollbar{{}, {}},
		// 使用新的 GridBackground
		Background: draw.NewGridBackground(20, color.NRGBA{R: 230, G: 230, B: 230, A: 255}),
	}
}

// activeItem 获取当前激活的子组件
func (g *GridComponent) activeItem() *ChildItem {
	if g.activeItemID == "" {
		return nil
	}
	return g.Children.GetByID(g.activeItemID)
}

// Layout 实现 Gio 的布局接口
func (g *GridComponent) Layout(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	// 1. 初始化 Draw 上下文
	// 将当前的 grid 状态传递给 Draw
	d := &draw.Draw{
		Context: gtx,
		Ops:     gtx.Ops,
		Scale:   g.scale,
		Scroll:  g.scroll,
	}
	// 2. 处理事件 (包括 Grid 自身的交互和 Draw 的缩放/平移)
	g.handleEvents(gtx, d)
	// 3. 同步状态回来 (因为 Draw 可能会处理缩放和滚轮)
	g.scale = d.Scale
	g.scroll = d.Scroll
	// 4. 渲染
	return layout.Stack{}.Layout(gtx,
		// 背景层：网格
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
			// 注册事件监听
			event.Op(gtx.Ops, g)

			// 使用 GridBackground 绘制
			g.Background.Draw(d, size)
			return layout.Dimensions{Size: size}
		}),
		// 内容层：子组件
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			g.drawChildren(gtx, d)
			return layout.Dimensions{Size: size}
		}),
		// 装饰层：选中框和控制点
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if g.activeItem() != nil {
				g.drawSelectionHelpers(gtx, d)
			}
			return layout.Dimensions{Size: size}
		}),
		// 滚动条层 (复用之前的逻辑，略作简化)
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

// handleEvents 处理交互逻辑
func (g *GridComponent) handleEvents(gtx layout.Context, d *draw.Draw) {
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
		// --- 1. 调用钩子 ---
		if g.EventHook != nil {
			if g.EventHook(gtx, e, d) {
				continue
			}
		}
		// --- 2. 底层逻辑 ---
		switch e.Kind {
		case pointer.Press:
			g.dragStartPos = e.Position
			// 检查是否点击了控制点 (Resize)
			if g.activeItemID != "" {
				activeItem := g.Children.GetByID(g.activeItemID)
				if activeItem != nil && activeItem.Resizable {
					rect := g.worldToScreenRect(d, activeItem.Pos, activeItem.Size)
					// 使用 draw.HitTestHandles
					if h := draw.HitTestHandles(e.Position, rect, 10.0); h != draw.HandleNone {
						g.mode = ModeResizing
						g.activeHandle = h
						g.itemStartPos = activeItem.Pos
						g.itemStartSize = activeItem.Size
						break
					}
				}
			}
			// 检查是否点击了组件 (Move)
			if g.mode == ModeNone {
				// 使用 d.WorldToScreenF32 辅助转换的闭包
				converter := func(pos, size image.Point) image.Rectangle {
					return g.worldToScreenRect(d, pos, size)
				}
				if child := g.Children.FindAtPosition(e.Position, converter); child != nil {
					if g.gridMode == GridModeView {
						return
					}
					g.activeItemID = child.ID
					if child.Movable {
						g.mode = ModeMoving
						g.itemStartPos = child.Pos
					}
				} else {
					g.activeItemID = ""
					g.mode = ModePanning
				}
			}
		case pointer.Drag:
			diff := e.Position.Sub(g.dragStartPos)
			switch g.mode {
			case ModePanning:
				// 手动更新 d.Scroll，因为我们不仅想用滚轮，还想用拖拽
				d.Scroll.X -= diff.X
				d.Scroll.Y -= diff.Y
				// 限制范围
				d.Scroll.X = max(0, d.Scroll.X)
				d.Scroll.Y = max(0, d.Scroll.Y)
				g.dragStartPos = e.Position
			case ModeMoving:
				if g.activeItemID != "" {
					worldDiff := image.Pt(int(diff.X/d.Scale), int(diff.Y/d.Scale))
					newPos := g.itemStartPos.Add(worldDiff)
					g.Children.UpdatePosition(g.activeItemID, g.SnapToGrid(newPos))
				}
			case ModeResizing:
				// 使用 ResizeGeometry 计算新几何属性
				rg := draw.NewResizeGeometry(g.itemStartPos, g.itemStartSize, g.activeHandle, d.Scale)
				newPos, newSize := rg.Update(diff)
				if activeItem := g.activeItem(); activeItem != nil {
					g.Children.UpdatePosition(activeItem.ID, newPos)
					g.Children.UpdateSize(activeItem.ID, newSize)
				}
			}
		case pointer.Release:
			if g.mode != ModeDrawing { // Drawing 模式通常由 NodeComponent 处理
				g.mode = ModeNone
				g.activeHandle = draw.HandleNone
			}
		case pointer.Scroll:
			// 委托给 Draw 处理缩放
			d.HandlePointerEvent(e)

		}
	}
}

// drawChildren 绘制所有子组件
func (g *GridComponent) drawChildren(gtx layout.Context, d *draw.Draw) {
	g.Children.Iterate(func(child *ChildItem) bool {
		// 使用 Draw 提供的坐标转换
		screenPos := d.WorldToScreenF32(child.Pos)
		stack := op.Offset(image.Pt(int(screenPos.X), int(screenPos.Y))).Push(gtx.Ops)
		scaleOp := op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(d.Scale, d.Scale))).Push(gtx.Ops)
		cgtx := gtx
		cgtx.Constraints = layout.Exact(child.Size)
		child.Widget(cgtx)
		scaleOp.Pop()
		stack.Pop()
		return true
	})
}

// drawSelectionHelpers 绘制选中装饰
func (g *GridComponent) drawSelectionHelpers(gtx layout.Context, d *draw.Draw) {
	child := g.activeItem()
	if child == nil || g.gridMode == GridModeView {
		return
	}
	rect := g.worldToScreenRect(d, child.Pos, child.Size)
	// 绘制包围框 (使用 Draw 包)
	d.DrawRect(rect, 2, color.NRGBA{}, color.NRGBA{R: 0, G: 120, B: 215, A: 255}, false)
	// 绘制 8 个控制点
	if child.Resizable {
		handleSize := float32(gtx.Dp(unit.Dp(8)))
		for h := draw.HandleN; h <= draw.HandleSE; h++ {
			p := draw.GetHandlePosition(rect, h)
			// DrawRect 需要中心对齐
			r := image.Rect(p.X-int(handleSize/2), p.Y-int(handleSize/2), p.X+int(handleSize/2), p.Y+int(handleSize/2))
			d.DrawRect(r, 1, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, color.NRGBA{R: 0, G: 120, B: 215, A: 255}, true)
		}
	}
}

// worldToScreenRect 辅助转换
func (g *GridComponent) worldToScreenRect(d *draw.Draw, pos, size image.Point) image.Rectangle {
	p1 := d.WorldToScreenF32(pos)
	// 计算右下角：需要先计算世界坐标的右下角，再转屏幕，保证缩放正确
	p2 := d.WorldToScreenF32(pos.Add(size))
	return image.Rect(int(p1.X), int(p1.Y), int(p2.X), int(p2.Y))
}

// SnapToGrid 吸附网格
func (g *GridComponent) SnapToGrid(p image.Point) image.Point {
	gridSize := g.Background.GridLineSize // 从 Background 获取配置
	x := int(math.Round(float64(p.X)/float64(gridSize))) * gridSize
	y := int(math.Round(float64(p.Y)/float64(gridSize))) * gridSize
	return image.Pt(x, y)
}

// pointInRect 检查点是否在矩形内。
// 用于判断鼠标事件是否发生在特定区域内，支持浮点坐标。
func pointInRect(p f32.Point, r image.Rectangle) bool {
	return float32(r.Min.X) <= p.X && p.X < float32(r.Max.X) &&
		float32(r.Min.Y) <= p.Y && p.Y < float32(r.Max.Y)
}

// SetGridMode / GetGridMode 保持不变
func (g *GridComponent) SetGridMode(mode GridMode) {
	g.gridMode = mode
	if mode != GridModeEdit {
		g.mode = ModeNone
		g.activeItemID = ""
		g.activeHandle = draw.HandleNone
	}
}

func (g *GridComponent) GetGridMode() GridMode {
	return g.gridMode
}
