// productionModules_test.go —— 生产级电路模块的规格验收（cmd/circuits/60–67）。
//
// 这些模块不是"演示电路"，而是可复用的功能块（输入保护 / LDO / 上电复位 /
// EMI 滤波器 / 低边开关 / 恒流 LED 驱动 / 桥式传感前端 / 4–20mA 接收），
// 每个模块都带**可判定的工程规格**（调整率、抑制比、延时、钳位范围、恒流精度…），
// 而不是"能跑完"。规格的解析依据写在各自网表头部注释里。
//
// 覆盖方式：
//   - 大多数规格用瞬态推进到稳态后读节点电压（复用 basicAnalog_test.go 的 runAnalog）；
//   - 63 号 EMI 滤波器用 AC 扫频与**解析传递函数**逐点比对（analysis.SweepAC）；
//   - 器件方向回归（MOSFET/JFET 必须正向导通）单列，防住曾出现的反向导通缺陷。
package main_test

import (
	"math"
	"strings"
	"testing"

	"circuit/analysis"
	"circuit/element"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

// prodNet 读模块网表，并按需做参数替换（同一模块的规格变体，如电源/负载/驱动变化）。
func prodNet(t *testing.T, name string, repl ...string) string {
	t.Helper()
	nl := readNetlist(t, name)
	for i := 0; i+1 < len(repl); i += 2 {
		if !strings.Contains(nl, repl[i]) {
			t.Fatalf("%s: 未找到待替换片段 %q", name, repl[i])
		}
		nl = strings.Replace(nl, repl[i], repl[i+1], 1)
	}
	return nl
}

// mustVolt 断言某个原始节点号上的电压落在 [want-tol, want+tol]。
func mustVolt(t *testing.T, label string, con *element.Context, node mna.NodeID, want, tol float64) {
	t.Helper()
	got := con.GetRawNodeVoltage(node)
	if math.Abs(got-want) > tol {
		t.Errorf("%s: node_%d = %.6g，期望 %.6g ± %g", label, node, got, want, tol)
	}
}

// TestProductionModuleSpecs 逐模块验收工程规格。
func TestProductionModuleSpecs(t *testing.T) {
	t.Run("60 输入保护: 正常 12V 输出≈11.2V", func(t *testing.T) {
		con := runAnalog(t, readNetlist(t, "60_input_protection.net"), 1e-4, 1e-6)
		mustVolt(t, "12V 正常", con, 3, 11.23, 0.15)
	})
	t.Run("60 输入保护: 故障 20V 被钳位（后级安全）", func(t *testing.T) {
		nl := prodNet(t, "60_input_protection.net", "0,0,0,0,12]", "0,0,0,0,20]")
		con := runAnalog(t, nl, 1e-4, 1e-6)
		v := con.GetRawNodeVoltage(3)
		if v < 13.0 || v > 15.5 {
			t.Errorf("20V 输入时输出 %.4gV，期望钳位在 13.0~15.5V（后级安全）", v)
		}
	})

	t.Run("61 LDO: 标称 3.3V", func(t *testing.T) {
		con := runAnalog(t, readNetlist(t, "61_ldo_3v3.net"), 1e-4, 1e-6)
		mustVolt(t, "Vin=5V/1kΩ", con, 3, 3.257, 0.10)
	})
	t.Run("61 LDO: 线性调整率 <50mV（5V→8V）", func(t *testing.T) {
		con5 := runAnalog(t, readNetlist(t, "61_ldo_3v3.net"), 1e-4, 1e-6)
		con8 := runAnalog(t, prodNet(t, "61_ldo_3v3.net", "0,0,0,0,5]", "0,0,0,0,8]"), 1e-4, 1e-6)
		if d := math.Abs(con8.GetRawNodeVoltage(3) - con5.GetRawNodeVoltage(3)); d > 0.05 {
			t.Errorf("线性调整率 %.1fmV 超限（应 <50mV；成因：齐纳动态电阻 × 偏置电流比）", d*1000)
		}
	})
	t.Run("61 LDO: 负载调整率 <150mV（1kΩ→100Ω）", func(t *testing.T) {
		con1k := runAnalog(t, readNetlist(t, "61_ldo_3v3.net"), 1e-4, 1e-6)
		con100 := runAnalog(t, prodNet(t, "61_ldo_3v3.net", "R2 [3,-1] [1000]", "R2 [3,-1] [100]"), 1e-4, 1e-6)
		if d := math.Abs(con100.GetRawNodeVoltage(3) - con1k.GetRawNodeVoltage(3)); d > 0.15 {
			t.Errorf("负载调整率 %.1fmV 超限（应 <150mV）", d*1000)
		}
	})

	t.Run("62 POR: 复位延时 6.93ms（±5%）", func(t *testing.T) {
		// 用回调采样定位电压越过 2.5V 的时刻（一次仿真同时得到波形与穿越时刻）。
		nl := readNetlist(t, "62_power_on_reset.net")
		c2, err := load.LoadString(nl)
		if err != nil {
			t.Fatalf("加载: %v", err)
		}
		tm, err := etime.NewTimeMNA(2e-2)
		if err != nil {
			t.Fatalf("NewTimeMNA: %v", err)
		}
		c2.Time = tm
		_ = tm.SetStepLimits(1e-9, 5e-4)
		tm.SetTimeStep(1e-6)
		crossing := -1.0
		if err := etime.TransientSimulation(c2, func([]float64) {
			if crossing < 0 && c2.GetRawNodeVoltage(2) >= 2.5 {
				crossing = c2.CurrentTime()
			}
		}); err != nil {
			t.Fatalf("仿真: %v", err)
		}
		want := 0.010 * math.Log(5.0/2.5) // R·C·ln(V/(V−Vth)) = 6.931ms
		if crossing < 0 {
			t.Fatalf("复位电压始终未达 2.5V")
		}
		if math.Abs(crossing-want)/want > 0.05 {
			t.Errorf("复位延时 %.4fms，期望 %.4fms（±5%%）", crossing*1000, want*1000)
		}
	})
	t.Run("62 POR: 充电终值 = 分压稳态 5·R2/(R1+R2)", func(t *testing.T) {
		con := runAnalog(t, readNetlist(t, "62_power_on_reset.net"), 0.1, 1e-4)
		// 终值不是 5V：R2=1MΩ 复位引脚负载与 R1=10kΩ 构成分压（≈4.9505V）。
		mustVolt(t, "0.1s 终值", con, 2, 5.0*1e6/(1e4+1e6), 0.02)
	})

	t.Run("64 低边开关: 导通 Vce<0.3V 且 Ic≈50mA", func(t *testing.T) {
		con := runAnalog(t, readNetlist(t, "64_low_side_switch.net"), 1e-4, 1e-6)
		vce := con.GetRawNodeVoltage(3)
		if vce > 0.3 {
			t.Errorf("导通压降 %.4gV 过大（应 <0.3V，深度饱和）", vce)
		}
		if ic := (5 - vce) / 100 * 1000; math.Abs(ic-50) > 1 {
			t.Errorf("负载电流 %.2fmA，期望 ≈50mA", ic)
		}
	})
	t.Run("64 低边开关: 关断 Vc≥4.9V", func(t *testing.T) {
		con := runAnalog(t, prodNet(t, "64_low_side_switch.net", "V2 [4,-1] [0,0,0,0,5]", "V2 [4,-1] [0,0,0,0,0]"), 1e-4, 1e-6)
		if v := con.GetRawNodeVoltage(3); v < 4.9 {
			t.Errorf("关断时集电极 %.4gV，应 ≥4.9V（负载应无电流）", v)
		}
	})

	t.Run("65 恒流 LED: 设定电流 ≈17mA", func(t *testing.T) {
		con := runAnalog(t, readNetlist(t, "65_led_constant_current.net"), 1e-4, 1e-6)
		i := con.GetRawNodeVoltage(4) / 100 * 1000
		if i < 15.3 || i > 18.7 {
			t.Errorf("LED 电流 %.2fmA，期望 15.3~18.7mA（(Vz−Vbe)/Re）", i)
		}
		if vf := 12 - con.GetRawNodeVoltage(3); math.Abs(vf-1.5) > 0.1 {
			t.Errorf("LED 正向压降 %.3gV，期望 ≈1.5V（说明 LED 未正常导通）", vf)
		}
	})
	t.Run("65 恒流 LED: 电源 12V→14V 电流变化 <3%", func(t *testing.T) {
		con12 := runAnalog(t, readNetlist(t, "65_led_constant_current.net"), 1e-4, 1e-6)
		con14 := runAnalog(t, prodNet(t, "65_led_constant_current.net", "0,0,0,0,12]", "0,0,0,0,14]"), 1e-4, 1e-6)
		i12 := con12.GetRawNodeVoltage(4) / 100
		i14 := con14.GetRawNodeVoltage(4) / 100
		if d := math.Abs(i14-i12) / i12; d > 0.03 {
			t.Errorf("电源 12→14V 电流变化 %.2f%%，超 3%%（恒流被破坏；齐纳基准应抑制电源变化）", d*100)
		}
	})

	t.Run("66 桥式前端: 差分增益 10（±3%）", func(t *testing.T) {
		con := runAnalog(t, readNetlist(t, "66_bridge_amp.net"), 1e-4, 1e-6)
		vA, vB := con.GetRawNodeVoltage(2), con.GetRawNodeVoltage(3)
		vout := con.GetRawNodeVoltage(5)
		gain := vout / (vB - vA)
		if math.Abs(gain-10) > 0.3 {
			t.Errorf("差分增益 %.3f，期望 10±0.3（Vout=%.4gV, 差模=%.4gmV）", gain, vout, (vB-vA)*1000)
		}
	})
	t.Run("66 桥式前端: 共模抑制（桥对称时输出≈0）", func(t *testing.T) {
		con := runAnalog(t, prodNet(t, "66_bridge_amp.net", "R4 [3,-1] [10500]", "R4 [3,-1] [10000]"), 1e-4, 1e-6)
		if v := math.Abs(con.GetRawNodeVoltage(5)); v > 0.02 {
			t.Errorf("共模 2.5V 下输出 %.4gV，应 <20mV（共模抑制失效）", v)
		}
	})

	t.Run("67 电流环接收: 4/12/20mA → 1/3/5V（±2%）", func(t *testing.T) {
		cases := []struct {
			i    string
			want float64
		}{{"0.004", 1.0}, {"0.012", 3.0}, {"0.02", 5.0}}
		for _, c := range cases {
			nl := prodNet(t, "67_current_loop_rx.net", "[-1,1] [0.004]", "[-1,1] ["+c.i+"]")
			con := runAnalog(t, nl, 1e-3, 1e-5)
			if v := con.GetRawNodeVoltage(2); math.Abs(v-c.want) > 0.02*c.want+0.005 {
				t.Errorf("I=%sA 时 ADC 端电压 %.5gV，期望 %.4gV ±2%%", c.i, v, c.want)
			}
		}
	})
}

// TestLCEmiFilterTransferFunction 用 AC 扫频与解析传递函数逐点比对（63 号模块）。
//
// 电路：V1 → R1(10Ω) → node2 → L1(1mH) → node3 → (C2=10µF ∥ R2=50Ω)
// 解析：Z2 = (R2 ∥ 1/(sC2))；H(s) = Z2 / (R1 + sL1 + Z2)
// 说明：C1 直接并在理想电压源两端，对 node3 的传递函数无影响（只加载源）。
func TestLCEmiFilterTransferFunction(t *testing.T) {
	nl := readNetlist(t, "63_lc_emi_filter.net")
	const (
		rs = 10.0
		l1 = 1e-3
		c2 = 10e-6
		rl = 50.0
	)
	analytic := func(f float64) float64 {
		w := 2 * math.Pi * f
		// Zc2 = 1/(jwC)
		zcRe, zcIm := 0.0, -1/(w*c2)
		// Z2 = R2 ∥ Zc
		numRe, numIm := rl*zcRe, rl*zcIm
		denRe, denIm := rl+zcRe, zcIm
		d := denRe*denRe + denIm*denIm
		z2Re, z2Im := (numRe*denRe+numIm*denIm)/d, (numIm*denRe-numRe*denIm)/d
		// H = Z2 / (Rs + jwL + Z2)
		hDenRe, hDenIm := rs+z2Re, w*l1+z2Im
		hd := hDenRe*hDenRe + hDenIm*hDenIm
		hRe, hIm := (z2Re*hDenRe+z2Im*hDenIm)/hd, (z2Im*hDenRe-z2Re*hDenIm)/hd
		return math.Hypot(hRe, hIm)
	}

	freqs := []float64{10, 1591.5494, 10000, 100000}
	for _, f := range freqs {
		res, err := analysis.AnalyzeAC(nl, f)
		if err != nil {
			t.Fatalf("AnalyzeAC(%gHz): %v", f, err)
		}
		got := math.Hypot(real(res.NodeVoltages[3]), imag(res.NodeVoltages[3]))
		want := analytic(f)
		if want <= 0 {
			t.Fatalf("解析值异常: %g", want)
		}
		if rel := math.Abs(got-want) / want; rel > 0.05 {
			t.Errorf("%gHz: |H|=%.6g，解析 %.6g（相对偏差 %.2f%% > 5%%）", f, got, want, rel*100)
		} else {
			t.Logf("%gHz: |H|=%.6g（解析 %.6g，偏差 %.2f%%）", f, got, want, rel*100)
		}
	}
	if a := analytic(1e5); a > 1e-3 {
		t.Errorf("解析高频衰减不足（%.3g），用例前提有误", a)
	} else {
		t.Logf("100kHz 高频衰减 %.1fdB（要求 <−60dB）", 20*math.Log10(a))
	}
}

// TestMosfetConductionDirection 器件方向回归：MOSFET 必须**正向**导通（漏→源）。
//
// 背景（2026-10-14 生产模块测试发现）：`mna.StampVCCS` 的实际注入方向是"流入 cn1"，
// mosfet.go/jfet.go 曾按直觉传 (drain, source) 导致**反向导通**——共源级把漏极推到
// 高于电源、源极跟随输出为负。修复后本用例锁死方向（判据非空：修复前必 FAIL）。
func TestMosfetConductionDirection(t *testing.T) {
	t.Run("源极跟随: D=VDD, G=5V, S 经 1kΩ 到地 → Vs>0", func(t *testing.T) {
		con := runAnalog(t, "V1 [1,-1] [0,0,0,0,5]\nM1 [1,3,2] [0,1.5,2e-5,0.01]\nR1 [2,-1] [1000]\nV2 [3,-1] [0,0,0,0,5]\n", 1e-4, 1e-6)
		vs := con.GetRawNodeVoltage(2)
		if vs <= 0 {
			t.Errorf("源极电位 %.4gV ≤ 0：MOSFET 反向导通（应 >0，解析 ≈0.12V）", vs)
		}
		if math.Abs(vs-0.12) > 0.05 {
			t.Errorf("源极电位 %.4gV，期望 ≈0.12V（kp=2e-5 时 Id≈0.12mA）", vs)
		}
	})
	t.Run("共源级: 5V--1kΩ--D, G=5V → Vd<5V（管子把漏极拉低）", func(t *testing.T) {
		con := runAnalog(t, "V1 [1,-1] [0,0,0,0,5]\nR1 [1,2] [1000]\nM1 [2,3,-1] [0,1.5,2e-5,0.01]\nV2 [3,-1] [0,0,0,0,5]\n", 1e-4, 1e-6)
		vd := con.GetRawNodeVoltage(2)
		if vd >= 5.0 {
			t.Errorf("漏极 %.4gV ≥ 电源：管子未正向导通或反向注入", vd)
		}
		if vd < 4.5 {
			t.Errorf("漏极 %.4gV 过低，与 kp=2e-5（Id≈0.13mA）不符", vd)
		}
	})
	t.Run("二极管接法: D=G，5V--1kΩ--D → 钳在 Vt+Vov（<5V 且 >1.5V）", func(t *testing.T) {
		con := runAnalog(t, "V1 [1,-1] [0,0,0,0,5]\nR1 [1,2] [1000]\nM1 [2,2,-1] [0,1.5,2e-5,0.01]\n", 1e-4, 1e-6)
		v := con.GetRawNodeVoltage(2)
		if v >= 5.0 {
			t.Errorf("二极管接法钳位 %.4gV ≥ 电源：方向错误（应 <5V）", v)
		}
		if v < 1.5 {
			t.Errorf("二极管接法钳位 %.4gV < VTO：管子未导通", v)
		}
	})
}

// TestJfetConductionDirection 器件方向回归：JFET 与 MOSFET 同源缺陷，一并锁死。
func TestJfetConductionDirection(t *testing.T) {
	// NJF：VTO=−2V（SPICE 夹断电压约定，导通条件 Vgs>VTO），BETA=1e-4。
	// 源极跟随：D=VDD、G=0V ⇒ Vgs≈0 > −2 ⇒ 应导通，S 电位由平方律决定（≈0.31V）。
	con := runAnalog(t, "V1 [1,-1] [0,0,0,0,5]\nJ1 [1,3,2] [0,-2,1e-4,0.02]\nR1 [2,-1] [1000]\nV2 [3,-1] [0,0,0,0,0]\n", 1e-4, 1e-6)
	vs := con.GetRawNodeVoltage(2)
	if vs <= 0 {
		t.Errorf("JFET 源极电位 %.4gV ≤ 0：反向导通（应 >0）", vs)
	}
	if math.Abs(vs-0.313) > 0.05 {
		t.Errorf("JFET 源极电位 %.4gV，期望 ≈0.31V（平方律 Id=BETA(Vgs−VTO)² 与 1kΩ 自洽解）", vs)
	}
	t.Logf("JFET 源极电位 %.4gV（正向导通，VTO=−2V 约定）", vs)
}
