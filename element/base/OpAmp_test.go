package base

import (
	"circuit/element"
	"circuit/element/time"
	"circuit/mna"
	"math"
	"testing"
)

func TestOpAmp(t *testing.T) {
	// 创建同相放大器电路：增益 = 1 + R2/R1
	ele := []element.NodeFace{
		// 输入电压源：直流 1V
		element.NewElementValue(VoltageType, int(WfDC), 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0),
		// 电阻 R1 = 1kΩ
		element.NewElementValue(ResistorType, 1000.0),
		// 电阻 R2 = 2kΩ
		element.NewElementValue(ResistorType, 2000.0),
		// 运算放大器
		element.NewElementValue(OpAmpType, 15.0, -15.0, 1e5),
	}

	// 设置引脚
	// 电压源：正极接节点0（输入），负极接地
	ele[0].SetNodePins(0, -1)
	// R1：一端接节点1（运放反相输入），另一端接地
	ele[1].SetNodePins(1, -1)
	// R2：一端接节点1（运放反相输入），另一端接节点2（运放输出）
	ele[2].SetNodePins(1, 2)
	// 运放：Vp接节点0（同相输入），Vn接节点1（反相输入），Vout接节点2（输出）
	ele[3].SetNodePins(0, 1, 2)
	ele[3].SetVoltSource(0, 1)
	// 创建求解
	mnaSolver := mna.NewUpdateMNA(time.GetNum(ele))
	timeMNA, err := time.NewTimeMNA(0.001)
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	// 求解
	if err := time.TransientSimulation(timeMNA, mnaSolver, ele, func(voltages []float64) {
	}); err != nil {
		t.Fatalf("运放仿真失败（可能模型问题）: %s", err)
	}

	// 验证输出电压
	// 同相放大器增益 = 1 + R2/R1 = 1 + 2000/1000 = 3
	// 输入电压 = 1V，输出电压 = 3V
	outputVoltage := mnaSolver.GetNodeVoltage(2)
	expectedOutputVoltage := 3.0
	// 允许较大误差，因为运放模型有非线性
	if math.Abs(outputVoltage-expectedOutputVoltage) > 0.5 {
		t.Errorf("运放输出电压不正确: 期望约 %vV, 实际 %vV", expectedOutputVoltage, outputVoltage)
	}

	// 验证输入电压
	inputVoltage := mnaSolver.GetNodeVoltage(0)
	expectedInputVoltage := 1.0
	if math.Abs(inputVoltage-expectedInputVoltage) > 0.1 {
		t.Errorf("输入电压不正确: 期望 %vV, 实际 %vV", expectedInputVoltage, inputVoltage)
	}

	// 验证运放反相输入电压（虚短）
	invertingInputVoltage := mnaSolver.GetNodeVoltage(1)
	// 理想运放：Vp ≈ Vn
	if math.Abs(invertingInputVoltage-inputVoltage) > 0.1 {
		t.Errorf("运放虚短特性不正确: Vp=%vV, Vn=%vV, 差值过大", inputVoltage, invertingInputVoltage)
	}

}
