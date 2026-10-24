// multiRate_test.go：多速率调度的默认路径、开关优先级、快慢组同步与步数/耗时对比。
//
// 口径：测试网表是纯阻性分压器 + 两个合成元件（MR1 慢参数 / MR2 快参数，输出恒 0，
// 不反馈进矩阵），因此开关前后的时间网格与解应逐点一致，差异只体现在慢组的
// DoStep 调用数上（全局步数不变，见 multiRate.go 的"已知边界"）。
package time

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	stdtime "time"

	"circuit/element"
	"circuit/element/logic"
	"circuit/load"

	"testing"
)

// 生产元件示例：RAM 已实现多速率接口（C9/D10），默认 stride=1 不参与。
var _ MultiRateSteppable = (*logic.RAM)(nil)

// TestRamMultiRateStride 验证 RAM 的 MultiRateStride 读实例槽 7（默认 1，可设 >1）。
func TestRamMultiRateStride(t *testing.T) {
	data, err := os.ReadFile("../../cmd/circuits/58_cpu8v2_macro.net")
	if err != nil {
		t.Fatalf("读取 58 网表失败: %v", err)
	}
	con, err := load.LoadString(string(data))
	if err != nil {
		t.Fatalf("加载 58 失败: %v", err)
	}
	var ram element.NodeFace
	for _, node := range con.Nodelist {
		if base := node.Base(); base != nil && strings.HasSuffix(base.InstanceName, "RAM1") {
			ram = node
			break
		}
	}
	if ram == nil {
		t.Fatalf("58 中未找到 RAM1")
	}
	face, ok := element.ElementList[ram.Type()]
	if !ok {
		t.Fatalf("RAM 类型未注册")
	}
	st, ok := face.(MultiRateSteppable)
	if !ok {
		t.Fatalf("RAM 未实现 MultiRateSteppable")
	}
	if got := st.MultiRateStride(ram); got != 1 {
		t.Errorf("默认 stride=%d，期望 1", got)
	}
	ram.SetFloat64(7, 2)
	if got := st.MultiRateStride(ram); got != 2 {
		t.Errorf("设置 stride=2 后返回 %d", got)
	}
}

// multiRateTestTemplate 是测试网表模板：%d = 慢组的步长倍率（快组固定 1）。
// 实例名以元件类型名 MRTEST 开头，网表解析器才会按类型 MRTEST + 数字 ID 拆分。
const multiRateTestTemplate = `
V1 [1,-1] [0,0,0,0,5]
R1 [1,2] [1000]
R2 [2,-1] [1000]
MRTEST1 [2,3] [%d]
MRTEST2 [2,4] [1]
`

// mrCounters 是一个合成元件在一次仿真后的观测值。
type mrCounters struct {
	advances int       // DoStep 推进次数
	holds    int       // HoldStep 调用次数
	times    []float64 // 每次推进的仿真时刻
}

// multiRateResult 是一次仿真的观测结果。
type multiRateResult struct {
	steps        int       // 全局接受步数
	solves       []float64 // 每个接受步求解的时刻（回调当前时间）
	lastVoltages []float64 // 最后一次回调的节点电压
	slow         mrCounters
	fast         mrCounters
	wall         stdtime.Duration
}

// multiRateLoad 装载测试网表（slowStride = MR1 的步长倍率）。
func multiRateLoad(t *testing.T, slowStride int) *element.Context {
	t.Helper()
	if got := element.ElementListName["mrtest"]; got != mrTestElementType {
		t.Fatalf("合成元件 MRTEST 未注册：NodeType=%d，期望 %d", got, mrTestElementType)
	}
	con, err := load.LoadString(fmt.Sprintf(multiRateTestTemplate, slowStride))
	if err != nil {
		t.Fatalf("装载多速率测试网表失败: %v", err)
	}
	con.Backend.Bind(false, false)
	return con
}

// runMultiRateSim 在给定 Context 上按固定参数跑一次瞬态仿真（只收集步数/时刻/电压/墙钟）。
// setup 在仿真启动前配置 TimeMNA（如 EnableMultiRate / DisableMultiRate）。
func runMultiRateSim(t *testing.T, con *element.Context, setup func(*TimeMNA)) multiRateResult {
	t.Helper()
	tm, err := NewTimeMNA(1e-3)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}
	if setup != nil {
		setup(tm)
	}
	if err := tm.SetTolerances(1e-6, 1e-4); err != nil {
		t.Fatalf("设置容差失败: %v", err)
	}
	if err := tm.SetStepLimits(1e-9, 1e-5); err != nil {
		t.Fatalf("设置步长失败: %v", err)
	}
	if err := tm.SetIterationLimits(200, 20); err != nil {
		t.Fatalf("设置迭代上限失败: %v", err)
	}
	if err := tm.SetMaxTimeSteps(10000); err != nil {
		t.Fatalf("设置最大步数失败: %v", err)
	}
	tm.SetTimeStep(1e-6)
	con.Time = tm
	con.InitResumeCond()

	res := multiRateResult{}
	start := stdtime.Now()
	if err := TransientSimulation(con, func(voltages []float64) {
		res.solves = append(res.solves, con.CurrentTime())
		res.lastVoltages = append([]float64(nil), voltages...)
	}); err != nil {
		t.Fatalf("瞬态仿真失败: %v", err)
	}
	res.wall = stdtime.Since(start)
	res.steps = tm.GoodSteps()
	return res
}

// runMultiRateCase 在 runMultiRateSim 之上读回两个合成元件的观测槽。
// 只有含 MRTEST 实例的网表才能用它；纯 V/R 网表用 runMultiRateSim。
func runMultiRateCase(t *testing.T, con *element.Context, setup func(*TimeMNA)) multiRateResult {
	t.Helper()
	res := runMultiRateSim(t, con, setup)
	res.slow = mrCountersOf(t, con, "MRTEST1")
	res.fast = mrCountersOf(t, con, "MRTEST2")
	return res
}

// mrCountersOf 按实例名读回合成元件的观测槽。
func mrCountersOf(t *testing.T, con *element.Context, name string) mrCounters {
	t.Helper()
	for _, elem := range con.Nodelist {
		if elem.Base().InstanceName != name {
			continue
		}
		return mrCounters{
			advances: int(elem.GetFloat64(mrSlotAdvances)),
			holds:    int(elem.GetFloat64(mrSlotHolds)),
			times:    append([]float64(nil), mrTimesOf(elem)...),
		}
	}
	t.Fatalf("找不到实例 %s", name)
	return mrCounters{}
}

// mrExpectedAdvances 返回 stride 倍率下慢组应有的推进次数（首步推进，之后每 stride 步一次）。
func mrExpectedAdvances(stepCount, stride int) int {
	if stepCount <= 0 || stride <= 0 {
		return 0
	}
	return (stepCount + stride - 1) / stride
}

// mrSolveTimes 从回调时刻还原"每个接受步的求解时刻"：
// 第 0 步在 t=0 求解；第 j 步的求解时刻 = 第 j-1 步接受后的回调时刻。
func mrSolveTimes(solves []float64) []float64 {
	times := make([]float64, len(solves))
	for j := range solves {
		if j == 0 {
			times[j] = 0
			continue
		}
		times[j] = solves[j-1]
	}
	return times
}

// TestMultiRateDefaultOff 验证默认路径（开关关）行为不变：所有元件每个接受步都走 DoStep，
// HoldStep 一次都不调用——包括声明了倍率参数的慢组元件。
func TestMultiRateDefaultOff(t *testing.T) {
	t.Setenv(multiRateEnvName, "")
	res := runMultiRateCase(t, multiRateLoad(t, 4), nil)
	if res.slow.advances != res.steps || res.slow.holds != 0 {
		t.Fatalf("默认路径下慢参数元件也必须每步 DoStep：steps=%d advances=%d holds=%d",
			res.steps, res.slow.advances, res.slow.holds)
	}
	if res.fast.advances != res.steps || res.fast.holds != 0 {
		t.Fatalf("默认路径下快组必须每步 DoStep：steps=%d advances=%d holds=%d",
			res.steps, res.fast.advances, res.fast.holds)
	}
	t.Logf("MRTEST default-off steps=%d slowAdvances=%d slowHolds=%d fastAdvances=%d",
		res.steps, res.slow.advances, res.slow.holds, res.fast.advances)
}

// TestMultiRateEnvSwitch 验证 CIRCUIT_MULTIRATE 环境变量开关：非空启用、0 关闭。
func TestMultiRateEnvSwitch(t *testing.T) {
	t.Setenv(multiRateEnvName, "1")
	on := runMultiRateCase(t, multiRateLoad(t, 4), nil)
	want := mrExpectedAdvances(on.steps, 4)
	if on.slow.advances != want {
		t.Fatalf("环境开关启用后慢组推进次数=%d，期望 %d（steps=%d stride=4）",
			on.slow.advances, want, on.steps)
	}
	if on.slow.holds == 0 {
		t.Fatal("环境开关启用后慢组必须出现 HoldStep（非宏步只重驱动输出）")
	}
	if on.fast.advances != on.steps || on.fast.holds != 0 {
		t.Fatalf("快组不得被节流：steps=%d advances=%d holds=%d",
			on.steps, on.fast.advances, on.fast.holds)
	}

	t.Setenv(multiRateEnvName, "0")
	off := runMultiRateCase(t, multiRateLoad(t, 4), nil)
	if off.slow.advances != off.steps || off.slow.holds != 0 {
		t.Fatalf("CIRCUIT_MULTIRATE=0 必须回到默认路径：steps=%d advances=%d holds=%d",
			off.steps, off.slow.advances, off.slow.holds)
	}
}

// TestMultiRateExplicitDisableBeatsEnv 验证 Context 显式关闭优先于环境变量。
func TestMultiRateExplicitDisableBeatsEnv(t *testing.T) {
	t.Setenv(multiRateEnvName, "1")
	res := runMultiRateCase(t, multiRateLoad(t, 4), func(tm *TimeMNA) { tm.DisableMultiRate() })
	if res.slow.advances != res.steps || res.slow.holds != 0 {
		t.Fatalf("显式 DisableMultiRate 应覆盖环境变量：steps=%d advances=%d holds=%d",
			res.steps, res.slow.advances, res.slow.holds)
	}
}

// TestMultiRateSlowFastSync 验证快慢组同步：慢组只在全局接受步 0、stride、2*stride… 推进，
// 快组每步推进；全局时间网格、步数与解和默认路径逐点一致。
func TestMultiRateSlowFastSync(t *testing.T) {
	t.Setenv(multiRateEnvName, "")
	baseline := runMultiRateCase(t, multiRateLoad(t, 4), func(tm *TimeMNA) { tm.DisableMultiRate() })
	enabled := runMultiRateCase(t, multiRateLoad(t, 4), func(tm *TimeMNA) { tm.EnableMultiRate() })

	if enabled.steps != baseline.steps {
		t.Fatalf("多速率不得改变全局步数：off=%d on=%d", baseline.steps, enabled.steps)
	}
	if !reflect.DeepEqual(enabled.solves, baseline.solves) {
		t.Fatalf("多速率不得改变时间网格：off=%v on=%v", baseline.solves, enabled.solves)
	}
	if !reflect.DeepEqual(enabled.lastVoltages, baseline.lastVoltages) {
		t.Fatalf("多速率不得改变解：off=%v on=%v", baseline.lastVoltages, enabled.lastVoltages)
	}

	want := mrExpectedAdvances(enabled.steps, 4)
	if enabled.slow.advances != want {
		t.Fatalf("慢组推进次数=%d，期望 %d（steps=%d stride=4）",
			enabled.slow.advances, want, enabled.steps)
	}
	if len(enabled.slow.times) != want {
		t.Fatalf("慢组推进时刻数=%d，期望 %d", len(enabled.slow.times), want)
	}
	if enabled.slow.holds == 0 {
		t.Fatal("慢组在非宏步必须走 HoldStep")
	}
	if enabled.fast.advances != enabled.steps || enabled.fast.holds != 0 {
		t.Fatalf("快组必须每步推进：steps=%d advances=%d holds=%d",
			enabled.steps, enabled.fast.advances, enabled.fast.holds)
	}

	solveTimes := mrSolveTimes(enabled.solves)
	for k := range enabled.slow.times {
		if got, want := enabled.slow.times[k], solveTimes[k*4]; got != want {
			t.Fatalf("第 %d 次慢组推进时刻=%g，期望全局第 %d 个接受步的求解时刻 %g",
				k, got, k*4, want)
		}
	}
	t.Logf("MRTEST sync steps=%d slowAdvances=%d slowHolds=%d fastAdvances=%d",
		enabled.steps, enabled.slow.advances, enabled.slow.holds, enabled.fast.advances)
}

// TestMultiRateNoSlowElementIsNoop 验证启用开关但无慢元件时回落默认路径（plan=nil）。
func TestMultiRateNoSlowElementIsNoop(t *testing.T) {
	t.Setenv(multiRateEnvName, "")
	con, err := load.LoadString(strategyTestNetlist)
	if err != nil {
		t.Fatalf("装载基线网表失败: %v", err)
	}
	con.Backend.Bind(false, false)
	res := runMultiRateSim(t, con, func(tm *TimeMNA) { tm.EnableMultiRate() })
	if res.steps == 0 {
		t.Fatal("无慢元件时仿真应正常完成（默认路径）")
	}
}

// TestMultiRatePlanNil 验证计划构建的 nil 路径：开关关闭、或开关打开但电路里没有
// 倍率 >1 的元件时，newMultiRatePlan 必须返回 nil，调用方才会走原
// CallMark/ParallelCallMark 路径。倍率 1 的实例不算慢组（与 MultiRateSteppable 契约一致）。
func TestMultiRatePlanNil(t *testing.T) {
	t.Setenv(multiRateEnvName, "")
	if plan, err := newMultiRatePlan(multiRateLoad(t, 4), false); err != nil || plan != nil {
		t.Fatalf("开关关闭时应返回 (nil,nil)：plan=%v err=%v", plan, err)
	}

	noSlow, err := load.LoadString(strategyTestNetlist)
	if err != nil {
		t.Fatalf("装载无慢元件网表失败: %v", err)
	}
	if plan, err := newMultiRatePlan(noSlow, true); err != nil || plan != nil {
		t.Fatalf("无慢元件时应返回 (nil,nil)：plan=%v err=%v", plan, err)
	}

	if plan, err := newMultiRatePlan(multiRateLoad(t, 1), true); err != nil || plan != nil {
		t.Fatalf("倍率 1 的实例不构成慢组，应返回 (nil,nil)：plan=%v err=%v", plan, err)
	}

	withSlow := multiRateLoad(t, 4)
	plan, err := newMultiRatePlan(withSlow, true)
	if err != nil {
		t.Fatalf("构建多速率计划失败: %v", err)
	}
	if plan == nil {
		t.Fatal("有慢元件时应构建计划，实际为 nil")
	}
	if plan.slow != 1 || len(plan.entries) != len(withSlow.Nodelist) {
		t.Fatalf("计划内容不符：slow=%d entries=%d nodelist=%d",
			plan.slow, len(plan.entries), len(withSlow.Nodelist))
	}
}

// TestMultiRateParallelRejected 验证多速率与并行 DoStep 互斥（显式报错，不静默降级）。
func TestMultiRateParallelRejected(t *testing.T) {
	con := multiRateLoad(t, 4)
	tm, err := NewTimeMNA(1e-3)
	if err != nil {
		t.Fatalf("创建 TimeMNA 失败: %v", err)
	}
	tm.EnableMultiRate()
	if err := tm.SetStepLimits(1e-9, 1e-5); err != nil {
		t.Fatalf("设置步长失败: %v", err)
	}
	tm.SetTimeStep(1e-6)
	con.Time = tm
	con.InitResumeCond()
	con.ParallelOpts = &element.ParallelOptions{StampWorkers: 2}
	err = TransientSimulation(con, func([]float64) {})
	if err == nil || !strings.Contains(err.Error(), "互斥") {
		t.Fatalf("并行 + 多速率应显式报错（含「互斥」），实际 err=%v", err)
	}
	t.Logf("MRTEST parallel-rejected err=%v", err)
}

// TestMultiRateStepWallClockComparison 给出要求的两组对比数据：
// 同一网表/同一全局步数下，步长倍率 4 与 8 时慢组 DoStep/HoldStep 次数与墙钟。
// 全局步数不下降（多速率骨架只省慢组 DoStep，真正的大步长需矩阵分块，见 multiRate.go）。
func TestMultiRateStepWallClockComparison(t *testing.T) {
	t.Setenv(multiRateEnvName, "")
	scenarios := []struct {
		name   string
		stride int
		enable bool
	}{
		{"off", 4, false},
		{"stride4", 4, true},
		{"stride8", 8, true},
	}
	for _, sc := range scenarios {
		setup := func(tm *TimeMNA) {}
		if sc.enable {
			setup = func(tm *TimeMNA) { tm.EnableMultiRate() }
		}
		res := runMultiRateCase(t, multiRateLoad(t, sc.stride), setup)
		if sc.enable {
			if want := mrExpectedAdvances(res.steps, sc.stride); res.slow.advances != want {
				t.Fatalf("%s: 慢组推进=%d，期望 %d", sc.name, res.slow.advances, want)
			}
		} else if res.slow.advances != res.steps {
			t.Fatalf("%s: 默认路径慢组应每步推进（%d != %d）", sc.name, res.slow.advances, res.steps)
		}
		t.Logf("MRTEST compare scenario=%s steps=%d slowAdvances=%d slowHolds=%d fastAdvances=%d wall=%v",
			sc.name, res.steps, res.slow.advances, res.slow.holds, res.fast.advances, res.wall)
	}
}
