package base

import (
	"circuit/element"
	"circuit/element/time"
	"circuit/mna"
	"math"
	"testing"
)

func TestTransistorCircuit(t *testing.T) {
	// 简单NPN开关电路测试
	vcc := 5.0
	vIn := 5.0       // 用于打开晶体管的输入电压
	rbVal := 10000.0 // 10kOhm 基极电阻
	rcVal := 1000.0  // 1kOhm 集电极电阻
	beta := 100.0

	ele := []element.NodeFace{
		element.NewElementValue(VoltageType, int(WfDC), 0.0, nil, nil, vcc), // VCC - VS ID 0
		element.NewElementValue(VoltageType, int(WfDC), 0.0, nil, nil, vIn), // 输入电压 - VS ID 1
		element.NewElementValue(ResistorType, rbVal),
		element.NewElementValue(ResistorType, rcVal),
		element.NewElementValue(TransistorType, false, beta), // NPN晶体管 - VS ID 2, 3, 4
	}

	// 引脚配置
	// 节点0: VCC节点
	// 节点1: 输入电压节点
	// 节点2: 基极节点
	// 节点3: 集电极节点
	// 接地: -1
	ele[0].SetNodePins(0, -1) // VCC连接到节点0和地
	ele[1].SetNodePins(1, -1) // 输入电压连接到节点1和地
	ele[0].SetVoltSource(0, 0)
	ele[1].SetVoltSource(0, 1)
	ele[2].SetNodePins(1, 2)     // Rb位于输入（节点1）和基极（节点2）之间
	ele[3].SetNodePins(0, 3)     // Rc位于VCC（节点0）和集电极（节点3）之间
	ele[4].SetNodePins(2, 3, -1) // 晶体管：基极(2)，集电极(3)，发射极(地)

	// 创建求解器
	mnaSolver := mna.NewUpdateMNA(time.GetNum(ele))
	timeMNA, err := time.NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建仿真时间失败: %s", err)
	}

	// 运行瞬态仿真
	if err := time.TransientSimulation(timeMNA, mnaSolver, ele, func(f []float64) {}); err != nil {
		t.Fatalf("瞬态仿真失败: %s", err)
	}

	// --- 验证 ---
	// 晶体管应处于饱和模式。
	baseVoltage := mnaSolver.GetNodeVoltage(2)
	collectorVoltage := mnaSolver.GetNodeVoltage(3)

	// 对于硅晶体管，Vbe应约为0.7V。
	// 我们期望基极电压接近此值。
	expectedBaseVoltage := 0.7
	if math.Abs(baseVoltage-expectedBaseVoltage) > 0.2 { // 容差较大
		t.Errorf("基极电压不正确：期望值约为 %v，实际值为 %v", expectedBaseVoltage, baseVoltage)
	}

	// 在饱和状态下，Vce非常低（Vce_sat），通常约为0.2V。
	expectedCollectorVoltage := 0.2
	if math.Abs(collectorVoltage-expectedCollectorVoltage) > 0.2 { // 容差较大
		t.Errorf("集电极电压不正确：期望值约为 %v，实际值为 %v", expectedCollectorVoltage, collectorVoltage)
	}

	// 验证电流
	// Ib = (V_in - V_be) / Rb = (5 - 0.7) / 10k = 0.43mA
	transistor := ele[4]
	ib := transistor.GetFloat64(8)
	expectedIb := (vIn - baseVoltage) / rbVal
	if math.Abs(ib-expectedIb)/expectedIb > 0.1 { // 10% 容差
		t.Errorf("基极电流不正确：期望值约为 %.3emA，实际值为 %.3emA", expectedIb*1e3, ib*1e3)
	}

	// Ic = (Vcc - Vce_sat) / Rc = (5 - 0.2) / 1k = 4.8mA
	ic := transistor.GetFloat64(6)
	expectedIc := (vcc - collectorVoltage) / rcVal
	if math.Abs(ic-expectedIc)/expectedIc > 0.1 { // 10% 容差
		t.Errorf("集电极电流不正确：期望值约为 %.3fmA，实际值为 %.3fmA", expectedIc*1e3, ic*1e3)
	}

}
