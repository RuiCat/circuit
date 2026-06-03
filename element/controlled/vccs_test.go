package controlled

import (
	_ "circuit/element/passive"
	_ "circuit/element/source"
	"circuit/element/time"
	"circuit/load"
	"math"
	"testing"
)

// TestVCCS 验证压控电流源：控制电压1V，跨导1e-3S，输出接1kΩ → 输出电流1mA, 输出电压1V
func TestVCCS(t *testing.T) {
	netlist := `
	v1 [1,-1] [0,0,0,0,1]
	g1 [1,-1,2,-1] [1e-3]
	r1 [2,-1] [1000]
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

	// 控制电压 = 1V, gm = 1e-3, 输出电流 = 1mA, 输出电压 = 1mA * 1kΩ = 1V
	// 原始节点 2 被压缩为紧凑索引 1
	vout := con.GetNodeVoltage(1)
	if math.Abs(vout-1.0) > 1e-3 {
		t.Errorf("输出电压不正确: 期望 1.0V, 实际 %.6fV", vout)
	}
}
