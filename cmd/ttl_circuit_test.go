// ttl_circuit_test.go：TTL/UART 元件的**电路级**判据（网表 + 引擎 + 真实源/负载/多器件）。
//
// 与包内单测的分工：`element/ttl` 的测试验"模型行为"（模型 + 测试器件），本文件验
// "把器件当电路元件用"——多器件串链、跨元件族混接（TTL ↔ 既有逻辑域 U 门）、扇出与过载。
// 每条判据都显式给步长上界（C1：TTL 器件是时序元件，默认自适应步长会让 tpd 判据失真）。
package main_test

import (
	"strings"
	"testing"

	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

// ttlCircuitRun 跑一段网表，返回指定**原始节点号**在 target 时刻的电压。
// （用 GetRawNodeVoltage：GetNodeVoltage 收的是 compact 下标，混用会静默读错节点——D24。）
func ttlCircuitRun(t *testing.T, netlist string, target, maxStep float64, rawNode mna.NodeID) float64 {
	t.Helper()
	con, err := load.LoadString(netlist)
	if err != nil {
		t.Fatalf("加载网表失败: %v\n%s", err, netlist)
	}
	tm, err := etime.NewTimeMNA(target)
	if err != nil {
		t.Fatalf("创建时间控制器失败: %v", err)
	}
	con.Time = tm
	if err := tm.SetStepLimits(1e-12, maxStep); err != nil {
		t.Fatalf("设置步长限制失败: %v", err)
	}
	var last float64
	if err := etime.TransientSimulation(con, func([]float64) { last = con.GetRawNodeVoltage(rawNode) }); err != nil {
		t.Fatalf("仿真失败: %v", err)
	}
	return last
}

// TestCircuitTtlGateChain 电路级：TTLNAND → TTLAND 两级链路（71 号网表）两档输入。
func TestCircuitTtlGateChain(t *testing.T) {
	// 档 1：两输入高 ⇒ NAND 低 ⇒ AND(低,高) 低。
	low := ttlCircuitRun(t, readNetlist(t, "71_ttl_gate_chain.net"), 2e-7, 8e-10, 5)
	if low > 0.4 {
		t.Errorf("档1 链路输出应低（NAND(1,1)=0 ⇒ AND(0,1)=0），实际 %.3fV", low)
	}
	// 档 2：把第二输入改为低 ⇒ NAND 高 ⇒ AND(高,高) 高（同一电路改一个源，验证链路两端都动）。
	net := readNetlist(t, "71_ttl_gate_chain.net")
	net = replaceOnce(t, net, "V2 [2,-1] [0,0,0,0,5]", "V2 [2,-1] [0,0,0,0,0]")
	high := ttlCircuitRun(t, net, 2e-7, 8e-10, 5)
	if high < 3.4 {
		t.Errorf("档2 链路输出应高（NAND(1,0)=1 ⇒ AND(1,1)=1，≥VOH 3.4V），实际 %.3fV", high)
	}
}

// TestCircuitTtlDrivesLogicGate 电路级：TTL 输出驱动既有逻辑域 U 门（72 号网表）。
// 这是"跨元件族"判据：TTL 的 0V / 4.8V 必须被逻辑门的 2.5V 阈值正确识别。
func TestCircuitTtlDrivesLogicGate(t *testing.T) {
	// TTL 输出低 ⇒ U 门反相输出高。
	net := readNetlist(t, "72_ttl_to_logic.net")
	if v := ttlCircuitRun(t, net, 2e-7, 8e-10, 3); v > 0.4 {
		t.Errorf("TTL 输出应低，实际 %.3fV", v)
	}
	if v := ttlCircuitRun(t, net, 2e-7, 8e-10, 4); v < 4.0 {
		t.Errorf("U 门反相输出应高（≈5V），实际 %.3fV", v)
	}
	// 反过来：TTL 输入改低 ⇒ TTL 输出高 ⇒ U 门输出低（双向都验，避免"只测一个方向"）。
	netLow := replaceOnce(t, net, "V2 [2,-1] [0,0,0,0,5]", "V2 [2,-1] [0,0,0,0,0]")
	if v := ttlCircuitRun(t, netLow, 2e-7, 8e-10, 3); v < 3.4 {
		t.Errorf("TTL 输出应高（≥3.4V），实际 %.3fV", v)
	}
	if v := ttlCircuitRun(t, netLow, 2e-7, 8e-10, 4); v > 0.5 {
		t.Errorf("U 门输出应低，实际 %.3fV", v)
	}
}

// TestCircuitTtlFanout 电路级：一个 TTL 输出驱动 **4 个 LS 输入**（扇出预算内）+ 100k 拉载。
//
// 历史：本判据第一版只记录"模型边界"——当时 4 输入被拉到 0.293 V 并自锁（负载 4×IIL=4 mS 压过
// 只有线性 RonHigh≈4 kΩ 的驱动器）。D28/D29 给高侧补了低电压硬段（ronHighShort，模型约定）后
// 实测 4.316 V ≥ VOH 3.4 V，故升级为**硬断言**（这才是真实 LS 的行为：保证扇出 8）。
func TestCircuitTtlFanout(t *testing.T) {
	one := `
V1 [1,-1] [0,0,0,0,0]
V2 [2,-1] [0,0,0,0,5]
TTLNAND 1 [1,2,3]
TTLAND 2 [3,2,4]
R1 [3,-1] [100000]
`
	if v := ttlCircuitRun(t, one, 2e-7, 8e-10, 3); v < 3.4 {
		t.Errorf("1 个 LS 输入（额定负载点）下输出应 ≥VOH 3.4V，实际 %.3fV", v)
	}
	four := `
V1 [1,-1] [0,0,0,0,0]
V2 [2,-1] [0,0,0,0,5]
TTLNAND 1 [1,2,3]
TTLAND 2 [3,2,4]
TTLAND 3 [3,2,5]
TTLAND 4 [3,2,6]
TTLAND 5 [3,2,7]
R1 [3,-1] [100000]
`
	if v := ttlCircuitRun(t, four, 2e-7, 8e-10, 3); v < 3.4 {
		t.Errorf("4 个 LS 输入（扇出 4）下输出应 ≥VOH 3.4V（真实 LS 保证扇出 8），实际 %.3fV", v)
	}
}

// TestCircuitTtlOverloadSags 电路级：超出 IOH 的负载必须让电平**可见下陷**（不静默当正常）。
// IOH=0.4mA、RonHigh=(5-3.4)/0.4mA=4kΩ；挂 1kΩ 时理论电平 5×1k/(1k+4k)=1.0V。
func TestCircuitTtlOverloadSags(t *testing.T) {
	net := `
V1 [1,-1] [0,0,0,0,0]
V2 [2,-1] [0,0,0,0,5]
TTLNAND 1 [1,2,3]
R1 [3,-1] [1000]
`
	v := ttlCircuitRun(t, net, 2e-7, 8e-10, 3)
	if v > 2.0 {
		t.Errorf("过载（1kΩ ⇒ 4.8mA > IOH 0.4mA）时输出应明显下陷（理论 ≈1.0V），实际 %.3fV", v)
	}
	if v < 0.5 {
		t.Errorf("过载下陷不应低到 0（Ron 是有限值，理论 ≈1.0V），实际 %.3fV", v)
	}
}

// replaceOnce 做一次字面替换；找不到或命中多次即失败（避免"静默没改到"）。
func replaceOnce(t *testing.T, s, old, new string) string {
	t.Helper()
	if n := strings.Count(s, old); n != 1 {
		t.Fatalf("替换 %q 命中 %d 次（期望 1）", old, n)
	}
	return strings.Replace(s, old, new, 1)
}
