package gui

import "math"

// Vec3 表示三维向量
type Vec3 struct {
	X, Y, Z float64
}

// NewVec3 创建新的三维向量
func NewVec3(x, y, z float64) Vec3 {
	return Vec3{X: x, Y: y, Z: z}
}

// Add 向量加法
func (v Vec3) Add(u Vec3) Vec3 {
	return Vec3{X: v.X + u.X, Y: v.Y + u.Y, Z: v.Z + u.Z}
}

// Sub 向量减法
func (v Vec3) Sub(u Vec3) Vec3 {
	return Vec3{X: v.X - u.X, Y: v.Y - u.Y, Z: v.Z - u.Z}
}

// Mul 向量标量乘法
func (v Vec3) Mul(s float64) Vec3 {
	return Vec3{X: v.X * s, Y: v.Y * s, Z: v.Z * s}
}

// Dot 向量点积
func (v Vec3) Dot(u Vec3) float64 {
	return v.X*u.X + v.Y*u.Y + v.Z*u.Z
}

// Cross 向量叉积
func (v Vec3) Cross(u Vec3) Vec3 {
	return Vec3{
		X: v.Y*u.Z - v.Z*u.Y,
		Y: v.Z*u.X - v.X*u.Z,
		Z: v.X*u.Y - v.Y*u.X,
	}
}

// Length 向量长度
func (v Vec3) Length() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

// Normalize 向量归一化
func (v Vec3) Normalize() Vec3 {
	length := v.Length()
	if length == 0 {
		return Vec3{}
	}
	return Vec3{X: v.X / length, Y: v.Y / length, Z: v.Z / length}
}

// Mat4 表示4x4矩阵，用于3D变换
type Mat4 [16]float64

// Mul 矩阵乘法
func (m Mat4) Mul(n Mat4) Mat4 {
	var result Mat4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			sum := 0.0
			for k := 0; k < 4; k++ {
				sum += m[i*4+k] * n[k*4+j]
			}
			result[i*4+j] = sum
		}
	}
	return result
}

// TransformVec3 用矩阵变换三维向量（齐次坐标）
func (m Mat4) TransformVec3(v Vec3) Vec3 {
	x := m[0]*v.X + m[1]*v.Y + m[2]*v.Z + m[3]
	y := m[4]*v.X + m[5]*v.Y + m[6]*v.Z + m[7]
	z := m[8]*v.X + m[9]*v.Y + m[10]*v.Z + m[11]
	w := m[12]*v.X + m[13]*v.Y + m[14]*v.Z + m[15]
	if w != 0 && w != 1 {
		return Vec3{X: x / w, Y: y / w, Z: z / w}
	}
	return Vec3{X: x, Y: y, Z: z}
}

// NewIdentityMat4 创建单位矩阵
func NewIdentityMat4() Mat4 {
	return Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// NewTranslationMat4 创建平移矩阵
func NewTranslationMat4(tx, ty, tz float64) Mat4 {
	return Mat4{
		1, 0, 0, tx,
		0, 1, 0, ty,
		0, 0, 1, tz,
		0, 0, 0, 1,
	}
}

// NewScaleMat4 创建缩放矩阵
func NewScaleMat4(sx, sy, sz float64) Mat4 {
	return Mat4{
		sx, 0, 0, 0,
		0, sy, 0, 0,
		0, 0, sz, 0,
		0, 0, 0, 1,
	}
}

// NewRotationXMat4 创建绕X轴旋转矩阵（角度为弧度）
func NewRotationXMat4(angle float64) Mat4 {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return Mat4{
		1, 0, 0, 0,
		0, cos, -sin, 0,
		0, sin, cos, 0,
		0, 0, 0, 1,
	}
}

// NewRotationYMat4 创建绕Y轴旋转矩阵（角度为弧度）
func NewRotationYMat4(angle float64) Mat4 {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return Mat4{
		cos, 0, sin, 0,
		0, 1, 0, 0,
		-sin, 0, cos, 0,
		0, 0, 0, 1,
	}
}

// NewRotationZMat4 创建绕Z轴旋转矩阵（角度为弧度）
func NewRotationZMat4(angle float64) Mat4 {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return Mat4{
		cos, -sin, 0, 0,
		sin, cos, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// NewPerspectiveMat4 创建透视投影矩阵
// fov: 视野角度（弧度）
// aspect: 宽高比（width/height）
// near: 近平面距离
// far: 远平面距离
func NewPerspectiveMat4(fov, aspect, near, far float64) Mat4 {
	f := 1.0 / math.Tan(fov/2.0)
	return Mat4{
		f / aspect, 0, 0, 0,
		0, f, 0, 0,
		0, 0, (far + near) / (near - far), (2 * far * near) / (near - far),
		0, 0, -1, 0,
	}
}

// NewOrthographicMat4 创建正交投影矩阵
// left, right, bottom, top, near, far: 投影体积的边界
func NewOrthographicMat4(left, right, bottom, top, near, far float64) Mat4 {
	return Mat4{
		2 / (right - left), 0, 0, -(right + left) / (right - left),
		0, 2 / (top - bottom), 0, -(top + bottom) / (top - bottom),
		0, 0, -2 / (far - near), -(far + near) / (far - near),
		0, 0, 0, 1,
	}
}
