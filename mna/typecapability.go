// 类型能力矩阵（c6）：把「数值类型 × 求解路径 → 支持 / 不支持」从隐式约定变成显式契约。
//
// 现状（父体实测）：maths.Number（maths/face.go:34）声明
// ~float32 | ~float64 | ~complex64 | ~complex128，但引擎里只有
// NewMnaUpdateT[float64]（mna/mna.go 经 NewMnaUpdate / NewMnaUpdateSparse）与
// NewMnaT[complex128]（mna/complex.go 的 NewMnaComplex，唯一消费者 analysis/ac.go）。
// float32 / complex64 在引擎路径零使用：泛型约束允许它们实例化编译通过，
// 但既没有实现覆盖、也没有任何报错路径——缺失只会在运行时以错误结果或奇异矩阵暴露。
//
// 本文件是这些「声明上支持、实际不支持」组合的唯一权威表：
// TypeCapabilityMatrix 给出全量条目，CheckNumericType / NewMnaUpdateFor 对不支持的组合
// 返回**具体错误值**（errors.Is 可判定），既不静默降级也不 panic。
// 新增求解路径或新增数值类型时必须先登记到 allSolvePaths / allNumericKinds 并给出 Reason，
// 否则查表返回「未知路径 / 未知类型」且 CheckNumericType 报错（不允许默认支持）。
package mna

import (
	"errors"
	"fmt"

	"circuit/maths"
)

// NumericKind 标识一个数值类型（maths.Number 的一个实例化）。
type NumericKind string

// maths.Number 约束允许的四种数值类型，对应关系见 NumericKindOf。
const (
	KindFloat32    NumericKind = "float32"
	KindFloat64    NumericKind = "float64"
	KindComplex64  NumericKind = "complex64"
	KindComplex128 NumericKind = "complex128"
)

// SolvePath 标识引擎中的一条求解路径（矩阵装配方式 + 线性求解器的组合）。
type SolvePath string

// 引擎里真实存在的求解路径。每条路径都必须能指出它的唯一消费者，
// 否则它只是"声明路径"而非"实现路径"。
const (
	// PathTransientDense 瞬态引擎稠密 LU 路径（element/time 主循环，float64）。
	PathTransientDense SolvePath = "transient-dense"
	// PathTransientSparse 瞬态引擎稀疏 LU 路径（element/time，CSR 存储 + 稀疏分解，float64）。
	PathTransientSparse SolvePath = "transient-sparse"
	// PathACPhasor 交流相量分析路径（analysis/ac.go，复数稳态求解）。
	PathACPhasor SolvePath = "ac-phasor"
)

// 能力缺口的具体错误值。调用方用 errors.Is 判定，
// 不要依赖错误文案（文案只用于人读与 grep）。
var (
	// ErrUnsupportedNumericType 数值类型在该求解路径上不受支持。
	ErrUnsupportedNumericType = errors.New("mna: 数值类型在该求解路径上不受支持")
	// ErrUnknownSolvePath 求解路径未登记（禁止默认支持）。
	ErrUnknownSolvePath = errors.New("mna: 未知求解路径")
	// ErrUnknownNumericKind 数值类型未登记（maths.Number 约束之外的实例化）。
	ErrUnknownNumericKind = errors.New("mna: 未知数值类型")
)

// TypeCapability 描述「一条求解路径 × 一个数值类型」的支持情况与依据。
type TypeCapability struct {
	Path      SolvePath   // 求解路径
	Kind      NumericKind // 数值类型
	Supported bool        // 是否支持
	Reason    string      // 支持/不支持的依据（不支持时必填，说明缺口而非"未实现"）
}

// allNumericKinds 能力矩阵的数值类型轴（顺序稳定，供 TypeCapabilityMatrix 与测试遍历）。
var allNumericKinds = []NumericKind{KindFloat32, KindFloat64, KindComplex64, KindComplex128}

// allSolvePaths 能力矩阵的求解路径轴（顺序稳定）。
var allSolvePaths = []SolvePath{PathTransientDense, PathTransientSparse, PathACPhasor}

// NumericKindOf 返回泛型参数 T 对应的数值类型标识。
// maths.Number 是封闭约束（四种类型），实际不会走到未知分支；
// 未知时返回空串，由 CheckNumericType 报 ErrUnknownNumericKind。
func NumericKindOf[T maths.Number]() NumericKind {
	var zero T
	switch any(zero).(type) {
	case float32:
		return KindFloat32
	case float64:
		return KindFloat64
	case complex64:
		return KindComplex64
	case complex128:
		return KindComplex128
	default:
		return ""
	}
}

// TypeCapabilityOf 查询「路径 × 类型」的支持情况。
// 未登记的路径或类型一律返回 Supported=false（不允许默认支持）。
func TypeCapabilityOf(path SolvePath, kind NumericKind) TypeCapability {
	cap := TypeCapability{Path: path, Kind: kind}
	switch path {
	case PathTransientDense, PathTransientSparse:
		switch kind {
		case KindFloat64:
			cap.Supported = true
			cap.Reason = "element/time 瞬态引擎唯一实例化类型（NewMnaUpdate / NewMnaUpdateSparse，float64）"
		case KindFloat32:
			cap.Reason = "float32 引擎路径零使用：无 LU 分解实现、无回归覆盖"
		case KindComplex64:
			cap.Reason = "complex64 引擎路径零使用：瞬态引擎只做实数时域求解"
		case KindComplex128:
			cap.Reason = "complex128 只走相量路径（ac-phasor）；瞬态引擎不含复数装配"
		default:
			cap.Reason = "未知数值类型：不在 maths.Number 约束的显式清单内"
		}
	case PathACPhasor:
		switch kind {
		case KindComplex128:
			cap.Supported = true
			cap.Reason = "analysis/ac.go 唯一实例化类型（NewMnaComplex，complex128）"
		case KindFloat64:
			cap.Reason = "实数类型无法承载相量（幅值+相位）；float64 走瞬态路径"
		case KindFloat32:
			cap.Reason = "实数类型无法承载相量（幅值+相位）；float32 引擎路径零使用"
		case KindComplex64:
			cap.Reason = "complex64 零使用：相量分析按 complex128 归一化，避免精度分叉"
		default:
			cap.Reason = "未知数值类型：不在 maths.Number 约束的显式清单内"
		}
	default:
		cap.Reason = "未知求解路径：先登记路径再声明能力（不允许默认支持）"
	}
	return cap
}

// TypeCapabilityMatrix 返回全量能力矩阵（路径轴 × 类型轴，顺序稳定）。
// 文档 docs/library-conventions.md 的类型能力矩阵与本函数一一对应。
func TypeCapabilityMatrix() []TypeCapability {
	out := make([]TypeCapability, 0, len(allSolvePaths)*len(allNumericKinds))
	for _, path := range allSolvePaths {
		for _, kind := range allNumericKinds {
			out = append(out, TypeCapabilityOf(path, kind))
		}
	}
	return out
}

// CheckNumericType 校验泛型参数 T 是否能在 path 上求解。
// 不支持时返回**具体错误值**（errors.Is(err, ErrUnsupportedNumericType) 可判定），
// 错误文案里同时带路径与类型标识，便于 grep 定位。
func CheckNumericType[T maths.Number](path SolvePath) error {
	kind := NumericKindOf[T]()
	if kind == "" {
		var zero T
		return fmt.Errorf("%w: 类型=%T 路径=%s", ErrUnknownNumericKind, zero, path)
	}
	cap := TypeCapabilityOf(path, kind)
	if cap.Supported {
		return nil
	}
	if cap.Reason == "" || !isRegisteredSolvePath(path) {
		return fmt.Errorf("%w: 路径=%s 类型=%s", ErrUnknownSolvePath, path, kind)
	}
	return fmt.Errorf("%w: 路径=%s 类型=%s（%s）", ErrUnsupportedNumericType, path, kind, cap.Reason)
}

// isRegisteredSolvePath 判断路径是否已在 allSolvePaths 登记。
func isRegisteredSolvePath(path SolvePath) bool {
	for _, p := range allSolvePaths {
		if p == path {
			return true
		}
	}
	return false
}

// NewMnaUpdateFor 按求解路径创建带更新/回滚能力的 MNA 求解器：
// 不支持的「路径 × 类型」组合返回具体错误值（errors.Is 可判定），不返回半成品实例。
//
// 与既有工厂的关系：PathTransientDense / PathACPhasor 用稠密存储（NewMnaUpdateT），
// PathTransientSparse 用 CSR 稀疏存储（NewMnaUpdateSparseT）。
// 引擎侧（element/time、analysis/ac.go）沿用各自既有工厂；本函数是新增路径的统一入口，
// 供新求解后端接入时先过能力校验。
func NewMnaUpdateFor[T maths.Number](path SolvePath, nodesNum, vsNum int) (*MnaUpdateType[T], error) {
	if err := CheckNumericType[T](path); err != nil {
		return nil, err
	}
	switch path {
	case PathTransientSparse:
		return NewMnaUpdateSparseT[T](nodesNum, vsNum), nil
	case PathTransientDense, PathACPhasor:
		return NewMnaUpdateT[T](nodesNum, vsNum), nil
	default:
		return nil, fmt.Errorf("%w: 路径=%s", ErrUnknownSolvePath, path)
	}
}
