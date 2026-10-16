// stamp_test.go：步进幂等——同一仿真时刻对同一元件最多推进一次。
package digital

import (
	"math"
	"testing"

	"circuit/element"
)

// TestStampIdempotentWithinSameTime 是幂等主用例：同一 now 连续调用只推进一次，换 now 再推进。
//
// 反证方式（本用例在 MarkStamp 被注释掉后必须 FAIL）：
// 去掉 MarkStamp 后 SinceLastStamp 恒为 +Inf，step() 每次都会推进，advances 变成 3 ≠ 1。
func TestStampIdempotentWithinSameTime(t *testing.T) {
	node := newSlotNode(4) // 元件把「最近推进时刻」放在槽位 0
	// 约定：未推进过用 NaN（或 nil）表示。若初始化为 0.0，则 now 恰为 0 时会被视为已推进一次
	// （见 TestStampSlotConventions 的文档化边界用例），故这里按推荐约定初始化。
	node.NodeValue[0] = math.NaN()

	if got := SinceLastStamp(node, 0, 0); !math.IsInf(got, 1) {
		t.Fatalf("尚未推进过应返回 +Inf（否则 t=0 的首步会被误判为已推进），实际 %v", got)
	}

	advances := 0
	now := 1e-6
	step := func() {
		if SinceLastStamp(node, 0, now) == 0 {
			return // 本时刻已推进过
		}
		advances++
		MarkStamp(node, 0, now)
	}
	step()
	step()
	step()
	if advances != 1 {
		t.Fatalf("同一 now 连续 3 次调用应只推进 1 次，实际 %d 次", advances)
	}

	now = 2e-6
	step()
	if advances != 2 {
		t.Fatalf("换到新的 now 应再推进 1 次，实际 %d 次", advances)
	}
	if got := SinceLastStamp(node, 0, 2e-6); got != 0 {
		t.Fatalf("刚推进过的 now 应立即返回 0，实际 %v", got)
	}
	if got := SinceLastStamp(node, 0, 3e-6); math.Abs(got-1e-6) > 1e-18 {
		t.Fatalf("距上次推进的时间差应为 1e-6（浮点减法按容差比较），实际 %v", got)
	}
}

// TestStampSlotConventions 覆盖「尚未推进」的表示、时间回退与非 float64 槽位不被覆盖。
func TestStampSlotConventions(t *testing.T) {
	// nil 槽位（未初始化）算尚未推进。
	nilSlot := &element.Node{NodeValue: make([]any, 2)}
	if got := SinceLastStamp(nilSlot, 0, 5); !math.IsInf(got, 1) {
		t.Fatalf("nil 槽位应视为尚未推进(+Inf)，实际 %v", got)
	}
	// 越界槽位同样算尚未推进（不得 panic）。
	if got := SinceLastStamp(newSlotNode(1), 3, 5); !math.IsInf(got, 1) {
		t.Fatalf("越界槽位应视为尚未推进(+Inf)，实际 %v", got)
	}
	// NaN 槽位是推荐的「尚未推进」初值；推进后按时间差返回。
	nanSlot := newSlotNode(2)
	nanSlot.NodeValue[0] = math.NaN()
	if got := SinceLastStamp(nanSlot, 0, 5); !math.IsInf(got, 1) {
		t.Fatalf("NaN 槽位应视为尚未推进(+Inf)，实际 %v", got)
	}
	MarkStamp(nanSlot, 0, 5)
	if got := SinceLastStamp(nanSlot, 0, 5); got != 0 {
		t.Fatalf("推进过的同一时刻应为 0，实际 %v", got)
	}
	if got := SinceLastStamp(nanSlot, 0, 7); got != 2 {
		t.Fatalf("时间差应为 2，实际 %v", got)
	}
	if got := SinceLastStamp(nanSlot, 0, 1); got != -4 {
		t.Fatalf("时间回退应返回负值（调用方按 !=0 处理），实际 %v", got)
	}
	// 非 float64 槽位不得被 MarkStamp 覆盖（保留元件其它字段的语义）。
	keep := newSlotNode(2)
	keep.NodeValue[1] = "state"
	MarkStamp(keep, 1, 3)
	if keep.NodeValue[1] != "state" {
		t.Fatalf("MarkStamp 不得覆盖非 float64 槽位，实际 %v", keep.NodeValue[1])
	}
	// 文档化边界：槽位初始化为 0.0 时，now 恰为 0 会被视为「已推进一次」。
	zeroed := newSlotNode(1)
	if got := SinceLastStamp(zeroed, 0, 0); got != 0 {
		t.Fatalf("0.0 初值 + now=0 应返回 0（文档化行为），实际 %v", got)
	}
}
