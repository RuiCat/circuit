// uartReset.go：Reset——把全部运行态槽复位到配置初值（C5：同一 Context 可重复运行）。
package serial

import "circuit/element"

// Reset 复位 UART 的全部运行态槽：槽 3..24 回到 Config.ValueInit 的初值
// （槽 3 回到 -1、幂等戳回到 NaN、相位/移位器/标志与 13 针握手槽清零），
// 并同步刷新 OrigValue——引擎每步的 MarkRollbackElements 只回滚 OrigValue 里登记的槽。
//
// 事件落点槽 15（ackFlag）一并清零：否则上一次运行残留的 ACK 会清掉第二次运行的首个 dataReady。
func (u *Uart) Reset(base element.NodeFace) {
	if base == nil {
		return
	}
	cfg := base.Config()
	node := base.Base()
	if cfg == nil || node == nil {
		return
	}
	init := cfg.ValueInit
	for i := slotTxByte; i < len(init) && i < len(node.NodeValue); i++ {
		node.NodeValue[i] = init[i]
	}
	for _, i := range cfg.OrigValue {
		if i >= 0 && i < len(init) {
			node.OrigValue[i] = init[i]
		}
	}
}
