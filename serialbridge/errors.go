// errors.go 定义串口桥的可判定错误：调用方一律用 errors.Is 判定，不解析错误文本。
package serialbridge

import "errors"

var (
	// ErrByteInFlight：上一字节尚未被引擎消费就再次写入同一入站事件（流控拒绝，绝不静默覆盖）。
	ErrByteInFlight = errors.New("serialbridge: 上一字节尚未被引擎消费")
	// ErrNoData：元件槽 dataReady 为 0，当前没有可读字节。
	ErrNoData = errors.New("serialbridge: 当前没有可读字节")
	// ErrStepBudget：等待期间用完步数预算（防止对"永远不出字节"的元件无界等待）。
	ErrStepBudget = errors.New("serialbridge: 步数预算耗尽")
	// ErrSlotValue：槽值越界或类型不符（例如 rxByte 不在 0..255）。
	ErrSlotValue = errors.New("serialbridge: 槽值越界或类型不符")
	// ErrMissingInstance：既未给 Instance 也未显式给事件名，无法确定入站/ACK 事件名。
	ErrMissingInstance = errors.New("serialbridge: 缺少 UART 实例名")
	// ErrTimeout：串口侧读/写在期限内没有完成（对端没给数据）。
	ErrTimeout = errors.New("serialbridge: 串口超时")
	// ErrLinkClosed：串口侧已关闭或对端已断开。
	ErrLinkClosed = errors.New("serialbridge: 串口通道已关闭")
)
