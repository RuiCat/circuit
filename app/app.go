package app

import (
	"image"
	"image/color"
	"log"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// CircuitApp 电路绘制应用程序
// 管理应用程序的主窗口、主题和网格组件
type CircuitApp struct {
	window *app.Window     // 应用程序窗口
	theme  *material.Theme // Gio 主题
	grid   *NodeComponent  // 节点组件（包含网格和电路元素）

	// 状态切换按钮
	viewModeBtn widget.Clickable // 查看模式按钮
	editModeBtn widget.Clickable // 编辑模式按钮
	drawModeBtn widget.Clickable // 绘制模式按钮
}

// NewCircuitApp 创建新的电路绘制应用程序
// 初始化应用程序窗口、主题和测试电路组件
// 返回配置好的 CircuitApp 实例
func NewCircuitApp() *CircuitApp {
	window := new(app.Window)
	theme := material.NewTheme()

	// 创建子组件列表（用于演示的测试组件）
	childrenList := &NodeChildren{
		&NodePoint{
			ChildItem: ChildItem{
				ID:      "rect1",
				Movable: true,
				Size:    image.Pt(200, 150),
				Pos:     image.Pt(100, 100),
				Widget: func(gtx layout.Context) layout.Dimensions {
					// 绘制一个红色矩形
					rect := image.Rect(0, 0, 200, 150)
					paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 100, B: 100, A: 255}, clip.Rect(rect).Op())
					// 添加文字标签
					return layout.Dimensions{Size: image.Pt(200, 150)}
				},
			},
		},
		&NodePoint{
			ChildItem: ChildItem{
				ID:      "rect2",
				Movable: true,
				Size:    image.Pt(150, 200),
				Pos:     image.Pt(400, 200),
				Widget: func(gtx layout.Context) layout.Dimensions {
					// 绘制一个蓝色矩形
					rect := image.Rect(0, 0, 150, 200)
					paint.FillShape(gtx.Ops, color.NRGBA{R: 100, G: 100, B: 255, A: 255}, clip.Rect(rect).Op())
					return layout.Dimensions{Size: image.Pt(150, 200)}
				},
			},
		},
		&NodePoint{
			ChildItem: ChildItem{
				ID:        "rect3",
				Movable:   true,
				Resizable: true,
				Size:      image.Pt(180, 120),
				Pos:       image.Pt(200, 400),
				Widget: func(gtx layout.Context) layout.Dimensions {
					// 绘制一个绿色矩形（可缩放）
					size := gtx.Constraints.Max
					rect := image.Rectangle{Max: size}
					paint.FillShape(gtx.Ops, color.NRGBA{R: 100, G: 255, B: 100, A: 255}, clip.Rect(rect).Op())
					// 返回实际使用的大小
					return layout.Dimensions{Size: size}
				},
			},
		},
	}

	grid := NewNodeComponent(childrenList)

	// 添加测试连接点（使用相对位置）
	grid.PointList = []*Point{
		{
			ID:       "point1",
			Type:     0,
			NodeID:   "rect1", // 属于第一个矩形
			Position: image.Pt(50, 75),
			IsMatrix: true, // 矩阵控制点，不能移动
		},
		{
			ID:       "point2",
			Type:     0,
			NodeID:   "rect2", // 属于第二个矩形
			Position: image.Pt(75, 100),
			IsMatrix: true, // 矩阵控制点，不能移动
		},
		{
			ID:       "point3",
			Type:     0,
			NodeID:   "rect3", // 属于第三个矩形
			Position: image.Pt(90, 60),
			IsMatrix: false, // 用户绘制的连接点，可以移动
		},
	}

	// 添加测试连线（使用新的数据结构）
	grid.Connections = []*Connection{
		{
			ID: "conn1",
			Segments: []*Segment{
				{
					ID:       "seg1",
					StartIdx: 0,
					EndIdx:   1,
					Control: []image.Point{
						{250, 150}, // 控制点1
						{300, 120}, // 控制点2
					},
					Selected: false,
					Color:    color.NRGBA{R: 255, G: 0, B: 0, A: 255},
					Width:    2,
				},
			},
			Nodes: []*IntersectionNode{
				{
					ID:       "node1",
					Position: image.Pt(300, 200),
					Segments: []string{"seg1"},
				},
			},
			Color: color.NRGBA{R: 255, G: 0, B: 0, A: 255},
			Width: 2,
		},
		{
			ID: "conn2",
			Segments: []*Segment{
				{
					ID:       "seg2",
					StartIdx: 1,
					EndIdx:   2,
					Selected: false,
					Color:    color.NRGBA{R: 0, G: 255, B: 0, A: 255},
					Width:    3,
				},
			},
			Color: color.NRGBA{R: 0, G: 255, B: 0, A: 255},
			Width: 2,
		},
		{
			ID: "conn3",
			Segments: []*Segment{
				{
					ID:       "seg3",
					StartIdx: 2,
					EndIdx:   0,
					Selected: false,
					Color:    color.NRGBA{R: 0, G: 0, B: 255, A: 255},
					Width:    2,
				},
			},
			Color: color.NRGBA{R: 0, G: 0, B: 255, A: 255},
			Width: 2,
		},
	}

	app := &CircuitApp{
		window: window,
		theme:  theme,
		grid:   grid,
	}
	return app
}

// Window 返回应用程序窗口
// 提供对底层 Gio 窗口的访问，用于自定义窗口操作
func (a *CircuitApp) Window() *app.Window {
	return a.window
}

// Run 运行应用程序主循环
// 启动 Gio 事件循环，处理窗口事件和渲染帧
// 该方法会阻塞直到应用程序退出
func (a *CircuitApp) Run() {
	a.window.Run(func() {
		var ops op.Ops
		for {
			e := a.window.Event()
			switch e := e.(type) {
			case app.DestroyEvent:
				log.Fatal(e.Err)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)

				// 处理按钮点击事件
				a.handleButtonEvents(gtx)

				// 绘制界面
				a.drawUI(gtx)

				e.Frame(gtx.Ops)
			}
		}
	})
}

// handleButtonEvents 处理状态切换按钮的点击事件
func (a *CircuitApp) handleButtonEvents(gtx layout.Context) {
	if a.viewModeBtn.Clicked(gtx) {
		a.grid.SetGridMode(GridModeView)
	}
	if a.editModeBtn.Clicked(gtx) {
		a.grid.SetGridMode(GridModeEdit)
	}
	if a.drawModeBtn.Clicked(gtx) {
		a.grid.SetGridMode(GridModeDraw)
	}
}

// drawUI 绘制应用程序界面
// 包括状态切换按钮和网格组件
func (a *CircuitApp) drawUI(gtx layout.Context) {
	// 获取当前模式以高亮对应按钮
	currentMode := a.grid.GetGridMode()

	// 使用堆叠布局：按钮在顶部，网格在底部
	layout.Stack{}.Layout(gtx,
		// 网格层（占据整个空间）
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return a.grid.Layout(gtx)
		}),
		// 按钮层（顶部工具栏）
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return a.drawToolbar(gtx, currentMode)
		}),
	)
}

// drawToolbar 绘制顶部工具栏
// 包含三个状态切换按钮
func (a *CircuitApp) drawToolbar(gtx layout.Context, currentMode GridMode) layout.Dimensions {
	// 工具栏高度
	toolbarHeight := gtx.Dp(unit.Dp(40))

	// 创建工具栏背景
	rect := image.Rect(0, 0, gtx.Constraints.Max.X, toolbarHeight)
	paint.FillShape(gtx.Ops, color.NRGBA{R: 240, G: 240, B: 240, A: 255}, clip.Rect(rect).Op())

	// 按钮布局
	return layout.UniformInset(unit.Dp(5)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Spacing:   layout.SpaceBetween,
			Alignment: layout.Middle,
		}.Layout(gtx,
			// 查看模式按钮
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btnColor := color.NRGBA{R: 100, G: 100, B: 100, A: 255}
				if currentMode == GridModeView {
					btnColor = color.NRGBA{R: 0, G: 120, B: 215, A: 255}
				}
				return a.drawModeButton(gtx, &a.viewModeBtn, "查看", btnColor)
			}),
			// 编辑模式按钮
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btnColor := color.NRGBA{R: 100, G: 100, B: 100, A: 255}
				if currentMode == GridModeEdit {
					btnColor = color.NRGBA{R: 0, G: 120, B: 215, A: 255}
				}
				return a.drawModeButton(gtx, &a.editModeBtn, "编辑", btnColor)
			}),
			// 绘制模式按钮
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btnColor := color.NRGBA{R: 100, G: 100, B: 100, A: 255}
				if currentMode == GridModeDraw {
					btnColor = color.NRGBA{R: 0, G: 120, B: 215, A: 255}
				}
				return a.drawModeButton(gtx, &a.drawModeBtn, "绘制", btnColor)
			}),
		)
	})
}

// drawModeButton 绘制单个模式按钮
func (a *CircuitApp) drawModeButton(gtx layout.Context, btn *widget.Clickable, label string, color color.NRGBA) layout.Dimensions {
	// 创建按钮
	button := material.Button(a.theme, btn, label)
	button.Background = color
	button.CornerRadius = unit.Dp(4)

	// 设置按钮内边距
	inset := layout.Inset{
		Top:    unit.Dp(5),
		Bottom: unit.Dp(5),
		Left:   unit.Dp(15),
		Right:  unit.Dp(15),
	}

	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return button.Layout(gtx)
	})
}
