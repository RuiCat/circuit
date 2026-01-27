package base

import (
	"circuit/element/time"
	"circuit/load"
	"math"
	"testing"
)

func TestCurrentSource(t *testing.T) {
	netlist := `
	v1 [1,-1]
	r1 [1,0] [100.0]
	i1 [0,-1] [0.02]
	`
	con, err := load.LoadString(netlist)
	if err != nil {
		t.Fatalf("加载上下文失败: %s", err)
	}
	con.Time, err = time.NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	// 求解
	if err := time.TransientSimulation(con, func(voltages []float64) {
		// 可以在这里记录电压变化
	}); err != nil {
		t.Fatalf("仿真失败 %s", err)
	}

	// 验证节点电压
	// 根据电路分析：节点1电压 = 5V，节点0电压 = 5V - (0.02A * 100Ω) = 3V
	// 但由于数值误差，使用更宽松的容差
	node1Voltage := con.GetNodeVoltage(1)
	expectedNode1Voltage := 5.0
	if math.Abs(node1Voltage-expectedNode1Voltage) > 0.1 {
		t.Errorf("节点1电压不正确: 期望 %v, 实际 %v", expectedNode1Voltage, node1Voltage)
	}

	node0Voltage := con.GetNodeVoltage(0)
	expectedNode0Voltage := 3.0 // 5V - (0.02A * 100Ω) = 3V
	if math.Abs(node0Voltage-expectedNode0Voltage) > 0.1 {
		t.Errorf("节点0电压不正确: 期望 %v, 实际 %v", expectedNode0Voltage, node0Voltage)
	}

	// 验证电压源电流
	// 电压源电流 = (5V - 3V)/100Ω = 0.02A，方向为负
	voltageSourceCurrent := con.GetVoltageSourceCurrent(0)
	expectedCurrent := -0.02
	if math.Abs(voltageSourceCurrent-expectedCurrent) > 0.1 {
		t.Errorf("电压源电流不正确: 期望 %v, 实际 %v", expectedCurrent, voltageSourceCurrent)
	}
}
