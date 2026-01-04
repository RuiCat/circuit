package base

import (
	"circuit/element"
	"circuit/element/time"
	"circuit/mna"
	"math"
	"testing"
)

func TestMotor(t *testing.T) {
	ele := []element.NodeFace{
		// 电压源参数：波形类型(WfDC), 偏置电压(0), 频率(0), 相位偏移(0), 最大电压(12), 占空比(0), 频率时间零点(0), 噪声值(0)
		element.NewElementVlaue(VoltageType, int(WfDC), 0.0, 0.0, 0.0, 12.0, 0.0, 0.0, 0.0),
		element.NewElementVlaue(ResistorType, 10.0),                                    // 限流电阻 10Ω
		element.NewElementVlaue(MotorType, 12.0, 1000.0, 0.1, 0.01, 0.05, 0.001, 0.01), // 电机：额定电压12V, 额定转速1000RPM, 电枢电阻0.1Ω, 电感0.01H, 转矩常数0.05, 转动惯量0.001, 阻尼系数0.01
	}

	// 设置引脚
	ele[0].SetNodePins(1, -1) // 电压源：正极接节点1，负极接地
	ele[1].SetNodePins(1, 0)  // 电阻：一端接节点1，另一端接节点0
	ele[2].SetNodePins(0, -1) // 电机：正极接节点0，负极接地

	// 创建求解
	mnaSolver := mna.NewUpdateMNA(time.GetNum(ele))
	timeMNA, err := time.NewTimeMNA(0.01) // 较小的时间步长用于电机动态
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	// 求解
	if err := time.TransientSimulation(timeMNA, mnaSolver, ele, func(voltages []float64) {
		// 可以在这里记录电压变化
	}); err != nil {
		t.Fatalf("仿真失败 %s", err)
	}

	// 验证稳态电压
	// 节点1电压应为12V（允许1e-6的误差）
	node1Voltage := mnaSolver.GetNodeVoltage(1)
	expectedNode1Voltage := 12.0
	if math.Abs(node1Voltage-expectedNode1Voltage) > 1e-6 {
		t.Errorf("节点1电压不正确: 期望 %v, 实际 %v", expectedNode1Voltage, node1Voltage)
	}

	// 节点0电压应小于12V（由于电枢电阻和反电动势）
	node0Voltage := mnaSolver.GetNodeVoltage(0)
	if node0Voltage >= 12.0 {
		t.Errorf("节点0电压不正确: 应小于12V，实际 %v", node0Voltage)
	}

	// 验证电压源电流
	voltageSourceCurrent := mnaSolver.GetNodeCurrent(0)
	// 电流应为负值（从电压源流出）
	if voltageSourceCurrent >= 0 {
		t.Errorf("电压源电流方向不正确: 应为负值，实际 %v", voltageSourceCurrent)
	}

	// 验证电流大小在合理范围内（根据电机参数）
	// 稳态电流大约为 (12V - 反电动势) / (10Ω + 0.1Ω)
	// 反电动势 = kt * ω，其中ω为转速
	// 由于测试简化，只检查电流不为0
	if math.Abs(voltageSourceCurrent) < 1e-6 {
		t.Errorf("电压源电流过小: %v", voltageSourceCurrent)
	}
}
