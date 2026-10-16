// uartAck.go：ACK 确认——槽 15 非 0 时本步内清 dataReady 与 ackFlag（K2）。
package serial

import "circuit/element"

// clearUartAck 处理宿主 ACK：槽 15 非 0 ⇒ dataReady=false、ackFlag=0，INTR 随后回落。
// 它在本步的状态推进里最先执行，故同一步内新到的字节仍会重新置位 dataReady（不丢字节）。
func clearUartAck(value element.NodeFace) {
	if value == nil || value.GetFloat64(slotAckFlag) == 0 {
		return
	}
	value.SetBool(slotDataReady, false)
	value.SetFloat64(slotAckFlag, 0)
}
