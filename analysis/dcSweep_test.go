// dcSweep_test.go：DC 扫描单测（m3，c3）。
package analysis

import (
	"math"
	"testing"
)

// TestDCSweep 对分压器做 0..5V 扫描，断言 V(out)=V1/2。
func TestDCSweep(t *testing.T) {
	netlist := `
V1 [1,-1] [0,0,0,0,5]
R1 [1,2] [1000]
R2 [2,-1] [1000]
`
	points, err := RunDCSweep(netlist, "V1", 0, 5, 1)
	if err != nil {
		t.Fatalf("RunDCSweep 失败: %v", err)
	}
	if len(points) != 6 {
		t.Fatalf("点数=%d，期望 6", len(points))
	}
	for _, p := range points {
		want := p.SourceValue / 2
		if got := p.NodeVoltages[2]; math.Abs(got-want) > 1e-6 {
			t.Errorf("V1=%g: V(out)=%g，期望 %g", p.SourceValue, got, want)
		}
	}
}
