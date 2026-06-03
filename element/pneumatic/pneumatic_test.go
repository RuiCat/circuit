package pneumatic

import (
	"circuit/element/time"
	"circuit/load"
	"math"
	"testing"
)

func TestPneumaticOrifice(t *testing.T) {
	netlist := `
	ps1 [0,-1] [101325]
	pr1 [0,1] [1e8]
	pr2 [1,-1] [1e8]
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
	expected := 50662.5
	if math.Abs(pmid-expected) > 100.0 {
		t.Errorf("分压不正确: 期望 %.1f Pa, 实际 %.6f Pa", expected, pmid)
	}
}

func TestPneumaticChamber(t *testing.T) {
	netlist := `
	ps1 [0,-1] [101325]
	pr1 [0,1] [1e1]
	pc1 [1] [1e-3]
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
	expected := 101325.0
	if math.Abs(pfinal-expected) > 200.0 {
		t.Errorf("气容最终压力不正确: 期望 %.1f Pa, 实际 %.6f Pa", expected, pfinal)
	}
}

func TestPneumaticCheckValve(t *testing.T) {
	netlist := `
	ps1 [0,-1] [200000]
	pcv1 [0,1] [1e5, 1e15, 0]
	pr1 [1,-1] [1e5]
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
	p2 := con.GetNodeVoltage(1)
	if math.Abs(p2) < 1e-12 {
		t.Errorf("单向阀节点电压为零，未正确连接: 实际 %.6e Pa", p2)
	}
}
