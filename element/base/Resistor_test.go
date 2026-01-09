package base

import (
	"circuit/element"
	"circuit/element/time"
	"circuit/mna"
	"testing"
)

func TestResistor(t *testing.T) {
	// 使用 LoadNetlistFromString 加载网表
	netlist := `
	v1 0 -1
	r1 0 -1 100
	`
	ele, err := element.LoadNetlistFromString(netlist)
	if err != nil {
		t.Fatalf("LoadNetlistFromString 失败: %v", err)
	}

	// 创建求解
	mnaSolver := mna.NewUpdateMNA(time.GetNum(ele))
	timeMNA, err := time.NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	// 求解
	if err := time.TransientSimulation(timeMNA, mnaSolver, ele, func(voltages []float64) {

	}); err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}
	// 验证求解状态
	// 节点0电压应为5V（允许1e-6的误差）
	node0Voltage := mnaSolver.GetNodeVoltage(0)
	expectedVoltage := 5.0
	if abs(node0Voltage-expectedVoltage) > 1e-6 {
		t.Errorf("节点0电压不正确: 期望 %v, 实际 %v", expectedVoltage, node0Voltage)
	}

	// 电压源电流应为-0.05A（负号表示电流方向）
	voltageSourceCurrent := mnaSolver.GetNodeCurrent(0)
	expectedCurrent := -0.05
	if abs(voltageSourceCurrent-expectedCurrent) > 1e-6 {
		t.Errorf("电压源电流不正确: 期望 %v, 实际 %v", expectedCurrent, voltageSourceCurrent)
	}

	// 电阻两端电压应为5V
	resistorVoltage := node0Voltage - 0 // 另一端接地
	if abs(resistorVoltage-expectedVoltage) > 1e-6 {
		t.Errorf("电阻电压不正确: 期望 %v, 实际 %v", expectedVoltage, resistorVoltage)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
