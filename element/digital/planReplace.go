// planReplace.go：DelayQueue 的覆盖式（惯性延时）排程——已有未到期排程时直接替换为新 (at, v)。
package digital

import (
	"fmt"
	"math"

	"circuit/element"
)

// PlanReplace 把一个 (at, v) 翻转覆盖式排入 base 处的槽位（惯性延时语义）。
//
// 与 Plan 的唯一语义差别在「base 处已有未到期排程」这一情形：Plan 返回错误
// （传输延时：一个跳变都不许丢），PlanReplace 直接以新的 (at, v) 覆盖旧排程
// （惯性延时：短于延时的脉冲被吞掉）。槽位布局与 Plan 相同，仍是
// base+[armed, at, value, now] 四个连续 float64 槽位。
//
// 返回 error 只表示参数非法，与是否已有排程无关：q/value 为 nil、槽位不足、
// 槽位类型不是 float64、at/v 不是有效数值、at 不晚于该 base 最近一次 Apply 观察到的
// 仿真时间（从未 Apply 时为 0）。参数非法时不改写任何槽位，既有排程原样保留。
func (q *DelayQueue) PlanReplace(value element.NodeFace, base int, at, v float64) error {
	if q == nil {
		return fmt.Errorf("digital: 覆盖排程失败：DelayQueue 为 nil")
	}
	if value == nil {
		return fmt.Errorf("digital: 覆盖排程失败：元件节点为 nil")
	}
	if !slotsFit(value, base, queueSlotsPerBase) {
		return fmt.Errorf("digital: 覆盖排程失败：base=%d 处需要 %d 个连续槽位", base, queueSlotsPerBase)
	}
	if !slotTypesFit(value, base, queueSlotsPerBase) {
		return fmt.Errorf("digital: 覆盖排程失败：base=%d 起的槽位存在非 float64 类型（不覆盖元件原有状态）", base)
	}
	if math.IsNaN(at) || math.IsNaN(v) {
		return fmt.Errorf("digital: 覆盖排程失败：时刻或电平不是有效数值（at=%g v=%g）", at, v)
	}
	now, _ := readFloatSlot(value, base+queueSlotNow)
	if !(at > now) {
		return fmt.Errorf("digital: 覆盖排程失败：到点时刻 %g 必须大于当前仿真时间 %g", at, now)
	}
	writeFloatSlot(value, base+queueSlotArmed, 1)
	writeFloatSlot(value, base+queueSlotAt, at)
	writeFloatSlot(value, base+queueSlotValue, v)
	writeFloatSlot(value, base+queueSlotNow, now)
	return nil
}
