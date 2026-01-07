package base

import (
	"circuit/element"
	"circuit/element/time"
	"circuit/mna"
	"math"
	"testing"
)

func TestCapacitor(t *testing.T) {
	ele := []element.NodeFace{
		// 电压源参数：波形类型(WfDC), 偏置电压(0), 频率(0), 相位偏移(0), 最大电压(5), 占空比(0), 频率时间零点(0), 噪声值(0)
		element.NewElementValue(VoltageType, int(WfDC)),
		element.NewElementValue(ResistorType, 100.0), // 电阻 100R
		element.NewElementValue(CapacitorType, 1e-6), // 电容 1μF
	}

	// 设置引脚
	ele[0].SetNodePins(1, -1) // 电压源：正极接节点1，负极接地
	ele[1].SetNodePins(1, 0)  // 电阻：一端接节点1，另一端接节点0
	ele[2].SetNodePins(0, -1) // 电容：一端接节点0，另一端接地

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

	// 验证稳态电压（电容在直流稳态下相当于开路）
	// 节点1电压应为5V（允许0.1的误差）
	node1Voltage := mnaSolver.GetNodeVoltage(1)
	expectedVoltage := 5.0
	if math.Abs(node1Voltage-expectedVoltage) > 0.1 {
		t.Errorf("节点1电压不正确: 期望 %v, 实际 %v", expectedVoltage, node1Voltage)
	}

	// 节点0电压应为5V（电容充电后，电容开路，节点0通过电阻连接到节点1）
	node0Voltage := mnaSolver.GetNodeVoltage(0)
	expectedNode0Voltage := 5.0
	if math.Abs(node0Voltage-expectedNode0Voltage) > 0.1 {
		t.Errorf("节点0电压不正确: 期望 %v, 实际 %v", expectedNode0Voltage, node0Voltage)
	}

	// 电压源电流应为0A（电容开路，没有电流）
	voltageSourceCurrent := mnaSolver.GetNodeCurrent(0)
	expectedCurrent := 0.0
	if math.Abs(voltageSourceCurrent-expectedCurrent) > 0.1 {
		t.Errorf("电压源电流不正确: 期望 %v, 实际 %v", expectedCurrent, voltageSourceCurrent)
	}
}
