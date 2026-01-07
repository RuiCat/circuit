package base

import (
	"circuit/element"
	"circuit/element/time"
	"circuit/mna"
	"math"
	"testing"
)

func TestSwitch(t *testing.T) {
	// 测试开关在导通和关断状态下的行为
	ele := []element.NodeFace{
		// 电压源参数：波形类型(WfDC), 偏置电压(0), 频率(0), 相位偏移(0), 最大电压(5), 占空比(0), 频率时间零点(0), 噪声值(0)
		element.NewElementValue(VoltageType, int(WfDC)),
		element.NewElementValue(ResistorType, 100.0),       // 负载电阻 100R
		element.NewElementValue(SwitchType, 1, 1e-6, 1e12), // 开关：初始状态为导通，导通电阻1e-6Ω，关断电阻1e12Ω
	}

	// 设置引脚
	ele[0].SetNodePins(1, -1) // 电压源：正极接节点1，负极接地
	ele[1].SetNodePins(1, 0)  // 电阻：一端接节点1，另一端接节点0
	ele[2].SetNodePins(0, -1) // 开关：一端接节点0，另一端接地

	// 创建求解
	mnaSolver := mna.NewUpdateMNA(time.GetNum(ele))
	timeMNA, err := time.NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	// 求解（开关导通状态）
	if err := time.TransientSimulation(timeMNA, mnaSolver, ele, func(voltages []float64) {
		// 可以在这里记录电压变化
	}); err != nil {
		t.Fatalf("仿真失败 %s", err)
	}

	// 验证开关导通状态
	// 开关导通时，节点0应接地（电压接近0）
	node0Voltage := mnaSolver.GetNodeVoltage(0)
	expectedVoltageWhenOn := 0.0
	if math.Abs(node0Voltage-expectedVoltageWhenOn) > 1e-6 {
		t.Errorf("开关导通时节点0电压不正确: 期望 %vV, 实际 %vV", expectedVoltageWhenOn, node0Voltage)
	}

	// 节点1电压应为5V
	node1Voltage := mnaSolver.GetNodeVoltage(1)
	expectedNode1Voltage := 5.0
	if math.Abs(node1Voltage-expectedNode1Voltage) > 1e-6 {
		t.Errorf("节点1电压不正确: 期望 %vV, 实际 %vV", expectedNode1Voltage, node1Voltage)
	}

	// 电压源电流应为-0.05A（通过100Ω电阻）
	voltageSourceCurrent := mnaSolver.GetNodeCurrent(0)
	expectedCurrent := -0.05
	if math.Abs(voltageSourceCurrent-expectedCurrent) > 1e-6 {
		t.Errorf("电压源电流不正确: 期望 %vA, 实际 %vA", expectedCurrent, voltageSourceCurrent)
	}

	// 现在将开关切换到关断状态
	ele[2].SetInt(0, 0) // 设置开关状态为关断

	// 重新创建求解器进行第二次测试
	mnaSolver2 := mna.NewUpdateMNA(2, 1)
	timeMNA2, err := time.NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建仿真时间失败 %s", err)
	}

	// 求解（开关关断状态）
	if err := time.TransientSimulation(timeMNA2, mnaSolver2, ele, func(voltages []float64) {
		// 可以在这里记录电压变化
	}); err != nil {
		t.Fatalf("仿真失败 %s", err)
	}

	// 验证开关关断状态
	// 开关关断时，节点0电压应接近节点1电压（因为开路）
	node0VoltageOff := mnaSolver2.GetNodeVoltage(0)
	node1VoltageOff := mnaSolver2.GetNodeVoltage(1)
	// 由于关断电阻很大但不是无限大，节点0电压应接近但不等于节点1电压
	if math.Abs(node0VoltageOff-node1VoltageOff) > 0.1 {
		t.Errorf("开关关断时节点0电压不正确: 应接近节点1电压 %vV, 实际 %vV", node1VoltageOff, node0VoltageOff)
	}

	// 电压源电流应非常小（关断状态漏电流）
	voltageSourceCurrentOff := mnaSolver2.GetNodeCurrent(0)
	// 漏电流应远小于导通电流
	if math.Abs(voltageSourceCurrentOff) >= 1e-6 {
		t.Errorf("开关关断时漏电流过大: %vA", voltageSourceCurrentOff)
	}
}
