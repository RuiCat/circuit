// loaderReset.go：Reset——把全部运行态槽恢复到实例初值，使同一 Context 可重复加载（C12）。
package loader

import "circuit/element"

// Reset 复位 LOADER 的全部运行态槽（22..33）：相位回 idle、帧状态回等 SYNC、
// error/done/addr/幂等戳清零。参数槽 0..3 保留网表给定值不动。
//
// 复位后相位回到 idle ⇒ 输出重新为 LOAD=1（CPU 冻结）、LWE=0、ACK=0，
// 与上电一致；RAM/DFF 自身的状态由 m5 的 Reset 负责，不在本元件写域内。
func (l *Loader) Reset(base element.NodeFace) {
	if base == nil {
		return
	}
	cfg := base.Config()
	node := base.Base()
	if cfg == nil || node == nil {
		return
	}
	init := cfg.ValueInit
	for i := slotError; i < len(init) && i < len(node.NodeValue); i++ {
		node.NodeValue[i] = init[i]
	}
	for _, i := range cfg.OrigValue {
		if i >= 0 && i < len(init) {
			node.OrigValue[i] = init[i]
		}
	}
}
