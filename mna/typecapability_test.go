// mna 包的第一张回归网：类型能力矩阵（c6）。
//
// mna 此前零测试（go list -f '{{.TestGoFiles}}' ./mna == []），
// 因此本文件同时承担三件事：
//  1. 冻结能力矩阵的全量形状（路径 × 类型 = 12 条，无重复、无缺项）；
//  2. 断言不支持的组合返回**具体错误值**（errors.Is 判定 + 文案可 grep），
//     而不是"没 panic""返回 nil 就算过"；
//  3. 断言受支持的组合真能装配并求解（避免能力表沦为纯声明）。
package mna

import (
	"errors"
	"strings"
	"testing"

	"circuit/maths"
)

// TestTypeCapabilityMatrixIsComplete 校验矩阵形状：全量 12 条、无重复、
// 不支持的条目必须给出 Reason（能力缺口要能说清依据）。
func TestTypeCapabilityMatrixIsComplete(t *testing.T) {
	matrix := TypeCapabilityMatrix()
	if len(matrix) != len(allSolvePaths)*len(allNumericKinds) {
		t.Fatalf("矩阵条目应为 %d 条，实际 %d", len(allSolvePaths)*len(allNumericKinds), len(matrix))
	}
	seen := map[string]bool{}
	supported := map[string]bool{}
	for _, cap := range matrix {
		key := string(cap.Path) + "|" + string(cap.Kind)
		if seen[key] {
			t.Fatalf("矩阵条目重复: %s", key)
		}
		seen[key] = true
		if cap.Reason == "" {
			t.Fatalf("条目 %s 缺 Reason（支持与否都必须给依据）", key)
		}
		supported[key] = cap.Supported
	}
	want := map[string]bool{
		"transient-dense|float64":     true,
		"transient-sparse|float64":    true,
		"ac-phasor|complex128":        true,
		"transient-dense|float32":     false,
		"transient-dense|complex64":   false,
		"transient-dense|complex128":  false,
		"transient-sparse|float32":    false,
		"transient-sparse|complex64":  false,
		"transient-sparse|complex128": false,
		"ac-phasor|float32":           false,
		"ac-phasor|float64":           false,
		"ac-phasor|complex64":         false,
	}
	if len(want) != len(matrix) {
		t.Fatalf("期望表 %d 条与矩阵 %d 条不一致（矩阵两轴变了要同步更新测试）", len(want), len(matrix))
	}
	for key, wantSupported := range want {
		got, ok := supported[key]
		if !ok {
			t.Fatalf("矩阵缺条目: %s", key)
		}
		if got != wantSupported {
			t.Fatalf("条目 %s 支持性应为 %v，实际 %v", key, wantSupported, got)
		}
	}
}

// TestTypeCapabilityUnsupportedReturnsConcreteError 逐类型断言：
// 不支持的路径必须返回 errors.Is 可判定的具体错误，且文案含路径与类型标识；
// 受支持的路径必须返回 nil（否则是"声明支持却报错"的反向缺口）。
func TestTypeCapabilityUnsupportedReturnsConcreteError(t *testing.T) {
	check := func(name string, path SolvePath, kind NumericKind, supported bool, err error) {
		t.Helper()
		if supported {
			if err != nil {
				t.Fatalf("%s: 受支持组合不应报错，实际 %v", name, err)
			}
			return
		}
		if err == nil {
			t.Fatalf("%s: 不支持组合必须返回具体错误值，实际 nil（静默接受）", name)
		}
		if !errors.Is(err, ErrUnsupportedNumericType) {
			t.Fatalf("%s: 错误应可 errors.Is(ErrUnsupportedNumericType)，实际 %v", name, err)
		}
		if !strings.Contains(err.Error(), string(path)) || !strings.Contains(err.Error(), string(kind)) {
			t.Fatalf("%s: 错误文案应含路径 %s 与类型 %s，实际 %q", name, path, kind, err.Error())
		}
	}

	check("float32/瞬态稠密", PathTransientDense, KindFloat32, false, CheckNumericType[float32](PathTransientDense))
	check("float32/瞬态稀疏", PathTransientSparse, KindFloat32, false, CheckNumericType[float32](PathTransientSparse))
	check("float32/相量", PathACPhasor, KindFloat32, false, CheckNumericType[float32](PathACPhasor))
	check("float64/瞬态稠密", PathTransientDense, KindFloat64, true, CheckNumericType[float64](PathTransientDense))
	check("float64/瞬态稀疏", PathTransientSparse, KindFloat64, true, CheckNumericType[float64](PathTransientSparse))
	check("float64/相量", PathACPhasor, KindFloat64, false, CheckNumericType[float64](PathACPhasor))
	check("complex64/瞬态稠密", PathTransientDense, KindComplex64, false, CheckNumericType[complex64](PathTransientDense))
	check("complex64/瞬态稀疏", PathTransientSparse, KindComplex64, false, CheckNumericType[complex64](PathTransientSparse))
	check("complex64/相量", PathACPhasor, KindComplex64, false, CheckNumericType[complex64](PathACPhasor))
	check("complex128/瞬态稠密", PathTransientDense, KindComplex128, false, CheckNumericType[complex128](PathTransientDense))
	check("complex128/瞬态稀疏", PathTransientSparse, KindComplex128, false, CheckNumericType[complex128](PathTransientSparse))
	check("complex128/相量", PathACPhasor, KindComplex128, true, CheckNumericType[complex128](PathACPhasor))
}

// TestTypeCapabilityUnknownPathIsRejected 未登记路径不得默认支持。
func TestTypeCapabilityUnknownPathIsRejected(t *testing.T) {
	const unknown SolvePath = "not-registered-path"
	cap := TypeCapabilityOf(unknown, KindFloat64)
	if cap.Supported {
		t.Fatalf("未登记路径不得默认支持: %+v", cap)
	}
	err := CheckNumericType[float64](unknown)
	if !errors.Is(err, ErrUnknownSolvePath) {
		t.Fatalf("未登记路径应报 ErrUnknownSolvePath，实际 %v", err)
	}
	if _, err := NewMnaUpdateFor[float64](unknown, 2, 1); !errors.Is(err, ErrUnknownSolvePath) {
		t.Fatalf("未登记路径的工厂调用应报 ErrUnknownSolvePath，实际 %v", err)
	}
}

// TestTypeCapabilityNumericKindOf 泛型参数 → 类型标识的映射必须与矩阵一致。
func TestTypeCapabilityNumericKindOf(t *testing.T) {
	cases := []struct {
		got  NumericKind
		want NumericKind
	}{
		{NumericKindOf[float32](), KindFloat32},
		{NumericKindOf[float64](), KindFloat64},
		{NumericKindOf[complex64](), KindComplex64},
		{NumericKindOf[complex128](), KindComplex128},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Fatalf("NumericKindOf 映射错误：期望 %s，实际 %s", c.want, c.got)
		}
	}
	// 每个类型标识都必须在矩阵两轴内，否则矩阵会出现"查不到"的盲区。
	for _, c := range cases {
		found := false
		for _, k := range allNumericKinds {
			if k == c.got {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("类型标识 %s 不在 allNumericKinds 内", c.got)
		}
	}
}

// TestTypeCapabilityNewMnaUpdateForUnsupported 工厂对不支持组合返回 (nil, 具体错误值)，
// 不返回半成品实例（半成品会让调用方在求解阶段才崩溃）。
func TestTypeCapabilityNewMnaUpdateForUnsupported(t *testing.T) {
	for _, path := range []SolvePath{PathTransientDense, PathTransientSparse, PathACPhasor} {
		solver, err := NewMnaUpdateFor[complex64](path, 2, 1)
		if err == nil {
			t.Fatalf("complex64 × %s 应报错", path)
		}
		if solver != nil {
			t.Fatalf("complex64 × %s 报错时不得返回实例", path)
		}
		if !errors.Is(err, ErrUnsupportedNumericType) {
			t.Fatalf("complex64 × %s 错误应可 errors.Is(ErrUnsupportedNumericType)，实际 %v", path, err)
		}
	}
	if solver, err := NewMnaUpdateFor[complex128](PathTransientDense, 2, 1); err == nil || solver != nil {
		t.Fatalf("complex128 × transient-dense 应报错且不返回实例，实际 err=%v solver=%v", err, solver)
	}
}

// TestTypeCapabilitySupportedPathSolves 受支持路径必须真能装配并求解：
// 1 节点、1mS 对地电导 + 1mA 注入 → V = 1V。
// 这条用例把"能力表说支持"与"内核确实能算"绑定在一起。
func TestTypeCapabilitySupportedPathSolves(t *testing.T) {
	for _, path := range []SolvePath{PathTransientDense, PathTransientSparse} {
		solver, err := NewMnaUpdateFor[float64](path, 1, 0)
		if err != nil {
			t.Fatalf("%s: 受支持路径不应报错: %v", path, err)
		}
		if solver == nil {
			t.Fatalf("%s: 受支持路径应返回实例", path)
		}
		if solver.GetNodeNum() != 1 || solver.GetVoltageSourcesNum() != 0 {
			t.Fatalf("%s: 维度错误 nodes=%d vs=%d", path, solver.GetNodeNum(), solver.GetVoltageSourcesNum())
		}
		solver.StampAdmittance(0, Gnd, 1e-3)
		// StampCurrentSource(n1, n2, i) 的方向是 n1→n2（Z[n1] -= i），
		// 因此从地注入节点 0 记为 (Gnd, 0, 1e-3) ⇒ V(0) = +1V。
		solver.StampCurrentSource(Gnd, 0, 1e-3)
		solver.Update()
		lu, err := maths.NewLU[float64](1)
		if err != nil {
			t.Fatalf("%s: 创建 LU 失败: %v", path, err)
		}
		if err := lu.Decompose(solver.GetA()); err != nil {
			t.Fatalf("%s: 分解失败: %v", path, err)
		}
		if err := lu.SolveReuse(solver.GetZ(), solver.GetX()); err != nil {
			t.Fatalf("%s: 求解失败: %v", path, err)
		}
		if got := solver.GetNodeVoltage(0); got < 1-1e-9 || got > 1+1e-9 {
			t.Fatalf("%s: 节点电压应为 1V，实际 %g", path, got)
		}
	}
}
