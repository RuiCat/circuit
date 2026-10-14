package maths

// benchlib_test.go —— K1 基准脚手架（冻结契约见 runs/circuit-perf-optimization/40-plan.md「接口契约 K1」）。
//
// 本文件只被 scripts/benchCompare.sh 调用，分四块：
//  1. 固定语料构造：全部基准输入由固定种子生成，保证 -count=N 的多次重复测同一份数据；
//  2. 原始输出聚合：解析 `go test -bench` 文本 → bench.json / bench.csv（含 maxMinRatio）；
//  3. 判定：基线 + 本次结果 → verdict.json（「性能阈值」与「噪声地板」分列，见 thresholds.md）；
//  4. maxMinRatio 自身的用例（判定口径本身可测）。
//
// 约束：只用标准库；不改动既有 7 个 Benchmark 的名称与所在文件。

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// benchlibRunFlag 是可重复的命令行参数（-benchlib-raw pkg=path、-benchlib-command "..."）。
type benchlibRunFlag []string

// String 返回参数拼接结果（flag.Value 接口）。
func (f *benchlibRunFlag) String() string { return strings.Join(*f, " | ") }

// Set 追加一个参数值（flag.Value 接口）。
func (f *benchlibRunFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

var (
	benchlibRawFiles   benchlibRunFlag
	benchlibCommands   benchlibRunFlag
	benchlibGroup      = flag.String("benchlib-group", "", "group 名称（m1..m6）")
	benchlibCommit     = flag.String("benchlib-commit", "", "被测 commit（逐字写入基线头）")
	benchlibLoadAvg    = flag.String("benchlib-loadavg", "", "采集时 load average（逐字写入基线头）")
	benchlibBenchtime  = flag.String("benchlib-benchtime", "", "被测命令的 -benchtime（逐字写入基线头）")
	benchlibCount      = flag.Int("benchlib-count", 0, "被测命令的 -count（=runs）")
	benchlibJSONOut    = flag.String("benchlib-json", "", "bench.json 输出路径")
	benchlibCSVOut     = flag.String("benchlib-csv", "", "bench.csv 输出路径")
	benchlibExpect     = flag.String("benchlib-expect", "", "该 group 的 benchmark 名单正则（校验实际运行集合与名单一致）")
	benchlibBaseline   = flag.String("benchlib-baseline", "", "基线 JSON 路径")
	benchlibCurrent    = flag.String("benchlib-current", "", "本次 bench.json 路径")
	benchlibVerdictOut = flag.String("benchlib-verdict", "", "verdict.json 输出路径")
)

func init() {
	flag.Var(&benchlibRawFiles, "benchlib-raw", "go test -bench 原始输出（pkg=path，可重复）")
	flag.Var(&benchlibCommands, "benchlib-command", "被测命令逐字（可重复）")
}

// ---------------------------------------------------------------------------
// 1. 固定语料构造（固定种子 → -count=N 的多次重复测同一份数据）
// ---------------------------------------------------------------------------

// benchlibRand 返回固定种子的随机源（不使用全局 rand，避免 randautoseed 造成每次重复数据漂移）。
func benchlibRand(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

// benchlibDenseDominant 构造 n×n 稠密对角占优矩阵（值域 [0,1)，对角 = 行和+1）。
func benchlibDenseDominant(n int, seed int64) Matrix[float64] {
	r := benchlibRand(seed)
	m := NewDenseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		sum := 0.0
		for j := 0; j < n; j++ {
			v := r.Float64()
			m.Set(i, j, v)
			sum += v
		}
		m.Set(i, i, sum+1)
	}
	return m
}

// benchlibSparseBand 构造 n×n 带状稀疏矩阵：|i-j| ≤ band 内取值，对角 = 行和+1（对角占优）。
func benchlibSparseBand(n, band int, seed int64) *rowSparseMatrix[float64] {
	r := benchlibRand(seed)
	m := newRowSparseMatrix[float64](n, n)
	for i := 0; i < n; i++ {
		sum := 0.0
		lo, hi := i-band, i+band
		if lo < 0 {
			lo = 0
		}
		if hi >= n {
			hi = n - 1
		}
		for j := lo; j <= hi; j++ {
			if j == i {
				continue
			}
			v := r.Float64()
			m.Set(i, j, v)
			sum += v
		}
		m.Set(i, i, sum+1)
	}
	return m
}

// benchlibSparseHub 构造"含 hub 行"的稀疏矩阵（少量高连接行 + 带状主体），
// 用于触发 computeStaticPerm 的最小度重排分支（maxDeg > 10×avgDeg）。
func benchlibSparseHub(n, band, hubs int, seed int64) *rowSparseMatrix[float64] {
	m := benchlibSparseBand(n, band, seed)
	r := benchlibRand(seed + 1)
	for h := 0; h < hubs; h++ {
		row := (h * 97) % n
		for k := 0; k < 40; k++ {
			col := r.Intn(n)
			if col == row {
				continue
			}
			m.Set(row, col, r.Float64())
		}
	}
	return m
}

// benchlibAdjacency 取矩阵每行的列号集合（供 MinimumDegreeOrder 基准使用，行内升序）。
func benchlibAdjacency(m *rowSparseMatrix[float64]) [][]int {
	adj := make([][]int, m.rows)
	for i := 0; i < m.rows; i++ {
		cols, _ := m.GetRow(i)
		sort.Ints(cols)
		adj[i] = cols
	}
	return adj
}

// benchlibPatchedUpdateMatrix 构造带 patches 个缓存修改的 updateMatrix（基础矩阵固定）。
func benchlibPatchedUpdateMatrix(base Matrix[float64], patches int, seed int64) UpdateMatrix[float64] {
	r := benchlibRand(seed)
	um := NewUpdateMatrix(base)
	rows, cols := base.Rows(), base.Cols()
	for k := 0; k < patches; k++ {
		um.Set(r.Intn(rows), r.Intn(cols), r.Float64()+1)
	}
	return um
}

// ---------------------------------------------------------------------------
// 2. 统计与聚合
// ---------------------------------------------------------------------------

// maxMinRatio 返回同一 benchmark 多次重复的 max/min（<1 或非有限值 → +Inf 视为不可用）。
// K1 用它作为「本轮噪声上界」的标定量：>1.15 即拒绝写基线。
func maxMinRatio(values []float64) float64 {
	if len(values) == 0 {
		return math.Inf(1)
	}
	lo, hi := values[0], values[0]
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return math.Inf(1)
		}
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if lo <= 0 {
		return math.Inf(1)
	}
	return hi / lo
}

// benchlibMedian 返回样本中位数（判定口径统一用中位数，见 thresholds.md）。
func benchlibMedian(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// benchlibMax 返回最大值（D29 的聚集性判据用；空切片返回 NaN）。
func benchlibMax(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// benchlibMin 返回最小值（D29 的聚集性判据用；空切片返回 NaN）。
func benchlibMin(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// benchlibEntryDoc 是 bench.json 的单条 benchmark 记录（字段名冻结于 K1）。
type benchlibEntryDoc struct {
	Name        string             `json:"name"`
	Package     string             `json:"package"`
	NsPerOp     []float64          `json:"nsPerOp"`
	MaxMinRatio float64            `json:"maxMinRatio"`
	BytesPerOp  float64            `json:"bytesPerOp"`
	AllocsPerOp float64            `json:"allocsPerOp"`
	Metrics     map[string]float64 `json:"metrics,omitempty"`
	NsPerStep   float64            `json:"nsPerStep,omitempty"`
}

// benchlibDoc 是 bench.json 的顶层结构。
type benchlibDoc struct {
	Group                string             `json:"group"`
	Commit               string             `json:"commit"`
	GoVersion            string             `json:"goVersion"`
	Nproc                int                `json:"nproc"`
	LoadAvg              string             `json:"loadAvg"`
	Benchtime            string             `json:"benchtime"`
	Count                int                `json:"count"`
	Runs                 int                `json:"runs"`
	Statistic            string             `json:"statistic"`
	GeneratedAt          string             `json:"generatedAt"`
	ThresholdMaxMinRatio float64            `json:"thresholdMaxMinRatio"`
	PerfThresholdPct     float64            `json:"perfThresholdPct"`
	NoiseSafetyFactor    float64            `json:"noiseSafetyFactor"`
	Commands             []string           `json:"commands"`
	Entries              []benchlibEntryDoc `json:"entries"`
}

// benchlibSample 聚合过程中的单条 benchmark 累计样本。
type benchlibSample struct {
	name     string
	pkg      string
	ns       []float64
	metrics  map[string][]float64
	bytes    []float64
	allocs   []float64
	firstPos int
}

var benchlibLineRe = regexp.MustCompile(`^(Benchmark\S+)\s+(\d+)\s+([0-9]+(?:\.[0-9]+)?(?:[eE][-+]?[0-9]+)?)\s+ns/op(?:\s+(.*))?$`)

var benchlibNameSuffixRe = regexp.MustCompile(`-[0-9]+$`)

// parseBenchlibRaw 解析一份 `go test -bench` 原始输出，按出现顺序追加到 samples。
func parseBenchlibRaw(data string, pkg string, samples map[string]*benchlibSample, order *[]string) error {
	seq := 0
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimRight(line, "\r")
		m := benchlibLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := benchlibNameSuffixRe.ReplaceAllString(m[1], "")
		ns, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			return fmt.Errorf("解析 ns/op 失败: %q", line)
		}
		key := pkg + "\x00" + name
		s := samples[key]
		if s == nil {
			s = &benchlibSample{name: name, pkg: pkg, metrics: map[string][]float64{}, firstPos: seq}
			samples[key] = s
			*order = append(*order, key)
		}
		seq++
		s.ns = append(s.ns, ns)
		fields := strings.Fields(m[4])
		for i := 0; i+1 < len(fields); i += 2 {
			val, err := strconv.ParseFloat(fields[i], 64)
			if err != nil {
				return fmt.Errorf("解析指标失败: %q (%q)", line, fields[i])
			}
			unit := fields[i+1]
			switch unit {
			case "B/op":
				s.bytes = append(s.bytes, val)
			case "kB/op":
				s.bytes = append(s.bytes, val*1000)
			case "MB/op":
				s.bytes = append(s.bytes, val*1e6)
			case "allocs/op":
				s.allocs = append(s.allocs, val)
			default:
				s.metrics[unit] = append(s.metrics[unit], val)
			}
		}
	}
	return nil
}

// TestBenchlibAggregate 由 scripts/benchCompare.sh 调用：把多份原始输出聚合成 bench.json / bench.csv。
// 未传 -benchlib-json 时跳过，保证裸 `go test ./maths/...` 仍然通过。
func TestBenchlibAggregate(t *testing.T) {
	if *benchlibJSONOut == "" {
		t.Skip("未传 -benchlib-json（本用例仅由 scripts/benchCompare.sh 驱动）")
	}
	if len(benchlibRawFiles) == 0 {
		t.Fatalf("缺少 -benchlib-raw（至少一份原始输出）")
	}
	samples := map[string]*benchlibSample{}
	var order []string
	for _, spec := range benchlibRawFiles {
		eq := strings.Index(spec, "=")
		if eq <= 0 {
			t.Fatalf("非法 -benchlib-raw %q（要求 pkg=path）", spec)
		}
		pkg, path := spec[:eq], spec[eq+1:]
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取原始输出失败: %v", err)
		}
		if err := parseBenchlibRaw(string(data), pkg, samples, &order); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if len(order) == 0 {
		t.Fatalf("原始输出中未解析到任何 Benchmark 行（-bench 正则或命令有误）")
	}
	var expectRe *regexp.Regexp
	if *benchlibExpect != "" {
		var err error
		expectRe, err = regexp.Compile(*benchlibExpect)
		if err != nil {
			t.Fatalf("非法 -benchlib-expect: %v", err)
		}
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := samples[order[i]], samples[order[j]]
		if a.name != b.name {
			return a.name < b.name
		}
		return a.pkg < b.pkg
	})
	doc := benchlibDoc{
		Group:             *benchlibGroup,
		Commit:            *benchlibCommit,
		GoVersion:         runtime.Version(),
		Nproc:             runtime.NumCPU(),
		LoadAvg:           *benchlibLoadAvg,
		Benchtime:         *benchlibBenchtime,
		Count:             *benchlibCount,
		Runs:              *benchlibCount,
		Statistic:         "median",
		GeneratedAt:       time.Now().Format(time.RFC3339),
		PerfThresholdPct:  benchlibPerfThresholdPct,
		NoiseSafetyFactor: benchlibNoiseSafetyFactor,
		Commands:          append([]string(nil), benchlibCommands...),
	}
	maxRatio := 0.0
	for _, key := range order {
		s := samples[key]
		if expectRe != nil {
			if !expectRe.MatchString(s.name) {
				t.Fatalf("benchmark %q 不在本 group 的名单正则 %q 内（名单与实际运行集合不一致）", s.name, *benchlibExpect)
			}
		}
		if *benchlibCount > 0 && len(s.ns) != *benchlibCount {
			t.Fatalf("benchmark %q 样本数 %d != 期望 -count=%d（同名 benchmark 在不同 -bench 行里重复或漏跑）",
				s.name, len(s.ns), *benchlibCount)
		}
		ratio := maxMinRatio(s.ns)
		if math.IsInf(ratio, 1) {
			t.Fatalf("benchmark %q 的 maxMinRatio 不可用（样本非正/非有限）", s.name)
		}
		if ratio > maxRatio {
			maxRatio = ratio
		}
		e := benchlibEntryDoc{
			Name:        s.name,
			Package:     s.pkg,
			NsPerOp:     s.ns,
			MaxMinRatio: ratio,
			BytesPerOp:  benchlibMedian(s.bytes),
			AllocsPerOp: benchlibMedian(s.allocs),
		}
		if len(s.metrics) > 0 {
			e.Metrics = map[string]float64{}
			for unit, vals := range s.metrics {
				e.Metrics[unit] = benchlibMedian(vals)
			}
			if steps := e.Metrics["steps/op"]; steps > 0 {
				e.NsPerStep = benchlibMedian(s.ns) / steps
			}
		}
		doc.Entries = append(doc.Entries, e)
	}
	doc.ThresholdMaxMinRatio = maxRatio
	if err := benchlibWriteJSON(*benchlibJSONOut, doc); err != nil {
		t.Fatalf("写 bench.json 失败: %v", err)
	}
	if *benchlibCSVOut != "" {
		if err := benchlibWriteCSV(*benchlibCSVOut, doc); err != nil {
			t.Fatalf("写 bench.csv 失败: %v", err)
		}
	}
	t.Logf("聚合完成: group=%s entries=%d thresholdMaxMinRatio=%.4f (%s)",
		doc.Group, len(doc.Entries), doc.ThresholdMaxMinRatio, *benchlibJSONOut)
}

// ---------------------------------------------------------------------------
// 3. 判定：噪声地板与性能阈值分列
// ---------------------------------------------------------------------------

// benchlibVerdictEntry 对应 K1 的 perBenchmark 元素。
type benchlibVerdictEntry struct {
	Name             string  `json:"name"`
	Package          string  `json:"package,omitempty"`
	BaselineNsPerOp  float64 `json:"baselineNsPerOp"`
	CurrentNsPerOp   float64 `json:"currentNsPerOp"`
	DeltaPct         float64 `json:"deltaPct"`
	Judgeable        bool    `json:"judgeable"`
	NoiseMaxMinRatio float64 `json:"noiseMaxMinRatio"`
	FailThresholdPct float64 `json:"failThresholdPct"`
	Reason           string  `json:"reason"`
}

// benchlibUngaugeable 对应 K1 的 ungaugeable 元素。
type benchlibUngaugeable struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// benchlibVerdictDoc 是 verdict.json 的顶层结构。
//
// environmentShiftPct / environmentShiftDetected 是**追加**的诊断字段（K1 未列，不改 K1 语义）：
// 同一台机器不同时间窗之间的公共模式漂移可能远大于单 entry 的噪声地板（本机实测 12%~42%），
// 此时 deltaPct 里混着环境变化。检测口径 = 所有可比 entry 的 deltaPct 中位数绝对值 >
// noiseSafetyFactor×(标定 maxMinRatio-1)×100。检出时判定仍按 K1 走（不自动豁免），
// 但会写入 notes 提示"需在同窗口重新标定后再判"。
type benchlibVerdictDoc struct {
	Group                    string  `json:"group"`
	Commit                   string  `json:"commit"`
	Verdict                  string  `json:"verdict"`
	BaselineFile             string  `json:"baselineFile"`
	CurrentFile              string  `json:"currentFile"`
	JudgedCount              int     `json:"judgedCount"`
	PerfThresholdPct         float64 `json:"perfThresholdPct"`
	BaselineLoadAvg          string  `json:"baselineLoadAvg,omitempty"`
	CurrentLoadAvg           string  `json:"currentLoadAvg,omitempty"`
	EnvironmentShiftPct      float64 `json:"environmentShiftPct"`
	EnvironmentShiftDetected bool    `json:"environmentShiftDetected"`
	// VerdictBeforeShift / PendingFails 是 D29 的追溯字段：检测到公共模式漂移时整组降级为
	// NOT_JUDGEABLE，但"降级前的判定"与"疑似回归条目"都要留痕，避免把真回归静默吞掉。
	VerdictBeforeShift string                 `json:"verdictBeforeShift,omitempty"`
	PendingFails       []string               `json:"pendingFails,omitempty"`
	Notes              []string               `json:"notes,omitempty"`
	PerBenchmark       []benchlibVerdictEntry `json:"perBenchmark"`
	Ungaugeable        []benchlibUngaugeable  `json:"ungaugeable"`
}

// benchlibJudge 是判定本体（纯函数：只吃两份 doc，不读文件、不碰 flag），
// 便于用合成输入做确定性自测（见 TestBenchlibJudge*）。判定规则（逐字见 thresholds.md）：
//   - 噪声地板 floorPct = max((max(基线,本次)maxMinRatio - 1)×100×noiseSafetyFactor, benchlibMinWindowFloorPct)；
//   - |deltaPct| ≤ judgeBand×floorPct → judgeable=false，只进 ungaugeable，不计入 PASS 的判定依据；
//   - 判定门槛 failPct = max(perfThresholdPct, 2×floorPct)；judgeable 且 deltaPct > failPct → FAIL；
//   - 基线里缺失的 benchmark 现在跑出来了 → 只记录，不判 FAIL（新增方向）；
//   - 基线里有、本次没有 → FAIL（名单被破坏，禁止静默通过）；
//   - **D14**：无 FAIL 且 judged==0（全部 entry 落在噪声地板内）→ verdict=NOT_JUDGEABLE，
//     由 scripts/benchCompare.sh 映射为退出码 3（"给不出判定"），不得读成 PASS。
func benchlibJudge(t *testing.T, base, cur benchlibDoc, baselinePath, currentPath string) benchlibVerdictDoc {
	if base.ThresholdMaxMinRatio <= 0 || base.Benchtime == "" || base.Runs <= 0 {
		t.Fatalf("基线缺字段（thresholdMaxMinRatio/benchtime/runs 必填）: %s", baselinePath)
	}
	if base.PerfThresholdPct <= 0 {
		t.Fatalf("基线缺 perfThresholdPct: %s", baselinePath)
	}
	if base.NoiseSafetyFactor <= 0 {
		t.Fatalf("基线缺 noiseSafetyFactor: %s", baselinePath)
	}
	curByName := map[string]benchlibEntryDoc{}
	for _, e := range cur.Entries {
		curByName[e.Package+"\x00"+e.Name] = e
	}
	doc := benchlibVerdictDoc{
		Group:            cur.Group,
		Commit:           cur.Commit,
		Verdict:          "PASS",
		BaselineFile:     baselinePath,
		CurrentFile:      currentPath,
		PerfThresholdPct: base.PerfThresholdPct,
		BaselineLoadAvg:  base.LoadAvg,
		CurrentLoadAvg:   cur.LoadAvg,
	}
	t.Logf("%-46s %14s %14s %9s %9s %9s %s", "benchmark",
		"baseline_ns", "current_ns", "delta%", "floor%", "perf%", "judge")
	var deltas []float64
	for _, be := range base.Entries {
		ce, ok := curByName[be.Package+"\x00"+be.Name]
		if !ok {
			doc.PerBenchmark = append(doc.PerBenchmark, benchlibVerdictEntry{
				Name: be.Name, Package: be.Package, Judgeable: false,
				Reason: "本次运行缺失该 benchmark（名单被破坏）",
			})
			doc.Verdict = "FAIL"
			t.Logf("%-46s %14s %14s %9s %9s %9s 缺失→FAIL", be.Name, "-", "-", "-", "-", "-")
			continue
		}
		baseNs := benchlibMedian(be.NsPerOp)
		curNs := benchlibMedian(ce.NsPerOp)
		deltaPct := (curNs - baseNs) / baseNs * 100
		deltas = append(deltas, deltaPct)
		// 噪声上界取"标定窗口 spread"与"本次窗口 spread"的较大者：若本次测量自身就不一致
		// （例如同一 benchmark 的 3 个样本相差 2 倍），用中位数与基线比出来的 delta 没有意义。
		noiseRatio := be.MaxMinRatio
		if ce.MaxMinRatio > noiseRatio {
			noiseRatio = ce.MaxMinRatio
		}
		floorPct := (noiseRatio - 1) * 100 * base.NoiseSafetyFactor
		if floorPct < benchlibMinWindowFloorPct {
			floorPct = benchlibMinWindowFloorPct
		}
		// 判 FAIL 的门槛还要再乘一次安全系数：回归必须同时越过人工允许（perfThresholdPct）
		// 与 2×地面噪声，避免把窗口漂移读成回归（本机跨窗口漂移实测 5%~6%）。
		failPct := base.PerfThresholdPct
		if 2*floorPct > failPct {
			failPct = 2 * floorPct
		}
		// 判定带：只有 |delta| > judgeBand×floorPct 才计入可判条目。地板附近的抖动（±1.5× 地板以内）
		// 只会在 PASS 与 NOT_JUDGEABLE 之间来回翻标签，不足以支撑判定（实测依据见 thresholds.md 变更记录）。
		judgeable := math.Abs(deltaPct) > benchlibJudgeBand*floorPct
		reason := fmt.Sprintf("delta 落在噪声地板的判定带内（|delta| ≤ %.2f×地板=%.2f%%，不计 PASS）",
			benchlibJudgeBand, benchlibJudgeBand*floorPct)
		if judgeable {
			reason = fmt.Sprintf("delta 越过判定带（> %.2f×地板=%.2f%%）", benchlibJudgeBand, benchlibJudgeBand*floorPct)
		}
		ve := benchlibVerdictEntry{
			Name: be.Name, Package: be.Package,
			BaselineNsPerOp: baseNs, CurrentNsPerOp: curNs, DeltaPct: deltaPct,
			Judgeable: judgeable, Reason: reason,
			NoiseMaxMinRatio: noiseRatio, FailThresholdPct: failPct,
		}
		if judgeable {
			doc.JudgedCount++
			if deltaPct > failPct {
				doc.Verdict = "FAIL"
				ve.Reason = fmt.Sprintf("回归 %.2f%% > 判定门槛 %.2f%%（性能阈值与 2×噪声地板取大）", deltaPct, failPct)
			}
		} else {
			doc.Ungaugeable = append(doc.Ungaugeable, benchlibUngaugeable{Name: be.Name, Reason: reason})
		}
		doc.PerBenchmark = append(doc.PerBenchmark, ve)
		t.Logf("%-46s %14.1f %14.1f %+9.2f %9.2f %9.2f %v",
			be.Name, baseNs, curNs, deltaPct, floorPct, failPct, judgeable)
	}
	// 公共模式漂移诊断：同向的整组偏移更像"换了个时间窗"，不是单点回归。
	ratioForShift := base.ThresholdMaxMinRatio
	if cur.ThresholdMaxMinRatio > ratioForShift {
		ratioForShift = cur.ThresholdMaxMinRatio
	}
	shiftLimitPct := (ratioForShift - 1) * 100 * base.NoiseSafetyFactor
	if shiftLimitPct < benchlibMinWindowFloorPct {
		shiftLimitPct = benchlibMinWindowFloorPct
	}
	if len(deltas) > 0 {
		doc.EnvironmentShiftPct = benchlibMedian(deltas)
		// D29 收紧后的判据：不仅要求"中位漂移显著"，还要求各条目**围绕中位聚集**（公共模式）。
		// 只用中位数会把"单点大回归 + 其余持平"误判成漂移（例：{+40%, +0.4%} 的中位 ≈ +20%），
		// 从而把真回归静默吞掉。聚集性判据（极差 ≤ max(shiftLimit, 0.5×|中位|)）能区分两者：
		//   {+35%, +33%, +36%} → 聚集 ⇒ 漂移；{+40%, +0.4%} → 不聚集 ⇒ 保留 FAIL 判定。
		if len(deltas) >= 2 {
			spread := benchlibMax(deltas) - benchlibMin(deltas)
			clusterLimit := shiftLimitPct
			if half := math.Abs(doc.EnvironmentShiftPct) * 0.5; half > clusterLimit {
				clusterLimit = half
			}
			doc.EnvironmentShiftDetected = math.Abs(doc.EnvironmentShiftPct) > shiftLimitPct && spread <= clusterLimit
		}
	}
	if doc.EnvironmentShiftDetected {
		doc.Notes = append(doc.Notes, fmt.Sprintf(
			"公共模式漂移 %+.2f%% 超过 %.2f%%（且各条目围绕该中位聚集）：本次与基线很可能不在同一负载窗口，"+
				"delta 仅供方向参考；结论请在原窗口重新 --write-baseline 后复测（标定 loadAvg=%q，本次 loadAvg=%q）",
			doc.EnvironmentShiftPct, shiftLimitPct, doc.BaselineLoadAvg, doc.CurrentLoadAvg))
		t.Logf("WARN: %s", doc.Notes[len(doc.Notes)-1])
	}
	// D14：全部 entry 都在噪声地板内 → 给不出判定（不是 PASS）。退出码由 benchCompare.sh 映射为 3。
	if doc.Verdict == "PASS" && doc.JudgedCount == 0 {
		doc.Verdict = "NOT_JUDGEABLE"
		doc.Notes = append(doc.Notes, fmt.Sprintf(
			"judged=0：%d 个 entry 全部落在噪声地板内，给不出判定（D14）→ 退出码 3；"+
				"每条原因见 ungaugeable", len(doc.Ungaugeable)))
	}
	// D29：检测到公共模式漂移时**不判 PASS/FAIL** —— 跨窗口的 delta 无法区分"真回归"与
	// "换了个时间窗"（实测：同一改动被跨窗口对比报 +36%，而同窗口受控交错 A/B 显示无变化）。
	// 逐条 delta 仍留在 perBenchmark；降级前的判定与疑似回归条目进 verdictBeforeShift/pendingFails，
	// 便于人工回同窗口复测，不做静默吞掉。
	if doc.EnvironmentShiftDetected {
		if doc.Verdict != "NOT_JUDGEABLE" {
			doc.VerdictBeforeShift = doc.Verdict
		}
		for _, e := range doc.PerBenchmark {
			if e.Judgeable && e.DeltaPct > e.FailThresholdPct {
				doc.PendingFails = append(doc.PendingFails, fmt.Sprintf("%s %+.2f%%（门槛 %.2f%%）", e.Name, e.DeltaPct, e.FailThresholdPct))
			}
		}
		doc.Verdict = "NOT_JUDGEABLE"
		doc.Notes = append(doc.Notes, fmt.Sprintf(
			"D29：公共模式漂移 → 整组降级为 NOT_JUDGEABLE（退出码 3）；降级前判定=%q，疑似回归 %d 条见 pendingFails。"+
				"策略类改动的收益/回归请用**同窗口受控交错 A/B**（两个二进制交替、各 3 次取中位），见 docs/library-conventions.md",
			doc.VerdictBeforeShift, len(doc.PendingFails)))
		t.Logf("WARN: %s", doc.Notes[len(doc.Notes)-1])
	}
	t.Logf("VERDICT=%s group=%s judged=%d ungaugeable=%d environmentShift=%+.2f%% detected=%v",
		doc.Verdict, doc.Group, doc.JudgedCount, len(doc.Ungaugeable),
		doc.EnvironmentShiftPct, doc.EnvironmentShiftDetected)
	return doc
}

// TestBenchlibVerdict 由 scripts/benchCompare.sh 调用：读基线 + 本次结果 → verdict.json。
func TestBenchlibVerdict(t *testing.T) {
	if *benchlibVerdictOut == "" {
		t.Skip("未传 -benchlib-verdict（本用例仅由 scripts/benchCompare.sh 驱动）")
	}
	if *benchlibBaseline == "" || *benchlibCurrent == "" {
		t.Fatalf("缺少 -benchlib-baseline / -benchlib-current")
	}
	var base, cur benchlibDoc
	if err := benchlibReadJSON(*benchlibBaseline, &base); err != nil {
		t.Fatalf("读基线失败: %v", err)
	}
	if err := benchlibReadJSON(*benchlibCurrent, &cur); err != nil {
		t.Fatalf("读本次结果失败: %v", err)
	}
	doc := benchlibJudge(t, base, cur, *benchlibBaseline, *benchlibCurrent)
	if err := benchlibWriteJSON(*benchlibVerdictOut, doc); err != nil {
		t.Fatalf("写 verdict.json 失败: %v", err)
	}
	if doc.Verdict == "FAIL" {
		t.Fatalf("判定 FAIL（详见 %s）", *benchlibVerdictOut)
	}
	// NOT_JUDGEABLE 不是测试失败（go test 无法返回 3），由 benchCompare.sh 读 verdict.json 映射为退出码 3。
}

// benchlibJudgeCase 是 benchlibJudge 自测用的合成输入。
type benchlibJudgeCase struct {
	name      string
	baseRat   float64 // 基线每 entry 的 maxMinRatio
	curRat    float64 // 本次每 entry 的 maxMinRatio（0 = 与基线相同）
	deltas    []float64
	missing   int // 本次缺失的 entry 数（从末尾开始缺）
	want      string
	wantJudge int
}

// TestBenchlibJudge 用合成输入覆盖判定的四种出口（D14 的 NOT_JUDGEABLE 必须可机检，不依赖机器噪声）。
func TestBenchlibJudge(t *testing.T) {
	cases := []benchlibJudgeCase{
		{name: "全部不可判→NOT_JUDGEABLE", baseRat: 1.10, deltas: []float64{0.4, -0.6, 1.2}, want: "NOT_JUDGEABLE", wantJudge: 0},
		{name: "有可判且无回归→PASS", baseRat: 1.02, deltas: []float64{8.0, 0.4}, want: "PASS", wantJudge: 1},
		{name: "落在判定带内→NOT_JUDGEABLE", baseRat: 1.02, deltas: []float64{7.0}, want: "NOT_JUDGEABLE", wantJudge: 0},
		{name: "有可判且回归越门槛→FAIL", baseRat: 1.02, deltas: []float64{40.0}, want: "FAIL", wantJudge: 1},
		{name: "回归在门槛内→PASS", baseRat: 1.02, deltas: []float64{9.0}, want: "PASS", wantJudge: 1},
		{name: "本次缺 entry→FAIL", baseRat: 1.02, deltas: []float64{0.1}, missing: 1, want: "FAIL", wantJudge: 0},
		{name: "本次窗口自身噪声大→不计 FAIL（NOT_JUDGEABLE）", baseRat: 1.02, curRat: 2.0,
			deltas: []float64{80.0}, want: "NOT_JUDGEABLE", wantJudge: 0},
		{name: "本次窗口自身噪声大但确实稳→仍可判", baseRat: 1.02, curRat: 1.001,
			deltas: []float64{40.0}, want: "FAIL", wantJudge: 1},
		{name: "公共模式漂移（聚集）→整组 NOT_JUDGEABLE（D29）", baseRat: 1.02, curRat: 1.001,
			deltas: []float64{35.0, 33.0, 36.0}, want: "NOT_JUDGEABLE", wantJudge: 3},
		{name: "单点大回归+其余持平→不判漂移，保留 FAIL（D29）", baseRat: 1.02, curRat: 1.001,
			deltas: []float64{40.0, 0.4}, want: "FAIL", wantJudge: 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := benchlibDoc{
				Group: "synthetic", Benchtime: "1s", Runs: 3, Count: 3,
				ThresholdMaxMinRatio: c.baseRat, PerfThresholdPct: 5, NoiseSafetyFactor: 2,
				Statistic: "median", Nproc: 1, GoVersion: "synthetic", Commit: "synthetic",
			}
			cur := benchlibDoc{Group: "synthetic", Benchtime: "1s", Runs: 3, Count: 3,
				ThresholdMaxMinRatio: c.baseRat, PerfThresholdPct: 5, NoiseSafetyFactor: 2,
				Statistic: "median", Nproc: 1, GoVersion: "synthetic", Commit: "synthetic"}
			curRat := c.curRat
			if curRat == 0 {
				curRat = c.baseRat
			}
			for i, d := range c.deltas {
				name := fmt.Sprintf("BenchSynthetic%d", i)
				base.Entries = append(base.Entries, benchlibEntryDoc{
					Name: name, Package: "./synthetic/", NsPerOp: []float64{100, 100, 100},
					MaxMinRatio: c.baseRat,
				})
				if i >= len(c.deltas)-c.missing {
					continue
				}
				cur.Entries = append(cur.Entries, benchlibEntryDoc{
					Name: name, Package: "./synthetic/", NsPerOp: []float64{100 + d, 100 + d, 100 + d},
					MaxMinRatio: curRat,
				})
			}
			got := benchlibJudge(t, base, cur, "synthetic-baseline", "synthetic-current")
			if got.Verdict != c.want {
				t.Fatalf("verdict=%s, 期望 %s（judged=%d, ungaugeable=%d, notes=%v）",
					got.Verdict, c.want, got.JudgedCount, len(got.Ungaugeable), got.Notes)
			}
			if got.JudgedCount != c.wantJudge {
				t.Fatalf("judgedCount=%d, 期望 %d", got.JudgedCount, c.wantJudge)
			}
			// NOT_JUDGEABLE 必须有**可追溯的原因**，不允许无解释的降级：
			//   - D14：条目全落在判定带内 → ungaugeable 逐条列出；
			//   - D29：公共模式漂移 → environmentShiftDetected 置位（并记录降级前判定与疑似回归）。
			if got.Verdict == "NOT_JUDGEABLE" && len(got.Ungaugeable) == 0 && !got.EnvironmentShiftDetected {
				t.Fatal("NOT_JUDGEABLE 必须逐条保留 ungaugeable 原因，或因检测到公共模式漂移而降级（D29）")
			}
		})
	}
}

// TestBenchlibMaxMinRatio 校验 maxMinRatio 的口径（判定量本身可测）。
func TestBenchlibMaxMinRatio(t *testing.T) {
	cases := []struct {
		in   []float64
		want float64
	}{
		{[]float64{10, 11}, 1.1},
		{[]float64{5, 5, 5}, 1},
		{[]float64{1, 3, 2}, 3},
		{nil, math.Inf(1)},
		{[]float64{0, 1}, math.Inf(1)},
		{[]float64{math.NaN(), 1}, math.Inf(1)},
	}
	for _, c := range cases {
		got := maxMinRatio(c.in)
		if got != c.want {
			t.Errorf("maxMinRatio(%v) = %v, 期望 %v", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 4. 工具函数
// ---------------------------------------------------------------------------

// benchlibPerfThresholdPct 是「性能阈值」默认值：判 FAIL 的回归上限（%）。
// 「噪声地板」是另一个身份，只能由每轮标定的 maxMinRatio 推出，不得与该值互相顶替。
const benchlibPerfThresholdPct = 5.0

// benchlibNoiseSafetyFactor 是噪声地板的保守系数：地板 = 标定 spread × 该系数。
const benchlibNoiseSafetyFactor = 2.0

// benchlibMinFloorPct 是噪声地板的下限（%）。
const benchlibMinFloorPct = 0.5

// benchlibJudgeBand 是判定带系数：可判条目要求 |delta| > 该系数 × 噪声地板。
// 取 1.5 是为了让"地板附近的抖动"不进判定，避免 group 标签在 PASS/NOT_JUDGEABLE 之间翻转。
const benchlibJudgeBand = 1.5

// benchlibMinWindowFloorPct 是**跨窗口复现性下限**（%）：标定 spread 只覆盖同一窗口内的重复性，
// 而同一份源码在两个负载窗口之间重测会出现更大的漂移（本机实测：标定 spread 1.3% 的 entry 跨窗口偏 +5.1%）。
// 故地板再取该下限，保证"判定只看超过窗口漂移的部分"。实测依据见 thresholds.md「跨窗口不可比性」。
const benchlibMinWindowFloorPct = 5.0

// benchlibWriteJSON 原子写 JSON（临时文件 + rename）。
func benchlibWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return benchlibWriteFileAtomic(path, data)
}

// benchlibReadJSON 读取 JSON（不忽略未知字段，字段缺失由调用方校验）。
func benchlibReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// benchlibWriteCSV 写 bench.csv：name,package,ns1..nsN,maxMinRatio,bytesPerOp,allocsPerOp。
func benchlibWriteCSV(path string, doc benchlibDoc) error {
	var sb strings.Builder
	sb.WriteString("name,package")
	for i := 1; i <= doc.Runs; i++ {
		fmt.Fprintf(&sb, ",ns%d", i)
	}
	sb.WriteString(",maxMinRatio,bytesPerOp,allocsPerOp\n")
	for _, e := range doc.Entries {
		fmt.Fprintf(&sb, "%s,%s", e.Name, e.Package)
		for i := 0; i < doc.Runs; i++ {
			if i < len(e.NsPerOp) {
				fmt.Fprintf(&sb, ",%.2f", e.NsPerOp[i])
			} else {
				sb.WriteString(",")
			}
		}
		fmt.Fprintf(&sb, ",%.4f,%.0f,%.0f\n", e.MaxMinRatio, e.BytesPerOp, e.AllocsPerOp)
	}
	return benchlibWriteFileAtomic(path, []byte(sb.String()))
}

// benchlibWriteFileAtomic 同目录临时文件 + rename，保证读者看不到半截文件。
func benchlibWriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
