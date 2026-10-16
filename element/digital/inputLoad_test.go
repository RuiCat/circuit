// inputLoad_test.go：InputLoad.ConductanceAt 的高/低两侧取值。
package digital

import (
	"math"
	"testing"
)

// TestInputLoadConductanceBothSides 覆盖高/低两侧分别给出 IIH/IIL 量级，以及阈值边界与 NaN。
func TestInputLoadConductanceBothSides(t *testing.T) {
	load := InputLoad{IIH: 20e-6, IIL: -0.4e-3} // 74LS 量级：IIH=20µA，IIL=-0.4mA
	const threshold = 2.5

	high := load.ConductanceAt(4.9, threshold)
	low := load.ConductanceAt(0.1, threshold)
	if high != load.IIH {
		t.Fatalf("高侧应取 IIH=%g，实际 %g", load.IIH, high)
	}
	if low != load.IIL {
		t.Fatalf("低侧应取 IIL=%g，实际 %g", load.IIL, low)
	}
	if math.Abs(high) != 20e-6 {
		t.Fatalf("高侧量级应为 20µA，实际 %g", math.Abs(high))
	}
	if math.Abs(low) != 0.4e-3 {
		t.Fatalf("低侧量级应为 0.4mA，实际 %g", math.Abs(low))
	}
	// 反证：两侧取值若相同，说明分段判据是空的。
	if high == low {
		t.Fatal("高/低两侧取值相同 ⇒ 分段判据为空")
	}
	// 边界：恰好等于阈值走低段；NaN 走低段。
	if got := load.ConductanceAt(threshold, threshold); got != load.IIL {
		t.Fatalf("恰好等于阈值应走低段，实际 %g", got)
	}
	if got := load.ConductanceAt(math.NaN(), threshold); got != load.IIL {
		t.Fatalf("NaN 应走低段，实际 %g", got)
	}
	// 零点负载（无输入电流）时两侧都是 0，不得产生 NaN。
	none := InputLoad{}
	if none.ConductanceAt(5, threshold) != 0 || none.ConductanceAt(0, threshold) != 0 {
		t.Fatal("零负载应返回 0")
	}
}
