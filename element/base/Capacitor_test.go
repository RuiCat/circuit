package base

import (
	"bufio"
	"circuit/element"
	"circuit/element/time"
	"math"
	"strings"
	"testing"
)

func TestCapacitor(t *testing.T) {
	netlist := `
	v1 1 -1
	r1 1 0 100
	c1 0 -1 1e-6
	`
	scanner := bufio.NewScanner(strings.NewReader(netlist))
	con, err := element.LoadContext(scanner)
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

	// 验证稳态电压（电容在直流稳态下相当于开路）
	// 节点1电压应为5V（允许0.1的误差）
	node1Voltage := con.GetNodeVoltage(1)
	expectedVoltage := 5.0
	if math.Abs(node1Voltage-expectedVoltage) > 0.1 {
		t.Errorf("节点1电压不正确: 期望 %v, 实际 %v", expectedVoltage, node1Voltage)
	}

	// 节点0电压应为5V（电容充电后，电容开路，节点0通过电阻连接到节点1）
	node0Voltage := con.GetNodeVoltage(0)
	expectedNode0Voltage := 5.0
	if math.Abs(node0Voltage-expectedNode0Voltage) > 0.1 {
		t.Errorf("节点0电压不正确: 期望 %v, 实际 %v", expectedNode0Voltage, node0Voltage)
	}

	// 电压源电流应为0A（电容开路，没有电流）
	voltageSourceCurrent := con.GetVoltageSourceCurrent(0)
	expectedCurrent := 0.0
	if math.Abs(voltageSourceCurrent-expectedCurrent) > 0.1 {
		t.Errorf("电压源电流不正确: 期望 %v, 实际 %v", expectedCurrent, voltageSourceCurrent)
	}
}
