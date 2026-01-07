package base

import (
	"circuit/element"
	"circuit/element/time"
	"circuit/mna"
	"math"
	"testing"
)

func TestVCVS(t *testing.T) {

	ele := []element.NodeFace{
		// 直流电压源：5V
		element.NewElementValue(VoltageType, int(WfDC), 0.0, 0.0, 0.0, 5.0, 0.0, 0.0, 0.0),

		element.NewElementValue(VCVSType, 1.0),       // 控制电压源，增益为1.0
		element.NewElementValue(ResistorType, 100.0), // 负载电阻 100Ω

	}

	// 设置引脚
	ele[0].SetNodePins(0, -1) // 电压源：正极接节点0，负极接地

	ele[1].SetNodePins(0, -1, 1, -1) // VCVS：控制正接节点0，控制负接地，输出正接节点1，输出负接地
	ele[1].SetVoltSource(0, 1)       // 设置电压源ID

	ele[2].SetNodePins(1, -1) // 电阻：一端接节点1，另一端接地

	mnaSolver := mna.NewUpdateMNA(time.GetNum(ele))
	timeMNA, err := time.NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	if err := time.TransientSimulation(timeMNA, mnaSolver, ele, func(voltages []float64) {
		// 可以在这里记录电压变化，但测试中不需要
	}); err != nil {
		t.Fatalf("仿真失败 %s", err)
	}

	// 验证节点电压
	// 节点0电压应为5V（来自直流电压源）
	node0Voltage := mnaSolver.GetNodeVoltage(0)
	expectedNode0Voltage := 5.0
	if math.Abs(node0Voltage-expectedNode0Voltage) > 1e-6 {
		t.Errorf("节点0电压不正确: 期望 %vV, 实际 %vV", expectedNode0Voltage, node0Voltage)
	}

	// 节点1电压应为5V（VCVS增益为1.0，输出电压等于输入电压）
	node1Voltage := mnaSolver.GetNodeVoltage(1)
	expectedNode1Voltage := 5.0
	if math.Abs(node1Voltage-expectedNode1Voltage) > 1e-6 {
		t.Errorf("节点1电压不正确: 期望 %vV, 实际 %vV", expectedNode1Voltage, node1Voltage)
	}

	// 验证电压源电流
	// 电压源电流应为-0.05A（5V/100Ω = 0.05A，方向为负）
	voltageSourceCurrent := mnaSolver.GetNodeCurrent(1)
	expectedCurrent := -0.05
	if math.Abs(voltageSourceCurrent-expectedCurrent) > 1e-6 {
		t.Errorf("电压源电流不正确: 期望 %vA, 实际 %vA", expectedCurrent, voltageSourceCurrent)
	}

	// 验证VCVS行为：输出电压应等于增益乘以输入电压
	// V_out = Gain * V_in = 1.0 * 5V = 5V
	// 已经通过节点1电压验证
}
