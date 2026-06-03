package semiconductor

import (
	_ "circuit/element/passive"
	_ "circuit/element/source"
	"circuit/element/time"
	"circuit/load"
	"math"
	"testing"
)

func TestZenerDiode(t *testing.T) {
	netlist := `
	v1 [0,-1] [0,0,0,0,15]
	r1 [0,1] [1000]
	z1 [-1,1] [1e-14, 5.1, 1, 0.1]
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
	v2 := con.GetNodeVoltage(1)
	if math.Abs(v2-5.1) > 0.5 {
		t.Errorf("稳压管钳位电压不正确: 期望约5.1V, 实际 %.6fV", v2)
	}
}

func TestLED(t *testing.T) {
	netlist := `
	v1 [0,-1] [0,0,0,0,5]
	r1 [0,1] [330]
	led1 [1,-1] [1e-18, 0, 1.5, 5]
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
	v2 := con.GetNodeVoltage(1)
	if v2 < 1.0 || v2 > 3.5 {
		t.Errorf("LED压降不正确: 期望1.0~3.5V, 实际 %.6fV", v2)
	}
}
