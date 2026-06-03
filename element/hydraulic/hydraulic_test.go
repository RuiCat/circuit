package hydraulic

import (
	_ "circuit/element/sensor"
	_ "circuit/element/source"
	"circuit/element/time"
	"circuit/load"
	"math"
	"testing"
)

func TestHydraulicOrifice(t *testing.T) {
	netlist := `
	hp1 [0,-1] [10000000]
	hr1 [0,1] [1e10]
	hr2 [1,-1] [1e10]
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
	pmid := con.GetNodeVoltage(1)
	expected := 5000000.0
	if math.Abs(pmid-expected) > 10000.0 {
		t.Errorf("分压不正确: 期望 %.1f Pa, 实际 %.6f Pa", expected, pmid)
	}
}

func TestHydraulicAccumulator(t *testing.T) {
	netlist := `
	hp1 [0,-1] [5000000]
	hr1 [0,1] [1e1]
	ha1 [1] [1e-3]
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
	pfinal := con.GetNodeVoltage(1)
	expected := 5000000.0
	if math.Abs(pfinal-expected) > 10000.0 {
		t.Errorf("蓄能器最终压力不正确: 期望 %.1f Pa, 实际 %.6f Pa", expected, pfinal)
	}
}

func TestEHConverter(t *testing.T) {
	netlist := `
	v1 [0,-1] [0,0,0,0,5]
	eh1 [0,-1,1,-1] [1000000]
	hr1 [1,-1] [1e10]
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
	pout := con.GetNodeVoltage(1)
	expected := 5000000.0
	if math.Abs(pout-expected) > 10000.0 {
		t.Errorf("输出液压不正确: 期望 %.1f Pa, 实际 %.6f Pa", expected, pout)
	}
}
