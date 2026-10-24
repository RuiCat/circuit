// expTable_test.go —— T4 表驱动指数的精度证据、开关语义与解析/查表耗时对比。
//
// 三条判据：
//  1. 默认关闭时与解析路径逐位一致（默认行为不变）；
//  2. 开启后表内最大相对误差 < 1e-5（实测约 1.9e-6，最坏点取每个插值区间中点）；
//  3. 表外/NaN 回退解析，结果与关闭路径逐位相同。
package semiconductor

import (
	"math"
	"testing"

	"circuit/element"
	"circuit/mna"
)

// expAnalytic 是"解析参考路径"：与关闭开关时的 deviceExp 同一算式。
func expAnalytic(x float64) float64 {
	return math.Exp(clampExpArg(x))
}

// resetTestToggles 把包级测试开关恢复到默认：迭代电压限制关闭、T4 表驱动关闭。
// 由 TestMain 调用，避免外部环境变量改变既有 golden / 位精确用例的前提。
func resetTestToggles() {
	_ = SetIterationLimiting(false, 0)
	SetExpTableEnabled(false)
}

// TestExpTableDefaultOffMatchesAnalytic 覆盖判据 1：默认关闭，逐位等于改造前 safeExp。
func TestExpTableDefaultOffMatchesAnalytic(t *testing.T) {
	SetExpTableEnabled(false)
	defer SetExpTableEnabled(false)
	if ExpTableEnabled() {
		t.Fatal("表驱动指数默认必须关闭（C1：默认行为不变）")
	}
	for _, x := range []float64{-700, -100, -32, -1.5, -1e-12, 0, 1e-12, 0.7, 23.5, 32, 100, 700} {
		got := deviceExp(x)
		want := expAnalytic(x)
		if math.Float64bits(got) != math.Float64bits(want) {
			t.Errorf("关闭时 deviceExp(%v) = %v，期望解析值 %v（位模式不同）", x, got, want)
		}
	}
}

// TestExpTableMaxRelativeError 覆盖判据 2：表中每个区间中点（插值最坏点）+ 1mV 密集网格，
// 最大相对误差必须 < 1e-5；同时断言表外与 NaN/±Inf 回退解析（判据 3）。
func TestExpTableMaxRelativeError(t *testing.T) {
	SetExpTableEnabled(true)
	defer SetExpTableEnabled(false)

	maxRel := 0.0
	maxAt := 0.0
	record := func(x, got float64) {
		want := expAnalytic(x)
		rel := math.Abs(got-want) / want
		if rel > maxRel {
			maxRel, maxAt = rel, x
		}
	}
	// 每个插值区间中点：线性插值 e^x 的最大相对误差点。
	for i := 0; i < expTableCount-1; i++ {
		x := -expTableMax + (float64(i)+0.5)*expTableStep
		record(x, expTableLookup(x))
	}
	// 1mV 网格（覆盖节点电压实际取值范围）。
	for j := 0; j <= 64000; j++ {
		x := -expTableMax + float64(j)*1e-3
		record(x, expTableLookup(x))
	}
	// 表边界。
	for _, x := range []float64{-expTableMax, expTableMax} {
		record(x, expTableLookup(x))
	}
	if !(maxRel < 1e-5) {
		t.Fatalf("表驱动最大相对误差 %g（@x=%g）超过阈值 1e-5", maxRel, maxAt)
	}
	t.Logf("表驱动指数 x∈[%g,%g] 最大相对误差 = %.3e @ x=%.6g（阈值 1e-5，理论 h²/8≈%.2e）",
		-expTableMax, expTableMax, maxRel, maxAt, expTableStep*expTableStep/8)

	// 判据 3：表外回退必须与解析路径逐位一致。
	for _, x := range []float64{-40, -32.001, 32.001, 40, 200, -200,
		math.Inf(1), math.Inf(-1)} {
		got, want := expTableLookup(x), expAnalytic(x)
		if math.Float64bits(got) != math.Float64bits(want) {
			t.Errorf("表外 deviceExp(%v) = %v，期望解析值 %v", x, got, want)
		}
	}
	if got := expTableLookup(math.NaN()); !math.IsNaN(got) {
		t.Errorf("NaN 输入应回退为 NaN，实际 %v", got)
	}
}

// TestExpTableSwitchAndEnv 覆盖开关的程序化与环境变量入口（非法值告警但不改变状态）。
func TestExpTableSwitchAndEnv(t *testing.T) {
	defer SetExpTableEnabled(false)
	SetExpTableEnabled(false)
	if ExpTableEnabled() {
		t.Fatal("初始应为关闭")
	}
	SetExpTableEnabled(true)
	if !ExpTableEnabled() {
		t.Fatal("SetExpTableEnabled(true) 后应为开启")
	}

	applyExpTableEnv("not-a-bool") // 非法值：告警并保持原状态
	if !ExpTableEnabled() {
		t.Fatal("非法环境变量不得静默改变开关状态")
	}
	applyExpTableEnv("0")
	if ExpTableEnabled() {
		t.Fatal("CIRCUIT_EXPTABLE=0 应关闭")
	}
	applyExpTableEnv("1")
	if !ExpTableEnabled() {
		t.Fatal("CIRCUIT_EXPTABLE=1 应开启")
	}
	applyExpTableEnv("")
	if !ExpTableEnabled() {
		t.Fatal("空环境变量表示不改动，应保持开启")
	}
}

// TestExpTableLedZenerShareDeviceExp 钉住任务 1：LED / Zener 的私有 clamp+math.Exp
// 已删除、统一走 deviceExp。判据两半：
//   - 关闭时与解析算式逐位一致（默认行为不变）；
//   - 开启时结果必须与解析值不同（证明真的走了查表插值，而非残留 math.Exp），
//     且相对误差 < 1e-5。
func TestExpTableLedZenerShareDeviceExp(t *testing.T) {
	defer SetExpTableEnabled(false)
	const vdcoef = 38.66
	// vd=0.35 ⇒ 参数 x≈13.53，落在 LED/Zener 典型导通区；先确认它不在 1/256 表节点上，
	// 否则插值恰好等于解析值，无法区分"查表"与"解析"。
	// vd 用变量而非无类型常量：保证 arg 与器件内 voltdiff*vdcoef 同一运算顺序与舍入。
	vd := 0.35
	arg := vd * vdcoef
	if pos := (arg + expTableMax) * expTableScale; pos == math.Trunc(pos) {
		t.Fatalf("测试参数 %v 恰落在表节点上，无法区分查表与解析", arg)
	}
	node := diodeHarnessNode()
	node.SetFloat64(7, vdcoef) // 1/(N·Vt)
	want := node.GetFloat64(13) * (expAnalytic(arg) - 1)

	for _, c := range []struct {
		name string
		fn   func(float64, element.NodeFace) float64
	}{
		{"LED", ledCalculateCurrent},
		{"Zener", zenerCalculateCurrent},
	} {
		SetExpTableEnabled(false)
		if off := c.fn(vd, node); math.Float64bits(off) != math.Float64bits(want) {
			t.Errorf("%s 关闭表驱动时 = %v，期望解析值 %v（位模式不同）", c.name, off, want)
		}
		SetExpTableEnabled(true)
		on := c.fn(vd, node)
		if math.Float64bits(on) == math.Float64bits(want) {
			t.Errorf("%s 开启表驱动后与解析值逐位相同：该路径未走 deviceExp", c.name)
		}
		if rel := math.Abs(on-want) / math.Abs(want); !(rel < 1e-5) {
			t.Errorf("%s 开启表驱动相对误差 %g 超过阈值 1e-5", c.name, rel)
		}
	}
}

// BenchmarkDeviceExp 对比单次指数求值：解析 math.Exp vs 查表插值。
func BenchmarkDeviceExp(b *testing.B) {
	xs := benchExpArgs()
	for _, mode := range []struct {
		name string
		on   bool
	}{{"analytic", false}, {"table", true}} {
		b.Run(mode.name, func(b *testing.B) {
			SetExpTableEnabled(mode.on)
			defer SetExpTableEnabled(false)
			b.ResetTimer()
			var sink float64
			for i := 0; i < b.N; i++ {
				sink += deviceExp(xs[i&(len(xs)-1)])
			}
			benchSink = sink
		})
	}
}

// BenchmarkDiodeStepExp 对比二极管两种指数分支的整段线性化：
// 正向（1 次 exp）与齐纳反偏（2 次 exp）；stamp 用空实现，只计指数与算术开销。
func BenchmarkDiodeStepExp(b *testing.B) {
	cases := []struct {
		name  string
		node  func() *element.Node
		voltd float64
	}{
		{name: "forward", node: forwardDiodeHarnessNode, voltd: 0.65},
		{name: "zener", node: zenerDiodeHarnessNode, voltd: -5.0},
	}
	for _, c := range cases {
		for _, mode := range []struct {
			name string
			on   bool
		}{{"analytic", false}, {"table", true}} {
			b.Run(c.name+"/"+mode.name, func(b *testing.B) {
				node := c.node()
				ts := &timeStub{}
				rec := discardStamp{}
				SetExpTableEnabled(mode.on)
				defer SetExpTableEnabled(false)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					doDiodeStep(rec, ts, node, c.voltd)
				}
			})
		}
	}
}

// benchSink 防止基准循环被编译器整体消除。
var benchSink float64

// benchExpArgs 生成确定性的指数参数样本：覆盖典型导通区 x≈[10,31] 与反偏小区间。
func benchExpArgs() []float64 {
	const n = 1024
	xs := make([]float64, n)
	for i := range xs {
		// 确定性伪随机：LCG 落在 [-5, 31]，与二极管实际参数区间一致。
		v := float64((uint64(i)*6364136223846793005+1442695040888963407)>>33&0x3fff) / 16383.0
		xs[i] = -5 + 36*v
	}
	return xs
}

// forwardDiodeHarnessNode 是常规二极管线性化节点（Vz=0 → 单次 exp）。
func forwardDiodeHarnessNode() *element.Node {
	return diodeHarnessNode()
}

// zenerDiodeHarnessNode 是齐纳反偏节点（Vz>0 且 voltdiff<0 → 双 exp 分支）。
func zenerDiodeHarnessNode() *element.Node {
	node := diodeHarnessNode()
	node.SetFloat64(1, 5.0)  // Vz
	node.SetFloat64(10, 4.3) // zoffset
	return node
}

// discardStamp 是基准用的空盖章器（只实现器件真正调用的方法，避免切片增长干扰计时）。
type discardStamp struct{ mna.Mna }

func (discardStamp) StampAdmittance(mna.NodeID, mna.NodeID, float64) {}

func (discardStamp) StampCurrentSource(mna.NodeID, mna.NodeID, float64) {}
