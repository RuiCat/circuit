// limitingBoundary_test.go —— T7：二极管限幅 limitDiodeStep（CircuitJS1 版 pnjlim）的
// 边界判据，以及与 SPICE3 pnjlim 的口径对照。
//
// 口径对照（逐条对应 Diode.go:212-269 的实现）：
//   - 一致：触发条件 vnew > vcrit 且 |vnew-vold| > 2·vscale，其中 vscale = N·Vt
//     （与 SPICE3 以 vt=N·kT/q 调用 pnjlim 同口径）；
//   - 一致：vold > 0 分支 vnew = vold + vscale·ln(1 + (vnew-vold)/vscale)，
//     即把「指数电流变化 e²」的迭代步长收回上一次的线性化点；
//   - 差异：vold ≤ 0 分支本实现钳到 vcrit（CircuitJS1 做法），
//     SPICE3 用 vnew = vscale·ln(vnew/vscale)。默认行为保持不变，本文件只把差异钉成可判定事实；
//   - 差异：齐纳分支是 CircuitJS1 的自定义平移（zoffset、用 Vt 而非 N·Vt），
//     SPICE3 的 pnjlim 无对应分支；
//   - 补充：limiting.go 的 limitJunctionStep 是「每次迭代电压变化 ≤ maxStep」的附加约束，
//     不是 SPICE pnjlim，二者可独立开关（默认均关闭）。
package semiconductor

import (
	"math"
	"testing"

	"circuit/element"
	"circuit/mna"
)

// noConvCounter 是只统计 NoConverged 次数的 mna.Time 替身（limitDiodeStep 只用该方法）。
type noConvCounter struct {
	timeBase
	n int
}

// NoConverged 记录一次未收敛信号。
func (c *noConvCounter) NoConverged() { c.n++ }

// limitHarnessNode 构造 limitDiodeStep 读取的槽位：6=vscale、8=Vt、10=zoffset、
// 11=vcrit、12=vzcrit（与 Diode.Reset 写入位置一致）。
func limitHarnessNode(vscale, Vt, vcrit, vzcrit, zoffset float64) *element.Node {
	vals := make([]any, 16)
	for i := range vals {
		vals[i] = float64(0)
	}
	vals[6] = vscale
	vals[8] = Vt
	vals[10] = zoffset
	vals[11] = vcrit
	vals[12] = vzcrit
	return &element.Node{NodeValue: vals}
}

// approxEqual 用于浮点判据（相对容差 1e-12）。
func approxEqual(got, want float64) bool {
	return math.Abs(got-want) <= 1e-12*math.Max(1, math.Abs(want))
}

// TestPnjlimNoClampBoundaries 覆盖「不触发」的两条边界：vnew 恰等于 vcrit、
// 电压变化量小于 2·vscale。
func TestPnjlimNoClampBoundaries(t *testing.T) {
	const vscale, Vt, vold = 0.02585, 0.02585, 0.5
	// 恰好等于 vcrit：触发条件用严格大于，不得钳位。
	node := limitHarnessNode(vscale, Vt, 0.6, 0, 0)
	ts := &noConvCounter{}
	if got := limitDiodeStep(0.6, vold, ts, node); got != 0.6 {
		t.Errorf("vnew==vcrit 不得钳位：得到 %v，期望 0.6", got)
	}
	// 变化量 1.5·vscale < 2·vscale：指数电流变化不足 e²，不得钳位。
	vnew := vold + 1.5*vscale
	if got := limitDiodeStep(vnew, vold, ts, node); got != vnew {
		t.Errorf("|Δv|=1.5·vscale 不得钳位：得到 %v，期望 %v", got, vnew)
	}
	if ts.n != 0 {
		t.Errorf("未触发限幅时不得报 NoConverged，实际 %d 次", ts.n)
	}
}

// TestPnjlimForwardClamp 覆盖触发后的 vold>0 分支：结果等于 vold+vscale·ln(1+Δ/vscale)，
// 且满足「指数电流比 = 1+Δ/vscale」这一 SPICE 判据语义。
func TestPnjlimForwardClamp(t *testing.T) {
	const vscale, Vt = 0.02585, 0.02585
	node := limitHarnessNode(vscale, Vt, 0.2, 0, 0)
	ts := &noConvCounter{}
	const vold, vnew = 0.5, 0.5 + 2.5*vscale // Δ=2.5·vscale > 2·vscale
	got := limitDiodeStep(vnew, vold, ts, node)
	want := vold + vscale*math.Log(1+(vnew-vold)/vscale)
	if !approxEqual(got, want) {
		t.Fatalf("正向钳位 = %v，期望 vold+vscale·ln(1+Δ/vscale) = %v", got, want)
	}
	if ts.n != 1 {
		t.Fatalf("钳位必须报一次 NoConverged，实际 %d 次", ts.n)
	}
	// 指数电流比：exp(√/vscale) 之比应等于 1+Δ/vscale（而非原始 e^Δ/vscale）。
	ratio := math.Exp((got - vold) / vscale)
	if !approxEqual(ratio, 1+(vnew-vold)/vscale) {
		t.Fatalf("钳位后的指数电流比 = %v，期望 %v", ratio, 1+(vnew-vold)/vscale)
	}

	// arg ≤ 0（大幅回退且仍高于 vcrit）：回退到 vcrit。
	nodeLowCrit := limitHarnessNode(vscale, Vt, 0, 0, 0)
	ts2 := &noConvCounter{}
	if got := limitDiodeStep(vold-3*vscale, vold, ts2, nodeLowCrit); got != 0 {
		t.Errorf("arg≤0 时应钳到 vcrit=0，实际 %v", got)
	}
	if ts2.n != 1 {
		t.Errorf("arg≤0 分支必须报 NoConverged，实际 %d 次", ts2.n)
	}
}

// TestPnjlimVoldNonPositive 钉住 CircuitJS1 与 SPICE3 的已知差异：
// vold≤0 时本实现钳到 vcrit；SPICE3 的 pnjlim 会用 vscale·ln(vnew/vscale)。
func TestPnjlimVoldNonPositive(t *testing.T) {
	const vscale, Vt, vcrit = 0.02585, 0.02585, 0.5
	node := limitHarnessNode(vscale, Vt, vcrit, 0, 0)
	ts := &noConvCounter{}
	got := limitDiodeStep(0.7, 0, ts, node)
	if got != vcrit {
		t.Errorf("vold≤0 分支应钳到 vcrit=%v（CircuitJS1 口径），实际 %v", vcrit, got)
	}
	spice3 := vscale * math.Log(0.7/vscale) // 仅作对照，不参与断言
	t.Logf("口径差异：本实现 %.6f V；SPICE3 pnjlim 同输入为 vscale·ln(vnew/vscale)=%.6f V（默认行为不改）",
		got, spice3)
	if ts.n != 1 {
		t.Errorf("vold≤0 钳位必须报 NoConverged，实际 %d 次", ts.n)
	}
}

// TestPnjlimZenerTranslated 覆盖齐纳平移分支：vnewTrans=-vnew-zoffset，阈值用 Vt（非 N·Vt）。
func TestPnjlimZenerTranslated(t *testing.T) {
	const Vt, zoffset = 0.02585, 3.0
	node := limitHarnessNode(0.02585, Vt, 0.6, 0.5, zoffset)
	ts := &noConvCounter{}

	// voldTrans>0：vold=-4 ⇒ voldTrans=1；vnew=-5 ⇒ vnewTrans=2，钳到 voldTrans+Vt·ln(1+Δ/Vt)。
	got := limitDiodeStep(-5.0, -4.0, ts, node)
	wantTrans := 1.0 + Vt*math.Log(1+(2.0-1.0)/Vt)
	if want := -(wantTrans + zoffset); !approxEqual(got, want) {
		t.Fatalf("齐纳 voldTrans>0 钳位 = %v，期望 %v", got, want)
	}
	if ts.n != 1 {
		t.Fatalf("齐纳钳位必须报 NoConverged，实际 %d 次", ts.n)
	}

	// voldTrans≤0：vold=-2 ⇒ voldTrans=-1；vnew=-4 ⇒ vnewTrans=1，
	// 走 vnewTrans=Vt·ln(vnewTrans/Vt) 分支。
	ts2 := &noConvCounter{}
	got2 := limitDiodeStep(-4.0, -2.0, ts2, node)
	wantTrans2 := Vt * math.Log(1.0/Vt)
	if want := -(wantTrans2 + zoffset); !approxEqual(got2, want) {
		t.Fatalf("齐纳 voldTrans≤0 钳位 = %v，期望 %v", got2, want)
	}
	if ts2.n != 1 {
		t.Errorf("齐纳 voldTrans≤0 钳位必须报 NoConverged，实际 %d 次", ts2.n)
	}
}

// TestPnjlimVscaleZeroSkips 覆盖 vscale==0（参数未初始化）时短路：原样返回、不报未收敛。
func TestPnjlimVscaleZeroSkips(t *testing.T) {
	node := limitHarnessNode(0, 0.02585, 0.5, 0.5, 3.0)
	ts := &noConvCounter{}
	if got := limitDiodeStep(5.0, -5.0, ts, node); got != 5.0 {
		t.Errorf("vscale==0 时应原样返回，实际 %v", got)
	}
	if ts.n != 0 {
		t.Errorf("vscale==0 时不得报 NoConverged，实际 %d 次", ts.n)
	}
}

// 编译期断言：noConvCounter 确实满足 limitDiodeStep 对 mna.Time 的要求。
var _ mna.Time = (*noConvCounter)(nil)
