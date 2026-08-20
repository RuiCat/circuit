package analysis

import (
	"math"
	"testing"
)

// TestSmallSignal_DiodeBias 二极管偏置 + 小信号注入:
// DC 工作点求 Vd → g_d = Is/Vt·e^(Vd/Vt) → 小信号分压可解析验证
func TestSmallSignal_DiodeBias(t *testing.T) {
	netlist := `
v1 [1,-1] [0,5,0,0,0]   # DC 5V 偏置
r1 [1,2] [1000]   # 偏置电阻
d1 [2,-1] [1e-12,0,1,0,300.15]   # 二极管 Is=1e-12, N=1, Rs=0
v2 [3,-1] [1,0,1000,0,0.1]   # AC 0.1V 信号源(工作点短路)
r2 [3,2] [100000]   # 信号注入电阻
`
	res, err := AnalyzeSmallSignal(netlist, 1000)
	if err != nil {
		t.Fatalf("AnalyzeSmallSignal 失败: %v", err)
	}
	// DC 工作点:偏置节点约 0.57V,源节点 5V / 0V
	if res.OperatingPoint[1] < 4.99 || res.OperatingPoint[1] > 5.01 {
		t.Errorf("DC 源节点工作点: 期望 5V, 实际 %g", res.OperatingPoint[1])
	}
	if res.OperatingPoint[3] > 1e-6 {
		t.Errorf("AC 源在工作点应短路为 0V, 实际 %g", res.OperatingPoint[3])
	}
	vd := res.OperatingPoint[2]
	if vd < 0.5 || vd > 0.65 {
		t.Errorf("二极管阳极工作点异常: %gV(期望 ≈0.57V)", vd)
	}
	// 小信号:理论 g_d = Is/Vt·e^(Vd/Vt),V_ac(2) = 0.1·(1/R2)/(1/R2+1/R1+g_d)
	const vt = 0.025865
	gd := 1e-12 / vt * math.Exp(vd/vt)
	vacExp := 0.1 * (1.0 / 100000) / (1.0/100000 + 1.0/1000 + gd)
	vac := res.NodeVoltages[2]
	if math.Abs(real(vac)-vacExp)/vacExp > 0.1 {
		t.Errorf("小信号响应: 期望 %.3e, 实际 %.3e(偏差 >10%%)", vacExp, real(vac))
	}
	if math.Abs(imag(vac)) > 1e-9 {
		t.Errorf("纯阻网络小信号应无相位, 实际虚部 %g", imag(vac))
	}
}

// TestSmallSignal_GateLow 逻辑门联动:非门输入高电平 → 输出 0V;小信号输出短路
func TestSmallSignal_GateLow(t *testing.T) {
	netlist := `
u1 [1,-1,2] [0,5]   # 非门
v1 [1,-1] [0,0,0,0,5]   # 输入 5V → 输出 0V
r1 [2,-1] [1000]   # 输出负载
`
	res, err := AnalyzeSmallSignal(netlist, 1000)
	if err != nil {
		t.Fatalf("AnalyzeSmallSignal 失败: %v", err)
	}
	if v := res.OperatingPoint[2]; v > 0.1 {
		t.Errorf("非门输出工作点应≈0V, 实际 %g", v)
	}
	if v := res.NodeVoltages[2]; math.Hypot(real(v), imag(v)) > 1e-6 {
		t.Errorf("门输出小信号应短路为 0, 实际 %v", v)
	}
}

// TestSmallSignal_GateHigh 逻辑门输出高电平工作点
func TestSmallSignal_GateHigh(t *testing.T) {
	netlist := `
u1 [1,-1,2] [0,5]   # 非门
v1 [1,-1] [0,0,0,0,0]   # 输入 0V → 输出 5V
r1 [2,-1] [1000]
`
	res, err := AnalyzeSmallSignal(netlist, 1000)
	if err != nil {
		t.Fatalf("AnalyzeSmallSignal 失败: %v", err)
	}
	if v := res.OperatingPoint[2]; v < 4.9 {
		t.Errorf("非门输出工作点应≈5V, 实际 %g", v)
	}
}

// TestSmallSignal_GateDriveDivider 门输出(交流地)参与信号分压网络
func TestSmallSignal_GateDriveDivider(t *testing.T) {
	netlist := `
u1 [1,-1,2] [0,5]
v1 [1,-1] [0,0,0,0,5]   # 输入高 → 门输出 0V
r1 [2,3] [1000]   # 门输出到节点3
v2 [4,-1] [1,0,1000,0,1]   # AC 1V 信号
r2 [4,3] [1000]
r3 [3,-1] [1000]   # 负载到地
`
	res, err := AnalyzeSmallSignal(netlist, 1000)
	if err != nil {
		t.Fatalf("AnalyzeSmallSignal 失败: %v", err)
	}
	// 小信号:门输出为交流地,节点3 = 1V·(1/1000)/(3/1000) = 1/3V
	vac := res.NodeVoltages[3]
	if math.Abs(real(vac)-1.0/3.0) > 1e-6 {
		t.Errorf("节点3小信号: 期望 1/3V, 实际 %v", vac)
	}
	if math.Abs(imag(vac)) > 1e-9 {
		t.Errorf("纯阻网络应无虚部: %v", vac)
	}
}

// TestSmallSignal_Errors 错误路径
func TestSmallSignal_Errors(t *testing.T) {
	// 含储能元件 → 报错
	if _, err := AnalyzeSmallSignal("v1 [1,-1] [0,0,0,0,5]\nr1 [1,2] [100]\nc1 [2,-1] [1e-6]", 1000); err == nil {
		t.Errorf("含电容的小信号分析应报错")
	}
	// 线性 AC 分析遇二极管 → 报错
	if _, err := AnalyzeAC("v1 [1,-1] [0,0,0,0,5]\nr1 [1,2] [100]\nd1 [2,-1] [1e-12]", 1000); err == nil {
		t.Errorf("线性 AC 遇二极管应报错")
	}
}
