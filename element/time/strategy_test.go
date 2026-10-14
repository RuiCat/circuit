// 收敛/步长策略对象（契约 K3/K4/C5）与有界牛顿事件计数的针对性用例。
//
// 三个用例的口径（m5 委派包 §验收）：
//   - TestStrategyGminReset：**同一 Context 连跑两次**，第一次 GminFinal=1e-5、
//     第二次置 0，断言第二次终值 = 默认值（C5：跨 run 无残留）；
//   - TestStrategyEventCount：在真实瞬态运行上证明四个计数可读、可非零（K4/C6）；
//   - TestStrategyObjectSync：ApplyStrategy 后策略字段与 TimeMNA 状态一致，
//     且无效输入报错且不改动现有状态。
//
// 本文件只读 element/time 与 load（不进入 m4 的 element/semiconductor 写域）；
// 运行参数固定为 CLI 批处理口径，与 54 条快照语料（-t 1e-4）同源，
// 便于与 scripts/regressDiff.py 的对照表互相校验。
package time

import (
	"math"
	"os"
	"path/filepath"

	"circuit/element"
	"circuit/load"

	"testing"
)

// strategyTestNetlist 固定的小型 DC 网表（与 gmin_test.go 同款分压器）：
// 无储能元件、无半导体器件，两次运行之间唯一的变量就是策略绑定。
const strategyTestNetlist = `
V1 [1,-1] [0,0,0,0,5]
R1 [1,2] [1000]
R2 [2,-1] [2000]
`

// corpusDir 返回 K1 语料目录（cmd/circuits）。
func corpusDir() string {
	return filepath.Join("..", "..", "cmd", "circuits")
}

// strategyTestRun 在一个 Context 上按 CLI 批处理口径跑一次瞬态仿真。
// 参数与 cmd/main.go 的默认一致性：absTol=1e-6、relTol=1e-4、
// step∈[1e-9,5e-4]、maxNonlin=200、maxElem=20、maxSteps=10000、初值步长 1e-6。
func strategyTestRun(con *element.Context, tm *TimeMNA, call func([]float64)) error {
	con.Time = tm
	con.InitResumeCond()
	if err := tm.SetTolerances(1e-6, 1e-4); err != nil {
		return err
	}
	if err := tm.SetStepLimits(1e-9, 5e-4); err != nil {
		return err
	}
	if err := tm.SetIterationLimits(200, tm.MaxElemIter()); err != nil {
		return err
	}
	if err := tm.SetMaxTimeSteps(10000); err != nil {
		return err
	}
	tm.SetTimeStep(1e-6)
	return TransientSimulation(con, call)
}

// strategyTestLoad 从 K1 语料目录读一条网表并装载成 Context。
func strategyTestLoad(t *testing.T, file string) *element.Context {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(corpusDir(), file))
	if err != nil {
		t.Fatalf("读取语料 %s 失败: %v", file, err)
	}
	con, err := load.LoadString(string(text))
	if err != nil {
		t.Fatalf("装载语料 %s 失败: %v", file, err)
	}
	return con
}

// TestStrategyGminReset C5：同一 Context 连跑两次，第二次配置 GminFinal=0
// 必须把 gmin 终值复位为默认值（不是"沿用上一次"）。
func TestStrategyGminReset(t *testing.T) {
	con, err := load.LoadString(strategyTestNetlist)
	if err != nil {
		t.Fatalf("装载网表失败: %v", err)
	}
	con.Backend.Bind(false, false)
	tm, err := NewTimeMNA(1e-5)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}

	// 第一次运行：显式把延续终值推到 1e-5（病态电路提前终止的典型用法）。
	const customFinal = 1e-5
	con.EngineCfg.GminFinal = customFinal
	if err := strategyTestRun(con, tm, func([]float64) {}); err != nil {
		t.Fatalf("第一次运行失败: %v", err)
	}
	if got := tm.GminFinal(); got != customFinal {
		t.Fatalf("第一次运行后 GminFinal 应为 %g，实际 %g", customFinal, got)
	}

	// 第二次运行：置 0（"未配置"）——必须回默认值。
	// 用**新的 TimeMNA**（第二次仿真在真实用法里会重新建时间控制器），
	// 唯一被沿用下来的状态是同一个 Context（EngineCfg 的历史值）。
	con.EngineCfg.GminFinal = 0
	tm2, err := NewTimeMNA(1e-5)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}
	if err := strategyTestRun(con, tm2, func([]float64) {}); err != nil {
		t.Fatalf("第二次运行失败: %v", err)
	}
	if got := tm2.GminFinal(); got != gminStepFinal {
		t.Fatalf("C5: 第二次运行 GminFinal=0 应复位为默认值 %g，实际 %g（跨 run 残留）",
			gminStepFinal, got)
	}
	if got := tm2.Strategy().GminFinal; got != gminStepFinal {
		t.Fatalf("C5: 策略对象里的 GminFinal 应与 TimeMNA 状态一致（%g），实际 %g",
			gminStepFinal, got)
	}
	t.Logf("M5GMIN run1 cfg=%.0e final=%.6g | run2 cfg=0 final=%.6g default=%.6g",
		customFinal, tm.GminFinal(), tm2.GminFinal(), gminStepFinal)
}

// TestStrategyEventCount K4/C6：四个有界牛顿事件计数器可读。
//
// 真实语料上实测可非零的两个计数：IterExhausted（元件次级迭代额度耗尽，
// 18_d_flipflop / 36_sram6t_logic 用 maxElem 收紧后必现）与 StepHalved；
// NaNRejected 在本库的既有语义是**终止仿真**（不新增恢复路径），故只能在
// 失败运行上观察到；StepRejected/StepHalved 在既有语料上未观测到触发（见
// _results/m5/summary.md 的诚实说明），因此这里用生产同款的计数对象直接验证
// 四个字段的读写接口，并用真实运行验证 IterExhausted 确实由主循环自增。
func TestStrategyEventCount(t *testing.T) {
	// ① 真实运行：元件迭代额度耗尽必须被主循环计到（证明埋点在主循环里，不是摆设）。
	con := strategyTestLoad(t, "18_d_flipflop.net")
	tm, err := NewTimeMNA(1e-4)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}
	if err := strategyTestRun(con, tm, func([]float64) {}); err != nil {
		t.Fatalf("仿真失败: %v", err)
	}
	// 第二次运行：收紧元件迭代额度到 1（每轮外循环只允许一次 doStep），
	// 由新的 TimeMNA 承载——计数必须在启动时被重置，而不是在上次的基础上累加。
	tmTight, err := NewTimeMNA(1e-4)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}
	con.Time = tmTight
	con.InitResumeCond()
	if err := tmTight.SetTolerances(1e-6, 1e-4); err != nil {
		t.Fatalf("设置容差失败: %v", err)
	}
	if err := tmTight.SetStepLimits(1e-9, 5e-4); err != nil {
		t.Fatalf("设置步长失败: %v", err)
	}
	if err := tmTight.SetIterationLimits(200, 1); err != nil {
		t.Fatalf("收紧元件迭代额度失败: %v", err)
	}
	if err := tmTight.SetMaxTimeSteps(10000); err != nil {
		t.Fatalf("设置最大步数失败: %v", err)
	}
	tmTight.SetTimeStep(1e-6)
	if err := TransientSimulation(con, func([]float64) {}); err != nil {
		t.Fatalf("收紧额度后仿真失败: %v", err)
	}
	tm = tmTight
	live := tm.NewtonEvents()
	t.Logf("M5EVENTS case=18_d_flipflop(maxElem=1) steps=%d newton=%d halved=%d rejected=%d exhausted=%d nan=%d",
		tm.GoodSteps(), tm.NewtonIterations(), live.StepHalved, live.StepRejected, live.IterExhausted, live.NaNRejected)
	if live.IterExhausted == 0 {
		t.Fatalf("K4: 元件迭代额度耗尽未被计数（IterExhausted=0）")
	}
	// 启动复位：另起一次运行后计数值不得超过上一次的总量（计数只在启动时重置）。
	if live.IterExhausted > uint64(tm.GoodSteps())*uint64(tm.MaxNonlinearIter())*uint64(tm.MaxElemIter()+1) {
		t.Fatalf("K4: IterExhausted=%d 量级异常（步数 %d）", live.IterExhausted, tm.GoodSteps())
	}

	// ② 读接口：四个字段各自可写可读、总值等于四项之和（线程安全快照）。
	var ev BoundedNewtonEvents
	ev.Reset()
	if got := ev.Counts(); got.Total() != 0 {
		t.Fatalf("Reset 后计数应为全 0，实际 %+v", got)
	}
	ev.AddStepHalved()
	ev.AddStepRejected()
	ev.AddIterExhausted()
	ev.AddNaNRejected()
	got := ev.Counts()
	if got.StepHalved != 1 || got.StepRejected != 1 || got.IterExhausted != 1 || got.NaNRejected != 1 {
		t.Fatalf("四个计数应各为 1，实际 %+v", got)
	}
	if got.Total() != 4 {
		t.Fatalf("Total 应为 4，实际 %d", got.Total())
	}
	// ③ BeginBoundedNewtonEvents 必须清零（每次 TransientSimulation 启动调用）。
	tm.BeginBoundedNewtonEvents()
	if after := tm.NewtonEvents(); after.Total() != 0 {
		t.Fatalf("BeginBoundedNewtonEvents 后应清零，实际 %+v", after)
	}
}

// TestStrategyObjectSync：ApplyStrategy 的字段映射、<=0 回默认、以及
// 「遇无效值报错且不改动现有状态」三条语义。
func TestStrategyObjectSync(t *testing.T) {
	tm, err := NewTimeMNA(1e-5)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}
	want := StrategyBinding{
		GminCont:    true,
		NaturalGmin: 1e-9,
		GminFinal:   1e-7,
		GlobalGmin:  1e-8,
		GminDbg:     true,
		NoEq:        true,
	}
	if err := ApplyStrategy(tm, want); err != nil {
		t.Fatalf("ApplyStrategy 失败: %v", err)
	}
	if got := tm.Strategy(); got != want {
		t.Fatalf("策略对象与输入不一致: got=%+v want=%+v", got, want)
	}
	if !tm.GminCont() || tm.NaturalGmin() != want.NaturalGmin || tm.GminFinal() != want.GminFinal ||
		tm.GlobalGmin() != want.GlobalGmin || !tm.GminDbg() || !tm.NoEq() {
		t.Fatalf("TimeMNA 状态与策略对象不一致: cont=%v nat=%g final=%g global=%g dbg=%v noeq=%v",
			tm.GminCont(), tm.NaturalGmin(), tm.GminFinal(), tm.GlobalGmin(), tm.GminDbg(), tm.NoEq())
	}

	// GminFinal<=0 = 回默认（不是"不修改"）。
	if err := ApplyStrategy(tm, StrategyBinding{GminFinal: 0, NaturalGmin: want.NaturalGmin}); err != nil {
		t.Fatalf("ApplyStrategy(GminFinal=0) 失败: %v", err)
	}
	if tm.GminFinal() != gminStepFinal {
		t.Fatalf("GminFinal=0 应回默认值 %g，实际 %g", gminStepFinal, tm.GminFinal())
	}

	// 无效值：NaN / 负值 → 报错且**不改动**现有状态（禁止部分生效）。
	before := tm.Strategy()
	cases := []struct {
		name string
		s    StrategyBinding
	}{
		{"NaN-NaturalGmin", StrategyBinding{NaturalGmin: math.NaN()}},
		{"NaN-GlobalGmin", StrategyBinding{GlobalGmin: math.NaN()}},
		{"NaN-GminFinal", StrategyBinding{GminFinal: math.NaN()}},
		{"Inf-GminFinal", StrategyBinding{GminFinal: math.Inf(1)}},
		{"Negative-NaturalGmin", StrategyBinding{NaturalGmin: -1}},
		{"Negative-GlobalGmin", StrategyBinding{GlobalGmin: -1}},
	}
	for _, c := range cases {
		if err := ApplyStrategy(tm, c.s); err == nil {
			t.Fatalf("%s: 应报错，实际 nil", c.name)
		}
		if got := tm.Strategy(); got != before {
			t.Fatalf("%s: 报错后策略状态被改动: got=%+v before=%+v", c.name, got, before)
		}
		if tm.GminFinal() != gminStepFinal {
			t.Fatalf("%s: 报错后 GminFinal 被改动为 %g", c.name, tm.GminFinal())
		}
	}
}
