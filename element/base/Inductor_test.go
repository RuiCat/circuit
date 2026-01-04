package base

import (
	"circuit/element"
	"circuit/element/time"
	"circuit/mna"
	"math"
	"testing"
)

func TestInductor(t *testing.T) {
	ele := []element.NodeFace{
		// 电压源参数：波形类型(WfDC), 偏置电压(0), 频率(0), 相位偏移(0), 最大电压(5), 占空比(0), 频率时间零点(0), 噪声值(0)
		element.NewElementVlaue(VoltageType, int(WfDC)),
		element.NewElementVlaue(ResistorType, 100.0), // 电阻 100R
		element.NewElementVlaue(InductorType, 0.001), // 电感 1mH
	}

	// 设置引脚
	ele[0].SetNodePins(1, -1) // 电压源：正极接节点1，负极接地
	ele[1].SetNodePins(1, 0)  // 电阻：一端接节点1，另一端接节点0
	ele[2].SetNodePins(0, -1) // 电感：一端接节点0，另一端接地

	// 创建求解
	mnaSolver := mna.NewUpdateMNA(time.GetNum(ele))
	timeMNA, err := time.NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	// 求解
	if err := time.TransientSimulation(timeMNA, mnaSolver, ele, func(voltages []float64) {
		// 可以在这里记录电压变化
	}); err != nil {
		t.Fatalf("仿真失败 %s", err)
	}

	// 验证稳态电压（在瞬态仿真中，电感有有限阻抗）
	// 节点1电压应为5V（允许1e-6的误差）
	node1Voltage := mnaSolver.GetNodeVoltage(1)
	expectedVoltage := 5.0
	if math.Abs(node1Voltage-expectedVoltage) > 1e-6 {
		t.Errorf("节点1电压不正确: 期望 %v, 实际 %v", expectedVoltage, node1Voltage)
	}

	// 节点0电压应为约4.7619V（电感有限阻抗下的分压）
	node0Voltage := mnaSolver.GetNodeVoltage(0)
	expectedNode0Voltage := 4.761904761904762 // 5 * (2000/2100)
	if math.Abs(node0Voltage-expectedNode0Voltage) > 1e-6 {
		t.Errorf("节点0电压不正确: 期望 %v, 实际 %v", expectedNode0Voltage, node0Voltage)
	}

	// 电压源电流应为约-0.00238095A
	voltageSourceCurrent := mnaSolver.GetNodeCurrent(0)
	expectedCurrent := -0.002380952380952381 // -5/2100
	if math.Abs(voltageSourceCurrent-expectedCurrent) > 1e-6 {
		t.Errorf("电压源电流不正确: 期望 %v, 实际 %v", expectedCurrent, voltageSourceCurrent)
	}
}
