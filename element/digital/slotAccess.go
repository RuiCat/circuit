// slotAccess.go：element.NodeFace 槽位的安全读写（越界/类型不符即失败，不越界写、不破坏元件状态）。
package digital

import "circuit/element"

// readFloatSlot 读取 value 的第 idx 个槽位：槽位存在且持有 float64 时返回 (值, true)；
// 越界、缺槽位、nil 或非 float64 时返回 (0, false)。
// 它不走 value.GetFloat64，以免对合法但尚未初始化的槽位打出类型断言告警日志。
func readFloatSlot(value element.NodeFace, idx int) (float64, bool) {
	if idx < 0 {
		return 0, false
	}
	base := nodeBase(value)
	if base == nil || idx >= len(base.NodeValue) {
		return 0, false
	}
	v, ok := base.NodeValue[idx].(float64)
	return v, ok
}

// writeFloatSlot 写入 value 的第 idx 个槽位：仅当槽位存在且为 nil 或 float64 时写入。
// 越界或槽位持有非 float64 状态时返回 false 且不改写（保留元件原有状态，不做覆盖式破坏）。
func writeFloatSlot(value element.NodeFace, idx int, v float64) bool {
	if idx < 0 {
		return false
	}
	base := nodeBase(value)
	if base == nil || idx >= len(base.NodeValue) {
		return false
	}
	if raw := base.NodeValue[idx]; raw != nil {
		if _, ok := raw.(float64); !ok {
			return false
		}
	}
	base.NodeValue[idx] = v
	return true
}

// slotsFit 判断 value 是否持有 [base, base+n) 连续槽位。
func slotsFit(value element.NodeFace, base, n int) bool {
	if base < 0 || n <= 0 {
		return false
	}
	node := nodeBase(value)
	return node != nil && base+n <= len(node.NodeValue)
}

// slotTypesFit 判断 [base, base+n) 的槽位是否都能承载 float64（nil 视为可初始化的空槽）。
// 只要有一个槽位持有非 nil 的非 float64 状态就返回 false，避免写坏元件其它字段的语义。
func slotTypesFit(value element.NodeFace, base, n int) bool {
	if !slotsFit(value, base, n) {
		return false
	}
	node := nodeBase(value)
	for i := base; i < base+n; i++ {
		if raw := node.NodeValue[i]; raw != nil {
			if _, ok := raw.(float64); !ok {
				return false
			}
		}
	}
	return true
}

// nodeBase 返回 value 的底层 *element.Node。
// value 为 nil、或为底层节点为 nil 的实现（typed-nil）时返回 nil，调用方据此走降级路径而不 panic。
func nodeBase(value element.NodeFace) *element.Node {
	if value == nil {
		return nil
	}
	return value.Base()
}
