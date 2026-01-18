package base

import (
	"bufio"
	"circuit/element"
	"circuit/element/time"
	"math"
	"strings"
	"testing"
)

func TestResistor(t *testing.T) {
	// 使用 LoadNetlistFromString 加载网表
	netlist := `
	v1 0 -1
	r1 0 -1 100
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

	}); err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}
	// 验证求解状态
	// 节点0电压应为5V（允许1e-6的误差）
	node0Voltage := con.GetNodeVoltage(0)
	expectedVoltage := 5.0
	if math.Abs(node0Voltage-expectedVoltage) > 1e-6 {
		t.Errorf("节点0电压不正确: 期望 %v, 实际 %v", expectedVoltage, node0Voltage)
	}

	// 电压源电流应为-0.05A（负号表示电流方向）
	voltageSourceCurrent := con.GetVoltageSourceCurrent(0)
	expectedCurrent := -0.05
	if math.Abs(voltageSourceCurrent-expectedCurrent) > 1e-6 {
		t.Errorf("电压源电流不正确: 期望 %v, 实际 %v", expectedCurrent, voltageSourceCurrent)
	}

	// 电阻两端电压应为5V
	resistorVoltage := node0Voltage - 0 // 另一端接地
	if math.Abs(resistorVoltage-expectedVoltage) > 1e-6 {
		t.Errorf("电阻电压不正确: 期望 %v, 实际 %v", expectedVoltage, resistorVoltage)
	}
}
