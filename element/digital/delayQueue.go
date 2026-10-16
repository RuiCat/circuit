// delayQueue.go：无触发点、无墙钟的延时翻转队列（状态全部落在 element.NodeFace 槽位上）。
package digital

import (
	"fmt"
	"math"

	"circuit/element"
	"circuit/mna"
)

// DelayQueue 是数字元件的延时翻转队列：排程 (时刻, 电平)，每步把到期项写回电压源。
//
// 队列自身不持有任何跨步状态、不注册触发点、不读墙钟。一个 base 承载一个待翻转事件，
// 占用 4 个连续 float64 槽位：
//
//	base+queueSlotArmed：0 = 空闲，1 = 已排程；
//	base+queueSlotAt   ：到点时刻（仿真时间，秒）；
//	base+queueSlotValue：到点后写入电压源的电平（V）；
//	base+queueSlotNow  ：最近一次 Apply 观察到的仿真时间（Plan 校验 at 的基准）。
//
// 元件必须把这 4 个槽位登记进 Config.ValueInit 与 Config.OrigValue（跨步状态必须可回滚），
// 并每步调用一次 Apply——Plan 用槽位里记录的最近观察时间校验，不每步 Apply 就看不到时间推进。
// 需要多个并发事件时用多个 base，相邻 base 间隔 queueSlotsPerBase 个槽位。
type DelayQueue struct{}

// 单个 base 的槽位布局（新增槽位必须追加在末尾并同步 queueSlotsPerBase）。
const (
	// queueSlotArmed 是 base+0：0 表示空闲，1 表示已排程。
	queueSlotArmed = iota
	// queueSlotAt 是 base+1：到点时刻（仿真时间，秒）。
	queueSlotAt
	// queueSlotValue 是 base+2：到点后写入电压源的电平（V）。
	queueSlotValue
	// queueSlotNow 是 base+3：最近一次 Apply 观察到的仿真时间（秒）。
	queueSlotNow
	// queueSlotsPerBase 是单个 base 占用的连续槽位数。
	queueSlotsPerBase
)

// Plan 把一个 (at, v) 翻转排入 base 处的槽位。
//
// at 必须严格大于该 base 最近一次 Apply 观察到的仿真时间（从未 Apply 时为 0）；
// 槽位不足、槽位类型不是 float64、at/v 不是有效数值、或已存在未到期排程时返回错误——
// 后者不静默覆盖旧排程（K1 没有取消通道，覆盖会丢事件）。
func (q *DelayQueue) Plan(value element.NodeFace, base int, at, v float64) error {
	if q == nil {
		return fmt.Errorf("digital: 排程失败：DelayQueue 为 nil")
	}
	if value == nil {
		return fmt.Errorf("digital: 排程失败：元件节点为 nil")
	}
	if !slotsFit(value, base, queueSlotsPerBase) {
		return fmt.Errorf("digital: 排程失败：base=%d 处需要 %d 个连续槽位", base, queueSlotsPerBase)
	}
	if !slotTypesFit(value, base, queueSlotsPerBase) {
		return fmt.Errorf("digital: 排程失败：base=%d 起的槽位存在非 float64 类型（不覆盖元件原有状态）", base)
	}
	if math.IsNaN(at) || math.IsNaN(v) {
		return fmt.Errorf("digital: 排程失败：时刻或电平不是有效数值（at=%g v=%g）", at, v)
	}
	now, _ := readFloatSlot(value, base+queueSlotNow)
	if !(at > now) {
		return fmt.Errorf("digital: 排程失败：到点时刻 %g 必须大于当前仿真时间 %g", at, now)
	}
	if armed, _ := readFloatSlot(value, base+queueSlotArmed); armed != 0 {
		pending, _ := readFloatSlot(value, base+queueSlotAt)
		return fmt.Errorf("digital: 排程失败：base=%d 已有未到期的排程（at=%g）", base, pending)
	}
	writeFloatSlot(value, base+queueSlotArmed, 1)
	writeFloatSlot(value, base+queueSlotAt, at)
	writeFloatSlot(value, base+queueSlotValue, v)
	writeFloatSlot(value, base+queueSlotNow, now)
	return nil
}

// Apply 每步调用一次：先记录当前仿真时间，再把已到期的排程写回电压源并清空排程。
// 到点判据为 now >= at（恰好到点即翻转）；翻转经 DriveNow 写入 vs。
//
// 本函数没有 error 通道，以下情况按「本步无可应用事件」静默返回：q/t/value 为 nil、
// 槽位不足、now 为 NaN、未排程、at 非 float64 或 NaN、电平槽非 float64。
// 槽位写入失败时不改写元件原有状态（见 writeFloatSlot）。
func (q *DelayQueue) Apply(m mna.Mna, t mna.Time, value element.NodeFace, base int, vs mna.VoltageID) {
	if q == nil || t == nil || value == nil {
		return
	}
	if !slotsFit(value, base, queueSlotsPerBase) {
		return
	}
	now := t.CurrentTime()
	if math.IsNaN(now) {
		return
	}
	writeFloatSlot(value, base+queueSlotNow, now)

	armed, ok := readFloatSlot(value, base+queueSlotArmed)
	if !ok || armed == 0 {
		return
	}
	at, ok := readFloatSlot(value, base+queueSlotAt)
	if !ok || math.IsNaN(at) {
		return
	}
	if now < at {
		return
	}
	v, ok := readFloatSlot(value, base+queueSlotValue)
	if !ok {
		return
	}
	DriveNow(m, vs, v)
	writeFloatSlot(value, base+queueSlotArmed, 0)
}
