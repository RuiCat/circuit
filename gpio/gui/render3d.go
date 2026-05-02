// Package gui 提供3D绘图功能
package gui

import (
	"math"
)

// Triangle3D 表示三维三角形
type Triangle3D struct {
	V0, V1, V2 Vec3
	Color      Color
}

// Mesh3D 表示三维网格
type Mesh3D struct {
	Vertices []Vec3
	Faces    []Triangle3D
}

// Renderer3D 3D渲染器
type Renderer3D struct {
	paint     *Paint
	modelMat  Mat4 // 模型变换矩阵
	viewMat   Mat4 // 视图变换矩阵
	projMat   Mat4 // 投影矩阵
	screenW   int  // 屏幕宽度
	screenH   int  // 屏幕高度
	cameraPos Vec3 // 相机位置
	cameraDir Vec3 // 相机方向
}

// NewRenderer3D 创建新的3D渲染器
func NewRenderer3D(paint *Paint) *Renderer3D {
	return &Renderer3D{
		paint:     paint,
		modelMat:  NewIdentityMat4(),
		viewMat:   NewIdentityMat4(),
		projMat:   NewIdentityMat4(),
		screenW:   paint.Width,
		screenH:   paint.Height,
		cameraPos: NewVec3(0, 0, -5),
		cameraDir: NewVec3(0, 0, 1),
	}
}

// SetModelMatrix 设置模型变换矩阵
func (r *Renderer3D) SetModelMatrix(m Mat4) {
	r.modelMat = m
}

// ResetModelMatrix 重置模型变换矩阵为单位矩阵
func (r *Renderer3D) ResetModelMatrix() {
	r.modelMat = NewIdentityMat4()
}

// TranslateModel 平移模型
func (r *Renderer3D) TranslateModel(tx, ty, tz float64) {
	translation := NewTranslationMat4(tx, ty, tz)
	r.modelMat = r.modelMat.Mul(translation)
}

// ScaleModel 缩放模型
func (r *Renderer3D) ScaleModel(sx, sy, sz float64) {
	scale := NewScaleMat4(sx, sy, sz)
	r.modelMat = r.modelMat.Mul(scale)
}

// RotateModelX 绕X轴旋转模型（角度为弧度）
func (r *Renderer3D) RotateModelX(angle float64) {
	rotation := NewRotationXMat4(angle)
	r.modelMat = r.modelMat.Mul(rotation)
}

// RotateModelY 绕Y轴旋转模型（角度为弧度）
func (r *Renderer3D) RotateModelY(angle float64) {
	rotation := NewRotationYMat4(angle)
	r.modelMat = r.modelMat.Mul(rotation)
}

// RotateModelZ 绕Z轴旋转模型（角度为弧度）
func (r *Renderer3D) RotateModelZ(angle float64) {
	rotation := NewRotationZMat4(angle)
	r.modelMat = r.modelMat.Mul(rotation)
}

// SetViewMatrix 设置视图变换矩阵
func (r *Renderer3D) SetViewMatrix(m Mat4) {
	r.viewMat = m
}

// SetProjectionMatrix 设置投影矩阵
func (r *Renderer3D) SetProjectionMatrix(m Mat4) {
	r.projMat = m
}

// SetCamera 设置相机位置和方向
func (r *Renderer3D) SetCamera(pos, target Vec3) {
	r.cameraPos = pos
	// 计算相机方向（看向目标）
	forward := target.Sub(pos).Normalize()
	// 选择与 forward 正交的上向量，避免万向节锁
	up := NewVec3(0, 1, 0)
	if math.Abs(forward.Dot(up)) > 0.99 {
		// forward 接近 Y 轴，改用 Z 轴作为参考上方向；
		// 若 forward 也接近 Z 轴，则退化为 X 轴
		if math.Abs(forward.Z) > 0.99 {
			up = NewVec3(1, 0, 0)
		} else {
			up = NewVec3(0, 0, 1)
		}
	}
	right := forward.Cross(up).Normalize()
	realUp := right.Cross(forward).Normalize()
	// 构建视图矩阵（LookAt矩阵）
	r.viewMat = Mat4{
		right.X, realUp.X, -forward.X, 0,
		right.Y, realUp.Y, -forward.Y, 0,
		right.Z, realUp.Z, -forward.Z, 0,
		-right.Dot(pos), -realUp.Dot(pos), forward.Dot(pos), 1,
	}
}

// SetPerspectiveProjection 设置透视投影
func (r *Renderer3D) SetPerspectiveProjection(fov, aspect, near, far float64) {
	r.projMat = NewPerspectiveMat4(fov, aspect, near, far)
}

// SetOrthographicProjection 设置正交投影
func (r *Renderer3D) SetOrthographicProjection(left, right, bottom, top, near, far float64) {
	r.projMat = NewOrthographicMat4(left, right, bottom, top, near, far)
}

// project 将三维坐标投影到二维屏幕坐标
func (r *Renderer3D) project(v Vec3) (int, int) {
	// 应用模型-视图-投影变换
	worldPos := r.modelMat.TransformVec3(v)
	viewPos := r.viewMat.TransformVec3(worldPos)
	projPos := r.projMat.TransformVec3(viewPos)
	// 归一化设备坐标到屏幕坐标
	screenX := (projPos.X + 1) * float64(r.screenW) / 2
	screenY := (1 - projPos.Y) * float64(r.screenH) / 2 // Y轴翻转
	return int(screenX + 0.5), int(screenY + 0.5)
}

// DrawTriangle3D 绘制三维三角形
func (r *Renderer3D) DrawTriangle3D(t Triangle3D) {
	// 投影三个顶点到屏幕空间
	x0, y0 := r.project(t.V0)
	x1, y1 := r.project(t.V1)
	x2, y2 := r.project(t.V2)
	// 使用Paint的三角形绘制功能
	r.paint.DrawTriangle(x0, y0, x1, y1, x2, y2, t.Color, DotPixel1x1, DrawFillFull)
}

// DrawWireframeTriangle3D 绘制三维三角形线框
func (r *Renderer3D) DrawWireframeTriangle3D(t Triangle3D, lineWidth DotPixel) {
	x0, y0 := r.project(t.V0)
	x1, y1 := r.project(t.V1)
	x2, y2 := r.project(t.V2)
	r.paint.DrawTriangle(x0, y0, x1, y1, x2, y2, t.Color, lineWidth, DrawFillEmpty)
}

// DrawMesh3D 绘制三维网格
func (r *Renderer3D) DrawMesh3D(mesh Mesh3D, wireframe bool, lineWidth DotPixel) {
	for _, face := range mesh.Faces {
		if wireframe {
			r.DrawWireframeTriangle3D(face, lineWidth)
		} else {
			r.DrawTriangle3D(face)
		}
	}
}

// DrawCube 绘制立方体
func (r *Renderer3D) DrawCube(center Vec3, size float64, color Color, wireframe bool, lineWidth DotPixel) {
	half := size / 2
	vertices := []Vec3{
		// 前面
		{center.X - half, center.Y - half, center.Z + half},
		{center.X + half, center.Y - half, center.Z + half},
		{center.X + half, center.Y + half, center.Z + half},
		{center.X - half, center.Y + half, center.Z + half},
		// 后面
		{center.X - half, center.Y - half, center.Z - half},
		{center.X + half, center.Y - half, center.Z - half},
		{center.X + half, center.Y + half, center.Z - half},
		{center.X - half, center.Y + half, center.Z - half},
	}
	// 定义立方体的面（三角形）
	faces := []Triangle3D{
		// 前面
		{V0: vertices[0], V1: vertices[1], V2: vertices[2], Color: color},
		{V0: vertices[0], V1: vertices[2], V2: vertices[3], Color: color},
		// 后面
		{V0: vertices[4], V1: vertices[6], V2: vertices[5], Color: color},
		{V0: vertices[4], V1: vertices[7], V2: vertices[6], Color: color},
		// 上面
		{V0: vertices[3], V1: vertices[2], V2: vertices[6], Color: color},
		{V0: vertices[3], V1: vertices[6], V2: vertices[7], Color: color},
		// 下面
		{V0: vertices[0], V1: vertices[5], V2: vertices[1], Color: color},
		{V0: vertices[0], V1: vertices[4], V2: vertices[5], Color: color},
		// 左面
		{V0: vertices[0], V1: vertices[3], V2: vertices[7], Color: color},
		{V0: vertices[0], V1: vertices[7], V2: vertices[4], Color: color},
		// 右面
		{V0: vertices[1], V1: vertices[5], V2: vertices[6], Color: color},
		{V0: vertices[1], V1: vertices[6], V2: vertices[2], Color: color},
	}
	// 绘制所有面
	for _, face := range faces {
		if wireframe {
			r.DrawWireframeTriangle3D(face, lineWidth)
		} else {
			r.DrawTriangle3D(face)
		}
	}
}

// DrawSphere 绘制球体（近似）
func (r *Renderer3D) DrawSphere(center Vec3, radius float64, color Color, segments int, wireframe bool, lineWidth DotPixel) {
	if segments < 4 {
		segments = 4
	}
	// 生成球体顶点
	var vertices [][]Vec3
	for i := 0; i <= segments; i++ {
		var row []Vec3
		lat := math.Pi * float64(i) / float64(segments) // 纬度，0到π
		for j := 0; j <= segments; j++ {
			lon := 2 * math.Pi * float64(j) / float64(segments) // 经度，0到2π
			x := radius * math.Sin(lat) * math.Cos(lon)
			y := radius * math.Cos(lat)
			z := radius * math.Sin(lat) * math.Sin(lon)
			row = append(row, NewVec3(center.X+x, center.Y+y, center.Z+z))
		}
		vertices = append(vertices, row)
	}
	// 创建三角形面片
	for i := 0; i < segments; i++ {
		for j := 0; j < segments; j++ {
			v00 := vertices[i][j]
			v01 := vertices[i][j+1]
			v10 := vertices[i+1][j]
			v11 := vertices[i+1][j+1]
			// 两个三角形构成一个四边形
			tri1 := Triangle3D{V0: v00, V1: v10, V2: v01, Color: color}
			tri2 := Triangle3D{V0: v01, V1: v10, V2: v11, Color: color}
			if wireframe {
				r.DrawWireframeTriangle3D(tri1, lineWidth)
				r.DrawWireframeTriangle3D(tri2, lineWidth)
			} else {
				r.DrawTriangle3D(tri1)
				r.DrawTriangle3D(tri2)
			}
		}
	}
}

// DrawCylinder 绘制圆柱体（近似）
func (r *Renderer3D) DrawCylinder(center Vec3, radius, height float64, color Color, segments int, wireframe bool, lineWidth DotPixel) {
	if segments < 3 {
		segments = 3
	}
	// 生成顶部和底部圆的顶点
	var topVerts, bottomVerts []Vec3
	for i := 0; i < segments; i++ {
		angle := 2 * math.Pi * float64(i) / float64(segments)
		x := radius * math.Cos(angle)
		z := radius * math.Sin(angle)
		topVerts = append(topVerts, NewVec3(center.X+x, center.Y+height/2, center.Z+z))
		bottomVerts = append(bottomVerts, NewVec3(center.X+x, center.Y-height/2, center.Z+z))
	}
	// 绘制顶部和底部圆
	for i := 0; i < segments; i++ {
		next := (i + 1) % segments
		// 顶部三角形
		topCenter := NewVec3(center.X, center.Y+height/2, center.Z)
		topTri := Triangle3D{V0: topCenter, V1: topVerts[i], V2: topVerts[next], Color: color}
		// 底部三角形（注意顶点顺序以确保法线正确）
		bottomCenter := NewVec3(center.X, center.Y-height/2, center.Z)
		bottomTri := Triangle3D{V0: bottomCenter, V1: bottomVerts[next], V2: bottomVerts[i], Color: color}
		if wireframe {
			r.DrawWireframeTriangle3D(topTri, lineWidth)
			r.DrawWireframeTriangle3D(bottomTri, lineWidth)
		} else {
			r.DrawTriangle3D(topTri)
			r.DrawTriangle3D(bottomTri)
		}
		// 绘制侧面四边形（两个三角形）
		sideTri1 := Triangle3D{V0: topVerts[i], V1: bottomVerts[i], V2: topVerts[next], Color: color}
		sideTri2 := Triangle3D{V0: bottomVerts[i], V1: bottomVerts[next], V2: topVerts[next], Color: color}
		if wireframe {
			r.DrawWireframeTriangle3D(sideTri1, lineWidth)
			r.DrawWireframeTriangle3D(sideTri2, lineWidth)
		} else {
			r.DrawTriangle3D(sideTri1)
			r.DrawTriangle3D(sideTri2)
		}
	}
}

// Clear 清空屏幕
func (r *Renderer3D) Clear(color Color) {
	r.paint.Clear(color)
}
