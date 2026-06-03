// 矩阵均衡化LU分解实现，通过行+列缩放预处理降低病态矩阵的条件数。
// 解决混合物理域仿真中矩阵元素跨数量级(1e-9~1e6)导致的LU分解精度损失。
package maths

import (
	"errors"
)

// scaleNum 将 Number 类型的值乘以一个 float64 缩放因子，返回同类型的值。
// 使用类型断言处理所有 Number 约束子类型，避免 Go 编译器拒绝
// complex64 → float64 的直接转换。
func scaleNum[T Number](v T, s float64) T {
	switch x := any(v).(type) {
	case float32:
		return any(float32(float64(x) * s)).(T)
	case float64:
		return any(x * s).(T)
	case complex64:
		r, i := float64(real(x)), float64(imag(x))
		return any(complex64(complex(r*s, i*s))).(T)
	case complex128:
		r, i := real(x), imag(x)
		return any(complex(r*s, i*s)).(T)
	}
	return v
}

// EquilibrateAndDecompose 对矩阵进行行+列均衡化后执行LU分解。
// 均衡化: A'[i][j] = rowScale[i]·A[i][j]·colScale[j]，使每行每列最大元素接近1。
// 参数: luSolver为LU分解器, A为待分解的方阵。返回行列缩放因子供SolveEquilibrated使用。
func EquilibrateAndDecompose[T Number](luSolver LU[T], A Matrix[T]) (rowScale, colScale []float64, err error) {
	n := A.Rows()
	if n == 0 || !A.IsSquare() {
		return nil, nil, errors.New("equilibrate: matrix must be non-empty square")
	}

	rowScale = make([]float64, n)
	colScale = make([]float64, n)

	// 计算行缩放因子：rowScale[i] = 1 / max_j(|A[i][j]|)
	for i := 0; i < n; i++ {
		maxAbs := 0.0
		for j := 0; j < n; j++ {
			abs := Abs(A.Get(i, j))
			if abs > maxAbs {
				maxAbs = abs
			}
		}
		if maxAbs > Epsilon {
			rowScale[i] = 1.0 / maxAbs
		} else {
			rowScale[i] = 1.0
		}
	}

	// 计算列缩放因子：colScale[j] = 1 / max_i(|A[i][j]|)
	for j := 0; j < n; j++ {
		maxAbs := 0.0
		for i := 0; i < n; i++ {
			abs := Abs(A.Get(i, j))
			if abs > maxAbs {
				maxAbs = abs
			}
		}
		if maxAbs > Epsilon {
			colScale[j] = 1.0 / maxAbs
		} else {
			colScale[j] = 1.0
		}
	}

	// 构建缩放后的矩阵
	scaledA := NewDenseMatrix[T](n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			rowColScale := rowScale[i] * colScale[j]
			val := scaleNum(A.Get(i, j), rowColScale)
			scaledA.Set(i, j, val)
		}
	}

	// LU分解缩放后的矩阵
	err = luSolver.Decompose(scaledA)
	return rowScale, colScale, err
}

// SolveEquilibrated 使用均衡化LU分解结果求解Ax=b。
// 自动处理b向量的行缩放和x向量的列反缩放: 先解 A'(C⁻¹x) = R·b，再x[j] = C[j]·tempX[j]。
func SolveEquilibrated[T Number](luSolver LU[T], b, x Vector[T], rowScale, colScale []float64) error {
	n := b.Length()

	// 创建缩放后的b：scaledB[i] = b[i] * rowScale[i]
	scaledB := NewDenseVector[T](n)
	for i := 0; i < n; i++ {
		scaledB.Set(i, scaleNum(b.Get(i), rowScale[i]))
	}

	// 创建临时解向量
	tempX := NewDenseVector[T](n)

	// 求解缩放系统
	if err := luSolver.SolveReuse(scaledB, tempX); err != nil {
		return err
	}

	// 反缩放：x[j] = tempX[j] * colScale[j]
	for j := 0; j < n; j++ {
		x.Set(j, scaleNum(tempX.Get(j), colScale[j]))
	}

	return nil
}
