// limiting_test.go —— m4「非线性迭代电压限制」的单元与对照用例。
//
// 覆盖三件事：
//  1. TestLimitingClampsDeviceJunctionStep：接线级差分——六个器件在同一固定激励序列下，
//     关闭限制时至少一步结电压变化 > 0.25V（证明激励确实是大跳变），开启限制后每一步
//     结电压变化都 ≤ 0.25V（证明限制真的接进了器件 DoStep，而不是只写了个函数）。
//  2. TestLimitingDisabledBitExactGolden：关闭限制时，六个器件在整个激励序列中的
//     盖章贡献（矩阵 A + 右端向量 Z）与状态槽位逐位一致于**接线前**抓取的 golden 摘要。
//     抓取方式：接线前运行 M4_CAPTURE_GOLDEN=1 go test -run TestLimitingDisabledBitExactGolden，
//     把打印的 M4_GOLDEN_SHA256 填进 preWireStampDigestSHA256。
//  3. TestLimitingIterationCountComparison：真实非线性语料上限制开/关的牛顿迭代轮次、
//     元件子迭代轮次、未收敛信号数、时间步数与钳位次数对照（K4 的 BoundedNewtonEvents
//     由 m5 在批次 C 落地，本用例用 mna.Time 接口上的既有计数方法做代理口径；见 compare.md）。
//
// 环境变量 M4_CAPTURE_GOLDEN 只用于抓取 golden，不参与验收；TestMain 强制单测默认关闭限制，
// 保证 CIRCUIT_ITERLIMIT_V 只影响 CLI/基准复跑、不影响包内测试的既有断言。
package semiconductor

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"circuit/element"
	_ "circuit/element/hydraulic"
	_ "circuit/element/passive"
	_ "circuit/element/sensor"
	_ "circuit/element/source"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

// ---------------------------------------------------------------------------
// D18 / UD10 器件级 harness：把「加性 gmin 是否被启用」变成可断言的事实。
//
// 判据的设计要点（可证伪）：
//   * 加性 gmin 直接进工作点切向电导 geq（Diode/led/zener 的 StampAdmittance），
//     所以「读回被戳入的导纳」就是读回 geq —— 不需要为测试改产品代码路径；
//   * 用包内 timeStub 同时提供**累计成功步数**（GoodIterations）与**当前步元件迭代数**
//     （CurrentStepElemIterations），从而能分别复现「旧口径误用」与「新口径」；
//   * 光有累计步数大（≥500）而元件迭代数小（1）时，旧口径会 +~1e-6、
//     新口径必须 +0 —— 这一条差异就是 D18 是否修好的判定依据。
// ---------------------------------------------------------------------------

// stampRecorder 记录器件 DoStep 戳入 MNA 的贡献；只实现器件真正调用的方法。
type stampRecorder struct {
	mna.Mna // 嵌入接口：未被调用的方法保持 nil，被调用即 panic（暴露意外用法）
	admits  []float64
}

func (r *stampRecorder) StampAdmittance(n1, n2 mna.NodeID, admittance float64) {
	r.admits = append(r.admits, admittance)
}

func (r *stampRecorder) StampCurrentSource(mna.NodeID, mna.NodeID, float64) {}

// timeStub 只实现器件 DoStep 读到的三个 Time 方法，其余沿用嵌入接口（nil）。
// timeBase 是为了避开 mna.Time 接口内 Time() 方法造成的**字段/方法同名冲突**：
// 直接写 `mna.Time` 匿名嵌入会生成名为 Time 的字段，与接口方法 Time() 冲突；
// 用一个同方法集的命名接口中转，匿名嵌入后字段名为 timeBase，冲突消失。
type timeBase interface {
	mna.Time
}

type timeStub struct {
	timeBase // 匿名嵌入：满足 mna.Time 其余方法（器件不调用它们，保持 nil）
	good     int
	elemIter int
}

func (t *timeStub) GoodIterations() int            { return t.good }
func (t *timeStub) CurrentElemIter() int           { return t.elemIter }
func (t *timeStub) CurrentStepElemIterations() int { return t.elemIter }
func (t *timeStub) GetContinuationGmin() float64   { return 0 }
func (t *timeStub) NoConverged()                   {}

// diodeHarnessNode 是 doDiodeStep 所需的最小 NodeFace：只填参数槽与引脚/内部节点。
//
// 参数槽（与 Diode.Base 的写入位置一致）：[1]=Vz、[7]=1/(N*Vt)、[9]=1/Vt、
// [10]=zoffset、[13]=leakage(Is)。其余槽位器件不读。
func diodeHarnessNode() *element.Node {
	vals := make([]any, 15)
	for i := range vals {
		vals[i] = float64(0)
	}
	vals[1] = float64(0)      // Vz=0 → 走常规二极管分支
	vals[7] = float64(38.0)   // 1/(N*Vt) ≈ 1/(1*0.02585)
	vals[9] = float64(38.0)   // 1/Vt
	vals[10] = float64(0)     // zoffset
	vals[13] = float64(1e-14) // leakage=Is
	vals[5] = float64(0)      // lastvoltdiff
	return &element.Node{
		NodeValue:    vals,
		Nodes:        []mna.NodeID{1, -1},
		NodeInternal: []mna.NodeID{0},
	}
}

// TestD18ExtraGminUsesCurrentStepIterations 是 D18/UD10 的主判据。
//
// 三组输入覆盖：旧口径误用（累计 500 步）、新口径真触发（150 次迭代）、阈值边界（100 次）。
func TestD18ExtraGminUsesCurrentStepIterations(t *testing.T) {
	const vd = 0.3

	run := func(good, elemIter int) (float64, uint64) {
		ResetExtraGminActivations()
		rec := &stampRecorder{}
		doDiodeStep(rec, &timeStub{good: good, elemIter: elemIter}, diodeHarnessNode(), vd)
		if len(rec.admits) == 0 {
			t.Fatalf("器件未戳入任何电导：good=%d elemIter=%d", good, elemIter)
		}
		return rec.admits[0], ExtraGminActivations()
	}

	// ① 旧口径误用：累计步数 500（>100）但当前步只迭代 1 次
	//    → 启用计数必须为 0，且不得叠加任何加性 gmin（差分 ≈ 0）
	geqOld, actsOld := run(500, 1)
	if actsOld != 0 {
		t.Errorf("累计步数 500 / 迭代 1：加性 gmin 启用计数应为 0，实际 %d（D18 未修好）", actsOld)
	}
	geqBase, _ := run(500, 1) // 同输入再跑一次，作为差分基准
	if diff := math.Abs(geqOld - geqBase); diff > 1e-15 {
		t.Errorf("累计步数 500 / 迭代 1：两次同输入 geq 差 %.3e（应为 0）", diff)
	}

	// ② 新口径真触发：当前步迭代 500 次（>100）
	//    → 启用计数必须为 1，且 geq 相对「迭代 1」的差分 ≈ extraGminForIterations(500) > 1e-13
	geqTrig, actsTrig := run(3, 500)
	if actsTrig != 1 {
		t.Errorf("迭代 500：加性 gmin 启用计数应为 1，实际 %d", actsTrig)
	}
	wantExtra := extraGminForIterations(500)
	if wantExtra <= 1e-13 {
		t.Fatalf("前置失败：extraGminForIterations(500)=%.3e 应 > 1e-13", wantExtra)
	}
	if diff := geqTrig - geqOld; math.Abs(diff-wantExtra) > wantExtra*1e-6 {
		t.Errorf("迭代 500 与迭代 1 的 geq 差分=%.6e，期望 %.6e（加性 gmin 未进入 geq）", diff, wantExtra)
	}

	// ③ 阈值边界：恰好 100 次不触发（>100 才触发）
	if _, actsAt100 := run(3, gminIterThreshold); actsAt100 != 0 {
		t.Errorf("迭代=%d（阈值本身）：不应触发，实际启用 %d 次", gminIterThreshold, actsAt100)
	}
	// ④ 上界钳位：迭代 1e6 次时 extraGmin 必须钳在 1e-6
	ResetExtraGminActivations()
	if up := extraGminForIterations(1000000); up != 1e-6 {
		t.Errorf("迭代 1e6：extraGmin=%.6e，期望钳位到 1e-6", up)
	}
}

// TestD18CumulativeStepsMustNotDriveExtraGmin 是 D18 的**判别式**用例（证明判据非空）。
//
// 逻辑：两条输入路径在"正确的语义"下必须给出**完全相同**的加性 gmin：
//
//	A) 当前步迭代 1 次、累计成功 500 步  → 正确语义 = 不加
//	B) 当前步迭代 1 次、累计成功   1 步  → 正确语义 = 不加
//
// 若实现退回「用累计成功步数当迭代数」（缺陷 D18），则 A 会 +extraGmin(500) 而 B 不加，
// 二者 geq 差分 = extraGminForIterations(500) ≈ 2.9e-10 ≠ 0 → 本用例失败。
//
// 因此本用例是 D18 是否修好的独立判据：把三处调用退回 GoodIterations() 后必须失败
// （父体已用 -overlay 实测，证据见 runs/circuit-perf-optimization/_results/ud10/）。
func TestD18CumulativeStepsMustNotDriveExtraGmin(t *testing.T) {
	geq := func(good, elemIter int) float64 {
		ResetExtraGminActivations()
		rec := &stampRecorder{}
		doDiodeStep(rec, &timeStub{good: good, elemIter: elemIter}, diodeHarnessNode(), 0.3)
		if len(rec.admits) == 0 {
			t.Fatalf("器件未戳入电导：good=%d elemIter=%d", good, elemIter)
		}
		return rec.admits[0]
	}

	longRun := geq(500, 1) // A：累计 500 步、当前步 1 次迭代
	shortRun := geq(1, 1)  // B：累计 1 步、当前步 1 次迭代
	if diff := longRun - shortRun; diff != 0 {
		t.Errorf("累计成功步数不得影响加性 gmin：500 步 geq=%.9e 与 1 步 geq=%.9e 相差 %.6e（D18 未修好）",
			longRun, shortRun, diff)
	}
}

// TestD18ExtraGminHelperIsPure 固定算式等价性：抽函数不得改变任何迭代数下的取值。
//
// 旧内联算式的期望值在此以闭式重算，两者必须逐位一致（含启用计数）。
func TestD18ExtraGminHelperIsPure(t *testing.T) {
	gi := func(iter int) float64 {
		if iter <= 100 {
			return 0
		}
		ratio := 1.0 - math.Min(float64(iter)/1000.0, 1.0)
		e := math.Exp(-12 * math.Ln10 * ratio)
		if e < 1e-15 {
			return 0
		}
		if e > 1e-6 {
			return 1e-6
		}
		return e
	}
	for _, iter := range []int{0, 1, 100, 101, 150, 500, 999, 1000, 5000} {
		ResetExtraGminActivations()
		got := extraGminForIterations(iter)
		want := gi(iter)
		if got != want {
			t.Errorf("extraGminForIterations(%d)=%v，期望 %v（算式等价性破坏）", iter, got, want)
		}
		wantActs := uint64(0)
		if want > 0 {
			wantActs = 1
		}
		if a := ExtraGminActivations(); a != wantActs {
			t.Errorf("extraGminForIterations(%d) 启用计数=%d，期望 %d", iter, a, wantActs)
		}
	}
}

const (
	// preWireStampDigestSHA256 接线前（器件未接入限制时）六个器件盖章轨迹的 sha256。
	// 接线后关闭限制必须复现该值；抓取命令见文件头。
	preWireStampDigestSHA256 = "2c675c73a05aca15888148170d49d0449f59ebb72c195d5a1606744a0e3e8cd6"
	// goldenCaptureEnv 置位时只打印当前摘要（抓取 golden 用），不做断言。
	goldenCaptureEnv = "M4_CAPTURE_GOLDEN"
	// abLimitVolts 对照用例开启限制时使用的限幅值（V）。
	abLimitVolts = 0.25
	// abCorpusDir 非线性语料目录（仓库内只读）。
	abCorpusDir = "../../cmd/circuits"
)

// TestMain 强制包内测试默认关闭限制：环境变量 CIRCUIT_ITERLIMIT_V 只应影响 CLI/基准复跑，
// 不应改变既有器件测试与 golden 用例的前提。
func TestMain(m *testing.M) {
	_ = SetIterationLimiting(false, 0)
	os.Exit(m.Run())
}

// TestLimitingStepBoundAndSwitch 验证限制开关语义与限幅函数本身。
func TestLimitingStepBoundAndSwitch(t *testing.T) {
	if err := SetIterationLimiting(false, 0); err != nil {
		t.Fatalf("关闭限制失败: %v", err)
	}
	if got := IterationLimitingMaxStep(); got != 0 {
		t.Fatalf("关闭后 MaxStep 期望 0，实际 %v", got)
	}
	for _, v := range []float64{3.7, -3.7, 0.1, 0, math.Inf(1)} {
		if got := limitJunctionStep(v, 0.2); math.Float64bits(got) != math.Float64bits(v) {
			t.Errorf("关闭时 limitJunctionStep(%v, 0.2) 期望原样返回，实际 %v（位模式 %x ≠ %x）",
				v, got, math.Float64bits(got), math.Float64bits(v))
		}
	}

	// 非法限幅值必须报错且不改变当前配置（关闭状态下仍为关闭）。
	for _, bad := range []float64{0, -1, MIN_LIMIT_VOLTS / 2, MAX_LIMIT_VOLTS * 2, math.NaN()} {
		if err := SetIterationLimiting(true, bad); err == nil {
			t.Errorf("SetIterationLimiting(true, %v) 期望报错，实际成功", bad)
		}
		if got := IterationLimitingMaxStep(); got != 0 {
			t.Errorf("非法值 %v 之后 MaxStep 期望仍为 0，实际 %v", bad, got)
		}
	}

	if err := SetIterationLimiting(true, abLimitVolts); err != nil {
		t.Fatalf("开启限制失败: %v", err)
	}
	defer func() { _ = SetIterationLimiting(false, 0) }()
	if got := IterationLimitingMaxStep(); got != abLimitVolts {
		t.Fatalf("开启后 MaxStep 期望 %v，实际 %v", abLimitVolts, got)
	}
	ResetLimitingActivations()
	cases := []struct {
		newVolt, oldVolt, want float64
		clamped                bool
	}{
		{3.7, 0, abLimitVolts, true},
		{-3.7, 0, -abLimitVolts, true},
		{0.1, 0, 0.1, false},
		{0.25, 0, 0.25, false},  // 恰好等于上界：不钳位
		{0.2501, 0, 0.25, true}, // 略超上界：钳到上界
		{-0.9, -0.5, -0.75, true},
		{-0.5, -0.5, -0.5, false},
	}
	for _, c := range cases {
		got := limitJunctionStep(c.newVolt, c.oldVolt)
		if got != c.want {
			t.Errorf("limitJunctionStep(%v, %v) = %v，期望 %v", c.newVolt, c.oldVolt, got, c.want)
		}
	}
	if got, want := LimitingActivations(), uint64(4); got != want {
		t.Errorf("钳位计数期望 %d，实际 %d", want, got)
	}
}

// junctionDevice 器件 DoStep 的最小接口面（供按类型构造具体器件）。
type junctionDevice interface {
	DoStep(mna.Mna, mna.Time, element.NodeFace)
}

// junctionProbe 一个器件的固定激励序列与结电压存储槽。
type junctionProbe struct {
	name     string
	netlist  string
	elemType element.NodeType
	device   func(cfg *element.Config) junctionDevice
	drive    func(value element.NodeFace) []mna.NodeID
	slots    []int
	stimuli  [][]float64
}

// junctionProbes 六个器件的固定激励：含正/反向大信号跳变与小幅步进。
var junctionProbes = []junctionProbe{
	{
		name: "diode", netlist: "d1 [0,-1] [1e-14,0,1.0,0.1,300.15]", elemType: DiodeType,
		device: func(cfg *element.Config) junctionDevice { return Diode{cfg} },
		drive:  diodeDrive,
		slots:  []int{5},
		stimuli: [][]float64{
			{0, 0}, {3, 3}, {3, 3}, {-4, -4}, {0.6, 0.6}, {0.75, 0.75}, {0.9, 0.9}, {-6, -6},
		},
	},
	{
		name: "zener", netlist: "z1 [0,-1] [1e-14,5.1,1,0.1]", elemType: ZenerType,
		device: func(cfg *element.Config) junctionDevice { return Zener{cfg} },
		drive:  diodeDrive,
		slots:  []int{5},
		stimuli: [][]float64{
			{0, 0}, {-5, -5}, {-5.2, -5.2}, {-5.05, -5.05}, {3, 3}, {-7, -7},
		},
	},
	{
		name: "led", netlist: "led1 [0,-1] [1e-18,0,1.5,5]", elemType: LEDType,
		device: func(cfg *element.Config) junctionDevice { return LED{cfg} },
		drive:  diodeDrive,
		slots:  []int{5},
		stimuli: [][]float64{
			{0, 0}, {2, 2}, {2.5, 2.5}, {1, 1}, {4, 4},
		},
	},
	{
		name: "transistor", netlist: "q1 [0,1,2] [false,100]", elemType: TransistorType,
		device: func(cfg *element.Config) junctionDevice { return Transistor{cfg} },
		drive:  threePinDrive,
		slots:  []int{3, 4},
		stimuli: [][]float64{
			{0, 0, 0}, {0.7, 5, 0}, {0.8, 5, 0}, {0, 5, 0}, {-0.7, 0, 0}, {0.75, 0.2, 0},
		},
	},
	{
		name: "mosfet", netlist: "m1 [0,1,2] [0,1.5,2e-5,0.01]", elemType: MOSFETType,
		device: func(cfg *element.Config) junctionDevice { return MOSFET{cfg} },
		drive:  threePinDrive,
		slots:  []int{4, 5},
		stimuli: [][]float64{
			{0, 0, 0}, {5, 3, 0}, {5, 4, 0}, {0, 3, 0}, {5, 0, 0},
		},
	},
	{
		name: "jfet", netlist: "j1 [0,1,2] [0,2.0,1e-4,0.02]", elemType: JFETType,
		device: func(cfg *element.Config) junctionDevice { return JFET{cfg} },
		drive:  threePinDrive,
		slots:  []int{4, 5},
		stimuli: [][]float64{
			{0, 0, 0}, {5, -1, 0}, {5, -3, 0}, {0, 0, 0}, {5, -2, 0},
		},
	},
}

// diodeDrive 二极管族（D/Z/LED）的受控节点：外部阳极 + 内部 PN 结节点。
func diodeDrive(value element.NodeFace) []mna.NodeID {
	return []mna.NodeID{value.GetNodes(0), value.GetNodesInternal(0)}
}

// threePinDrive 三端器件（BJT/MOS/JFET）的受控节点：三个外部引脚。
func threePinDrive(value element.NodeFace) []mna.NodeID {
	return []mna.NodeID{value.GetNodes(0), value.GetNodes(1), value.GetNodes(2)}
}

// newProbeContext 按固定网表构建上下文（完成 Reset）并取回目标器件。
func newProbeContext(t *testing.T, netlist string, elemType element.NodeType) (*element.Context, element.NodeFace) {
	t.Helper()
	con, err := load.LoadString(netlist)
	if err != nil {
		t.Fatalf("加载网表失败 %q: %v", netlist, err)
	}
	tm, err := etime.NewTimeMNA(1e-4)
	if err != nil {
		t.Fatalf("创建仿真时间失败: %v", err)
	}
	con.Time = tm
	if err := con.CallMark(element.MarkReset); err != nil {
		t.Fatalf("元件重置失败: %v", err)
	}
	for _, elem := range con.Nodelist {
		if elem.Type() == elemType {
			return con, elem
		}
	}
	t.Fatalf("网表中未找到类型 %d 的器件: %q", elemType, netlist)
	return nil, nil
}

// driveProbe 清空器件盖章并写入本步激励电压（地节点电压恒为 0，跳过）。
func driveProbe(con *element.Context, nodes []mna.NodeID, volts []float64) {
	con.A.Rollback()
	con.Z.Rollback()
	con.GetA().Base().Zero()
	con.GetZ().Base().Zero()
	x := con.GetX()
	for i, n := range nodes {
		if n <= mna.Gnd || int(n) >= x.Length() {
			continue
		}
		x.Set(int(n), volts[i])
	}
}

// readSlots 读取器件状态槽位（结电压历史）。
func readSlots(node element.NodeFace, slots []int) []float64 {
	out := make([]float64, len(slots))
	for i, s := range slots {
		out[i] = node.GetFloat64(s)
	}
	return out
}

// TestLimitingClampsDeviceJunctionStep 接线级差分：同一激励序列下限制开/关的结电压步进对照。
func TestLimitingClampsDeviceJunctionStep(t *testing.T) {
	maxStep := abLimitVolts
	for _, p := range junctionProbes {
		t.Run(p.name, func(t *testing.T) {
			run := func(enabled bool) [][]float64 {
				if err := SetIterationLimiting(enabled, maxStep); err != nil {
					t.Fatalf("设置限制开关失败: %v", err)
				}
				defer func() { _ = SetIterationLimiting(false, 0) }()
				con, node := newProbeContext(t, p.netlist, p.elemType)
				dev := p.device(node.Config())
				nodes := p.drive(node)
				var deltas [][]float64
				prev := readSlots(node, p.slots)
				for step, volts := range p.stimuli {
					driveProbe(con, nodes, volts)
					dev.DoStep(con, con.Time, node)
					cur := readSlots(node, p.slots)
					if step > 0 {
						row := make([]float64, len(cur))
						for i := range cur {
							row[i] = math.Abs(cur[i] - prev[i])
						}
						deltas = append(deltas, row)
					}
					prev = cur
				}
				return deltas
			}

			offDeltas := run(false)
			onDeltas := run(true)
			offMax := 0.0
			for _, row := range offDeltas {
				for _, d := range row {
					offMax = math.Max(offMax, d)
				}
			}
			if offMax <= maxStep {
				t.Fatalf("关闭限制时最大结电压步进 %.6gV 未超过 %.2fV：该激励不是大信号跳变，用例失去判别力",
					offMax, maxStep)
			}
			for i, row := range onDeltas {
				for j, d := range row {
					if d > maxStep+1e-12 {
						t.Errorf("开启限制后第 %d 步槽位 %d 的结电压变化 %.9gV 超过上限 %.2fV",
							i+1, p.slots[j], d, maxStep)
					}
				}
			}
			t.Logf("M4CLAMP device=%s offMaxDelta=%.9g onMaxDelta=%.9g maxStep=%.2f", p.name, offMax, maxOf(onDeltas), maxStep)
		})
	}
}

// maxOf 返回二维切片中的最大值（空集合返回 0）。
func maxOf(rows [][]float64) float64 {
	out := 0.0
	for _, row := range rows {
		for _, v := range row {
			out = math.Max(out, v)
		}
	}
	return out
}

// stampTrace 收集六个器件在固定激励序列下的盖章（A + Z）与状态槽位的位模式轨迹。
// 返回值同时给出非零值计数，用于防止"轨迹全零"导致的假通过。
func stampTrace(t *testing.T) (string, int) {
	t.Helper()
	var sb strings.Builder
	nonZero := 0
	buf := make([]byte, 8)
	writeF := func(v float64) {
		binary.LittleEndian.PutUint64(buf, math.Float64bits(v))
		sb.Write(buf)
		if v != 0 {
			nonZero++
		}
	}
	for _, p := range junctionProbes {
		con, node := newProbeContext(t, p.netlist, p.elemType)
		dev := p.device(node.Config())
		nodes := p.drive(node)
		for step, volts := range p.stimuli {
			driveProbe(con, nodes, volts)
			dev.DoStep(con, con.Time, node)
			writeF(float64(step))
			for _, s := range p.slots {
				writeF(node.GetFloat64(s))
			}
			a := con.GetA()
			for i := 0; i < a.Rows(); i++ {
				for j := 0; j < a.Cols(); j++ {
					writeF(a.Get(i, j))
				}
			}
			z := con.GetZ()
			for i := 0; i < z.Length(); i++ {
				writeF(z.Get(i))
			}
		}
	}
	return sb.String(), nonZero
}

// TestLimitingDisabledBitExactGolden 关闭限制时器件盖章轨迹必须逐位复现接线前的 golden 摘要。
func TestLimitingDisabledBitExactGolden(t *testing.T) {
	if err := SetIterationLimiting(false, 0); err != nil {
		t.Fatalf("关闭限制失败: %v", err)
	}
	trace, nonZero := stampTrace(t)
	if nonZero == 0 {
		t.Fatal("盖章轨迹全零：用例失去判别力（器件未产生任何盖章）")
	}
	sum := sha256.Sum256([]byte(trace))
	got := hex.EncodeToString(sum[:])
	if os.Getenv(goldenCaptureEnv) != "" {
		fmt.Printf("M4_GOLDEN_SHA256=%s\n", got)
		t.Logf("M4_GOLDEN_SHA256=%s nonZero=%d", got, nonZero)
		return
	}
	if got != preWireStampDigestSHA256 {
		t.Errorf("关闭限制时盖章轨迹与接线前不一致：实际 sha256=%s，期望 %s（nonZero=%d）",
			got, preWireStampDigestSHA256, nonZero)
	}
	t.Logf("M4GOLDEN sha256=%s nonZero=%d", got, nonZero)
}

// timeDecorator 以不同字段名内嵌 mna.Time：直接内嵌 mna.Time 会与接口方法
// Time() float64 同名冲突（字段遮蔽方法，包装类型不满足 mna.Time）。
// timeDecorator 转发**引擎状态面** EngineTime（UD7 约定写法）。
//
// 起初它只嵌入 mna.Time，但 TransientSimulation 需要策略同步/有界牛顿事件计数/
// 强制重盖标志这些方法（见 element/time.EngineTime）；只转发 mna.Time 的包装会被
// 入口显式拒绝（UD7：宁可报错，也不静默丢语义）。匿名嵌入 EngineTime 即可整体转发。
type timeDecorator interface {
	etime.EngineTime
}

// countingTime 包装 EngineTime，统计引擎的非线性迭代轮次与未收敛信号。
// K4 的 BoundedNewtonEvents 由 m5 落地；本包装只复用接口上既有的计数方法，
// 不进入 element/time 写域（m5）。
type countingTime struct {
	timeDecorator
	newtonRounds int
	elemRounds   int
	noConverged  int
}

// NextNonlinearIter 统计执行到的外层牛顿迭代轮次。
func (c *countingTime) NextNonlinearIter() bool {
	ok := c.timeDecorator.NextNonlinearIter()
	if ok {
		c.newtonRounds++
	}
	return ok
}

// NextElemIter 统计执行到的元件子迭代轮次。
func (c *countingTime) NextElemIter() bool {
	ok := c.timeDecorator.NextElemIter()
	if ok {
		c.elemRounds++
	}
	return ok
}

// NoConverged 统计器件级未收敛信号次数。
func (c *countingTime) NoConverged() {
	c.noConverged++
	c.timeDecorator.NoConverged()
}

// engineRun 一次完整瞬态仿真的可观测结果。
type engineRun struct {
	steps        int
	newtonRounds int
	elemRounds   int
	noConverged  int
	clamped      uint64
	final        []float64
	err          error
}

// runEngineCase 按 CLI 批处理同口径参数跑一次语料，限制开关由 limitVolts 决定（0=关闭）。
func runEngineCase(t *testing.T, file string, limitVolts float64) engineRun {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(abCorpusDir, file))
	if err != nil {
		t.Fatalf("读取语料失败 %s: %v", file, err)
	}
	if err := SetIterationLimiting(limitVolts > 0, limitVolts); err != nil {
		t.Fatalf("设置限制开关失败: %v", err)
	}
	defer func() { _ = SetIterationLimiting(false, 0) }()
	ResetLimitingActivations()

	con, err := load.LoadString(string(text))
	if err != nil {
		return engineRun{err: err}
	}
	tm, err := etime.NewTimeMNA(1e-4)
	if err != nil {
		return engineRun{err: err}
	}
	ct := &countingTime{timeDecorator: tm}
	con.Time = ct
	con.InitResumeCond()
	if err := tm.SetTolerances(1e-6, 1e-4); err != nil {
		return engineRun{err: err}
	}
	if err := tm.SetStepLimits(1e-9, 5e-4); err != nil {
		return engineRun{err: err}
	}
	if err := tm.SetIterationLimits(200, tm.MaxElemIter()); err != nil {
		return engineRun{err: err}
	}
	if err := tm.SetMaxTimeSteps(10000); err != nil {
		return engineRun{err: err}
	}
	tm.SetTimeStep(1e-6)

	run := engineRun{}
	run.err = etime.TransientSimulation(con, func(v []float64) {
		run.steps++
		run.final = append(run.final[:0], v...)
	})
	run.newtonRounds = ct.newtonRounds
	run.elemRounds = ct.elemRounds
	run.noConverged = ct.noConverged
	run.clamped = LimitingActivations()
	return run
}

// abCases 对照语料：契约列出的六条非线性网表（26 号不含半导体器件，作阴性对照）。
var abCases = []string{
	"03_diode_rectifier.net",
	"04_transistor_switch.net",
	"14_zener_regulator.net",
	"15_mosfet_switch.net",
	"16_led_driver.net",
	"26_eh_converter.net",
}

// TestLimitingIterationCountComparison 在固定语料上对照限制开/关的迭代与步数。
//
// 硬断言只保留两条必须成立的不变量：①关闭限制的基线运行必须成功；
// ②两种模式都成功时，末步节点电压的相对偏差必须落在求解器容差量级内（≤1e-3，
// 即组件级 relTol=1e-4 的 10 倍）。收益（迭代轮次是否下降）以日志给出，
// 由 compare.md 记录；不下降即记负结果，不在此处用断言掩盖。
func TestLimitingIterationCountComparison(t *testing.T) {
	const relTolBound = 1e-3
	for _, file := range abCases {
		off := runEngineCase(t, file, 0)
		if off.err != nil {
			t.Fatalf("基线（限制关闭）运行失败 %s: %v", file, off.err)
		}
		on := runEngineCase(t, file, abLimitVolts)
		t.Logf("M4AB case=%s mode=off steps=%d newton=%d elem=%d noConv=%d clamped=%d err=%v",
			file, off.steps, off.newtonRounds, off.elemRounds, off.noConverged, off.clamped, off.err)
		t.Logf("M4AB case=%s mode=on steps=%d newton=%d elem=%d noConv=%d clamped=%d err=%v",
			file, on.steps, on.newtonRounds, on.elemRounds, on.noConverged, on.clamped, on.err)
		if on.err != nil {
			t.Logf("M4AB case=%s 限制开启后运行失败（负结果，记入 compare.md）: %v", file, on.err)
			continue
		}
		if len(on.final) != len(off.final) {
			t.Errorf("%s 末步解向量长度不一致：on=%d off=%d", file, len(on.final), len(off.final))
			continue
		}
		maxRel := 0.0
		for i := range off.final {
			den := math.Max(1, math.Abs(off.final[i]))
			maxRel = math.Max(maxRel, math.Abs(on.final[i]-off.final[i])/den)
		}
		t.Logf("M4AB case=%s maxRelFinalVoltageDiff=%.3g (bound=%.0e)", file, maxRel, relTolBound)
		if maxRel > relTolBound {
			t.Errorf("%s 限制开启后末步解偏离基线 %.3g，超过求解器容差量级 %.0e", file, maxRel, relTolBound)
		}
	}
}
