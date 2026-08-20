package powerflow

import (
	"math"
	"testing"
)

// TestSolve_TwoBus 2 母线解析解验证:
// Slack(1.0∠0) + PQ(P=0.5, Q=0.2),线路 Z=j0.1。
// 解析解: V2 = 1.0172+j0.05, |V2|≈1.01843, θ2≈0.04914rad;
// Slack 注入 S1 = -0.5-j0.172。
func TestSolve_TwoBus(t *testing.T) {
	in := Input{
		Buses: []Bus{
			{ID: 1, Type: BusSlack, V: 1.0},
			{ID: 2, Type: BusPQ, P: 0.5, Q: 0.2},
		},
		Branches: []Branch{
			{From: 1, To: 2, R: 0, X: 0.1},
		},
	}
	res, err := Solve(in, nil)
	if err != nil {
		t.Fatalf("Solve 失败: %v", err)
	}
	if !res.Converged {
		t.Fatalf("未收敛,最大失配 %g", res.IterationLog[len(res.IterationLog)-1])
	}
	v2 := res.BusV[2]
	mag := math.Hypot(real(v2), imag(v2))
	theta := math.Atan2(imag(v2), real(v2))
	if math.Abs(mag-1.018429) > 1e-5 {
		t.Errorf("|V2|: 期望 1.018429, 实际 %.7f", mag)
	}
	if math.Abs(theta-0.0491147) > 1e-5 {
		t.Errorf("θ2: 期望 0.0491147, 实际 %.7f", theta)
	}
	s1 := res.GenPower[1]
	if math.Abs(real(s1)+0.5) > 1e-5 || math.Abs(imag(s1)+0.17204) > 1e-5 {
		t.Errorf("Slack 注入: 期望 -0.5-j0.17204, 实际 %v", s1)
	}
	// 支路损耗 = Slack + PQ 注入总和(≈0 实部)
	bf := res.BranchFlow["1->2"]
	lossP, _ := bf.Loss()
	if math.Abs(lossP) > 1e-9 {
		t.Errorf("纯电抗支路有功损耗应为 0, 实际 %g", lossP)
	}
}

// TestSolve_ThreeBus 3 母线经典算例:
// Slack(1.05∠0) + PV(1.0, P=0.5) + PQ(P=-1.0, Q=-0.5),三角环网 R=0.01/X=0.1。
// 验证收敛、功率平衡、负荷压降。
func TestSolve_ThreeBus(t *testing.T) {
	in := Input{
		Buses: []Bus{
			{ID: 1, Type: BusSlack, V: 1.05},
			{ID: 2, Type: BusPV, V: 1.0, P: 0.5},
			{ID: 3, Type: BusPQ, P: -1.0, Q: -0.5},
		},
		Branches: []Branch{
			{From: 1, To: 2, R: 0.01, X: 0.1},
			{From: 1, To: 3, R: 0.01, X: 0.1},
			{From: 2, To: 3, R: 0.01, X: 0.1},
		},
	}
	res, err := Solve(in, nil)
	if err != nil {
		t.Fatalf("Solve 失败: %v", err)
	}
	if !res.Converged {
		t.Fatalf("未收敛,迭代 %d 轮,最后失配 %g", res.Iterations, res.IterationLog[len(res.IterationLog)-1])
	}
	if res.Iterations > 12 {
		t.Errorf("平启动应 10 轮内收敛,实际 %d 轮", res.Iterations)
	}
	// PV 母线电压保持 1.0
	v2 := res.BusV[2]
	if math.Abs(math.Hypot(real(v2), imag(v2))-1.0) > 1e-6 {
		t.Errorf("PV 母线电压幅值应保持 1.0, 实际 %g", math.Hypot(real(v2), imag(v2)))
	}
	// 负荷母线压降
	v3 := res.BusV[3]
	if math.Hypot(real(v3), imag(v3)) > 1.0 {
		t.Errorf("负荷母线电压应低于 1.0, 实际 %g", math.Hypot(real(v3), imag(v3)))
	}
	// 功率平衡:Σ注入 = 支路损耗(1e-6 容差)
	s1 := res.GenPower[1]
	s2 := res.GenPower[2]
	gen := complex(real(s1)+real(s2), imag(s1)+imag(s2))
	load := complex(-1.0, -0.5) // PQ 母线注入
	loss := res.TotalLoss
	diff := gen + load - loss
	if math.Abs(real(diff)) > 1e-6 || math.Abs(imag(diff)) > 1e-6 {
		t.Errorf("功率不平衡: Σgen=%v load=%v loss=%v diff=%v", gen, load, loss, diff)
	}
}

// TestSolve_PVReactiveLimit PV 无功越限转 PQ:
// PV(P=0.3, V=1.0, Qmax=0.1),PQ 大无功负荷 → Q 需求超限,固定 Q=0.1。
func TestSolve_PVReactiveLimit(t *testing.T) {
	in := Input{
		Buses: []Bus{
			{ID: 1, Type: BusSlack, V: 1.0},
			{ID: 2, Type: BusPV, V: 1.0, P: 0.3, Qmax: 0.1},
			{ID: 3, Type: BusPQ, P: -0.6, Q: -0.9},
		},
		Branches: []Branch{
			{From: 1, To: 2, R: 0.01, X: 0.1},
			{From: 2, To: 3, R: 0.01, X: 0.1},
		},
	}
	res, err := Solve(in, nil)
	if err != nil {
		t.Fatalf("Solve 失败: %v", err)
	}
	if !res.Converged {
		t.Fatalf("未收敛")
	}
	// PV 越限后转 PQ:Q2 应停在边界 ≈ 0.1(注入)
	q2 := imag(res.GenPower[2])
	if math.Abs(q2-0.1) > 1e-4 {
		t.Errorf("PV 越限后 Q2 应固定在 0.1, 实际 %g", q2)
	}
	// 电压幅值不再保持 1.0
	v2 := res.BusV[2]
	if mag := math.Hypot(real(v2), imag(v2)); mag > 0.999 {
		t.Errorf("越限转 PQ 后电压应下降, 实际 %g", mag)
	}
}

// TestSolve_Errors 错误路径
func TestSolve_Errors(t *testing.T) {
	// 少于 2 母线
	if _, err := Solve(Input{Buses: []Bus{{ID: 1, Type: BusSlack, V: 1.0}}}, nil); err == nil {
		t.Errorf("少于 2 母线应报错")
	}
	// 无 Slack
	in := Input{
		Buses: []Bus{
			{ID: 1, Type: BusPV, V: 1.0, P: 0.5},
			{ID: 2, Type: BusPQ, P: -0.5, Q: -0.2},
		},
		Branches: []Branch{{From: 1, To: 2, R: 0.01, X: 0.1}},
	}
	if _, err := Solve(in, nil); err == nil {
		t.Errorf("无 Slack 母线应报错")
	}
	// 非法支路阻抗
	in2 := Input{
		Buses: []Bus{{ID: 1, Type: BusSlack, V: 1.0}, {ID: 2, Type: BusPQ, P: 0.5, Q: 0.2}},
		Branches: []Branch{{From: 1, To: 2, R: 0, X: 0}},
	}
	if _, err := Solve(in2, nil); err == nil {
		t.Errorf("零阻抗支路应报错")
	}
	// 支路端点不是母线
	in3 := Input{
		Buses: []Bus{{ID: 1, Type: BusSlack, V: 1.0}, {ID: 2, Type: BusPQ, P: 0.5, Q: 0.2}},
		Branches: []Branch{{From: 1, To: 99, R: 0.01, X: 0.1}},
	}
	if _, err := Solve(in3, nil); err == nil {
		t.Errorf("支路端点非法应报错")
	}
}
