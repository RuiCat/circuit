package time_test

// benchmark_test.go —— 端到端基准（外部测试包：load 依赖 element/time，内部测试包会成环）（K1 的 group m1/m4/m5/m6 判定口径）。
//
// 语料 = cmd/circuits/*.net（只读），4 个真实网表覆盖三类：纯电阻小电路 / 含储能电路 /
// 非线性电路 / ≥800 元件稀疏数字电路。每个语料两个 Benchmark：
//   - BenchmarkE2ELoad<Case>：只测 load.LoadString（解析 + 展开 + 元件装配）；
//   - BenchmarkE2EStep<Case>：只测 TransientSimulation 推进（Context 在 StopTimer 区间重建），
//     并用 metrics["steps/op"] 上报总步数；每步平均耗时 = ns/op ÷ steps/op（由 benchlib 聚合）。
//
// 仿真参数与 CLI 批处理默认值逐字一致（cmd/main.go:221-235）：-t 1e-4（语料清单同值）、
// -step 1e-6、-min-step 1e-9、-max-step 5e-4、-abs-tol 1e-6、-rel-tol 1e-4、
// -max-iter 200、-max-steps 10000；串行（不设 ParallelOpts），与 scripts/regressSnapshot.sh 同口径。
//
// 迭代数（牛顿迭代）出口由 m5/K4 落地，本文件当前不采集，bench.json 预留字段待回填。

import (
	"os"
	"path/filepath"
	"testing"

	"circuit/element"
	_ "circuit/element/register"
	etime "circuit/element/time"
	"circuit/load"
)

// benchNetlistCase 是一个端到端语料及其运行参数（参数逐字进 baselines/corpus.txt）。
type benchNetlistCase struct {
	name       string
	file       string
	targetTime float64
}

// benchNetlistCases 冻结语料清单（新增/删除都要同步 baselines/corpus.txt 与 thresholds.md）。
var benchNetlistCases = []benchNetlistCase{
	{name: "Resistor", file: "01_resistor_divider.net", targetTime: 1e-4},
	{name: "RcFilter", file: "02_rc_filter.net", targetTime: 1e-4},
	{name: "Diode", file: "03_diode_rectifier.net", targetTime: 1e-4},
	{name: "Ram16x8", file: "41_ram16x8.net", targetTime: 1e-4},
}

// benchNetlistRoot 返回仓库根目录（go test 的工作目录是包目录 element/time）。
// BENCH_NETLIST_DIR 可覆盖（供其它工作目录下复跑）。
func benchNetlistRoot() string {
	if dir := os.Getenv("BENCH_NETLIST_DIR"); dir != "" {
		return dir
	}
	return filepath.Join("..", "..")
}

// benchReadNetlist 读取只读语料；缺失即失败（静默跳过会让 group 名单与基线漂移）。
func benchReadNetlist(tb testing.TB, file string) string {
	tb.Helper()
	path := filepath.Join(benchNetlistRoot(), "cmd", "circuits", file)
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("读取语料失败: %v", err)
	}
	return string(data)
}

// benchNewContext 按 CLI 批处理默认参数构建仿真上下文（见文件头）。
func benchNewContext(text string, targetTime float64) (*element.Context, error) {
	con, err := load.LoadString(text)
	if err != nil {
		return nil, err
	}
	tm, err := etime.NewTimeMNA(targetTime)
	if err != nil {
		return nil, err
	}
	con.Time = tm
	con.InitResumeCond()
	if err := tm.SetTolerances(1e-6, 1e-4); err != nil {
		return nil, err
	}
	if err := tm.SetStepLimits(1e-9, 5e-4); err != nil {
		return nil, err
	}
	if err := tm.SetIterationLimits(200, tm.MaxElemIter()); err != nil {
		return nil, err
	}
	if err := tm.SetMaxTimeSteps(10000); err != nil {
		return nil, err
	}
	tm.SetTimeStep(1e-6)
	return con, nil
}

// benchRunCase 跑一次完整瞬态仿真，返回被回调的时间步数。
func benchRunCase(tb testing.TB, text string, targetTime float64) int {
	tb.Helper()
	con, err := benchNewContext(text, targetTime)
	if err != nil {
		tb.Fatalf("构建仿真上下文失败: %v", err)
	}
	steps := 0
	if err := etime.TransientSimulation(con, func([]float64) { steps++ }); err != nil {
		tb.Fatalf("仿真失败: %v", err)
	}
	return steps
}

// benchLoadCase 加载基准的公共实现。
func benchLoadCase(b *testing.B, file string) {
	text := benchReadNetlist(b, file)
	if _, err := load.LoadString(text); err != nil {
		b.Fatalf("语料不可加载: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		con, err := load.LoadString(text)
		if err != nil {
			b.Fatalf("加载失败: %v", err)
		}
		if con == nil {
			b.Fatal("加载返回 nil Context")
		}
	}
}

// benchStepCase 推进基准的公共实现：Context 每轮重建（仿真状态不可跨轮复用），
// 重建在 StopTimer 区间内，ns/op 只含 TransientSimulation。
func benchStepCase(b *testing.B, file string, targetTime float64) {
	text := benchReadNetlist(b, file)
	steps := benchRunCase(b, text, targetTime)
	if steps == 0 {
		b.Fatal("语料未产生任何时间步")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		con, err := benchNewContext(text, targetTime)
		if err != nil {
			b.Fatalf("构建仿真上下文失败: %v", err)
		}
		steps = 0
		b.StartTimer()
		if err := etime.TransientSimulation(con, func([]float64) { steps++ }); err != nil {
			b.Fatalf("仿真失败: %v", err)
		}
	}
	if steps == 0 {
		b.Fatal("仿真未产生任何时间步")
	}
	b.ReportMetric(float64(steps), "steps/op")
}

// BenchmarkE2ELoadResistor 纯电阻小电路的加载。
func BenchmarkE2ELoadResistor(b *testing.B) { benchLoadCase(b, "01_resistor_divider.net") }

// BenchmarkE2ELoadRcFilter 含储能元件（C/L）电路的加载。
func BenchmarkE2ELoadRcFilter(b *testing.B) { benchLoadCase(b, "02_rc_filter.net") }

// BenchmarkE2ELoadDiode 非线性（二极管）电路的加载。
func BenchmarkE2ELoadDiode(b *testing.B) { benchLoadCase(b, "03_diode_rectifier.net") }

// BenchmarkE2ELoadRam16x8 ≥800 元件稀疏数字电路的加载（总线展开 + 子电路展开）。
func BenchmarkE2ELoadRam16x8(b *testing.B) { benchLoadCase(b, "41_ram16x8.net") }

// BenchmarkE2EStepResistor 纯电阻小电路的每步推进。
func BenchmarkE2EStepResistor(b *testing.B) { benchStepCase(b, "01_resistor_divider.net", 1e-4) }

// BenchmarkE2EStepRcFilter 含储能电路的每步推进。
func BenchmarkE2EStepRcFilter(b *testing.B) { benchStepCase(b, "02_rc_filter.net", 1e-4) }

// BenchmarkE2EStepDiode 非线性电路的每步推进（m4 的判定口径）。
func BenchmarkE2EStepDiode(b *testing.B) { benchStepCase(b, "03_diode_rectifier.net", 1e-4) }

// BenchmarkE2EStepRam16x8 大稀疏数字电路的每步推进（m3/m5/m6 的判定口径）。
func BenchmarkE2EStepRam16x8(b *testing.B) { benchStepCase(b, "41_ram16x8.net", 1e-4) }

// TestE2EBenchCorpusSteps 校验 4 个端到端语料在进程内可用且步数可复现：
// 步数必须 >0，且同一语料两次独立运行的步数完全一致（否则 ns/step 指标无意义）。
func TestE2EBenchCorpusSteps(t *testing.T) {
	for _, c := range benchNetlistCases {
		text := benchReadNetlist(t, c.file)
		first := benchRunCase(t, text, c.targetTime)
		second := benchRunCase(t, text, c.targetTime)
		if first == 0 {
			t.Fatalf("语料 %s 未产生任何时间步", c.file)
		}
		if first != second {
			t.Fatalf("语料 %s 步数不可复现: 第一次 %d, 第二次 %d", c.file, first, second)
		}
		t.Logf("语料 %s: 步数=%d (targetTime=%g)", c.file, first, c.targetTime)
	}
}
