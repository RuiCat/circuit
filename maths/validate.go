// Package maths 提供数值线性代数计算所需的核心数据结构与算法。
// 本文件提供数值验证工具，用于检查浮点数 (float64) 和复数 (complex128)
// 是否为有限数值（非 NaN，非 ±Inf），供各模块在 MNA 加盖前进行安全性校验。
package maths

import "math"

// IsValidFloat64 检查 float64 值是否为有限数值（非 NaN，非 ±Inf）。
func IsValidFloat64(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// IsValidComplex128 检查 complex128 值是否为有限数值。
func IsValidComplex128(v complex128) bool {
	return !math.IsNaN(real(v)) && !math.IsInf(real(v), 0) &&
		!math.IsNaN(imag(v)) && !math.IsInf(imag(v), 0)
}
