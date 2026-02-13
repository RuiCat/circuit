package draw

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
)

// Handle 表示调整大小的控制点位置
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

// ResizeGeometry 处理 8 个方向的缩放逻辑
// 这个结构体封装了调整大小的几何计算，可以独立于 GridComponent 使用
type ResizeGeometry struct {
	// 初始状态
	StartPos  image.Point // 拖拽开始时组件的位置
	StartSize image.Point // 拖拽开始时组件的大小

	// 当前状态
	ActiveHandle Handle // 当前激活的控制点

	// 配置
	MinSize int     // 最小尺寸限制
	Scale   float32 // 缩放比例（用于屏幕坐标到世界坐标的转换）
}

// NewResizeGeometry 创建新的调整大小几何计算器
func NewResizeGeometry(startPos, startSize image.Point, activeHandle Handle, scale float32) *ResizeGeometry {
	return &ResizeGeometry{
		StartPos:     startPos,
		StartSize:    startSize,
		ActiveHandle: activeHandle,
		MinSize:      20, // 默认最小尺寸
		Scale:        scale,
	}
}

// HandleEvents 统一处理所有交互逻辑
// 这个方法处理与调整大小相关的指针事件
// 它检测控制点点击、处理拖拽，并返回更新后的位置和大小
func (rg *ResizeGeometry) HandleEvents(gtx layout.Context, rect image.Rectangle, threshold float32) (newPos, newSize image.Point, handleHit Handle, isDragging bool) {
	// 初始化返回值
	newPos = rg.StartPos
	newSize = rg.StartSize
	handleHit = HandleNone
	isDragging = false
	// 局部变量记录拖拽起始位置
	var dragStartPos f32.Point
	// 检查是否有指针事件
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Kinds: pointer.Press | pointer.Drag | pointer.Release,
		})
		if !ok {
			break
		}
		e, _ := ev.(pointer.Event)
		rg.HandlePointerEvent(e, rect, threshold, &newPos, &newSize, &handleHit, &isDragging, &dragStartPos)
	}
	return newPos, newSize, handleHit, isDragging
}

// HandlePointerEvent 处理单个指针事件
// 这个方法可以被其他组件调用，将事件委托给 ResizeGeometry 处理
func (rg *ResizeGeometry) HandlePointerEvent(e pointer.Event, rect image.Rectangle, threshold float32, newPos, newSize *image.Point, handleHit *Handle, isDragging *bool, dragStartPos *f32.Point) {
	switch e.Kind {
	case pointer.Press:
		// 检测是否点击了控制点
		*handleHit = HitTestHandles(e.Position, rect, threshold)
		if *handleHit != HandleNone {
			// 记录初始状态
			rg.StartPos = image.Pt(rect.Min.X, rect.Min.Y)
			rg.StartSize = image.Pt(rect.Dx(), rect.Dy())
			rg.ActiveHandle = *handleHit
			*dragStartPos = e.Position
			*isDragging = true
		}
	case pointer.Drag:
		if rg.ActiveHandle != HandleNone && *isDragging {
			// 计算拖拽差值
			diff := e.Position.Sub(*dragStartPos)
			// 更新拖拽起始位置，以便下一次计算相对差值
			*dragStartPos = e.Position
			*newPos, *newSize = rg.Update(diff)
		}
	case pointer.Release:
		if *isDragging {
			// 重置状态
			rg.ActiveHandle = HandleNone
			*isDragging = false
		}
	}
}

// Update 根据屏幕坐标的拖拽差值更新组件的位置和大小
// diff: 屏幕坐标的拖拽差值
// 返回: 新的位置和大小
func (rg *ResizeGeometry) Update(diff f32.Point) (newPos, newSize image.Point) {
	// 将屏幕坐标差值转换为世界坐标差值
	worldDiffX := diff.X / rg.Scale
	worldDiffY := diff.Y / rg.Scale
	dx, dy := int(worldDiffX), int(worldDiffY)
	newPos = rg.StartPos
	newSize = rg.StartSize
	// 垂直方向
	switch rg.ActiveHandle {
	case HandleNW, HandleN, HandleNE:
		newPos.Y += dy
		newSize.Y -= dy
	case HandleSW, HandleS, HandleSE:
		newSize.Y += dy
	}
	// 水平方向
	switch rg.ActiveHandle {
	case HandleNW, HandleW, HandleSW:
		newPos.X += dx
		newSize.X -= dx
	case HandleNE, HandleE, HandleSE:
		newSize.X += dx
	}
	// 应用最小尺寸限制
	newPos, newSize = rg.applyMinSizeConstraint(newPos, newSize)
	return newPos, newSize
}

// applyMinSizeConstraint 应用最小尺寸限制
func (rg *ResizeGeometry) applyMinSizeConstraint(pos, size image.Point) (image.Point, image.Point) {
	if size.X < rg.MinSize {
		if rg.ActiveHandle == HandleNW || rg.ActiveHandle == HandleW || rg.ActiveHandle == HandleSW {
			pos.X -= (rg.MinSize - size.X)
		}
		size.X = rg.MinSize
	}
	if size.Y < rg.MinSize {
		if rg.ActiveHandle == HandleNW || rg.ActiveHandle == HandleN || rg.ActiveHandle == HandleNE {
			pos.Y -= (rg.MinSize - size.Y)
		}
		size.Y = rg.MinSize
	}
	return pos, size
}

// GetHandlePosition 获取指定控制点在矩形上的位置
func GetHandlePosition(rect image.Rectangle, h Handle) image.Point {
	switch h {
	case HandleN:
		return image.Pt((rect.Min.X+rect.Max.X)/2, rect.Min.Y)
	case HandleS:
		return image.Pt((rect.Min.X+rect.Max.X)/2, rect.Max.Y)
	case HandleE:
		return image.Pt(rect.Max.X, (rect.Min.Y+rect.Max.Y)/2)
	case HandleW:
		return image.Pt(rect.Min.X, (rect.Min.Y+rect.Max.Y)/2)
	case HandleNW:
		return rect.Min
	case HandleNE:
		return image.Pt(rect.Max.X, rect.Min.Y)
	case HandleSW:
		return image.Pt(rect.Min.X, rect.Max.Y)
	case HandleSE:
		return rect.Max
	}
	return image.Point{}
}

// HitTestHandles 检测鼠标位置是否命中控制点
func HitTestHandles(mousePos f32.Point, rect image.Rectangle, threshold float32) Handle {
	for h := HandleN; h <= HandleSE; h++ {
		hp := GetHandlePosition(rect, h)
		dist := float32(math.Hypot(float64(mousePos.X-float32(hp.X)), float64(mousePos.Y-float32(hp.Y))))
		if dist < threshold {
			return h
		}
	}
	return HandleNone
}
