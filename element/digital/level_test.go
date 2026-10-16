// level_test.go：Level.IsHigh 的阈值边界（VHigh/2 两侧）与默认阈值。
package digital

import (
	"math"
	"testing"
)

// TestIsHighThresholdBoundary 覆盖阈值两侧与恰好等于阈值的取样点。
func TestIsHighThresholdBoundary(t *testing.T) {
	lvl := Level{VHigh: 5, VLow: 0, Threshold: 2.5} // 阈值 = VHigh/2
	cases := []struct {
		name string
		v    float64
		want bool
	}{
		{"低电平", 0.1, false},
		{"略低于 VHigh/2", 2.499, false},
		{"恰好等于 VHigh/2（判低）", 2.5, false},
		{"略高于 VHigh/2", 2.501, true},
		{"高电平", 5.0, true},
		{"NaN 判低", math.NaN(), false},
	}
	for _, c := range cases {
		if got := lvl.IsHigh(c.v); got != c.want {
			t.Errorf("%s: IsHigh(%v) = %v，期望 %v", c.name, c.v, got, c.want)
		}
	}
	// 反证：阈值两侧若给出相同判决，说明这条边界判据是空的。
	if lvl.IsHigh(2.499) == lvl.IsHigh(2.501) {
		t.Fatalf("VHigh/2 两侧判决相同 ⇒ 阈值边界判据为空")
	}
}

// TestIsHighDefaultThreshold 覆盖 Threshold 零值时的摆幅中点默认值与显式阈值覆盖。
func TestIsHighDefaultThreshold(t *testing.T) {
	defaulted := Level{VHigh: 5, VLow: 0} // Threshold 零值 ⇒ 默认 2.5
	if defaulted.IsHigh(2.499) {
		t.Fatalf("默认阈值 2.5：2.499 应判低")
	}
	if !defaulted.IsHigh(2.501) {
		t.Fatalf("默认阈值 2.5：2.501 应判高")
	}
	explicit := Level{VHigh: 5, VLow: 0, Threshold: 1.4}
	if !explicit.IsHigh(2.5) {
		t.Fatalf("显式阈值 1.4：2.5 应判高（显式阈值必须覆盖默认中点）")
	}
	if explicit.IsHigh(1.4) {
		t.Fatalf("显式阈值 1.4：恰好等于阈值应判低")
	}
	negSwing := Level{VHigh: 1, VLow: -1} // 中点 0
	if negSwing.IsHigh(-0.001) {
		t.Fatalf("中点 0：-0.001 应判低")
	}
	if !negSwing.IsHigh(0.001) {
		t.Fatalf("中点 0：0.001 应判高")
	}
}
