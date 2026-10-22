// multiRateConfig.go：多速率调度的 Context 侧开关（TimeMNA 上的显式配置与优先级解析）。
package time

import "circuit/element"

// multiRateState 是多速率调度的显式开关状态（零值 = 未配置 = 关闭，允许环境变量生效）。
type multiRateState struct {
	enabled  bool // 是否启用。
	explicit bool // 是否显式配置过（true 后覆盖 CIRCUIT_MULTIRATE）。
}

// EnableMultiRate 显式启用多速率调度，下次 TransientSimulation 启动时生效。
// 启用后只有实现 MultiRateSteppable 且倍率 >1 的元件按倍率调度，其余元件与默认路径
// 完全相同；显式配置优先于 CIRCUIT_MULTIRATE 环境变量。
func (t *TimeMNA) EnableMultiRate() {
	t.multiRate.enabled = true
	t.multiRate.explicit = true
}

// DisableMultiRate 显式关闭多速率调度（覆盖 CIRCUIT_MULTIRATE 环境变量）。默认即关闭。
func (t *TimeMNA) DisableMultiRate() {
	t.multiRate.enabled = false
	t.multiRate.explicit = true
}

// MultiRateEnabled 返回显式配置的开关值；未显式配置时为零值 false（环境变量是否生效见仿真入口）。
func (t *TimeMNA) MultiRateEnabled() bool { return t.multiRate.enabled }

// multiRateSwitch 供仿真入口读取显式配置：explicit=false 表示未配置，允许环境变量生效。
func (t *TimeMNA) multiRateSwitch() (enabled, explicit bool) {
	return t.multiRate.enabled, t.multiRate.explicit
}

// multiRateSwitcher 是仿真入口使用的可选配置面（同包私有接口）：*TimeMNA 实现它；
// 匿名嵌入 *TimeMNA 的装饰器会转发；只嵌入 mna.Time 的装饰器不实现，此时回落为
// 环境变量开关（不报错，也不改变默认行为）。
type multiRateSwitcher interface {
	multiRateSwitch() (enabled, explicit bool)
}

// resolveMultiRateEnabled 决定本次仿真是否启用多速率调度：显式 Context 配置优先，
// 否则读 CIRCUIT_MULTIRATE；两者都没有时返回 false（默认路径）。
func resolveMultiRateEnabled(con *element.Context) bool {
	if con == nil || con.Time == nil {
		return false
	}
	if sw, ok := con.Time.(multiRateSwitcher); ok {
		if enabled, explicit := sw.multiRateSwitch(); explicit {
			return enabled
		}
	}
	return multiRateEnabledFromEnv()
}
