// stamp.go：步进幂等——同一仿真时刻对同一元件最多推进一次。
package digital

import (
	"math"

	"circuit/element"
)

// SinceLastStamp 返回距离该槽位上一次 MarkStamp 的仿真时间差，供元件做步进幂等短路：
//
//	if digital.SinceLastStamp(value, idx, t.Time()) == 0 {
//		return // 本时刻已推进过，不再推进
//	}
//	// …推进状态…
//	digital.MarkStamp(value, idx, t.Time())
//
// 尚未推进过时返回 +Inf（表示「从未推进、必须推进」）——槽位缺省/越界/nil/非 float64、
// 或槽位值为 NaN 都算尚未推进。这样仿真从 t=0 开始时，首次调用不会被误判成「本时刻已推进」。
// 约定：元件请把该槽位初始化为 math.NaN()（或保持 nil）表示尚未推进；若初始化为 0.0，
// 则在 now 恰为 0 时会返回 0（被当作已推进一次）。
// now 早于上次推进时间（时间回退）时返回负值；调用方按「!= 0 即可推进」处理即可。
func SinceLastStamp(value element.NodeFace, idx int, now float64) float64 {
	last, ok := readFloatSlot(value, idx)
	if !ok || math.IsNaN(last) {
		return math.Inf(1)
	}
	return now - last
}

// MarkStamp 记录「该元件在仿真时刻 now 已推进一次」。
// 槽位缺省/nil/float64 时写入；槽位越界或持有非 float64 状态时不写入——本函数没有 error 通道，
// 此时保持元件原有状态不被覆盖（K1 签名固定）。
func MarkStamp(value element.NodeFace, idx int, now float64) {
	writeFloatSlot(value, idx, now)
}
