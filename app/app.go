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
	"gioui.org/widget/material"
)

// CircuitApp 电路绘制应用程序
type CircuitApp struct {
	window *app.Window
	theme  *material.Theme
	grid   *GridComponent
}

// NewCircuitApp 创建新的电路绘制应用程序
func NewCircuitApp() *CircuitApp {
	window := new(app.Window)
	theme := material.NewTheme()

	grid := NewGridComponent()
	grid.Children = []*ChildItem{
		{
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
		{
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
		{
			Movable:   true,
			Resizable: true,
			Size:      image.Pt(180, 120),
			Pos:       image.Pt(200, 400),
			Widget: func(gtx layout.Context) layout.Dimensions {
				// 绘制一个绿色矩形（可缩放）
				rect := image.Rect(0, 0, 180, 120)
				paint.FillShape(gtx.Ops, color.NRGBA{R: 100, G: 255, B: 100, A: 255}, clip.Rect(rect).Op())
				return layout.Dimensions{Size: image.Pt(180, 120)}
			},
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
func (a *CircuitApp) Window() *app.Window {
	return a.window
}

// Run 运行应用程序
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

				// 绘制网格
				a.grid.Layout(gtx)

				e.Frame(gtx.Ops)
			}
		}
	})
}
