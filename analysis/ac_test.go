package analysis

import (
	"math"
	"testing"
)

// magPhase 幅值/相位(度)辅助
func magPhase(v complex128) (float64, float64) {
	return math.Hypot(real(v), imag(v)), math.Atan2(imag(v), real(v)) * 180 / math.Pi
}

// TestAC_ResistorDivider 纯阻分压:DC 源 5V,无频响,相位 0
func TestAC_ResistorDivider(t *testing.T) {
	netlist := `
v1 [1,-1] [0,0,0,0,5]
r1 [1,2] [1000]
r2 [2,-1] [2000]
`
	res, err := AnalyzeAC(netlist, 50)
	if err != nil {
		t.Fatalf("AnalyzeAC 失败: %v", err)
	}
	v2 := res.NodeVoltages[2]
	mag, phase := magPhase(v2)
	if math.Abs(mag-5.0*2000/3000) > 1e-9 {
		t.Errorf("分压器输出幅值: 期望 %.9f, 实际 %.9f", 5.0*2000/3000, mag)
	}
	if math.Abs(phase) > 1e-9 {
		t.Errorf("分压器输出相位: 期望 0, 实际 %.3e", phase)
	}
	// 功率守恒:源消耗 = -|I|²R_total,全网 TotalPower ≈ 0
	itot := 5.0 / 3000
	expSource := -itot * itot * 3000 // 源净输出(负消耗)
	if math.Abs(real(res.SourcesPower)-expSource) > 1e-9 {
		t.Errorf("源功率: 期望 %.9f, 实际 %.9f", expSource, real(res.SourcesPower))
	}
	if math.Abs(real(res.TotalPower)) > 1e-9 || math.Abs(imag(res.TotalPower)) > 1e-9 {
		t.Errorf("TotalPower 应≈0(守恒): 实际 %v", res.TotalPower)
	}
}

// TestAC_RCLowPass RC 低通:1kΩ + 100nF,截止 f0=1/(2πRC)≈1591.55Hz 处 -3dB/-45°
func TestAC_RCLowPass(t *testing.T) {
	netlist := `
v1 [1,-1] [1,0,0,0,1]
r1 [1,2] [1000]
c1 [2,-1] [100e-9]
`
	f0 := 1 / (2 * math.Pi * 1000 * 100e-9)
	// 理论 H = 1/(1+jωRC)
	hTheo := func(f float64) complex128 {
		wRC := 2 * math.Pi * f * 1000 * 100e-9
		return complex(1, 0) / complex(1, wRC)
	}
	for _, f := range []float64{f0 / 100, f0, f0 * 10, f0 * 100} {
		res, err := AnalyzeAC(netlist, f)
		if err != nil {
			t.Fatalf("f=%g AnalyzeAC 失败: %v", f, err)
		}
		vout := res.NodeVoltages[2]
		want := hTheo(f)
		if math.Abs(real(vout)-real(want)) > 1e-9 || math.Abs(imag(vout)-imag(want)) > 1e-9 {
			t.Errorf("f=%g: 期望 %v, 实际 %v", f, want, vout)
		}
	}
	// 截止频率处: -3.01dB, -45°
	res, err := AnalyzeAC(netlist, f0)
	if err != nil {
		t.Fatalf("AnalyzeAC 失败: %v", err)
	}
	mag, phase := magPhase(res.NodeVoltages[2])
	if math.Abs(mag-1/math.Sqrt2) > 1e-6 {
		t.Errorf("截止频率幅值: 期望 %.9f, 实际 %.9f", 1/math.Sqrt2, mag)
	}
	if math.Abs(phase+45) > 1e-6 {
		t.Errorf("截止频率相位: 期望 -45°, 实际 %.6f°", phase)
	}
}

// TestAC_RLCSeries 串联 RLC 谐振:Q = (1/R)√(L/C),谐振时电容电压 = Q·V
func TestAC_RLCSeries(t *testing.T) {
	netlist := `
v1 [1,-1] [1,0,0,0,1]
r1 [1,2] [10]
l1 [2,3] [1e-3]
c1 [3,-1] [1e-6]
`
	f0 := 1 / (2 * math.Pi * math.Sqrt(1e-3*1e-6))
	q := (1.0 / 10.0) * math.Sqrt(1e-3/1e-6) // 3.1623
	res, err := AnalyzeAC(netlist, f0)
	if err != nil {
		t.Fatalf("AnalyzeAC 失败: %v", err)
	}
	mag, phase := magPhase(res.NodeVoltages[3])
	if math.Abs(mag-q) > 1e-6 {
		t.Errorf("谐振电容电压: 期望 %.6f, 实际 %.6f", q, mag)
	}
	if math.Abs(phase+90) > 1e-3 {
		t.Errorf("谐振电容相位: 期望 -90°, 实际 %.6f°", phase)
	}
	// 谐振时总阻抗纯阻:V(2) 与 V(1) 同相
	v2 := res.NodeVoltages[2]
	if math.Abs(imag(v2)) > 1e-6 {
		t.Errorf("谐振时节点2电压应纯实: %v", v2)
	}
}

// TestAC_FrequencyFromNetlist 频率取自网表 V 源 frequency 参数(freq=0 时)
func TestAC_FrequencyFromNetlist(t *testing.T) {
	netlist := `
v1 [1,-1] [1,0,50,0,1]
r1 [1,2] [1000]
c1 [2,-1] [100e-9]
`
	res, err := AnalyzeAC(netlist, 0)
	if err != nil {
		t.Fatalf("AnalyzeAC 失败: %v", err)
	}
	if math.Abs(res.Frequency-50) > 1e-12 {
		t.Errorf("频率应取自网表: 期望 50, 实际 %g", res.Frequency)
	}
	// 与显式传频一致
	res2, err := AnalyzeAC(netlist, 50)
	if err != nil {
		t.Fatalf("AnalyzeAC(50) 失败: %v", err)
	}
	if res2.NodeVoltages[2] != res.NodeVoltages[2] {
		t.Errorf("两种频率来源结果不一致")
	}
}

// TestSweepAC_RCLowPass 频率扫描:-3dB 点与理论一致
func TestSweepAC_RCLowPass(t *testing.T) {
	netlist := `
v1 [1,-1] [1,0,0,0,1]
r1 [1,2] [1000]
c1 [2,-1] [100e-9]
`
	f0 := 1 / (2 * math.Pi * 1000 * 100e-9)
	res, err := SweepAC(netlist, f0/10, f0*10, 201, true)
	if err != nil {
		t.Fatalf("SweepAC 失败: %v", err)
	}
	if len(res.Frequencies) != 201 {
		t.Fatalf("点数: 期望 201, 实际 %d", len(res.Frequencies))
	}
	// 对数扫描首尾
	if math.Abs(res.Frequencies[0]-f0/10)/(f0/10) > 1e-9 || math.Abs(res.Frequencies[200]-f0*10)/(f0*10) > 1e-9 {
		t.Errorf("扫描范围错误: %g ~ %g", res.Frequencies[0], res.Frequencies[200])
	}
	// 在 f0 处幅值 ≈ 0.7071(最近采样点对数间隔 ≤1.2%,幅值误差 <3%)
	bestIdx, bestDist := -1, math.Inf(1)
	for i, f := range res.Frequencies {
		if d := math.Abs(f - f0); d < bestDist {
			bestDist, bestIdx = d, i
		}
	}
	if bestIdx < 0 || math.Abs(res.Magnitudes[2][bestIdx]-1/math.Sqrt2) > 0.03 {
		t.Errorf("最近扫描点 %g: 幅值 %g 偏离 -3dB 点超过 3%%", res.Frequencies[bestIdx], res.Magnitudes[2][bestIdx])
	}
}

// TestAC_Errors 错误路径:未知元件、非法阻值、频率缺失
func TestAC_Errors(t *testing.T) {
	if _, err := AnalyzeAC("d1 [1,-1] [0.7]", 50); err == nil {
		t.Errorf("未知元件类型应报错")
	}
	if _, err := AnalyzeAC("r1 [1,-1] [0]", 50); err == nil {
		t.Errorf("零阻值应报错")
	}
	if _, err := AnalyzeAC("v1 [1,-1] [1,0,0,0,1]\nr1 [1,-1] [100]", 0); err == nil {
		t.Errorf("无频率应报错")
	}
	if _, err := AnalyzeAC("", 50); err == nil {
		t.Errorf("空网表应报错")
	}
	if _, err := SweepAC("v1 [1,-1] [1,0,0,0,1]\nr1 [1,-1] [100]", 100, 10, 10, true); err == nil {
		t.Errorf("无效频率范围应报错")
	}
}
