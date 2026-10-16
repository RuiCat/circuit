// delayQueue_test.go：DelayQueue 的排程校验、到期翻转与取走清空。
package digital

import (
	"math"
	"testing"

	"circuit/mna"
)

// testVoltRow 是 mna.NewMnaUpdate(2, 1) 中电压源 0 在 Z 里的行号（节点数 2 + 电压源序号 0）。
const testVoltRow = 2

// TestDelayQueueFiresWhenDue 覆盖未到期不翻转、恰好到点翻转、翻转后不二次驱动。
func TestDelayQueueFiresWhenDue(t *testing.T) {
	node := newSlotNode(queueSlotsPerBase)
	q := &DelayQueue{}
	m := mna.NewMnaUpdate(2, 1)

	m.UpdateVoltageSource(0, 1.0)
	if err := q.Plan(node, 0, 10e-9, 5.0); err != nil {
		t.Fatalf("Plan(10ns, 5V) 应成功，实际: %v", err)
	}

	q.Apply(m, &stubTime{now: 5e-9}, node, 0, 0) // 未到期
	if got := m.GetZ().Get(testVoltRow); got != 1.0 {
		t.Fatalf("未到期不得翻转：电压源=%v，期望 1", got)
	}

	q.Apply(m, &stubTime{now: 10e-9}, node, 0, 0) // 恰好到点 ⇒ 到期
	if got := m.GetZ().Get(testVoltRow); got != 5.0 {
		t.Fatalf("到点应翻转：电压源=%v，期望 5", got)
	}

	// 取走后不得二次驱动：先把电压源改成 1，再 Apply 一次，值必须保持 1。
	m.UpdateVoltageSource(0, 1.0)
	q.Apply(m, &stubTime{now: 20e-9}, node, 0, 0)
	if got := m.GetZ().Get(testVoltRow); got != 1.0 {
		t.Fatalf("排程已取走，不得二次翻转：电压源=%v，期望 1", got)
	}
	if armed, _ := readFloatSlot(node, queueSlotArmed); armed != 0 {
		t.Fatalf("翻转后排程应清空，armed=%v", armed)
	}
}

// TestDelayQueuePlanRejectsNonFutureTime 覆盖 at<=now 报错、当前时间随 Apply 推进、以及不静默覆盖旧排程。
func TestDelayQueuePlanRejectsNonFutureTime(t *testing.T) {
	node := newSlotNode(queueSlotsPerBase)
	q := &DelayQueue{}

	// 未 Apply 过时基准时间是槽位初值 0：at=0 必须被拒。
	if err := q.Plan(node, 0, 0, 5.0); err == nil {
		t.Fatal("at=0 且当前时间为 0 时必须报错（at 必须严格大于当前时间）")
	}
	if err := q.Plan(node, 0, 10e-9, 5.0); err != nil {
		t.Fatalf("at=10ns 应可排程，实际: %v", err)
	}

	// 空闲状态下 Apply 也要推进观察时间（Plan 的基准靠它）。
	q.Apply(nil, &stubTime{now: 30e-9}, node, 0, 0)
	if err := q.Plan(node, 0, 30e-9, 5.0); err == nil {
		t.Fatal("at == 当前时间(30ns) 必须报错")
	}
	if err := q.Plan(node, 0, 29e-9, 5.0); err == nil {
		t.Fatal("at < 当前时间 必须报错")
	}
	if err := q.Plan(node, 0, 31e-9, 5.0); err != nil {
		t.Fatalf("at=31ns 应可排程，实际: %v", err)
	}
	// 已有未到期排程时再排必须报错（不得静默丢弃旧事件）。
	if err := q.Plan(node, 0, 32e-9, 0); err == nil {
		t.Fatal("已有未到期排程时必须报错而不是静默覆盖")
	}
	if at, _ := readFloatSlot(node, queueSlotAt); at != 31e-9 {
		t.Fatalf("报错后原排程时刻应保持 31ns，实际 %v", at)
	}
}

// TestDelayQueuePlanRejectsBadSlots 覆盖槽位不足、槽位类型不符、nil 节点、负 base 与 NaN 参数。
func TestDelayQueuePlanRejectsBadSlots(t *testing.T) {
	q := &DelayQueue{}

	if err := q.Plan(newSlotNode(queueSlotsPerBase-1), 0, 1e-9, 5); err == nil {
		t.Fatal("槽位不足必须报错")
	}
	bad := newSlotNode(queueSlotsPerBase + 1)
	bad.NodeValue[queueSlotNow] = "keep"
	if err := q.Plan(bad, 0, 1e-9, 5); err == nil {
		t.Fatal("槽位类型不是 float64 必须报错")
	}
	if bad.NodeValue[queueSlotNow] != "keep" {
		t.Fatalf("报错时不得改写元件原有槽位：%v", bad.NodeValue[queueSlotNow])
	}
	if err := q.Plan(nil, 0, 1e-9, 5); err == nil {
		t.Fatal("value 为 nil 必须报错")
	}
	if err := q.Plan(newSlotNode(2*queueSlotsPerBase), -1, 1e-9, 5); err == nil {
		t.Fatal("base<0 必须报错")
	}
	if err := q.Plan(newSlotNode(queueSlotsPerBase), 0, math.NaN(), 5); err == nil {
		t.Fatal("at=NaN 必须报错")
	}
	if err := q.Plan(newSlotNode(queueSlotsPerBase), 0, 1e-9, math.NaN()); err == nil {
		t.Fatal("电平为 NaN 必须报错（不得把 NaN 写进电压源）")
	}
}

// TestDelayQueueIndependentBases 覆盖同一元件上多个 base 各自独立排程与翻转。
func TestDelayQueueIndependentBases(t *testing.T) {
	node := newSlotNode(2 * queueSlotsPerBase) // base 0 与 base 4
	q := &DelayQueue{}
	m := mna.NewMnaUpdate(2, 2)

	if err := q.Plan(node, 0, 10e-9, 5.0); err != nil {
		t.Fatalf("base 0 排程失败: %v", err)
	}
	if err := q.Plan(node, queueSlotsPerBase, 20e-9, 0.0); err != nil {
		t.Fatalf("base 4 排程失败: %v", err)
	}
	m.UpdateVoltageSource(1, 0.0)

	q.Apply(m, &stubTime{now: 10e-9}, node, 0, 0)
	if got := m.GetZ().Get(testVoltRow); got != 5.0 {
		t.Fatalf("base 0 到点应把电压源 0 驱动到 5，实际 %v", got)
	}
	if armed, _ := readFloatSlot(node, queueSlotsPerBase+queueSlotArmed); armed != 1 {
		t.Fatalf("base 4 尚未到点，应保持已排程，armed=%v", armed)
	}
	m.UpdateVoltageSource(1, 5.0)
	q.Apply(m, &stubTime{now: 20e-9}, node, queueSlotsPerBase, 1)
	if got := m.GetZ().Get(testVoltRow + 1); got != 0.0 {
		t.Fatalf("base 4 到点应把电压源 1 驱动到 0，实际 %v", got)
	}
}
