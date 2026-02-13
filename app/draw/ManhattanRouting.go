package draw

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
)

// OrthogonalPolyline 表示正交布线（Manhattan Routing）的折线
// 只能包含垂直或水平的线段
type OrthogonalPolyline struct {
	P1, P2    image.Point   // 两个端点（世界坐标）
	Control   []image.Point // 控制点（世界坐标）
	AllPoints []f32.Point   // 计算出的正交折线点（屏幕坐标）
}

// NewOrthogonalPolyline 创建新的正交折线
func NewOrthogonalPolyline(p1, p2 image.Point, control []image.Point) *OrthogonalPolyline {
	return &OrthogonalPolyline{
		P1:        p1,
		P2:        p2,
		Control:   control,
		AllPoints: make([]f32.Point, 1),
	}
}

// Add 绘制线
func (c *OrthogonalPolyline) Add(cd *Draw, width float32, color color.NRGBA) {
	// 计算正交折线的所有点
	points := c.CalculatePoints(cd)
	// 使用 DrawPolyline 方法绘制折线
	if len(points) >= 2 {
		cd.DrawPolyline(points, width, color)
	}
}

// CalculatePoints 计算正交折线的所有点（屏幕坐标）
// 使用与 DrawSegment 相同的正交布线逻辑
func (c *OrthogonalPolyline) CalculatePoints(cd *Draw) []f32.Point {
	s1 := cd.WorldToScreenF32(c.P1)
	s2 := cd.WorldToScreenF32(c.P2)
	if len(c.Control) == 0 {
		// 无控制点的正交布线
		return c.calculateOrthogonalPoints(s1, s2)
	}
	// 有控制点的正交折线
	allPoints := c.AllPoints[:1]
	allPoints[0] = s1
	// 添加所有控制点
	for _, cp := range c.Control {
		allPoints = append(allPoints, cd.WorldToScreenF32(cp))
	}
	// 添加终点
	allPoints = append(allPoints, s2)
	// 计算正交折线点
	result := []f32.Point{}
	prev := allPoints[0]
	result = append(result, prev)
	for i := 1; i < len(allPoints); i++ {
		points := c.calculateOrthogonalPoints(prev, allPoints[i])
		// 跳过第一个点（已经是prev）
		if len(points) > 1 {
			result = append(result, points[1:]...)
		}
		prev = allPoints[i]
	}
	c.AllPoints = result
	return result
}

// calculateOrthogonalPoints 计算两个点之间的正交折线点
func (c *OrthogonalPolyline) calculateOrthogonalPoints(p1, p2 f32.Point) []f32.Point {
	dx := math.Abs(float64(p2.X - p1.X))
	dy := math.Abs(float64(p2.Y - p1.Y))
	// 容错阈值：1像素
	const epsilon = 1.0
	if dx < epsilon {
		// X坐标基本相同，直接画垂直线
		return []f32.Point{p1, p2}
	} else if dy < epsilon {
		// Y坐标基本相同，直接画水平线
		return []f32.Point{p1, p2}
	} else {
		// 智能选择布线方向：选择较短的方向先走
		if dx < dy {
			// X方向差异较小，先水平后垂直
			return []f32.Point{p1, f32.Pt(p2.X, p1.Y), p2}
		} else {
			// Y方向差异较小，先垂直后水平
			return []f32.Point{p1, f32.Pt(p1.X, p2.Y), p2}
		}
	}
}

// DistanceToPoint 计算点到正交折线的最短距离
func (c *OrthogonalPolyline) DistanceToPoint(pos f32.Point, cd *Draw) float32 {
	points := c.CalculatePoints(cd)
	if len(points) < 2 {
		return math.MaxFloat32
	}
	minDist := float32(math.MaxFloat32)
	for i := 0; i < len(points)-1; i++ {
		dist := DistToSegment(pos, points[i], points[i+1])
		if dist < minDist {
			minDist = dist
		}
	}
	return minDist
}

// GetSegmentPoints 获取线段的所有点（包括控制点转换后的点）
func (c *OrthogonalPolyline) GetSegmentPoints(cd *Draw) []f32.Point {
	if c.AllPoints == nil {
		c.CalculatePoints(cd)
	}
	return c.AllPoints
}

// DistToSegment 计算点到线段的最短距离
// 使用向量投影算法，支持任意方向的线段
func DistToSegment(p, a, b f32.Point) float32 {
	dx, dy := b.X-a.X, b.Y-a.Y
	if dx == 0 && dy == 0 {
		return float32(math.Hypot(float64(p.X-a.X), float64(p.Y-a.Y)))
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	nx, ny := a.X+t*dx, a.Y+t*dy
	return float32(math.Hypot(float64(p.X-nx), float64(p.Y-ny)))
}
