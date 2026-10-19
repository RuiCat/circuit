// link.go 定义桥的串口侧字节通道抽象 Link（平台无关）。
//
// 平台实现在 linkUnix.go（Unix tty，用 poll 提供期限）与 portWindows.go（Windows COM 端口，
// 用 COMMTIMEOUTS 提供期限）；调用方只依赖本接口，不出现平台判断。
package serialbridge

import "time"

// DefaultReadTimeout 是 Link 未显式给出超时时的默认值。
// 有期限是新通道的硬约束：无数据时必须返回 ErrTimeout，不得无限阻塞。
const DefaultReadTimeout = 200 * time.Millisecond

// Link 是串口侧的字节通道（真实设备或内存假实现）。
// 约定：ReadByte 必须有期限——无数据返回 ErrTimeout，对端关闭返回 ErrLinkClosed；
// 调用方据此给出可判定错误，而不是 panic 或永久阻塞。
type Link interface {
	ReadByte() (byte, error)
	WriteBytes(p []byte) (int, error)
	Close() error
}
