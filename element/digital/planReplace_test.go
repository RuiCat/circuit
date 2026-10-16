// planReplace_test.go：PlanReplace 的覆盖语义（惯性延时）与参数校验，含 Plan 原行为的回归对照。
package digital

import (
	"math"
	"testing"

	"circuit/mna"
)

// TestDelayQueuePlanReplaceOverwritesPending 覆盖「已有未到期排程时不报错」与
// 「覆盖后只驱动新 (at,v)、旧的 (at,v) 一并失效」。
//
// 反证：把 PlanReplace 改成直接调用 Plan，覆盖调用会返回错误而在此 FAIL；
// 把 PlanReplace 改成"已有排程就不覆盖"，15ns 处会驱动旧值 5V 而在此 FAIL。
func TestDelayQueuePlanReplaceOverwritesPending(t *testing.T) {
	node := newSlotNode(queueSlotsPerBase)
	q := &DelayQueue{}
	m := mna.NewMnaUpdate(2, 1)

	if err := q.Plan(node, 0, 10e-9, 5.0); err != nil {
		t.Fatalf("Plan(10ns, 5V) 应成功，实际: %v", err)
	}
	m.UpdateVoltageSource(0, 1.0)
	q.Apply(nil, &stubTime{now: 1e-9}, node, 0, 0) // 只推进观察时间，尚未到点

	if err := q.PlanReplace(node, 0, 20e-9, 0.0); err != nil {
		t.Fatalf("已有未到期排程时 PlanReplace 必须成功（惯性延时覆盖），实际: %v", err)
	}
	if at, _ := readFloatSlot(node, queueSlotAt); at != 20e-9 {
		t.Fatalf("覆盖后槽位时刻应为 20ns，实际 %v", at)
	}
	if v, _ := readFloatSlot(node, queueSlotValue); v != 0.0 {
		t.Fatalf("覆盖后槽位电平应为 0V，实际 %v", v)
	}

	// 旧的 at=10ns 已被覆盖：15ns 到点不得驱动电压源。
	q.Apply(m, &stubTime{now: 15e-9}, node, 0, 0)
	if got := m.GetZ().Get(testVoltRow); got != 1.0 {
		t.Fatalf("旧排程 (10ns,5V) 已被覆盖，15ns 不得驱动：电压源=%v，期望 1", got)
	}
	if armed, _ := readFloatSlot(node, queueSlotArmed); armed != 1 {
		t.Fatalf("新排程 (20ns) 尚未到点，应保持已排程，armed=%v", armed)
	}

	// 新的 (20ns, 0V) 到点驱动。
	q.Apply(m, &stubTime{now: 20e-9}, node, 0, 0)
	if got := m.GetZ().Get(testVoltRow); got != 0.0 {
		t.Fatalf("新排程到点应驱动 0V，实际 %v", got)
	}
	if armed, _ := readFloatSlot(node, queueSlotArmed); armed != 0 {
		t.Fatalf("驱动后排程应清空，armed=%v", armed)
	}
}

// TestDelayQueuePlanReplaceRejectsBadParams 覆盖 PlanReplace 的参数非法路径：
// 槽位不足、负 base、nil 节点、槽位类型不符、NaN，均须返回 error 且不改写槽位。
func TestDelayQueuePlanReplaceRejectsBadParams(t *testing.T) {
	q := &DelayQueue{}

	if err := q.PlanReplace(newSlotNode(queueSlotsPerBase-1), 0, 1e-9, 5); err == nil {
		t.Fatal("槽位不足必须报错")
	}
	if err := q.PlanReplace(newSlotNode(2*queueSlotsPerBase), -1, 1e-9, 5); err == nil {
		t.Fatal("base<0 必须报错")
	}
	if err := q.PlanReplace(nil, 0, 1e-9, 5); err == nil {
		t.Fatal("value 为 nil 必须报错")
	}
	bad := newSlotNode(queueSlotsPerBase + 1)
	bad.NodeValue[queueSlotNow] = "keep"
	if err := q.PlanReplace(bad, 0, 1e-9, 5); err == nil {
		t.Fatal("槽位类型不是 float64 必须报错")
	}
	if bad.NodeValue[queueSlotNow] != "keep" {
		t.Fatalf("报错时不得改写元件原有槽位：%v", bad.NodeValue[queueSlotNow])
	}
	if err := q.PlanReplace(newSlotNode(queueSlotsPerBase), 0, math.NaN(), 5); err == nil {
		t.Fatal("at=NaN 必须报错")
	}
	if err := q.PlanReplace(newSlotNode(queueSlotsPerBase), 0, 1e-9, math.NaN()); err == nil {
		t.Fatal("电平为 NaN 必须报错（不得把 NaN 写进电压源）")
	}
}

// TestDelayQueuePlanReplaceRejectsPastAndKeepsPlanRegression 覆盖 at<=now 报错、
// 报错后既有排程不被破坏（仍按原 (at,v) 触发），以及 Plan 的原行为回归：
// 已有未到期排程时 Plan 仍必须报错。
func TestDelayQueuePlanReplaceRejectsPastAndKeepsPlanRegression(t *testing.T) {
	node := newSlotNode(queueSlotsPerBase)
	q := &DelayQueue{}
	m := mna.NewMnaUpdate(2, 1)

	if err := q.Plan(node, 0, 30e-9, 5.0); err != nil {
		t.Fatalf("Plan(30ns, 5V) 应成功，实际: %v", err)
	}
	q.Apply(nil, &stubTime{now: 10e-9}, node, 0, 0) // 观察时间推进到 10ns，排程未到点

	if err := q.PlanReplace(node, 0, 10e-9, 0.0); err == nil {
		t.Fatal("at == 当前时间(10ns) 必须报错")
	}
	if err := q.PlanReplace(node, 0, 5e-9, 0.0); err == nil {
		t.Fatal("at < 当前时间 必须报错")
	}
	// Plan 的原行为不得被本次增补改动：已有未到期排程时仍须报错。
	if err := q.Plan(node, 0, 40e-9, 0.0); err == nil {
		t.Fatal("Plan 在已有未到期排程时必须报错而不是静默覆盖")
	}

	// 三次报错都不得破坏原排程：槽位仍是 (30ns, 5V) 且已排程。
	if at, _ := readFloatSlot(node, queueSlotAt); at != 30e-9 {
		t.Fatalf("报错后原排程时刻应保持 30ns，实际 %v", at)
	}
	if v, _ := readFloatSlot(node, queueSlotValue); v != 5.0 {
		t.Fatalf("报错后原排程电平应保持 5V，实际 %v", v)
	}
	if armed, _ := readFloatSlot(node, queueSlotArmed); armed != 1 {
		t.Fatalf("报错后原排程应保持已排程，armed=%v", armed)
	}
	m.UpdateVoltageSource(0, 1.0)
	q.Apply(m, &stubTime{now: 30e-9}, node, 0, 0)
	if got := m.GetZ().Get(testVoltRow); got != 5.0 {
		t.Fatalf("原排程到点应驱动 5V，实际 %v", got)
	}
}
