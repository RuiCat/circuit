// multiRateEnv.go：多速率调度开关的环境变量解析（过渡实现，受控例外）。
//
// 架构约定（engineConfig.go:2-18）：引擎配置应由 load 包解析进 element.EngineConfig，
// element/time 不直接读环境变量。本文件是**本轮写域的受控例外**：T6 的写域只有
// element/time，无法改 load/element 两个包。待 Lead 裁决后，应把 CIRCUIT_MULTIRATE
// 迁入 element.EngineConfig，并删除本文件（见交付报告的"待决/技术债"）。
package time

import (
	"os"
	"strings"
)

// multiRateEnvName 是多速率调度开关的环境变量名。
const multiRateEnvName = "CIRCUIT_MULTIRATE"

// multiRateEnabledFromEnv 读取 CIRCUIT_MULTIRATE 开关。
// 非空且不是 0/false/off/no 时启用；未设置或解析为关闭时返回 false（默认关闭）。
// 解析失败按项目既有约定回退默认值（关闭），不中断仿真。
func multiRateEnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(multiRateEnvName))) {
	case "", "0", "false", "off", "no":
		return false
	default:
		return true
	}
}
