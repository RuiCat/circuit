package controlled

import (
	_ "circuit/element/passive"
	_ "circuit/element/source"
	"circuit/element/time"
	"circuit/load"
	"math"
	"testing"
)

// TestCCVS 验证流控电压源：控制电流=5mA, 跨阻=200Ω → 输出电压=1V
func TestCCVS(t *testing.T) {
	netlist := `
	v1 [1,-1] [0,0,0,0,10]
	r1 [1,2] [2000]
	h1 [2,-1,3,-1] [200]
	r2 [3,-1] [1000]
	`
	con, err := load.LoadString(netlist)
	if err != nil {
		t.Fatalf("加载网表失败: %s", err)
	}
	con.Time, err = time.NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建仿真时间失败: %s", err)
	}
	if err := time.TransientSimulation(con, func(v []float64) {}); err != nil {
		t.Fatalf("仿真失败: %s", err)
	}

	// 控制电流 = 10V/2000Ω = 5mA, 跨阻=200Ω → V=1V
	// 原始节点 3 → 紧凑 2
	vout := con.GetNodeVoltage(2)
	if math.Abs(vout-1.0) > 1e-3 {
		t.Errorf("输出电压不正确: 期望 1.0V, 实际 %.6fV", vout)
	}
}
