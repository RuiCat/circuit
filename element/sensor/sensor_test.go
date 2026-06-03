package sensor

import (
	_ "circuit/element/passive"
	_ "circuit/element/pneumatic"
	_ "circuit/element/source"
	"circuit/element/time"
	"circuit/load"
	"math"
	"testing"
)

func TestPressureSensor(t *testing.T) {
	netlist := `
	ps1 [1,-1] [200000]
	pr1 [1,2] [1e8]
	pr2 [2,-1] [1e8]
	psens1 [2,-1,3,-1] [1e-4, 0, 0]
	r1 [3,-1] [10000]
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
	p2 := con.GetRawNodeVoltage(2)
	expectedP := 100000.0
	if math.Abs(p2-expectedP) > 100.0 {
		t.Errorf("气压不正确: 期望 %.0f Pa, 实际 %.6f Pa", expectedP, p2)
	}
	vout := con.GetRawNodeVoltage(3)
	expectedV := 10.0
	if math.Abs(vout-expectedV) > 0.1 {
		t.Errorf("传感器输出电压不正确: 期望 %.1fV, 实际 %.6fV", expectedV, vout)
	}
}

func TestEPConverter(t *testing.T) {
	netlist := `
	v1 [1,-1] [0,0,0,0,5]
	ep1 [1,-1,2,-1] [100000]
	pr1 [2,-1] [1e9]
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
	pout := con.GetRawNodeVoltage(2)
	expectedP := 500000.0
	if math.Abs(pout-expectedP) > 1000.0 {
		t.Errorf("输出气压不正确: 期望 %.0f Pa, 实际 %.6f Pa", expectedP, pout)
	}
}

func TestCurrentSensor(t *testing.T) {
	netlist := `
	v1 [1,-1] [0,0,0,0,5]
	r1 [1,2] [1000]
	cs1 [2,-1,3,-1] [100, 0]
	r2 [3,-1] [10000]
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
	vout := con.GetRawNodeVoltage(3)
	expectedV := 0.5
	if math.Abs(vout-expectedV) > 0.01 {
		t.Errorf("传感器输出电压不正确: 期望 %.3fV, 实际 %.6fV", expectedV, vout)
	}
}
