// kluOptions.go 从环境变量构造 klu 稀疏 LU 选项，供 NewLUSparse 入口使用。
package maths

import (
	"os"
	"strconv"
	"strings"
)

// newSparseLUFromEnv 按环境变量构造稀疏 LU：CIRCUIT_KLU_BTF=1 启用 BTF 块三角化，
// CIRCUIT_KLU_DROP 设置 drop tolerance；默认全关（精确分解）。
func newSparseLUFromEnv[T Number](n int) (LU[T], error) {
	opt := SparseLUOptions{}
	if v := strings.TrimSpace(os.Getenv("CIRCUIT_KLU_BTF")); v == "1" || strings.EqualFold(v, "true") {
		opt.UseBTF = true
	}
	if s := strings.TrimSpace(os.Getenv("CIRCUIT_KLU_DROP")); s != "" {
		if f, err := strconv.ParseFloat(s, 64); err == nil && f >= 0 {
			opt.DropTolerance = f
		}
	}
	return NewSparseLUWithOptions[T](n, opt)
}
