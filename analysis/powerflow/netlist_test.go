package powerflow

import (
	"math"
	"testing"
)

// TestParseNetlist_ThreeBus 解析 cmd/circuits/29_powerflow_3bus.net 同款网表并求解,
// 验证解析结果与 3 母线经典算例(TestSolve_ThreeBus)一致。
func TestParseNetlist_ThreeBus(t *testing.T) {
	const netlist = `
# 3 母线潮流算例 (标幺值, 牛顿-拉夫逊)
# B1: Slack 1.05∠0° | B2: PV 1.0, P=0.5 | B3: PQ P=-1.0, Q=-0.5
B1 slack   V=1.05
B2 pv      V=1.0  P=0.5
B3 pq      P=-1.0 Q=-0.5
BR1 1-2    R=0.01 X=0.1
BR2 1-3    R=0.01 X=0.1
BR3 2-3    R=0.01 X=0.1
`
	in, err := ParseNetlist(netlist)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(in.Buses) != 3 || len(in.Branches) != 3 {
		t.Fatalf("期望 3 母线 3 支路,实际 %d/%d", len(in.Buses), len(in.Branches))
	}
	b1, b2, b3 := in.Buses[0], in.Buses[1], in.Buses[2]
	if b1.Type != BusSlack || b1.V != 1.05 || b1.Theta != 0 {
		t.Fatalf("母线 1 应为 slack V=1.05 θ=0,实际 %+v", b1)
	}
	if b2.Type != BusPV || b2.V != 1.0 || b2.P != 0.5 || b2.Q != 0 {
		t.Fatalf("母线 2 应为 pv V=1.0 P=0.5,实际 %+v", b2)
	}
	if b3.Type != BusPQ || b3.P != -1.0 || b3.Q != -0.5 {
		t.Fatalf("母线 3 应为 pq P=-1.0 Q=-0.5,实际 %+v", b3)
	}
	br1 := in.Branches[0]
	if br1.From != 1 || br1.To != 2 || br1.R != 0.01 || br1.X != 0.1 {
		t.Fatalf("支路 1 解析错误: %+v", br1)
	}
	res, err := Solve(*in, nil)
	if err != nil {
		t.Fatalf("求解失败: %v", err)
	}
	if !res.Converged {
		t.Fatalf("应收敛,实际 %v", res.Converged)
	}
	// 与经典算例一致:全网功率平衡(网损 = Σ注入)
	if math.Abs(real(res.TotalLoss)) < 1e-6 {
		t.Fatalf("网损实部应 > 0,实际 %g", real(res.TotalLoss))
	}
	// B3 负荷压降:电压幅值 < 1.0
	if v := res.BusV[3]; math.Hypot(real(v), imag(v)) >= 1.0 {
		t.Fatalf("母线 3 应压降 < 1.0,实际 %g", math.Hypot(real(v), imag(v)))
	}
}

// TestParseNetlist_ThetaDegrees Theta 以度为单位换算为弧度。
func TestParseNetlist_ThetaDegrees(t *testing.T) {
	in, err := ParseNetlist("B1 slack V=1.0 Theta=30\nB2 pq P=1 Q=0\nBR1 1-2 R=1 X=1\n")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	want := math.Pi / 6
	if math.Abs(in.Buses[0].Theta-want) > 1e-12 {
		t.Fatalf("Theta=30° 应换算为 %v 弧度,实际 %v", want, in.Buses[0].Theta)
	}
}

// TestParseNetlist_CaseInsensitive 参数键与母线类型不区分大小写。
func TestParseNetlist_CaseInsensitive(t *testing.T) {
	in, err := ParseNetlist("B1 SLACK v=1.05\nB2 PQ p=-1.0 Q=-0.5 qmin=-1 qmax=1\nBR1 1-2 r=0.01 X=0.1\n")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if in.Buses[0].Type != BusSlack || in.Buses[0].V != 1.05 {
		t.Fatalf("大小写不敏感解析失败: %+v", in.Buses[0])
	}
	if in.Buses[1].Qmin != -1 || in.Buses[1].Qmax != 1 {
		t.Fatalf("qmin/qmax 解析失败: %+v", in.Buses[1])
	}
}

// TestParseNetlist_Errors 错误路径。
func TestParseNetlist_Errors(t *testing.T) {
	cases := []struct {
		name, netlist string
	}{
		{"空网表", ""},
		{"只有注释", "# 无母线\n"},
		{"无母线", "BR1 1-2 R=1 X=1\n"},
		{"未知母线类型", "B1 generator V=1.0\n"},
		{"母线缺类型", "B1 V=1.0\n"},
		{"未知参数", "B1 slack V=1.0 W=2\n"},
		{"参数缺等号", "B1 slack V 1.0\n"},
		{"阻抗为零", "B1 slack V=1.0\nB2 pq P=1\nBR1 1-2 R=0 X=0\n"},
		{"端点非法", "B1 slack V=1.0\nBR1 1a-2 R=1 X=1\n"},
		{"未知行", "X1 [1,0] [2]\n"},
	}
	for _, c := range cases {
		if _, err := ParseNetlist(c.netlist); err == nil {
			t.Errorf("%s: 应报错", c.name)
		}
	}
}
