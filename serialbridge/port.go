// port.go 定义跨平台的宿主串口端点入口：调用方只依赖 PortConfig 与 OpenPort，
// 平台差异（Unix tty / Windows COM）全部落在 portUnix.go 与 portWindows.go。
package serialbridge

import "time"

// PortConfig 是打开宿主串口端点所需的参数。
type PortConfig struct {
	// Name 是平台端点名：Unix 上是 tty 路径（如 /dev/pts/3），Windows 上是 COM 端口名（如 COM3）。
	Name string
	// Baud 是波特率，仅 Windows 生效（Unix tty 的波特率由打开方自行配置）。
	Baud int
	// Timeout 是读/写期限；<=0 时取 DefaultReadTimeout。
	Timeout time.Duration
}
