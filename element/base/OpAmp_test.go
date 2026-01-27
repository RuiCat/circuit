package base

import (
	"circuit/element/time"
	"circuit/load"
	"math"
	"testing"
)

func TestOpAmp(t *testing.T) {
	// 创建同相放大器电路：增益 = 1 + R2/R1
	netlist := `
	v1 [0,-1] [0,0,0,0,1]
	r1 [1,-1] [1000]
	r2 [1,2] [2000]
	opamp1 [0,1,2] [15,-15,1e5]
	`
	con, err := load.LoadString(netlist)
	if err != nil {
		t.Fatalf("加载上下文失败: %s", err)
	}
	con.Time, err = time.NewTimeMNA(0.001)
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	// 求解
	if err := time.TransientSimulation(con, func(voltages []float64) {
	}); err != nil {
		t.Fatalf("运放仿真失败（可能模型问题）: %s", err)
	}

	// 验证输出电压
	// 同相放大器增益 = 1 + R2/R1 = 1 + 2000/1000 = 3
	// 输入电压 = 1V，输出电压 = 3V
	outputVoltage := con.GetNodeVoltage(2)
	expectedOutputVoltage := 3.0
	// 允许较大误差，因为运放模型有非线性
	if math.Abs(outputVoltage-expectedOutputVoltage) > 0.5 {
		t.Errorf("运放输出电压不正确: 期望约 %vV, 实际 %vV", expectedOutputVoltage, outputVoltage)
	}

	// 验证输入电压
	inputVoltage := con.GetNodeVoltage(0)
	expectedInputVoltage := 1.0
	if math.Abs(inputVoltage-expectedInputVoltage) > 0.1 {
		t.Errorf("输入电压不正确: 期望 %vV, 实际 %vV", expectedInputVoltage, inputVoltage)
	}

	// 验证运放反相输入电压（虚短）
	invertingInputVoltage := con.GetNodeVoltage(1)
	// 理想运放：Vp ≈ Vn
	if math.Abs(invertingInputVoltage-inputVoltage) > 0.1 {
		t.Errorf("运放虚短特性不正确: Vp=%vV, Vn=%vV, 差值过大", inputVoltage, invertingInputVoltage)
	}

}
