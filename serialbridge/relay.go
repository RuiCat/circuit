// relay.go 实现桥的两个搬运方向：串口 → 仿真（Ingest）与 仿真 → 串口（Drain）。
package serialbridge

import "errors"

// Ingest 从串口通道读最多 n 个字节写进入站事件（串口 → 仿真）。
// 每个字节都等到引擎报告消费后才发下一个（C15：一次一字节 + 确认/推进）；
// 通道读超时/关闭、引擎推进失败都按错误返回，已搬运的字节数一并返回。
func (br *Bridge) Ingest(link Link, n int) (int, error) {
	got := 0
	for got < n {
		b, err := link.ReadByte()
		if err != nil {
			return got, err
		}
		if err := br.SendByte(b); err != nil {
			if !errors.Is(err, ErrByteInFlight) {
				return got, err
			}
			// 上一字节还在途：先推进等元件取走，再补发本字节；仍失败则上报，不丢字节。
			if err := br.WaitTxDrained(); err != nil {
				return got, err
			}
			if err := br.SendByte(b); err != nil {
				return got, err
			}
		}
		if err := br.WaitTxDrained(); err != nil {
			return got, err
		}
		// D16：等发送侧回到帧间空闲再发下一字节，否则入站槽被覆盖 ⇒ 静默丢字节。
		if err := br.WaitTxIdle(); err != nil {
			return got, err
		}
		got++
	}
	return got, nil
}

// Drain 把最多 n 个出站字节写进串口通道（仿真 → 串口）。
// 返回 (已写出字节数, 帧错误字节数, 错误)：无数据时推进仿真，
// 连续空闲步数超过预算返回 ErrStepBudget（不无界等待、不 panic）。
func (br *Bridge) Drain(link Link, n int) (int, int, error) {
	sent := 0
	frameErrs := 0
	idle := 0
	for sent < n {
		b, frameErr, err := br.ReceiveByte()
		switch {
		case err == nil:
			idle = 0
			if _, werr := link.WriteBytes([]byte{b}); werr != nil {
				return sent, frameErrs, werr
			}
			sent++
			if frameErr {
				frameErrs++
			}
		case errors.Is(err, ErrNoData):
			idle++
			if idle > br.stepBudget {
				return sent, frameErrs, ErrStepBudget
			}
			if err := br.Advance(); err != nil {
				return sent, frameErrs, err
			}
		default:
			return sent, frameErrs, err
		}
	}
	return sent, frameErrs, nil
}
