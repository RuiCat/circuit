package logic

import (
	_ "circuit/element/passive"
	_ "circuit/element/source"
	"circuit/element/time"
	"circuit/load"
	"math"
	"testing"
)

func TestComparator(t *testing.T) {
	netlist := `
	v1 [1,0] [0,0,0,0,5]
	v2 [2,0] [0,0,0,0,3]
	cmp1 [1,2,3] [5,0]
	r1 [3,0] [10000]
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
	vout := con.GetNodeVoltage(3)
	if math.Abs(vout-5.0) > 0.1 {
		t.Errorf("比较器输出不正确: 期望 5.0V, 实际 %.6fV", vout)
	}
}

func TestSchmittTrigger(t *testing.T) {
	netlist := `
	v1 [1,0] [0,0,0,0,5]
	st1 [1,2] [5,0,3,2]
	r1 [2,0] [10000]
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
	vout := con.GetNodeVoltage(2)
	if math.Abs(vout-5.0) > 0.1 {
		t.Errorf("施密特触发器输出不正确: 期望 5.0V, 实际 %.6fV", vout)
	}
}

func TestSRLatch(t *testing.T) {
	netlist := `
	v1 [1,0] [0,0,0,0,5]
	sr1 [1,0,2,3] [5]
	r1 [2,0] [10000]
	r2 [3,0] [10000]
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
	q := con.GetNodeVoltage(2)
	if math.Abs(q-5.0) > 0.1 {
		t.Errorf("SR锁存器Q输出不正确: 期望 5.0V, 实际 %.6fV", q)
	}
}
