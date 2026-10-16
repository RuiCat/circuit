// stepGuard.go：时序类元件的步长上界自检。
package digital

import (
	"fmt"
	"math"

	"circuit/mna"
)

// StepGuard 自检当前步长是否超过上界 limit：超界（或 limit <= 0）即返回错误。
// limit 由元件按自己的标称时序算，且必须满足 limit <= 标称时序/10——时序类元件
// 在使用采样/跳变时刻前必须先调用本函数，确保步长细于自己的标称时序。
//
// 错误文本含「当前步长」与「上界」两项。K1 的签名里没有元件名参数，元件名由调用方
// 用 fmt.Errorf("元件 %s: %w", name, err) 包装补上（错误字符串不复制元件配置）。
//
// 本函数不产生引擎副作用：不调用 t.NoConverged()，是否因超界而拒绝本步由调用方决定。
func StepGuard(t mna.Time, limit float64) error {
	if t == nil {
		return fmt.Errorf("digital: 步长自检失败：仿真时间接口为 nil（上界 %g，当前步长未知）", limit)
	}
	step := t.CurrentStep()
	if limit <= 0 {
		return fmt.Errorf("digital: 当前步长 %g 无法与上界 %g 比较：上界必须为正", step, limit)
	}
	if math.IsNaN(step) {
		return fmt.Errorf("digital: 当前步长 %g 不是有效数值，无法与上界 %g 比较", step, limit)
	}
	if step > limit {
		return fmt.Errorf("digital: 当前步长 %g 超过上界 %g（时序采样/跳变时刻不可靠，请减小 max-step 或限制标称时序）", step, limit)
	}
	return nil
}
