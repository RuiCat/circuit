package controlled

import (
	_ "circuit/element/passive"
	_ "circuit/element/source"
	"circuit/element/time"
	"circuit/load"
	"math"
	"testing"
)

// TestCCCS 验证流控电流源：控制电流=10mA(10V/1kΩ), 增益=10 → 输出电流=100mA, 输出电阻100Ω → 输出电压10V
func TestCCCS(t *testing.T) {
	netlist := `
	v1 [1,-1] [0,0,0,0,10]
	r1 [1,2] [1000]
	f1 [2,-1,3,-1] [10]
	r2 [3,-1] [100]
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

	// 控制电流 = 10V/1000Ω = 10mA, 增益=10, 输出=100mA, R2=100Ω → V=10V
	// 原始节点 3 → 紧凑 2，原始节点 2 → 紧凑 1
	vout := con.GetNodeVoltage(2)
	if math.Abs(vout-10.0) > 0.1 {
		t.Errorf("输出电压不正确: 期望 10.0V, 实际 %.6fV", vout)
	}
	// 节点2应为0V（0V测量电压源的一端接地）
	v2 := con.GetNodeVoltage(1)
	if math.Abs(v2) > 0.1 {
		t.Errorf("测量节点电压应为0V: 实际 %.6fV", v2)
	}
}
