package base

import (
	"bufio"
	"circuit/element"
	"circuit/element/time"
	"math"
	"strings"
	"testing"
)

func TestInductor(t *testing.T) {
	netlist := `
	v1 1 -1
	r1 1 0 100
	l1 0 -1 0.001
	`
	scanner := bufio.NewScanner(strings.NewReader(netlist))
	con, err := element.LoadContext(scanner)
	if err != nil {
		t.Fatalf("加载上下文失败: %s", err)
	}
	con.Time, err = time.NewTimeMNA(1e-6)
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	// 求解
	if err := time.TransientSimulation(con, func(voltages []float64) {
		// 可以在这里记录电压变化
	}); err != nil {
		t.Fatalf("仿真失败 %s", err)
	}

	// 验证稳态电压（在瞬态仿真中，电感有有限阻抗）
	// 节点1电压应为5V（允许1e-6的误差）
	node1Voltage := con.GetNodeVoltage(1)
	expectedVoltage := 5.0
	if math.Abs(node1Voltage-expectedVoltage) > 1e-6 {
		t.Errorf("节点1电压不正确: 期望 %v, 实际 %v", expectedVoltage, node1Voltage)
	}

	// 节点0电压应为约4.7619V（电感有限阻抗下的分压）
	node0Voltage := con.GetNodeVoltage(0)
	expectedNode0Voltage := 4.761904761904762 // 5 * (2000/2100)
	if math.Abs(node0Voltage-expectedNode0Voltage) > 1e-6 {
		t.Errorf("节点0电压不正确: 期望 %v, 实际 %v", expectedNode0Voltage, node0Voltage)
	}

	// 电压源电流应为约-0.00238095A
	voltageSourceCurrent := con.GetVoltageSourceCurrent(0)
	expectedCurrent := -0.002380952380952381 // -5/2100
	if math.Abs(voltageSourceCurrent-expectedCurrent) > 1e-6 {
		t.Errorf("电压源电流不正确: 期望 %v, 实际 %v", expectedCurrent, voltageSourceCurrent)
	}
}
