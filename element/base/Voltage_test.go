package base

import (
	"bufio"
	"circuit/element"
	"circuit/element/time"
	"math"
	"strings"
	"testing"
)

func TestVoltageSource(t *testing.T) {
	// 测试直流电压源
	t.Run("DCVoltage", func(t *testing.T) {
		netlist := `
	v1 0 -1 0 0.0 0.0 0.0 5.0 0.0 0.0 0.0
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

		if err := time.TransientSimulation(con, func(voltages []float64) {}); err != nil {
			t.Fatalf("仿真失败 %s", err)
		}

		node0Voltage := con.GetNodeVoltage(0)
		expectedVoltage := 5.0
		if math.Abs(node0Voltage-expectedVoltage) > 1e-6 {
			t.Errorf("直流电压源输出电压不正确: 期望 %vV, 实际 %vV", expectedVoltage, node0Voltage)
		}

		current := con.GetVoltageSourceCurrent(0)
		expectedCurrent := -0.05 // 5V / 100Ω = 0.05A，方向为负
		if math.Abs(current-expectedCurrent) > 1e-6 {
			t.Errorf("直流电压源输出电流不正确: 期望 %vA, 实际 %vA", expectedCurrent, current)
		}
	})

	// 测试交流电压源
	t.Run("ACVoltage", func(t *testing.T) {

		netlist := `
	v1 0 -1 1 0.0 1.0 0.0 5.0 0.0 0.0 0.0
	r1 0 -1 100
	`
		scanner := bufio.NewScanner(strings.NewReader(netlist))
		con, err := element.LoadContext(scanner)
		if err != nil {
			t.Fatalf("加载上下文失败: %s", err)
		}
		con.Time, err = time.NewTimeMNA(0.01) // 较小时间步长用于交流
		if err != nil {
			t.Fatalf("创建仿真时间失败 %s", err)
		}

		if err := time.TransientSimulation(con, func(voltages []float64) {}); err != nil {
			t.Fatalf("仿真失败 %s", err)
		}

		// 交流电压源在某个时刻的输出电压应在-5V到5V之间
		node0Voltage := con.GetNodeVoltage(0)
		if math.Abs(node0Voltage) > 5.1 {
			t.Errorf("交流电压源输出电压超出范围: 实际 %vV，应在±5V之间", node0Voltage)
		}

		// 电流也应在合理范围内
		current := con.GetVoltageSourceCurrent(0)
		expectedMaxCurrent := 0.05 // 5V / 100Ω = 0.05A
		if math.Abs(current) > expectedMaxCurrent*1.1 {
			t.Errorf("交流电压源输出电流超出范围: 实际 %vA，最大应为 %vA", current, expectedMaxCurrent)
		}
	})

	// 测试方波电压源
	t.Run("SquareWaveVoltage", func(t *testing.T) {
		netlist := `
	v1 0 -1 2 0.0 1.0 0.0 5.0 5.0 0.0 0.0
	r1 0 -1 100
	`
		scanner := bufio.NewScanner(strings.NewReader(netlist))
		con, err := element.LoadContext(scanner)
		if err != nil {
			t.Fatalf("加载上下文失败: %s", err)
		}
		con.Time, err = time.NewTimeMNA(0.01)
		if err != nil {
			t.Fatalf("创建仿真时间失败 %s", err)
		}

		if err := time.TransientSimulation(con, func(voltages []float64) {}); err != nil {
			t.Fatalf("仿真失败 %s", err)
		}

		// 方波电压应在±5V之间
		node0Voltage := con.GetNodeVoltage(0)
		if math.Abs(node0Voltage) > 5.1 && math.Abs(node0Voltage) < 4.9 {
			t.Errorf("方波电压源输出电压异常: 实际 %vV，应接近±5V", node0Voltage)
		}
	})
}
