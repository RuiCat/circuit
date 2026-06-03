package maths

import (
	"math"
	"testing"
)

// TestEquilibrateLU_Hilbert 测试均衡化LU分解在 Hilbert 矩阵上的精度
// Hilbert矩阵是经典的病态矩阵，条件数随维度指数增长
// n=6时的条件数约为1.5e7，直接LU分解会严重损失精度
func TestEquilibrateLU_Hilbert(t *testing.T) {
	n := 6
	A := NewDenseMatrix[float64](n, n)
	b := NewDenseVector[float64](n)
	expected := NewDenseVector[float64](n)

	// 构造 Hilbert 矩阵 H[i][j] = 1/(i+j+1)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			A.Set(i, j, 1.0/float64(i+j+1))
		}
		// 精确解 x = [1,1,1,1,1,1]
		expected.Set(i, 1.0)
	}

	// 计算 b = A * x
	for i := 0; i < n; i++ {
		sum := 0.0
		for j := 0; j < n; j++ {
			sum += A.Get(i, j) * expected.Get(j)
		}
		b.Set(i, sum)
	}

	// 使用均衡化LU求解
	x := NewDenseVector[float64](n)
	lu, err := NewLU[float64](n)
	if err != nil {
		t.Fatalf("创建LU分解器失败: %v", err)
	}

	rowScale, colScale, err := EquilibrateAndDecompose[float64](lu, A)
	if err != nil {
		t.Fatalf("均衡化LU分解失败: %v", err)
	}
	if err := SolveEquilibrated[float64](lu, b, x, rowScale, colScale); err != nil {
		t.Fatalf("均衡化求解失败: %v", err)
	}

	// 验证解的精度（每个分量误差应 < 1e-6）
	for i := 0; i < n; i++ {
		err := math.Abs(float64(x.Get(i)) - expected.Get(i))
		if err > 1e-6 {
			t.Errorf("分量 %d 误差过大: 期望 %v, 实际 %v, 误差 %v", i, expected.Get(i), x.Get(i), err)
		}
	}
}

// TestEquilibrateLU_MixedScale 测试跨数量级矩阵的均衡化效果
// 模拟混合物理域仿真的典型矩阵：元素跨 1e-9 到 1e6
func TestEquilibrateLU_MixedScale(t *testing.T) {
	n := 4
	A := NewDenseMatrix[float64](n, n)
	b := NewDenseVector[float64](n)
	expected := NewDenseVector[float64](n)

	// 构造一个跨数量级的非奇异矩阵（类似混合域MNA矩阵）
	// 行列式非零确保可解
	A.Set(0, 0, 1e-3)  // 电导域
	A.Set(0, 1, -1e-3)
	A.Set(0, 3, 1.0)   // 电压源KCL支路

	A.Set(1, 0, -1e-3)
	A.Set(1, 1, 2e-3)  // 不同于行0避免线性相关
	A.Set(1, 3, -1.0)

	A.Set(2, 2, 1e-9)  // 气路导纳(极小)
	A.Set(2, 3, 1e5)   // 传感器增益(极大)

	A.Set(3, 0, 1.0)   // 电压源约束
	A.Set(3, 1, -1.0)
	A.Set(3, 2, 1e-5)  // 跨域耦合(极小)

	// 精确解
	expected.Set(0, 5.0) // ~5V
	expected.Set(1, 0.0)
	expected.Set(2, 1e5) // ~100kPa
	expected.Set(3, -0.05)

	// b = A * x
	for i := 0; i < n; i++ {
		sum := 0.0
		for j := 0; j < n; j++ {
			sum += A.Get(i, j) * expected.Get(j)
		}
		b.Set(i, sum)
	}

	// 均衡化LU求解
	x := NewDenseVector[float64](n)
	lu, err := NewLU[float64](n)
	if err != nil {
		t.Fatalf("创建LU分解器失败: %v", err)
	}

	rowScale, colScale, err := EquilibrateAndDecompose[float64](lu, A)
	if err != nil {
		t.Fatalf("均衡化LU分解失败: %v", err)
	}
	if err := SolveEquilibrated[float64](lu, b, x, rowScale, colScale); err != nil {
		t.Fatalf("均衡化求解失败: %v", err)
	}

	// 验证解的精度（跨数量级情况下应仍有较好精度）
	for i := 0; i < n; i++ {
		expectedVal := float64(expected.Get(i))
		actualVal := float64(x.Get(i))
		relErr := math.Abs(actualVal - expectedVal)
		// 使用相对容差，对极小值放宽
		tol := 1e-6 * math.Max(math.Abs(expectedVal), 1.0)
		if relErr > tol {
			t.Errorf("分量 %d 误差过大: 期望 %.6e, 实际 %.6e, 相对误差 %.6e", i, expectedVal, actualVal, relErr)
		}
	}
}

// TestEquilibrateLU_Singular 测试均衡化对奇异矩阵的处理
func TestEquilibrateLU_Singular(t *testing.T) {
	n := 3
	A := NewDenseMatrix[float64](n, n)
	// 构造奇异矩阵（两行成比例）
	A.Set(0, 0, 1)
	A.Set(0, 1, 2)
	A.Set(0, 2, 3)
	A.Set(1, 0, 2)
	A.Set(1, 1, 4)
	A.Set(1, 2, 6)
	A.Set(2, 0, 0)
	A.Set(2, 1, 0)
	A.Set(2, 2, 0)

	lu, _ := NewLU[float64](n)
	_, _, err := EquilibrateAndDecompose[float64](lu, A)
	if err == nil {
		t.Error("奇异矩阵应该返回错误")
	}
}
