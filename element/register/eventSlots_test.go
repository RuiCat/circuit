// eventSlots_test.go：C3 的机检用例——遍历**全部已注册元件**，核对事件槽契约。
//
// 存在理由：独立复核指出 C3 的判定①（"新增 Go 用例遍历全局 element.ElementList"）在本轮
// 证据表里没有对应产物，而 C3 一旦失守的后果是**宿主写入被静默丢弃**（引擎对类型不符的事件
// 直接 continue，随后无条件把事件置 nil），且不报错。故该判据必须真的存在、并对全部元件生效。
//
// 断言两条（与 30-constraints.md 的 C3 一致）：
//  ① 事件目标槽（EventSlots 的正值项）在 ValueInit 里非 nil，且类型是宿主可写的 float64/bool/string；
//  ② 事件目标槽**不得**登记进 OrigValue——否则引擎在步首回滚会把刚写入的事件抹掉。
// 另外核对事件名槽（EventSlots 的键）是 string 型（缺省事件名非空由元件自证）。
package register_test

import (
	"testing"

	"circuit/element"
	_ "circuit/element/register" // 触发全部元件的 init() 注册（C7 口径：走注册路径）
	"circuit/load/ast"
)

// TestEventSlotTargetsAreWritableAndNotRolledBack 是 C3 判定①的机检用例。
func TestEventSlotTargetsAreWritableAndNotRolledBack(t *testing.T) {
	checked := 0
	for _, face := range element.ElementList {
		if face == nil {
			continue
		}
		cfg := face.Base(ast.ElementNode{})
		if cfg == nil || len(cfg.EventSlots) == 0 {
			continue
		}
		orig := make(map[int]bool, len(cfg.OrigValue))
		for _, i := range cfg.OrigValue {
			orig[i] = true
		}
		for nameIdx, valIdx := range cfg.EventSlots {
			// 负值 = 生产者（PullEvents 按元件遍历），不走"宿主写入"路径，不属本判据范围。
			if valIdx < 0 {
				continue
			}
			checked++
			if valIdx >= len(cfg.ValueInit) {
				t.Errorf("C3①：元件 %q 的事件目标槽 %d 越界（ValueInit 长度 %d）", cfg.GetName(), valIdx, len(cfg.ValueInit))
				continue
			}
			switch v := cfg.ValueInit[valIdx].(type) {
			case float64, bool, string:
				_ = v
			case nil:
				t.Errorf("C3①：元件 %q 的事件目标槽 %d 初值为 nil ⇒ PushEvents 的类型检查会让宿主写入静默丢弃", cfg.GetName(), valIdx)
			default:
				t.Errorf("C3①：元件 %q 的事件目标槽 %d 类型 %T 不是宿主可写类型（float64/bool/string）", cfg.GetName(), valIdx, v)
			}
			if orig[valIdx] {
				t.Errorf("C3②：元件 %q 的事件目标槽 %d 登记进了 OrigValue ⇒ 事件会在步首被回滚，元件永远读不到", cfg.GetName(), valIdx)
			}
			if nameIdx >= 0 && nameIdx < len(cfg.ValueInit) {
				if _, ok := cfg.ValueInit[nameIdx].(string); !ok {
					t.Errorf("C3：元件 %q 的事件名槽 %d 不是 string 型（%T）", cfg.GetName(), nameIdx, cfg.ValueInit[nameIdx])
				}
			}
		}
	}
	if checked == 0 {
		t.Fatalf("C3 判据是空的：没有任何元件带消费者事件槽（说明遍历没跑到注册表）")
	}
	t.Logf("C3 机检：核对了 %d 个消费者事件槽", checked)
}
