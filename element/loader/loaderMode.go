// loaderMode.go：CpuMode 参数化——同一模型承载 v3（55）与 v2（54 改造网表）两种加载时序。
package loader

import (
	"strings"

	"circuit/element"
)

// CpuMode 的合法取值（K2：一个模型参数化，不复制两套元件代码）。
const (
	modeV3 = "v3" // 55_cpu8_regfile.net：8 位加载地址，LWE 单步写脉冲。
	modeV2 = "v2" // 54 改造网表：iram16x8 只有 LA[0:3]（16 字节地址空间），写脉冲需跨过未门控的 CPU 时钟。
)

// loaderV3Capacity / loaderV2Capacity 是两种时序的地址空间上限（v3 = 8 位 LA，v2 = 54 的 LA[0:3]）。
const (
	loaderV3Capacity = 256
	loaderV2Capacity = 16
)

// loaderMode 是一个 CpuMode 实际生效的时序参数。
type loaderMode struct {
	name       string // 规范化后的模式名。
	capacity   int    // 可加载的最大字节数（地址空间）。
	pulseSteps int    // 每字节 LWE 高电平保持的被接受仿真时刻数。
}

// loaderModes 是已冻结的两种时序参数。
var loaderModes = map[string]loaderMode{
	modeV3: {name: modeV3, capacity: loaderV3Capacity, pulseSteps: 1},
	// v2：54 的 iram16x8 是 SR 电平写、PC 用裸 CLK（C3 证据 54:897-898,927），
	// 故写脉冲保持 2 个被接受步，保证至少覆盖一个完整 CPU 时钟周期的最小窗口。
	modeV2: {name: modeV2, capacity: loaderV2Capacity, pulseSteps: 2},
}

// loaderModeOf 读槽 2 的 CpuMode 参数；缺省或未识别时回退 v3（默认值）。
func loaderModeOf(value element.NodeFace) loaderMode {
	name := strings.ToLower(strings.TrimSpace(value.GetString(slotCpuMode)))
	if mode, ok := loaderModes[name]; ok {
		return mode
	}
	return loaderModes[modeV3]
}
