package app

import (
	"circuit/app/draw"
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// PointType 表示连接点的类型
type PointType int

const PointDefault PointType = iota

// Point 表示电路中的连接点
type Point struct {
	ID       string
	Type     PointType
	NodeID   string
	Position image.Point
	IsMatrix bool
}

// NodePoint 节点组件包装器
type NodePoint struct{ ChildItem }

// NodeChildren 节点子组件列表
type NodeChildren []*NodePoint

func (c *NodeChildren) GetByID(id string) *ChildItem {
	for i := range *c {
		if (*c)[i].ID == id {
			return &(*c)[i].ChildItem
		}
	}
	return nil
}

func (c *NodeChildren) Len() int { return len(*c) }

func (c *NodeChildren) Iterate(fn func(child *ChildItem) bool) {
	for i := range *c {
		if !fn(&(*c)[i].ChildItem) {
			break
		}
	}
}

func (c *NodeChildren) UpdatePosition(id string, pos image.Point) {
	if item := c.GetByID(id); item != nil {
		item.Pos = pos
	}
}

func (c *NodeChildren) UpdateSize(id string, size image.Point) {
	if item := c.GetByID(id); item != nil {
		item.Size = size
	}
}

func (c *NodeChildren) FindAtPosition(pos f32.Point, worldToScreen func(pos, size image.Point) image.Rectangle) *ChildItem {
	for i := len(*c) - 1; i >= 0; i-- {
		rect := worldToScreen((*c)[i].Pos, (*c)[i].Size)
		if pos.X >= float32(rect.Min.X) && pos.X <= float32(rect.Max.X) &&
			pos.Y >= float32(rect.Min.Y) && pos.Y <= float32(rect.Max.Y) {
			return &(*c)[i].ChildItem
		}
	}
	return nil
}

// Segment 表示连接线的一段
type Segment struct {
	ID               string
	StartIdx, EndIdx int
	Control          []image.Point
	Selected         bool
	Color            color.NRGBA
	Width            float32
}

// IntersectionNode 表示连接线的交叉节点
type IntersectionNode struct {
	ID       string
	Position image.Point
	Segments []string
}

// Connection 表示完整的电路连接
type Connection struct {
	ID       string
	Segments []*Segment
	Nodes    []*IntersectionNode
	Color    color.NRGBA
	Width    float32
}

// NodeComponent 电路节点组件
type NodeComponent struct {
	*GridComponent
	Children    *NodeChildren
	PointList   []*Point
	Connections []*Connection

	// 交互状态
	drawingStartPoint *Point
	drawingTempPos    image.Point
	selectedSegment   *Segment
	selectedControl   *image.Point
	selectedPoint     *Point
}

// NewNodeComponent 创建新的节点组件实例
func NewNodeComponent(children *NodeChildren) *NodeComponent {
	grid := NewGridComponent(material.NewTheme())
	grid.Children = children
	n := &NodeComponent{
		GridComponent: grid,
		Children:      children,
	}
	grid.EventHook = n.handleHookEvent
	return n
}

// Layout 绘制实现
func (n *NodeComponent) Layout(gtx layout.Context) layout.Dimensions {
	n.GridComponent.Update(gtx)
	size := gtx.Constraints.Max
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return n.GridComponent.Layout(gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			// 在上层创建临时的 Draw 上下文用于绘制电路
			d := &draw.Draw{
				Context: gtx,
				Ops:     gtx.Ops,
				Scale:   n.GridComponent.scale,
				Scroll:  n.GridComponent.scroll,
			}
			n.drawConnections(d)
			n.drawDrawingPreview(d)
			n.drawPoints(gtx, d)
			return layout.Dimensions{Size: size}
		}),
	)
}

// Update 处理交互事件并更新组件状态（缩放、滚动等）。
// 必须在 Layout 之前调用，以确保所有依赖该状态的子组件都能获取到最新数据。
func (g *GridComponent) Update(gtx layout.Context) {
	// 初始化临时的 Draw 上下文用于计算事件
	d := &draw.Draw{
		Context: gtx,
		Ops:     gtx.Ops,
		Scale:   g.scale,
		Scroll:  g.scroll,
	}
	// 处理事件 (HandlePointerEvent 等)
	// 注意：这里调用 handleEvents 会消耗掉当前帧的事件队列
	g.handleEvents(gtx, d)
	// 将 Draw 中计算出的最新状态同步回 GridComponent
	g.scale = d.Scale
	g.scroll = d.Scroll
}

// handleHookEvent 核心交互逻辑
func (n *NodeComponent) handleHookEvent(gtx layout.Context, e pointer.Event, d *draw.Draw) bool {
	switch e.Kind {
	case pointer.Press:
		return n.onPress(e, d)
	case pointer.Drag:
		return n.onDrag(e, d)
	case pointer.Release:
		return n.onRelease(e, d)
	}
	return false
}

// onPress 处理鼠标按下
func (n *NodeComponent) onPress(e pointer.Event, d *draw.Draw) bool {
	gridMode := n.GetGridMode()
	// 1. 检查连接点 (优先级最高)
	if pt := n.findPointAt(e.Position, d); pt != nil {
		if gridMode == GridModeEdit && !pt.IsMatrix {
			// 编辑模式：移动点
			n.clearSelection()
			n.selectedPoint = pt
			n.mode = ModeMoving
			return true
		}
		if gridMode == GridModeDraw {
			// 绘制模式：从现有连接点开始绘制
			n.clearSelection()
			n.drawingStartPoint = pt
			n.drawingTempPos = pt.Position // 初始临时位置等于起点
			n.mode = ModeDrawing
			return true
		}
	}
	// 2. 检查控制点 (仅编辑模式)
	if cp, seg := n.findControlPointAt(e.Position, d); cp != nil && gridMode == GridModeEdit {
		n.clearSelection()
		n.selectedControl = cp
		n.selectedSegment = seg
		seg.Selected = true
		n.mode = ModeEditing
		return true
	}
	// 3. 检查线段 (编辑或绘制模式)
	if seg := n.findSegmentAt(e.Position, d); seg != nil {
		if gridMode == GridModeEdit {
			// 编辑模式：选中线段
			n.clearSelection()
			seg.Selected = true
			n.selectedSegment = seg
			n.mode = ModeSelecting
			return true
		}
		if gridMode == GridModeDraw {
			// 绘制模式：【关键逻辑】从线段上开始绘制（打断线段）
			// 1. 找到线段所属的 Connection
			var targetConn *Connection
			for _, conn := range n.Connections {
				for _, s := range conn.Segments {
					if s == seg {
						targetConn = conn
						break
					}
				}
				if targetConn != nil {
					break
				}
			}
			if targetConn != nil {
				// 2. 执行打断并在该位置创建新节点
				if intersectionPoint, ok := n.splitSegmentAtPosition(seg, targetConn, e.Position, d); ok {
					n.clearSelection()
					// 3. 从新创建的交叉点开始绘制
					n.drawingStartPoint = intersectionPoint
					n.drawingTempPos = intersectionPoint.Position
					n.mode = ModeDrawing
					return true
				}
			}
		}
	}
	n.clearSelection()
	// 4. 点击空白处
	if gridMode == GridModeDraw {
		// 绘制模式：在空白处创建新点并开始绘制
		snapPos := n.SnapToGrid(n.screenToWorld(e.Position, d))
		newPoint := &Point{
			ID:       fmt.Sprintf("p%d", time.Now().UnixNano()),
			Position: snapPos,
			IsMatrix: false,
		}
		n.PointList = append(n.PointList, newPoint)
		n.drawingStartPoint = newPoint
		n.drawingTempPos = snapPos
		n.mode = ModeDrawing
		return true
	}
	return false
}

// onDrag 处理拖拽
func (n *NodeComponent) onDrag(e pointer.Event, d *draw.Draw) bool {
	if n.mode == ModeDrawing {
		// 更新绘制时的临时终点（跟随鼠标并吸附网格）
		n.drawingTempPos = n.SnapToGrid(n.screenToWorld(e.Position, d))
		return true
	}
	if n.mode == ModeEditing && n.selectedControl != nil {
		*n.selectedControl = n.SnapToGrid(n.screenToWorld(e.Position, d))
		return true
	}
	if n.mode == ModeMoving && n.selectedPoint != nil {
		n.selectedPoint.Position = n.SnapToGrid(n.screenToWorld(e.Position, d))
		return true
	}
	return false
}

// onRelease 处理释放
func (n *NodeComponent) onRelease(e pointer.Event, d *draw.Draw) bool {
	if n.mode == ModeDrawing {
		// 完成绘制
		n.finishConnection(e.Position, d)
		n.mode = ModeNone
		return true
	}
	if n.mode != ModeSelecting {
		n.mode = ModeNone
	}
	return false
}

// finishConnection 完成连线
func (n *NodeComponent) finishConnection(pos f32.Point, d *draw.Draw) {
	// 1. 尝试找到鼠标释放位置下的现有点
	endPt := n.findPointAt(pos, d)
	// 标记是否创建了新的终点
	createdNewEndPoint := false
	// 2. 如果没有找到，创建一个新点
	if endPt == nil {
		snapPos := n.SnapToGrid(n.screenToWorld(pos, d))
		// 优化：如果新计算的位置和起点位置一模一样，就不要创建新点了，直接算作原地点击
		if n.drawingStartPoint != nil && snapPos.Eq(n.drawingStartPoint.Position) {
			endPt = n.drawingStartPoint
		} else {
			endPt = &Point{
				ID:       fmt.Sprintf("j%d", time.Now().UnixNano()),
				Position: snapPos,
				IsMatrix: false,
			}
			n.PointList = append(n.PointList, endPt)
			createdNewEndPoint = true
		}
	}
	// 3. 判断操作类型
	if n.drawingStartPoint != nil {
		// 情况 A: 起点和终点不同 -> 创建连线
		if endPt != n.drawingStartPoint {
			startIdx := n.getPtIdx(n.drawingStartPoint)
			endIdx := n.getPtIdx(endPt)
			if startIdx != -1 && endIdx != -1 {
				seg := &Segment{
					ID:       fmt.Sprintf("seg_%d", time.Now().UnixNano()),
					StartIdx: startIdx,
					EndIdx:   endIdx,
					Width:    2,
					Color:    color.NRGBA{A: 255},
				}
				n.Connections = append(n.Connections, &Connection{
					ID:       fmt.Sprintf("conn_%d", time.Now().UnixNano()),
					Segments: []*Segment{seg},
					Color:    color.NRGBA{A: 255},
				})
			}
		} else {
			// 情况 B: 起点和终点相同 (原地点击或拖拽回原点)
			// 如果这个终点是我们刚刚为了这次操作创建的（在 onPress 里创建的），
			// 并且现在没有连线连着它，它就是个孤立点，需要删除。
			// 如果刚刚在第2步里创建了终点（createdNewEndPoint），但发现位置重叠又变成了起点，
			// 这种逻辑下 createdNewEndPoint 会是 true 但 point 是同一个，不过上面的 if 逻辑规避了重复添加。
			// 核心逻辑：尝试删除 drawingStartPoint。
			// 如果它是组件引脚或已连接旧线段，removePointIfUnused 会自动跳过，不用担心误删。
			n.removePointIfUnused(n.drawingStartPoint)
			// 如果我们在第2步创建了 endPt (并且它不是 StartPoint，虽然这在 else 分支不太可能发生，除非逻辑极其特殊)，
			// 也尝试清理它
			if createdNewEndPoint && endPt != n.drawingStartPoint {
				n.removePointIfUnused(endPt)
			}
		}
	}
	n.drawingStartPoint = nil
}

// splitSegmentAtPosition 在指定位置分割线段
// 【修复逻辑】计算最近点，创建新节点，更新连接关系
func (n *NodeComponent) splitSegmentAtPosition(seg *Segment, conn *Connection, pos f32.Point, d *draw.Draw) (*Point, bool) {
	// 1. 获取线段的世界坐标端点
	_, ok1 := n.getPointWorldPos(seg.StartIdx)
	_, ok2 := n.getPointWorldPos(seg.EndIdx)
	if !ok1 || !ok2 {
		return nil, false
	}
	// 2. 计算点击位置对应的世界坐标（吸附网格）
	// 注意：这里我们使用 Draw 包提供的投影算法来找到线段上最近的点，而不是鼠标的精确点击点
	// 这样可以确保新节点正好在线上
	// 为了简单且符合电路绘制习惯，我们直接取鼠标吸附后的网格点
	// 假设用户点击位置已经在 findSegmentAt 中验证过足够靠近线段
	intersectionWorldPos := n.SnapToGrid(n.screenToWorld(pos, d))
	// 3. 创建新的交叉点
	intersectionPoint := &Point{
		ID:       fmt.Sprintf("intersect_%d", time.Now().UnixNano()),
		Position: intersectionWorldPos,
		IsMatrix: false,
	}
	// 4. 将新点添加到点列表
	n.PointList = append(n.PointList, intersectionPoint)
	intersectionIdx := len(n.PointList) - 1
	// 5. 创建两段新线段来替代旧线段
	// 第一段：原起点 -> 交叉点
	seg1 := &Segment{
		ID:       fmt.Sprintf("%s_a", seg.ID),
		StartIdx: seg.StartIdx,
		EndIdx:   intersectionIdx,
		Control:  make([]image.Point, len(seg.Control)), // 简单的复制控制点可能不准确，如果是复杂曲线需要重新计算，这里针对直线或正交线
		Selected: false,
		Color:    seg.Color,
		Width:    seg.Width,
	}
	// 第二段：交叉点 -> 原终点
	seg2 := &Segment{
		ID:       fmt.Sprintf("%s_b", seg.ID),
		StartIdx: intersectionIdx,
		EndIdx:   seg.EndIdx,
		Control:  []image.Point{},
		Selected: false,
		Color:    seg.Color,
		Width:    seg.Width,
	}
	// 6. 更新 Connection 的线段列表
	newSegments := []*Segment{}
	for _, s := range conn.Segments {
		if s != seg {
			newSegments = append(newSegments, s)
		}
	}
	newSegments = append(newSegments, seg1, seg2)
	conn.Segments = newSegments
	return intersectionPoint, true
}

// removePointIfUnused 检查点是否被使用，如果未被使用且不是组件引脚，则删除它
// 注意：由于 Segment 使用索引引用点，删除点后必须更新所有受影响的线段索引
func (n *NodeComponent) removePointIfUnused(pt *Point) {
	// 1. 保护性检查：不要删除属于组件的固定引脚
	if pt.NodeID != "" {
		return
	}
	// 2. 获取点在列表中的索引
	idx := n.getPtIdx(pt)
	if idx == -1 {
		return
	}
	// 3. 检查该点是否被任何线段使用
	isUsed := false
	for _, conn := range n.Connections {
		for _, seg := range conn.Segments {
			if seg.StartIdx == idx || seg.EndIdx == idx {
				isUsed = true
				break
			}
		}
		if isUsed {
			break
		}
	}
	// 如果被使用了，则不能删除
	if isUsed {
		return
	}
	// 4. 执行删除操作
	// 从切片中移除该点
	n.PointList = append(n.PointList[:idx], n.PointList[idx+1:]...)
	// 5. 【关键】更新所有线段的索引引用
	// 因为 idx 位置的点被删除了，所有索引大于 idx 的点，其在数组中的位置都前移了 1 位
	for _, conn := range n.Connections {
		for _, seg := range conn.Segments {
			if seg.StartIdx > idx {
				seg.StartIdx--
			}
			if seg.EndIdx > idx {
				seg.EndIdx--
			}
		}
	}
}

// --- 绘制逻辑 ---

func (n *NodeComponent) drawConnections(d *draw.Draw) {
	poly := &draw.OrthogonalPolyline{
		AllPoints: make([]f32.Point, 1),
	}
	for _, conn := range n.Connections {
		for _, seg := range conn.Segments {
			p1, ok1 := n.getPointWorldPos(seg.StartIdx)
			p2, ok2 := n.getPointWorldPos(seg.EndIdx)
			if !ok1 || !ok2 {
				continue
			}
			poly.P1, poly.P2 = p1, p2
			poly.Control = seg.Control
			c := seg.Color
			if c.A == 0 {
				c.A = 255
			} // 防止透明
			w := seg.Width
			if seg.Selected {
				c = color.NRGBA{R: 255, G: 165, B: 0, A: 255}
				w *= 1.5
			}
			poly.Add(d, w, c)
			// 绘制控制点
			if seg.Selected {
				for _, cp := range seg.Control {
					sp := d.WorldToScreenF32(cp)
					rect := image.Rect(int(sp.X)-4, int(sp.Y)-4, int(sp.X)+4, int(sp.Y)+4)
					d.DrawRect(rect, 1, color.NRGBA{R: 0, G: 120, B: 215, A: 255}, color.NRGBA{}, true)
				}
			}
		}
	}
}

func (n *NodeComponent) drawDrawingPreview(d *draw.Draw) {
	if n.drawingStartPoint == nil {
		return
	}
	p1, _ := n.getPointWorldPosByID(n.drawingStartPoint)
	// 预览线使用半透明灰色
	poly := &draw.OrthogonalPolyline{
		P1:        p1,
		P2:        n.drawingTempPos,
		AllPoints: make([]f32.Point, 1),
	}
	poly.Add(d, 2, color.NRGBA{R: 100, G: 100, B: 100, A: 150})
}

func (n *NodeComponent) drawPoints(gtx layout.Context, d *draw.Draw) {
	for _, p := range n.PointList {
		pos, _ := n.getPointWorldPosByID(p)
		sp := d.WorldToScreenF32(pos)
		radius := float32(gtx.Dp(unit.Dp(4))) * d.Scale
		// 绘制点：灰色填充
		d.DrawCircle(sp, radius, 1, color.NRGBA{R: 50, G: 50, B: 50, A: 255}, color.NRGBA{}, true)
	}
}

// --- 辅助查找与工具 ---

func (n *NodeComponent) findSegmentAt(pos f32.Point, d *draw.Draw) *Segment {
	threshold := float32(8.0)
	poly := &draw.OrthogonalPolyline{
		AllPoints: make([]f32.Point, 1),
	}
	for _, conn := range n.Connections {
		for _, seg := range conn.Segments {
			p1, ok1 := n.getPointWorldPos(seg.StartIdx)
			p2, ok2 := n.getPointWorldPos(seg.EndIdx)
			if !ok1 || !ok2 {
				continue
			}
			poly.P1, poly.P2 = p1, p2
			poly.Control = seg.Control
			if poly.DistanceToPoint(pos, d) < threshold {
				return seg
			}
		}
	}
	return nil
}

func (n *NodeComponent) findPointAt(pos f32.Point, d *draw.Draw) *Point {
	limit := float32(10.0)
	for _, p := range n.PointList {
		wpos, _ := n.getPointWorldPosByID(p)
		spos := d.WorldToScreenF32(wpos)
		if math.Hypot(float64(pos.X-spos.X), float64(pos.Y-spos.Y)) < float64(limit) {
			return p
		}
	}
	return nil
}

func (n *NodeComponent) findControlPointAt(pos f32.Point, d *draw.Draw) (*image.Point, *Segment) {
	limit := float32(8.0)
	for _, c := range n.Connections {
		for _, s := range c.Segments {
			if !s.Selected {
				continue
			}
			for i := range s.Control {
				spos := d.WorldToScreenF32(s.Control[i])
				if math.Hypot(float64(pos.X-spos.X), float64(pos.Y-spos.Y)) < float64(limit) {
					return &s.Control[i], s
				}
			}
		}
	}
	return nil, nil
}

func (n *NodeComponent) screenToWorld(sp f32.Point, d *draw.Draw) image.Point {
	return image.Pt(
		int((sp.X+d.Scroll.X)/d.Scale),
		int((sp.Y+d.Scroll.Y)/d.Scale),
	)
}

func (n *NodeComponent) getPointWorldPos(idx int) (image.Point, bool) {
	if idx < 0 || idx >= len(n.PointList) {
		return image.Point{}, false
	}
	return n.getPointWorldPosByID(n.PointList[idx])
}

func (n *NodeComponent) getPointWorldPosByID(p *Point) (image.Point, bool) {
	if p.NodeID == "" {
		return p.Position, true
	}
	child := n.Children.GetByID(p.NodeID)
	if child == nil {
		return image.Point{}, false
	}
	return child.Pos.Add(p.Position), true
}

func (n *NodeComponent) getPtIdx(p *Point) int {
	for i, pt := range n.PointList {
		if pt == p {
			return i
		}
	}
	return -1
}

func (n *NodeComponent) clearSelection() {
	for _, c := range n.Connections {
		for _, s := range c.Segments {
			s.Selected = false
		}
	}
	n.selectedSegment = nil
	n.selectedControl = nil
	n.selectedPoint = nil
}
