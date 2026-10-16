// gate_test.go：TTL 电气级门模型的判据与反证——Ron 反推、输入负载折线、步长守卫、
// 惯性延时（含短脉冲吞噬）与输出驱动下陷。
//
// 本文件自带一个**测试专用**的最小器件（TTLGATE，NodeType 取运行时空闲值），
// 只把 GateModel 接到引擎上；生产元件注册属 m4，不在本文件。
//
// 反证注入（不改真实文件）：
//
//	sed -e '/queue := &digital.DelayQueue{}/d' \
//	    -e 's|return queue.PlanReplace(value, g.slots.QueueBase, at, levelVolt)|g.setDrive(value, levelVolt > g.level.Threshold)\n\treturn nil|' \
//	    element/ttl/gate.go > /tmp/gate-rev/immediate.go      # 延时 → 立即驱动：TestGateDelayStep 必须红
//	sed 's|g.queue.PlanReplace(|g.queue.Plan(|'                # 覆盖语义 → 不覆盖：TestGateInertialSwallowsShortPulse 必须红
//	sed 's|g.ronFor(g.driveHigh(value))|0.0|'                  # 串联电阻 → 0：TestGateOutputDriveUnderLoad 必须红
//	sed 's|g.load.ConductanceAt(m.GetNodeVoltage(node), g.level.Threshold)|g.load.IIH|'  # 两侧同值：TestGateInputLoadConductanceSides 必须红
package ttl

import (
	_ "circuit/element/passive"
	_ "circuit/element/source"

	"circuit/element"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
	"log"
	"math"
	"strings"
	"testing"
)

// 测试元件的槽位布局（对应 GateSlots）。
const (
	gateTestQueueBase = 0 // DelayQueue 占 0..3（armed/at/value/now）
	gateTestGoalSlot  = 4
	gateTestDriveSlot = 5
	gateTestStampSlot = 6
	gateTestValueNum  = 7
)

// gateTestVcc 是测试供电轨（V）。
const gateTestVcc = 5.0

// gateTestParams 是测试自用的电气参数（74LS 量级）。生产数值只允许存在于参数表（spec.go）。
var gateTestParams = GateParams{
	Model: "TTLGATE",
	TpdLH: 20e-9,
	TpdHL: 20e-9,
	IIH:   7.4e-6,
	IIL:   1e-3,
	IOH:   400e-6,
	IOL:   8e-3,
	VOH:   2.7,
	VOL:   0.4,
}

// gateTestModel 组装测试元件使用的门模型（输出引脚 1、输入引脚 0、内部节点 0、电压源 0）。
func gateTestModel() (GateModel, error) {
	return NewGateModel(gateTestParams, gateTestVcc,
		GateOutput{Pin: 1, Internal: 0, Source: 0},
		GateSlots{QueueBase: gateTestQueueBase, Goal: gateTestGoalSlot, Drive: gateTestDriveSlot, Stamp: gateTestStampSlot},
		0)
}

// gateTestNode 建一个带完整状态槽与引脚映射的元件节点（不加载网表，供单元用例直接盖章）。
func gateTestNode() *element.Node {
	node := &element.Node{
		NodeValue:    make([]any, gateTestValueNum),
		Nodes:        []mna.NodeID{0, 1},
		NodeInternal: []mna.NodeID{2},
		VoltSource:   []mna.VoltageID{0},
	}
	for i := range node.NodeValue {
		node.NodeValue[i] = float64(0)
	}
	node.NodeValue[gateTestStampSlot] = math.NaN() // K1：幂等槽初值必须是 NaN
	return node
}

// gateTestTimeBase 承载 mna.Time 其余（门模型不调用的）方法；单独抽一层是因为
// 直接嵌入 mna.Time 会让字段名 Time 与替身自己声明的 Time() 方法重名而编译失败。
type gateTestTimeBase struct {
	mna.Time
}

// gateTestTime 是只实现门模型会用到的方法的 mna.Time 替身。
type gateTestTime struct {
	gateTestTimeBase
	now         float64
	step        float64
	noConverged int
}

func (s *gateTestTime) Time() float64        { return s.now }
func (s *gateTestTime) CurrentTime() float64 { return s.now }
func (s *gateTestTime) CurrentStep() float64 { return s.step }
func (s *gateTestTime) NoConverged()         { s.noConverged++ }

// TestGateRonReverseDerivation 判据 3（反推自洽）：Ron 由 IOH/IOL 与 VOL/VOH 反推，
// 且两个方向分别按各自的额定点反推，不是共用一个数。
func TestGateRonReverseDerivation(t *testing.T) {
	p := gateTestParams
	ronHigh, err := p.RonHigh(gateTestVcc)
	if err != nil {
		t.Fatalf("RonHigh 反推失败: %v", err)
	}
	if want := (gateTestVcc - p.VOH) / p.IOH; math.Abs(ronHigh-want) > 1e-9 {
		t.Fatalf("RonHigh 应为 (VCC-VOH)/IOH=%g，实际 %g", want, ronHigh)
	}
	ronLow, err := p.RonLow(gateTestVcc)
	if err != nil {
		t.Fatalf("RonLow 反推失败: %v", err)
	}
	if want := p.VOL / p.IOL; math.Abs(ronLow-want) > 1e-9 {
		t.Fatalf("RonLow 应为 VOL/IOL=%g，实际 %g", want, ronLow)
	}
	ron, err := p.Ron(gateTestVcc)
	if err != nil {
		t.Fatalf("Ron 反推失败: %v", err)
	}
	if ron != math.Max(ronHigh, ronLow) {
		t.Fatalf("Ron 应取两方向更保守者 %g，实际 %g", math.Max(ronHigh, ronLow), ron)
	}

	// 反证方向：越界参数（VOH ≥ VCC、驱动上限非正）必须报错，不得静默建模。
	bad := p
	bad.VOH = gateTestVcc
	if _, err := bad.Ron(gateTestVcc); err == nil {
		t.Fatal("VOH ≥ VCC 时必须报错")
	}
	bad = p
	bad.IOH = 0
	if _, err := bad.Ron(gateTestVcc); err == nil {
		t.Fatal("IOH=0 时必须报错（拒绝除零与凭空驱动）")
	}
	bad = p
	bad.TpdLH = 0
	if _, err := bad.Ron(gateTestVcc); err == nil {
		t.Fatal("TpdLH=0 时必须报错（延时不能为零）")
	}
}

// TestGateParamsLookupWired 记录 K3 的接线状态：器件层已把 SpecOf 接到 gateSpecLookup
// （element/ttl/deviceGate.go 的 init），故 ParamsFromModel 对已取证型号必须成功；
// 把接线拆掉后必须报错——这一对断言互为反证（原判据的前提是"默认未接线"，集成后已不成立）。
func TestGateParamsLookupWired(t *testing.T) {
	if _, err := ParamsFromModel("74LS00"); err != nil {
		t.Fatalf("器件层接线后 ParamsFromModel(\"74LS00\") 必须成功: %v", err)
	}
	if _, err := ParamsFromModel(""); err == nil {
		t.Fatal("空型号必须报错")
	}
	original := gateSpecLookup
	gateSpecLookup = func(model string) (GateParams, bool) {
		if model == "FAKE" {
			return gateTestParams, true
		}
		return GateParams{}, false
	}
	defer func() { gateSpecLookup = original }()
	got, err := ParamsFromModel("FAKE")
	if err != nil {
		t.Fatalf("接线后应能查到 FAKE: %v", err)
	}
	if got.Model != "TTLGATE" || got.TpdLH != gateTestParams.TpdLH {
		t.Fatalf("接线后返回的参数不符: %+v", got)
	}
	if _, err := ParamsFromModel("NOPE"); err == nil {
		t.Fatal("接线后未知型号仍必须报错")
	}
}

// TestGateStepGuardCallsNoConverged 判据（K1 硬约束②）：步长超过上界时 Step 必须返回错误，
// 且**自己**调用 t.NoConverged()（不能只返回错误、把标记推给调用方）。
func TestGateStepGuardCallsNoConverged(t *testing.T) {
	model, err := gateTestModel()
	if err != nil {
		t.Fatalf("组装模型失败: %v", err)
	}
	node := gateTestNode()
	m := mna.NewMna(3, 1)

	// 上界 = min(Tpd)/10 = 2ns；步长 1µs 明显越界。
	st := &gateTestTime{step: 1e-6}
	err = model.Step(m, st, node, true)
	if err == nil {
		t.Fatal("步长 1e-6 远超上界 2e-9，必须返回错误")
	}
	if st.noConverged != 1 {
		t.Fatalf("StepGuard 失败分支必须自己调 NoConverged 恰好一次，实际 %d 次", st.noConverged)
	}
	if !strings.Contains(err.Error(), "当前步长") || !strings.Contains(err.Error(), "上界") {
		t.Fatalf("错误文本应含「当前步长」与「上界」，实际: %v", err)
	}

	// 反证：步长合规时同一调用必须不报错、且不得调 NoConverged。
	st = &gateTestTime{step: model.StepLimit()}
	if err := model.Step(m, st, node, true); err != nil {
		t.Fatalf("步长 2e-9 等于上界，应通过，实际: %v", err)
	}
	if st.noConverged != 0 {
		t.Fatalf("合规步长不得调 NoConverged，实际 %d 次", st.noConverged)
	}
}

// TestGateInputLoadConductanceSides 判据 4：输入节点在高/低两侧分别按 IIH/IIL 盖章导纳。
// 反证：把两侧折线改成同值（overlay）⇒ 本用例必须红。
func TestGateInputLoadConductanceSides(t *testing.T) {
	model, err := gateTestModel()
	if err != nil {
		t.Fatalf("组装模型失败: %v", err)
	}
	cases := []struct {
		name string
		v    float64
		want float64
	}{
		{"高侧", gateTestVcc, gateTestParams.IIH},
		{"低侧", 0, gateTestParams.IIL},
	}
	var stamped [2]float64
	for i, c := range cases {
		m := mna.NewMna(3, 1)
		node := gateTestNode()
		m.GetX().Set(0, c.v) // 让 GetNodeVoltage(0) 读到该侧电平（读的是解向量）
		model.stamp(m, node)
		stamped[i] = m.GetA().Get(0, 0)
		if math.Abs(stamped[i]-c.want) > 1e-12 {
			t.Errorf("%s：输入节点导纳应为 %g，实际 %g", c.name, c.want, stamped[i])
		}
	}
	if stamped[0] == stamped[1] {
		t.Fatalf("高/低两侧导纳必须不同（IIH=%g IIL=%g），实际两侧都是 %g",
			gateTestParams.IIH, gateTestParams.IIL, stamped[0])
	}
}

// ---------------------------------------------------------------------------
// 网表级用例：测试专用器件 + 真实瞬态引擎
// ---------------------------------------------------------------------------

// gateTestElement 是测试专用最小器件：把 wantHigh 判决（单输入缓冲）与 GateModel 接到引擎上。
type gateTestElement struct{ *element.Config }

// Reset 复位全部运行态槽（C5：同一 Context 连跑两次结果一致）。
func (gateTestElement) Reset(base element.NodeFace) {
	for i := 0; i < gateTestValueNum; i++ {
		if i == gateTestStampSlot {
			continue
		}
		base.SetFloat64(i, 0)
	}
	base.SetFloat64(gateTestStampSlot, math.NaN())
}

// DoStep 单输入缓冲：输入过阈值即 wantHigh，其余交给 GateModel（K1 的步长/幂等/延时都在模型里）。
func (e gateTestElement) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	model, err := gateTestModel()
	if err != nil {
		log.Printf("gate 测试元件: %v", err)
		return
	}
	wantHigh := m.GetNodeVoltage(value.GetNodes(0)) > gateTestVcc/2
	if err := model.Step(m, t, value, wantHigh); err != nil {
		log.Printf("gate 测试元件: %v", err)
	}
}

// registerGateTestElement 注册测试元件：NodeType 取当前空闲值，避免与其它子任务已注册的号段冲突。
func registerGateTestElement() element.NodeType {
	for n := element.NodeType(500); ; n++ {
		if _, used := element.ElementList[n]; used {
			continue
		}
		return element.AddElement(n, gateTestElement{&element.Config{
			Name: "TTLGATE",
			Pin:  element.SetPin(element.PinLowVoltage, "in", "out"),
			ValueInit: []any{
				float64(0), float64(0), float64(0), float64(0), // 0..3: DelayQueue [armed, at, value, now]
				float64(0), // 4: 目标电平
				float64(0), // 5: 驱动电平
				math.NaN(), // 6: 步进幂等槽（K1：必须初值 NaN）
			},
			ValueName: []string{"armed", "at", "value", "now", "goal", "drive", "stamp"},
			OrigValue: []int{0, 1, 2, 3, 4, 5, 6},
			Voltage:   []string{"v"},
			Internal:  []string{"int_out"},
			Flags:     element.FlagNonlinear,
		}})
	}
}

// gateTestType 是测试元件的注册结果（网表里用 TTLGATE<id> 引用）。
var gateTestType = registerGateTestElement()

// gateSample 是一次采样（仿真时刻 + 输出节点电压）。
type gateSample struct {
	t float64
	v float64
}

// runGateNetlist 加载网表并跑到 target：显式给出步长上界（C1），返回采样与时间控制器。
func runGateNetlist(t *testing.T, netlist string, target, maxStep float64, outNode mna.NodeID) ([]gateSample, *etime.TimeMNA) {
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
		t.Fatalf("SetStepLimits(1e-12, %g) 失败: %v", maxStep, err)
	}
	var samples []gateSample
	if err := etime.TransientSimulation(con, func([]float64) {
		samples = append(samples, gateSample{t: tm.CurrentTime(), v: con.GetRawNodeVoltage(outNode)})
	}); err != nil {
		t.Fatalf("仿真失败: %v", err)
	}
	if len(samples) == 0 {
		t.Fatal("仿真没有任何采样点")
	}
	return samples, tm
}

// assertNoElemIterExhausted 判据（C6）：时序判据的证据必须带「迭代耗尽=0」。
func assertNoElemIterExhausted(t *testing.T, tm *etime.TimeMNA) {
	t.Helper()
	if n := tm.NewtonEvents().IterExhausted; n != 0 {
		t.Errorf("C6：迭代耗尽=%d，必须为 0（否则本步解是额度耗尽后被静默接受的）", n)
	}
}

// TestGateDelayStep 判据 1：阶跃输入后输出翻转时刻 ≈ TpdLH（容差内），且迭代耗尽=0。
// 反证：把 PlanReplace 换成「立即驱动」（overlay）⇒ 翻转时刻断言必须 FAIL。
func TestGateDelayStep(t *testing.T) {
	maxStep := gateTestParams.TpdLH / 10 // 显式步长上界（C1）
	netlist := `
V1 [1,-1] [0,0,0,0,5]
TTLGATE1 [1,2] [0]
R1 [2,-1] [1e6]
`
	samples, tm := runGateNetlist(t, netlist, 60e-9, maxStep, 2)
	assertNoElemIterExhausted(t, tm)

	flip := -1.0
	for _, s := range samples {
		if s.v > gateTestVcc/2 {
			flip = s.t
			break
		}
	}
	if flip < 0 {
		t.Fatalf("输出在 %g 内没有翻转（样本数 %d，末值 %g）", samples[len(samples)-1].t, len(samples), samples[len(samples)-1].v)
	}
	t.Logf("阶跃后翻转时刻 %g s（TpdLH=%g，步长上界 %g）", flip, gateTestParams.TpdLH, maxStep)
	if math.Abs(flip-gateTestParams.TpdLH) > 3*maxStep {
		t.Errorf("翻转时刻 %g 偏离 TpdLH=%g 超过 3 个步长上界（%g）", flip, gateTestParams.TpdLH, 3*maxStep)
	}
	if flip <= maxStep {
		t.Errorf("翻转时刻 %g 不能早于第一个步长 %g（否则等于没有延时）", flip, maxStep)
	}
}

// TestGateInertialSwallowsShortPulse 判据 2：短于 Tpd 的输入脉冲被吞掉（输出始终不翻转）。
// 反证：把覆盖式排程换成不覆盖语义（overlay：PlanReplace → Plan）⇒ 本断言必须 FAIL。
func TestGateInertialSwallowsShortPulse(t *testing.T) {
	maxStep := gateTestParams.TpdLH / 10
	// PULSE：freq=1MHz、duty=0.005 ⇒ 高电平窗口 5ns < TpdLH=20ns。
	netlist := `
V1 [1,-1] [5,0,1e6,0,5,0.005,0,0]
TTLGATE1 [1,2] [0]
R1 [2,-1] [1e6]
`
	samples, tm := runGateNetlist(t, netlist, 60e-9, maxStep, 2)
	assertNoElemIterExhausted(t, tm)

	maxV := 0.0
	for _, s := range samples {
		if s.v > maxV {
			maxV = s.v
		}
		if s.v > gateTestVcc/2 {
			t.Fatalf("短脉冲（5ns < Tpd %g）必须被惯性延时吞掉，但 t=%g 时输出已达 %g",
				gateTestParams.TpdLH, s.t, s.v)
		}
	}
	t.Logf("短脉冲期间输出最大 %g V（未翻转）", maxV)
}

// TestGateOutputDriveUnderLoad 判据 3：输出驱动——额定负载点等于 VOH/VOL，过载时电平可见下陷。
// 反证：把串联电阻设为 0（overlay）⇒ 额定点与过载两条断言必须 FAIL。
func TestGateOutputDriveUnderLoad(t *testing.T) {
	ronHigh, err := gateTestParams.RonHigh(gateTestVcc)
	if err != nil {
		t.Fatalf("RonHigh 反推失败: %v", err)
	}
	ronLow, err := gateTestParams.RonLow(gateTestVcc)
	if err != nil {
		t.Fatalf("RonLow 反推失败: %v", err)
	}

	// 高侧：输入恒高。
	//   空载：内部节点 = VCC，串联电阻上无压降 ⇒ 输出 = VCC（且 ≥ VOH，不越轨）。
	//   额定：负载吃掉 IOH ⇒ 串联电阻上压降 VCC-VOH ⇒ 输出 = VOH。
	//   过载：负载再小 10 倍 ⇒ 输出明显低于 VOH。
	highUnloaded := gateRunFinalVoltage(t, `
V1 [1,-1] [0,0,0,0,5]
TTLGATE1 [1,2] [0]
R1 [2,-1] [1e9]
`)
	if math.Abs(highUnloaded-gateTestVcc) > 0.05 {
		t.Errorf("高侧空载输出应等于轨电压 %g（串联电阻上无压降），实际 %g", gateTestVcc, highUnloaded)
	}
	if highUnloaded < gateTestParams.VOH {
		t.Errorf("高侧空载输出 %g 不得低于 VOH=%g", highUnloaded, gateTestParams.VOH)
	}

	ratedHigh := gateRunFinalVoltage(t, `
V1 [1,-1] [0,0,0,0,5]
TTLGATE1 [1,2] [0]
R1 [2,-1] [6750]
`)
	// 额定负载 R = VOH/IOH：输出应落在 VOH（容差取 5%）。
	if math.Abs(ratedHigh-gateTestParams.VOH) > 0.05*gateTestParams.VOH {
		t.Errorf("高侧额定负载（R=VOH/IOH ⟹ RonHigh=%g）输出应为 VOH=%g，实际 %g",
			ronHigh, gateTestParams.VOH, ratedHigh)
	}

	overloadHigh := gateRunFinalVoltage(t, `
V1 [1,-1] [0,0,0,0,5]
TTLGATE1 [1,2] [0]
R1 [2,-1] [675]
`)
	if overloadHigh > 0.5*gateTestParams.VOH {
		t.Errorf("高侧过载（R=VOH/(10·IOH)）输出必须明显下陷（应 < %g），实际 %g",
			0.5*gateTestParams.VOH, overloadHigh)
	}

	// 低侧：输入恒低，负载把输出拉向 VCC；额定点应落在 VOL。
	ratedLow := gateRunFinalVoltage(t, `
V1 [1,-1] [0,0,0,0,0]
V2 [3,-1] [0,0,0,0,5]
TTLGATE1 [1,2] [0]
R1 [3,2] [575]
`)
	if math.Abs(ratedLow-gateTestParams.VOL) > 0.05 {
		t.Errorf("低侧额定负载（R=(VCC-VOL)/IOL ⟹ RonLow=%g）输出应为 VOL=%g，实际 %g",
			ronLow, gateTestParams.VOL, ratedLow)
	}
	t.Logf("驱动：空载=%g 额定高=%g 过载高=%g 额定低=%g（RonHigh=%g RonLow=%g）",
		highUnloaded, ratedHigh, overloadHigh, ratedLow, ronHigh, ronLow)
}

// gateRunFinalVoltage 跑一段固定网表并返回输出节点（2）在末次采样上的电压。
func gateRunFinalVoltage(t *testing.T, netlist string) float64 {
	t.Helper()
	samples, tm := runGateNetlist(t, netlist, 60e-9, gateTestParams.TpdLH/10, 2)
	assertNoElemIterExhausted(t, tm)
	return samples[len(samples)-1].v
}
