// loaderFrame.go：SYNC+LEN+DATA+CHK 帧解码、校验与帧数据槽的写时复制读写。
package loader

import "circuit/element"

// 帧解码状态（槽 26 的取值）。
const (
	frameWaitSync  = iota // 等 SYNC（非 SYNC 字节静默忽略，便于流内重新同步）。
	frameWaitLen          // 等 LEN。
	frameWaitData         // 收 LEN 个数据字节。
	frameWaitCheck        // 等 CHK。
)

// 帧协议常量（U4 最小帧：SYNC + LEN + DATA + CHK；无重传、无协议级 ACK、无超时）。
const (
	frameSyncByte = 0xA5 // SYNC 字节；与 HALT 指令 0x64 等可区分。
	checksumMask  = 0xFF // CHK = (LEN + ΣDATA) & 0xFF。
)

// decodeFrameByte 把一个经 4 相握手收到的字节喂给帧解码状态机。
// 校验成功才进入写时序；坏校验/超长/超短帧一律只置 error，不写 RAM、不释放 CPU。
func decodeFrameByte(value element.NodeFace, b int, mode loaderMode) {
	switch int(value.GetFloat64(slotFrame)) {
	case frameWaitSync:
		if b == frameSyncByte {
			value.SetFloat64(slotFrame, frameWaitLen)
		}
	case frameWaitLen:
		if b < 1 || b > effectiveLenMax(value, mode) {
			latchLoaderError(value)
			return
		}
		value.SetFloat64(slotFrameLen, float64(b))
		value.SetFloat64(slotFramePos, 0)
		value.SetFloat64(slotCheckSum, float64(b))
		setFrameData(value, nil)
		value.SetFloat64(slotFrame, frameWaitData)
	case frameWaitData:
		appendFrameByte(value, byte(b))
		value.SetFloat64(slotCheckSum, value.GetFloat64(slotCheckSum)+float64(b))
		pos := int(value.GetFloat64(slotFramePos)) + 1
		value.SetFloat64(slotFramePos, float64(pos))
		if pos >= int(value.GetFloat64(slotFrameLen)) {
			value.SetFloat64(slotFrame, frameWaitCheck)
		}
	case frameWaitCheck:
		if b != int(value.GetFloat64(slotCheckSum))&checksumMask {
			latchLoaderError(value)
			return
		}
		value.SetFloat64(slotAddr, 0)
		value.SetFloat64(slotPhase, phaseSetup)
	}
}

// effectiveLenMax 返回本帧允许的长度上限：LenMax 参数与模式地址空间取较小者（C14/C2.5）。
func effectiveLenMax(value element.NodeFace, mode loaderMode) int {
	limit := int(value.GetFloat64(slotLenMax))
	if limit <= 0 {
		limit = defaultLenMax
	}
	if limit > mode.capacity {
		limit = mode.capacity
	}
	return limit
}

// latchLoaderError 锁存错误：error=1、相位进 error。LOAD 由相位派生保持高电平，CPU 不释放。
func latchLoaderError(value element.NodeFace) {
	value.SetFloat64(slotError, 1)
	value.SetFloat64(slotPhase, phaseError)
}

// loaderDataAt 返回本帧第 addr 个数据字节；越界返回 0（LA/LD 在写阶段外保持稳定）。
func loaderDataAt(value element.NodeFace, addr int) byte {
	data := frameDataOf(value)
	if addr < 0 || addr >= len(data) {
		return 0
	}
	return data[addr]
}

// frameDataOf 读取帧数据槽。槽位缺失或类型不符时返回 nil（不 panic、不写坏其它字段）。
func frameDataOf(value element.NodeFace) []byte {
	node := value.Base()
	if node == nil || slotFrameData < 0 || slotFrameData >= len(node.NodeValue) {
		return nil
	}
	data, _ := node.NodeValue[slotFrameData].([]byte)
	return data
}

// appendFrameByte 追加一个帧数据字节（写时复制）。
// 不原地扩容：引擎的 Update/Rollback 只做槽位的浅拷贝，共用底层数组会把未接受的写入带过回滚。
func appendFrameByte(value element.NodeFace, b byte) {
	old := frameDataOf(value)
	next := make([]byte, len(old)+1)
	copy(next, old)
	next[len(old)] = b
	setFrameData(value, next)
}

// setFrameData 写入帧数据槽；槽位存在且类型可承载时才写。
func setFrameData(value element.NodeFace, data []byte) {
	node := value.Base()
	if node == nil || slotFrameData < 0 || slotFrameData >= len(node.NodeValue) {
		return
	}
	if raw := node.NodeValue[slotFrameData]; raw != nil {
		if _, ok := raw.([]byte); !ok {
			return
		}
	}
	node.NodeValue[slotFrameData] = data
}
