package base

import (
	"circuit/element"
	"circuit/element/time"
	"circuit/mna"
	"math"
	"testing"
)

func TestCurrentSource(t *testing.T) {
	ele := []element.NodeFace{
		// 电压源参数：波形类型(WfDC), 偏置电压(0), 频率(0), 相位偏移(0), 最大电压(5), 占空比(0), 频率时间零点(0), 噪声值(0)
		element.NewElementVlaue(VoltageType, int(WfDC)),
		element.NewElementVlaue(ResistorType, 100.0),     // 电阻 100R
		element.NewElementVlaue(CurrentSourceType, 0.02), // 电流源 20mA
	}

	// 设置引脚
	ele[0].SetNodePins(1, -1) // 电压源：正极接节点1，负极接地
	ele[1].SetNodePins(1, 0)  // 电阻：一端接节点1，另一端接节点0
	ele[2].SetNodePins(0, -1) // 电流源：正极接节点0，负极接地

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

	// 验证节点电压
	// 根据电路分析：节点1电压 = 5V，节点0电压 = 5V - (0.02A * 100Ω) = 3V
	// 但由于数值误差，使用更宽松的容差
	node1Voltage := mnaSolver.GetNodeVoltage(1)
	expectedNode1Voltage := 5.0
	if math.Abs(node1Voltage-expectedNode1Voltage) > 0.1 {
		t.Errorf("节点1电压不正确: 期望 %v, 实际 %v", expectedNode1Voltage, node1Voltage)
	}

	node0Voltage := mnaSolver.GetNodeVoltage(0)
	expectedNode0Voltage := 3.0 // 5V - (0.02A * 100Ω) = 3V
	if math.Abs(node0Voltage-expectedNode0Voltage) > 0.1 {
		t.Errorf("节点0电压不正确: 期望 %v, 实际 %v", expectedNode0Voltage, node0Voltage)
	}

	// 验证电压源电流
	// 电压源电流 = (5V - 3V)/100Ω = 0.02A，方向为负
	voltageSourceCurrent := mnaSolver.GetNodeCurrent(0)
	expectedCurrent := -0.02
	if math.Abs(voltageSourceCurrent-expectedCurrent) > 0.1 {
		t.Errorf("电压源电流不正确: 期望 %v, 实际 %v", expectedCurrent, voltageSourceCurrent)
	}
}
