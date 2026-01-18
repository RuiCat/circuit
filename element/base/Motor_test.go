package base

import (
	"bufio"
	"circuit/element"
	"circuit/element/time"
	"math"
	"strings"
	"testing"
)

func TestMotor(t *testing.T) {
	netlist := `
	v1 1 -1 0 0 0 0 12
	r1 1 0 10
	motor1 0 -1 12.0 1000.0 0.1 0.01 0.05 0.001 0.01
	`
	scanner := bufio.NewScanner(strings.NewReader(netlist))
	con, err := element.LoadContext(scanner)
	if err != nil {
		t.Fatalf("加载上下文失败: %s", err)
	}
	con.Time, err = time.NewTimeMNA(0.01) // 较小的时间步长用于电机动态
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	// 求解
	if err := time.TransientSimulation(con, func(voltages []float64) {
		// 可以在这里记录电压变化
	}); err != nil {
		t.Fatalf("仿真失败 %s", err)
	}

	// 验证稳态电压
	// 节点1电压应为12V（允许1e-6的误差）
	node1Voltage := con.GetNodeVoltage(1)
	expectedNode1Voltage := 12.0
	if math.Abs(node1Voltage-expectedNode1Voltage) > 1e-6 {
		t.Errorf("节点1电压不正确: 期望 %v, 实际 %v", expectedNode1Voltage, node1Voltage)
	}

	// 节点0电压应小于12V（由于电枢电阻和反电动势）
	node0Voltage := con.GetNodeVoltage(0)
	if node0Voltage >= 12.0 {
		t.Errorf("节点0电压不正确: 应小于12V，实际 %v", node0Voltage)
	}

	// 验证电压源电流
	voltageSourceCurrent := con.GetVoltageSourceCurrent(0)
	// 电流应为负值（从电压源流出）
	if voltageSourceCurrent >= 0 {
		t.Errorf("电压源电流方向不正确: 应为负值，实际 %v", voltageSourceCurrent)
	}

	// 验证电流大小在合理范围内（根据电机参数）
	// 稳态电流大约为 (12V - 反电动势) / (10Ω + 0.1Ω)
	// 反电动势 = kt * ω，其中ω为转速
	// 由于测试简化，只检查电流不为0
	if math.Abs(voltageSourceCurrent) < 1e-6 {
		t.Errorf("电压源电流过小: %v", voltageSourceCurrent)
	}
}
