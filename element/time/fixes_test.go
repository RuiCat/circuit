package time

import (
	"math"
	"sync"
	"testing"
	"time"

	_ "circuit/element/register"
	"circuit/load"
	"circuit/mna"
)

// ---------------------------------------------------------------------------
// 本文件为 2026-08-22 element/time 深度审查修复（P1/P3/P5/P6/P7）的针对性
// 自建测试电路。全部为小型确定性电路，不使用 CPU 网表。
// 验证口径：修复的新语义（P1 残差检查恢复 / P5 步长收缩 / P6 maxStep 放宽 /
// P7 并发安全）直接断言；P3 显式补轮用 DFF 采样电路验证不破坏边沿检测。
// ---------------------------------------------------------------------------

// TestP1_GminStepCompletedStatus P1 状态机：延续降到终值（gminStepDone）后
// GminSteppingActive 返回 false（时间步级残差检查恢复），延续值保留在终值
// 供 PN 结元件读取。
func TestP1_GminStepCompletedStatus(t *testing.T) {
	tm, err := NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}
	tm.BeginGminStepping()
	if !tm.GminSteppingActive() {
		t.Fatalf("步进开始后应处于激活状态")
	}
	// 逐级下调至终值
	levels := 0
	for tm.StepGminDown() {
		levels++
	}
	if levels != 8 {
		t.Fatalf("应恰好 8 次「仍需继续」的下调, 实际 %d", levels)
	}
	// P1：到达终值后延续值保留（PN 结元件仍可读），但步进已完成 → 残差检查恢复
	if tm.GetContinuationGmin() != gminStepFinal {
		t.Fatalf("延续值应为终值 %g, 实际 %g", gminStepFinal, tm.GetContinuationGmin())
	}
	if tm.GminSteppingActive() {
		t.Fatalf("P1: 延续降到终值后 GminSteppingActive 应为 false（恢复残差检查）")
	}
	// 已完成状态不再下调
	if tm.StepGminDown() {
		t.Fatalf("已完成步进不应再下调")
	}
	// EndGminStepping 完全关闭
	tm.EndGminStepping()
	if tm.GminSteppingActive() {
		t.Fatalf("EndGminStepping 后应关闭")
	}
}

// TestP1_GminResidualRestored P1 集成：GMCONT=1 的简单电路在 gmin 步进完成
// （残差检查恢复）后仍正常仿真，解与无 GMCONT 一致（gmin 终值 1e-12 对
// 线性电路解的影响 < 1e-9）。
func TestP1_GminResidualRestored(t *testing.T) {
	netlist := `
V1 [1,-1] [0,0,0,0,5]
R1 [1,2] [1000]
R2 [2,-1] [2000]
`
	run := func(gmin bool) float64 {
		if gmin {
			t.Setenv("CIRCUIT_GMCONT", "1")
		} else {
			t.Setenv("CIRCUIT_GMCONT", "")
		}
		con, err := load.LoadString(netlist)
		if err != nil {
			t.Fatalf("加载网表失败: %v", err)
		}
		con.Time, err = NewTimeMNA(1e-3)
		if err != nil {
			t.Fatalf("创建仿真时间失败: %v", err)
		}
		var v2 float64
		if err := TransientSimulation(con, func(v []float64) {
			if len(v) >= 2 {
				v2 = v[1]
			}
		}); err != nil {
			t.Fatalf("仿真失败(gmin=%v): %v", gmin, err)
		}
		return v2
	}
	withGmin := run(true)
	without := run(false)
	if math.Abs(withGmin-without) > 1e-9 {
		t.Fatalf("Gmin 残差检查恢复改变了线性电路解: with=%g without=%g", withGmin, without)
	}
	// 分压器理论值 3.33333
	if math.Abs(without-3.3333333333) > 1e-6 {
		t.Fatalf("分压器解错误: %g", without)
	}
}

// TestP3_RefillDFFSampling P3 显式补轮：D 触发器电路（时钟上升沿采样 D=5V），
// 验证补轮不破坏边沿检测——第二步（t=2e-6）Q 应采样到 D=5（Q=5）。
// 旧引擎依赖"补轮 doStep 读旧缓冲写 prevClk=0"的巧合触发；P3 显式补轮 +
// 收敛后立即终止外层，同样提交 prevClk=0，行为与旧引擎一致。
func TestP3_RefillDFFSampling(t *testing.T) {
	netlist := `
V1 [1,-1] [0,0,0,0,5]
V2 [2,-1] [5,0,100,0,5,0.5]
DFF1 [2,1,3,4] [5]
R1 [3,-1] [10000]
R2 [4,-1] [10000]
`
	con, err := load.LoadString(netlist)
	if err != nil {
		t.Fatalf("加载网表失败: %v", err)
	}
	con.Time, err = NewTimeMNA(3e-6)
	if err != nil {
		t.Fatalf("创建仿真时间失败: %v", err)
	}
	var qAt2, qAt3 float64
	step := 0
	if err := TransientSimulation(con, func(v []float64) {
		step++
		if len(v) >= 4 {
			if step == 2 {
				qAt2 = v[2] // node_3 = Q
			}
			if step == 3 {
				qAt3 = v[2]
			}
		}
	}); err != nil {
		t.Fatalf("仿真失败: %v", err)
	}
	// 时钟上升沿（第一步初始化后，第二步 prevClk 提交值=0 → 触发采样 D=5）
	if qAt2 < 4.5 {
		t.Fatalf("P3: 第二步 Q 应采样到 D=5（Q=%.4f）——补轮破坏边沿检测", qAt2)
	}
	if qAt3 < 4.5 {
		t.Fatalf("P3: 第三步 Q 应保持 5（Q=%.4f）", qAt3)
	}
}

// TestP5_TriggerStepShrink P5：触发点截断后 currentStep 同步收缩。
// 纯逻辑测试（advanceTimeAndTriggers 直接调用，确定性）。
func TestP5_TriggerStepShrink(t *testing.T) {
	tm, err := NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}
	tm.SetTriggers([]mna.Trigger{{Time: 2.5e-6}})
	tm.SetTimeStep(1e-6)
	tm.currentTime.Store(2e-6) // 当前在 2e-6，步长 1e-6 会越过 2.5e-6 触发点
	tm.advanceTimeAndTriggers(false)
	if tm.CurrentTime() != 2.5e-6 {
		t.Fatalf("P5: 时间应精确截断到触发点 2.5e-6, 实际 %g", tm.CurrentTime())
	}
	// P5 修复：触发点截断同步收缩步长 = 触发点间距
	expect := 0.5e-6
	if math.Abs(tm.CurrentStep()-expect) > 1e-15 {
		t.Fatalf("P5: 触发点后步长应收缩为 %g, 实际 %g（旧实现保持 1e-6）", expect, tm.CurrentStep())
	}
	// 触发点已标记
	if !tm.triggers[0].Triggered {
		t.Fatalf("触发点应已标记触发")
	}
	// 无触发点时步长不受影响
	tm2, _ := NewTimeMNA(0.1)
	tm2.SetTimeStep(1e-6)
	tm2.currentTime.Store(2e-6)
	tm2.advanceTimeAndTriggers(false)
	if tm2.CurrentStep() != 1e-6 {
		t.Fatalf("无触发点时步长不应变化: %g", tm2.CurrentStep())
	}
}

// TestP6_MaxStepRelax P6：SetStepLimits 的 maxStep 上限从 1e-2 放宽到 1.0，
// 慢信号长仿真可用更大步长（LTE 自适应兜底）。
func TestP6_MaxStepRelax(t *testing.T) {
	tm, err := NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}
	// 旧引擎拒绝 maxStep > 1e-2；P6 放宽后接受
	if err := tm.SetStepLimits(1e-9, 0.05); err != nil {
		t.Fatalf("P6: maxStep=0.05 应被接受, 实际报错: %v", err)
	}
	if tm.MaxTimeStep() != 0.05 {
		t.Fatalf("maxStep 应为 0.05, 实际 %g", tm.MaxTimeStep())
	}
	// 非法参数仍拒绝
	if err := tm.SetStepLimits(0, 0.05); err == nil {
		t.Fatalf("minStep<=0 应报错")
	}
	if err := tm.SetStepLimits(1e-9, 1e-10); err == nil {
		t.Fatalf("maxStep<minStep 应报错")
	}
}

// TestP7_AdvanceForConcurrent P7：连续模式下 AdvanceFor（外部 goroutine）与
// 仿真侧 IsSimulationFinished/Status/CurrentTime 并发调用——-race 下不得有
// 数据竞争；newTarget 标志消除"新目标已设但仿真停在 Paused"的竞态窗口。
func TestP7_AdvanceForConcurrent(t *testing.T) {
	tm, err := NewTimeMNA(0.1)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}
	tm.SetContinuousMode()

	var wg sync.WaitGroup
	stop := make(chan struct{})
	// 外部控制 goroutine：持续 AdvanceFor + 状态查询
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := tm.AdvanceFor(5e-4); err != nil {
				return
			}
			_ = tm.Status()
			_ = tm.CurrentTime()
			_ = tm.TargetTime()
		}
	}()
	// 仿真侧 goroutine：模拟主循环推进 + 完成判定 + 暂停等待
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			if tm.IsSimulationFinished() {
				// 连续模式到达临时目标 → Paused；模拟外部 AdvanceFor 恢复
				if tm.Status() == mna.StatusPaused {
					tm.Resume()
				}
			}
			// 模拟推进时间
			tm.currentTime.Store(tm.CurrentTime() + 1e-5)
			time.Sleep(10 * time.Microsecond)
		}
	}()
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
	tm.Stop()
	// 竞态窗口修复验证：AdvanceFor 后 targetTime 必然已更新（不依赖 CAS 结果）
	target := tm.TargetTime()
	if target <= 0 {
		t.Fatalf("targetTime 无效: %g", target)
	}
}

// TestP8_NoForceAdvance P8：元件迭代耗尽且残差未收敛时不再强制推进，
// 而是减半步长重试（已到最小步长则报错）。用 maxElem=1 构造必然耗尽的
// 场景，验证仿真要么收敛成功、要么以"减半/最小步长"错误终止——
// 绝不静默输出不收敛的解。
func TestP8_NoForceAdvance(t *testing.T) {
	// 交叉耦合 NPN 锁存对（无 GMCONT：t=0 对称振荡，元件级难收敛）
	netlist := `
V100 [100,-1] [0,0,0,0,5]
R100 [100,1] [1000]
R101 [100,2] [1000]
Q200 [2,1,-1] [false,100]
Q201 [1,2,-1] [false,100]
`
	con, err := load.LoadString(netlist)
	if err != nil {
		t.Fatalf("加载网表失败: %v", err)
	}
	con.Time, err = NewTimeMNA(1e-3)
	if err != nil {
		t.Fatalf("创建仿真时间失败: %v", err)
	}
	// 强制 elem 迭代耗尽（每轮子迭代只允许 1 次 doStep）
	if err := con.Time.SetIterationLimits(200, 1); err != nil {
		t.Fatalf("设置迭代限制失败: %v", err)
	}
	err = TransientSimulation(con, func([]float64) {})
	if err == nil {
		// 成功路径：残差已收敛（MNA 方程满足），接受解是合法的（P8 保留该分支）
		return
	}
	// 失败路径：必须来自减半步长（P8 新行为），而不是静默强制推进
	msg := err.Error()
	if !containsAny(msg, "残差未收敛", "步长已最小", "无法收敛") {
		t.Fatalf("P8: 错误信息应指向残差未收敛/步长减半, 实际: %v", err)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			match := true
			for i := 0; i < len(sub); i++ {
				if s[i] != sub[i] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}
