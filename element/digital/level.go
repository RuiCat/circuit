// level.go：数字电平规格（阈值判决与驱动电平的单一来源）。
package digital

// Level 是一个数字元件的电平规格：VHigh/VLow 为驱动电平，Threshold 为输入判决阈值。
// Threshold 为零值时按摆幅中点 (VHigh+VLow)/2 判决——VLow 为 0 时即默认阈值 VHigh/2。
type Level struct {
	VHigh     float64 // 逻辑高电平（V）。
	VLow      float64 // 逻辑低电平（V）。
	Threshold float64 // 输入判决阈值（V）；零值表示取摆幅中点。
}

// IsHigh 判断输入电压 v 是否处于逻辑高：v 严格大于有效判决阈值即为高。
// 恰好等于阈值判为低；v 为 NaN 时判为低。
func (l Level) IsHigh(v float64) bool {
	return v > l.effectiveThreshold()
}

// effectiveThreshold 返回实际使用的判决阈值：Threshold 非零时用它，否则用 (VHigh+VLow)/2。
func (l Level) effectiveThreshold() float64 {
	if l.Threshold != 0 {
		return l.Threshold
	}
	return (l.VHigh + l.VLow) / 2
}
