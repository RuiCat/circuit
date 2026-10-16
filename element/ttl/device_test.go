// device_test.go：m4 器件库的网表级判据（真值表 / 正沿采样 / 不完整型号响亮失败 / Reset）。
//
// 判据口径：TTL 器件是**非线性 + 有延时**的元件，故每条判据都显式给步长上界（C1），
// 且只用网表 + 引擎跑（走 init() 注册路径，C7）。
package ttl

import (
	"fmt"
	"testing"

	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

// ttlRun 跑一段网表并在 target 时刻返回指定**原始节点号**的电压（用 GetRawNodeVoltage：
// GetNodeVoltage 收的是 compact 下标，混用会读错节点——本文件踩过）。
func ttlRun(t *testing.T, netlist string, target, maxStep float64, outNode mna.NodeID) float64 {
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
	if err := etime.TransientSimulation(con, func([]float64) { last = con.GetRawNodeVoltage(outNode) }); err != nil {
		t.Fatalf("仿真失败: %v", err)
	}
	return last
}

// ttlRunErr 跑一段网表并返回仿真错误（用于"必须响亮失败"的判据）。
func ttlRunErr(netlist string, target, maxStep float64) error {
	con, err := load.LoadString(netlist)
	if err != nil {
		return err
	}
	tm, err := etime.NewTimeMNA(target)
	if err != nil {
		return err
	}
	con.Time = tm
	if err := tm.SetStepLimits(1e-12, maxStep); err != nil {
		return err
	}
	return etime.TransientSimulation(con, func([]float64) {})
}

// nandNet 生成二输入与非门网表：a=V(aV)、b=V(bV)、输出节点 3 接 100k 轻载。
func nandNet(aV, bV float64) string {
	return fmt.Sprintf(`
V1 [1,-1] [0,0,0,0,%g]
V2 [2,-1] [0,0,0,0,%g]
TTLNAND 1 [1,2,3]
R1 [3,-1] [100000]
`, aV, bV)
}

// TestDeviceNandTruthTable 74LS00 与非门真值表（含两档判决）。
func TestDeviceNandTruthTable(t *testing.T) {
	cases := []struct {
		a, b float64
		want bool // 输出是否应为高
	}{
		{0, 0, true}, {0, 5, true}, {5, 0, true}, {5, 5, false},
	}
	for _, c := range cases {
		got := ttlRun(t, nandNet(c.a, c.b), 200e-9, 0.9e-9, 3)
		high := got > 2.5
		if high != c.want {
			t.Errorf("NAND a=%g b=%g: 输出 %.3fV（判为高=%v），期望高=%v", c.a, c.b, got, high, c.want)
		}
		// 电平还要落在数据手册限值一侧：高 ≥ VOH(3.4V)、低 ≤ VOL(0.35V)（D19 口径：额定负载内）。
		if c.want && got < 3.4 {
			t.Errorf("NAND a=%g b=%g: 高电平 %.3fV 低于 VOH=3.4V", c.a, c.b, got)
		}
		if !c.want && got > 0.4 {
			t.Errorf("NAND a=%g b=%g: 低电平 %.3fV 高于 VOL≈0.35V", c.a, c.b, got)
		}
	}
}

// TestDeviceAndTruthTable 74LS08 与门真值表（两档关键组合）。
func TestDeviceAndTruthTable(t *testing.T) {
	net := func(a, b float64) string {
		return fmt.Sprintf(`
V1 [1,-1] [0,0,0,0,%g]
V2 [2,-1] [0,0,0,0,%g]
TTLAND 1 [1,2,3]
R1 [3,-1] [100000]
`, a, b)
	}
	if got := ttlRun(t, net(0, 5), 200e-9, 0.8e-9, 3); got > 0.4 {
		t.Errorf("AND 0·1 应为低，实际 %.3fV", got)
	}
	if got := ttlRun(t, net(5, 5), 200e-9, 0.8e-9, 3); got < 3.4 {
		t.Errorf("AND 1·1 应为高（≥VOH 3.4V），实际 %.3fV", got)
	}
}

// TestDeviceDffSamplesOnRisingEdge 74LS74 正沿采样：t=0 时 clk 由低到高，
// 同刻 d 被采样；q 跟随 d，nq 取反。
func TestDeviceDffSamplesOnRisingEdge(t *testing.T) {
	net := func(dV float64) string {
		return fmt.Sprintf(`
V1 [1,-1] [0,0,0,0,%g]
V2 [2,-1] [0,0,0,0,5]
TTLDFF 1 [1,2,2,2,3,4]
R1 [3,-1] [100000]
R2 [4,-1] [100000]
`, dV)
	}
	qHigh := ttlRun(t, net(5), 300e-9, 1.3e-9, 3)
	if qHigh < 3.4 {
		t.Errorf("d=1 时正沿后 q 应为高（≥3.4V），实际 %.3fV", qHigh)
	}
	qLow := ttlRun(t, net(0), 300e-9, 1.3e-9, 3)
	if qLow > 0.4 {
		t.Errorf("d=0 时正沿后 q 应为低，实际 %.3fV", qLow)
	}
	// nq 与 q 互补（d=1 时 nq 应低）。
	nqLow := ttlRun(t, net(5), 300e-9, 1.3e-9, 4)
	if nqLow > 0.4 {
		t.Errorf("d=1 时 nq 应与 q 互补（低），实际 %.3fV", nqLow)
	}
}

// TestDeviceDffAsyncPreClr 异步 PRE/CLR（低有效）：与 clk 无关，pre 优先。
func TestDeviceDffAsyncPreClr(t *testing.T) {
	// pre=0（V3=0V 接 pre），clr=高 ⇒ 异步置位：q 高，即使 d=0。
	set := `
V1 [1,-1] [0,0,0,0,0]
V2 [2,-1] [0,0,0,0,5]
V3 [3,-1] [0,0,0,0,0]
TTLDFF 1 [1,2,3,2,4,5]
R1 [4,-1] [100000]
R2 [5,-1] [100000]
`
	if v := ttlRun(t, set, 300e-9, 1.3e-9, 4); v < 3.4 {
		t.Errorf("pre=0 时应异步置位（q 高 ≥3.4V，与 d=0 无关），实际 %.3fV", v)
	}
	if v := ttlRun(t, set, 300e-9, 1.3e-9, 5); v > 0.4 {
		t.Errorf("pre=0 时 nq 应为低，实际 %.3fV", v)
	}
	// clr=0（V3=0V 接 clr），pre=高 ⇒ 异步复位：q 低，即使 d=1。
	rst := `
V1 [1,-1] [0,0,0,0,5]
V2 [2,-1] [0,0,0,0,5]
V3 [3,-1] [0,0,0,0,0]
TTLDFF 1 [1,2,2,3,4,5]
R1 [4,-1] [100000]
R2 [5,-1] [100000]
`
	if v := ttlRun(t, rst, 300e-9, 1.3e-9, 4); v > 0.4 {
		t.Errorf("clr=0 时应异步复位（q 低，与 d=1 无关），实际 %.3fV", v)
	}
}

// TestDeviceModelWiringAndRefusal 接线与拒绝：完整型号能建模，不完整/未知型号必须报错。
//
// 为什么不在网表里写型号：值 token 会被切分（`74LS00` → `74` + `LS00`，实测警告
// "网表参数值 'LS00' 无法解析为 float64"）⇒ 型号只能由器件类型的默认值给出。
func TestDeviceModelWiringAndRefusal(t *testing.T) {
	if _, err := ParamsFromModel("74LS00"); err != nil {
		t.Fatalf("74LS00 应能建模（参数表接线失败？）: %v", err)
	}
	if p138, err := ParamsFromModel("74LS138"); err != nil {
		t.Fatalf("74LS138 查表应成功（查不到才是另一种错）: %v", err)
	} else if _, err := NewGateModel(p138, ttlVcc, GateOutput{Pin: 2, Internal: 0, Source: 0},
		GateSlots{QueueBase: 1, Goal: 5, Drive: 6, Stamp: 7}, 0, 1); err == nil {
		t.Fatalf("参数不完整的型号必须建模失败（不许静默用 0 建模）")
	}
	if _, err := ParamsFromModel("74LS999"); err == nil {
		t.Fatalf("未知型号必须报错")
	}
	// 反证：拆掉接线后第一句必须变成报错。
	old := gateSpecLookup
	gateSpecLookup = func(string) (GateParams, bool) { return GateParams{}, false }
	defer func() { gateSpecLookup = old }()
	if _, err := ParamsFromModel("74LS00"); err == nil {
		t.Fatalf("拆掉接线后必须报错（反证：接线是承重的）")
	}
}

// TestDeviceResetReproduces 同一 Context 连跑两次结果一致（C5：Reset 必须清运行态）。
func TestDeviceResetReproduces(t *testing.T) {
	con, err := load.LoadString(nandNet(5, 5))
	if err != nil {
		t.Fatalf("加载网表失败: %v", err)
	}
	runOnce := func() float64 {
		tm, err := etime.NewTimeMNA(200e-9)
		if err != nil {
			t.Fatalf("创建时间控制器失败: %v", err)
		}
		con.Time = tm
		if err := tm.SetStepLimits(1e-12, 0.9e-9); err != nil {
			t.Fatalf("设置步长限制失败: %v", err)
		}
		var last float64
		if err := etime.TransientSimulation(con, func([]float64) { last = con.GetRawNodeVoltage(3) }); err != nil {
			t.Fatalf("仿真失败: %v", err)
		}
		return last
	}
	first, second := runOnce(), runOnce()
	if first != second {
		t.Fatalf("同一 Context 连跑两次结果应一致（Reset/C5），实际 %.6f vs %.6f", first, second)
	}
}
