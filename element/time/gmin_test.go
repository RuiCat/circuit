package time

import (
	"math"
	"testing"

	_ "circuit/element/register"
	"circuit/load"
)

// TestGminSteppingStateMachine 验证 Gmin 延续状态机：
// 启动 → 逐级下调 → 到达终值 → 关闭；以及再阻尼恢复的额度限制。
func TestGminSteppingStateMachine(t *testing.T) {
	tm, err := NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}

	// 初始关闭
	if tm.GminSteppingActive() {
		t.Fatalf("初始状态应为关闭")
	}
	if tm.GetContinuationGmin() != 0 {
		t.Fatalf("初始延续值应为 0, 实际 %g", tm.GetContinuationGmin())
	}

	// 启动：延续值设为起始大值
	tm.BeginGminStepping()
	if !tm.GminSteppingActive() {
		t.Fatalf("启动后应处于激活状态")
	}
	if tm.GetContinuationGmin() != gminStepStart {
		t.Fatalf("启动后延续值应为 %g, 实际 %g", gminStepStart, tm.GetContinuationGmin())
	}

	// 逐级下调：1e-3 → 1e-4 → ... → 1e-12
	// StepGminDown 在下调后返回「是否仍需继续步进」，
	// 前 8 次下调仍高于终值返回 true，第 9 次到达终值返回 false。
	expected := gminStepStart
	levels := 0
	for tm.StepGminDown() {
		levels++
		expected = math.Max(gminStepFinal, expected*gminStepScale)
		if tm.GetContinuationGmin() != expected {
			t.Fatalf("第 %d 级下调后延续值应为 %g, 实际 %g", levels, expected, tm.GetContinuationGmin())
		}
	}
	if levels != 8 {
		t.Fatalf("应恰好 8 次「仍需继续」的下调, 实际 %d", levels)
	}
	// 循环结束即到达终值
	if tm.GetContinuationGmin() != gminStepFinal {
		t.Fatalf("步进结束后延续值应为终值 %g, 实际 %g", gminStepFinal, tm.GetContinuationGmin())
	}

	// 结束：恢复自然 gmin
	tm.EndGminStepping()
	if tm.GminSteppingActive() {
		t.Fatalf("结束后应关闭")
	}

	// 再阻尼恢复：从终值回退
	tm.SetContinuationGmin(gminStepFinal)
	recoveries := 0
	for tm.RecoverGmin() {
		recoveries++
	}
	if recoveries != gminMaxRecover {
		t.Fatalf("恢复次数应为 %d, 实际 %d", gminMaxRecover, recoveries)
	}
	if tm.GetContinuationGmin() != gminStepStart {
		t.Fatalf("恢复结束后应回到起始值 %g, 实际 %g", gminStepStart, tm.GetContinuationGmin())
	}
	// 已达起始值后不再恢复
	if tm.RecoverGmin() {
		t.Fatalf("已在起始阻尼，不应再恢复")
	}
}

// TestGminSteppingLatchConvergence 验证交叉耦合锁存器在 t=0 直流求解时的收敛：
// 开启 Gmin 延续后，双稳态电路被阻尼成单稳态，牛顿迭代应逐级收敛（不报错、无 NaN）。
func TestGminSteppingLatchConvergence(t *testing.T) {
	// 交叉耦合 NPN 锁存对：两个 RTL 反相器交叉耦合，形成双稳态。
	netlist := `
V100 [100,-1] [0,0,0,0,5]
R100 [100,1] [1000]
R101 [100,2] [1000]
Q200 [2,1,-1] [false,100]
Q201 [1,2,-1] [false,100]
`
	con, err := load.LoadString(netlist)
	if err != nil {
		t.Fatalf("加载锁存网表失败: %v", err)
	}
	var terr error
	con.Time, terr = NewTimeMNA(1e-3)
	if terr != nil {
		t.Fatalf("创建仿真时间失败: %v", terr)
	}

	// 开启 Gmin 延续（与 CLI 的 CIRCUIT_GMCONT 一致）
	t.Setenv("CIRCUIT_GMCONT", "1")

	var count int
	var last1, last2 float64
	if err := TransientSimulation(con, func(voltages []float64) {
		count++
		if len(voltages) >= 2 {
			last1 = voltages[0]
			last2 = voltages[1]
		}
	}); err != nil {
		t.Fatalf("Gmin 延续下锁存仿真失败: %v", err)
	}

	if count == 0 {
		t.Fatalf("仿真未产生任何步")
	}
	// 结果必须为有限电压（阻尼收敛未引入 NaN/Inf）
	if math.IsNaN(last1) || math.IsInf(last1, 0) || math.IsNaN(last2) || math.IsInf(last2, 0) {
		t.Fatalf("锁存输出含 NaN/Inf: node1=%g node2=%g", last1, last2)
	}
	// 电压应在电源轨范围内
	if last1 < -0.5 || last1 > 5.5 || last2 < -0.5 || last2 > 5.5 {
		t.Fatalf("锁存输出电压越界: node1=%g node2=%g", last1, last2)
	}
}

// TestGminSteppingMatchesUnstepped 验证 Gmin 延续不会改变单稳态电路的解：
// 对纯线性电路，开启/关闭 Gmin 延续应得到相同的解。
func TestGminSteppingMatchesUnstepped(t *testing.T) {
	netlist := `
V1 [1,-1] [0,0,0,0,5]
R1 [1,2] [1000]
R2 [2,-1] [2000]
`
	run := func(gmin bool) float64 {
		con, err := load.LoadString(netlist)
		if err != nil {
			t.Fatalf("加载网表失败: %v", err)
		}
		con.Time, err = NewTimeMNA(1e-3)
		if err != nil {
			t.Fatalf("创建仿真时间失败: %v", err)
		}
		if gmin {
			t.Setenv("CIRCUIT_GMCONT", "1")
		} else {
			t.Setenv("CIRCUIT_GMCONT", "")
		}
		var v2 float64
		if err := TransientSimulation(con, func(voltages []float64) {
			if len(voltages) >= 2 {
				v2 = voltages[1]
			}
		}); err != nil {
			t.Fatalf("仿真失败: %v", err)
		}
		return v2
	}

	withGmin := run(true)
	without := run(false)
	if math.Abs(withGmin-without) > 1e-9 {
		t.Fatalf("Gmin 延续改变了线性电路解: with=%g without=%g", withGmin, without)
	}
	// 分压器理论值：5 * 2000/(1000+2000) ≈ 3.3333
	if math.Abs(without-3.3333333333) > 1e-6 {
		t.Fatalf("分压器解错误: 实际 %g, 期望约 3.33333", without)
	}
}
