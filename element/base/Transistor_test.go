package base

import (
	"bufio"
	"circuit/element"
	"circuit/element/time"
	"math"
	"strings"
	"testing"
)

func TestTransistorCircuit(t *testing.T) {
	// 简单NPN开关电路测试
	vcc := 5.0
	vIn := 5.0       // 用于打开晶体管的输入电压
	rbVal := 10000.0 // 10kOhm 基极电阻
	rcVal := 1000.0  // 1kOhm 集电极电阻

	netlist := `
	v1 0 -1 5.0
	v2 1 -1 5.0
	r1 1 2 10000.0
	r2 0 3 1000.0
	q1 2 3 -1 100.0
	`
	scanner := bufio.NewScanner(strings.NewReader(netlist))
	con, err := element.LoadContext(scanner)
	if err != nil {
		t.Fatalf("加载上下文失败: %s", err)
	}
	timeMNA, err := time.NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建仿真时间失败: %s", err)
	}

	// 运行瞬态仿真
	if err := time.TransientSimulation(timeMNA, con, func(f []float64) {}); err != nil {
		t.Fatalf("瞬态仿真失败: %s", err)
	}

	// --- 验证 ---
	// 晶体管应处于饱和模式。
	baseVoltage := con.GetNodeVoltage(2)
	collectorVoltage := con.GetNodeVoltage(3)

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
	transistor := con.Nodelist[4]
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
