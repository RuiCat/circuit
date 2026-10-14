// basicAnalog_test.go —— 基础仿真电路实例的**解析解断言**。
//
// 背景：仓库此前只有数字电路的断言（verify_digital_test.go 等），基础模拟电路
// （分压/RC/二极管/三极管/运放/受控源/齐纳/LED/子电路）只验证过"能跑完"，
// 没有断言过数值。本文件把每类基础电路的**解析解**写成断言，防止后续改动
// 静默改变基础器件行为。
//
// 判据口径：
//   - 线性元件与 DC 路径：与解析解比对，容差按器件精度给（见各用例注释）；
//   - 半导体器件：指数模型存在"软膝"，断言的是**物理行为**（钳位范围、饱和压降）
//     而不是理想化数值——例如齐纳在 6.9mA 下比名义 Vz 高约 21mV（动态电阻 ≈2Ω）；
//   - 电容隔直：级联 RC 的各级输出 DC 极限是 0V（串联 R 无直流压降），不是输入电压。
package main_test

import (
	"math"
	"strconv"
	"testing"

	"circuit/element"
	_ "circuit/element/register"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mna"
)

// analogCheck 是一条节点电压断言。
type analogCheck struct {
	node mna.NodeID // 原始网表节点号（如 2 表示 node_2）
	want float64
	tol  float64
	desc string
}

// analogCase 是一个基础电路实例（网表 + 仿真参数 + 断言集）。
type analogCase struct {
	name   string
	net    string // 文件名（cmd/circuits/），空表示用内联网表
	inline string
	target float64
	step   float64
	checks []analogCheck
}

// runAnalog 加载并推进到目标时间，返回上下文供读取节点电压。
func runAnalog(t *testing.T, nl string, target, step float64) *element.Context {
	t.Helper()
	con, err := load.LoadString(nl)
	if err != nil {
		t.Fatalf("加载网表失败: %v", err)
	}
	tm, err := etime.NewTimeMNA(target)
	if err != nil {
		t.Fatalf("NewTimeMNA: %v", err)
	}
	con.Time = tm
	if err := tm.SetStepLimits(1e-9, 5e-4); err != nil {
		t.Fatalf("SetStepLimits: %v", err)
	}
	tm.SetTimeStep(step)
	if err := etime.TransientSimulation(con, func([]float64) {}); err != nil {
		t.Fatalf("TransientSimulation: %v", err)
	}
	return con
}

// TestBasicAnalogCircuits 对基础模拟电路实例做解析解断言。
func TestBasicAnalogCircuits(t *testing.T) {
	cases := []analogCase{
		{
			name: "01 电阻分压器", net: "01_resistor_divider.net", target: 1e-4, step: 1e-6,
			checks: []analogCheck{{node: 2, want: 3.333333333, tol: 1e-3, desc: "5V·2k/(1k+2k)"}},
		},
		{
			// τ=RC=1kΩ·1µF=1ms；t=5ms → 5(1-e^-5)
			name: "02 RC 低通 5τ 充电", net: "02_rc_filter.net", target: 5e-3, step: 1e-5,
			checks: []analogCheck{{node: 2, want: 4.966310265, tol: 5e-3, desc: "5(1-e^-5)"}},
		},
		{
			// 50Hz 5V 正弦，t=5ms 取输入峰值；Vf≈0.693V（I≈4.3mA，Is=1e-14）
			name: "03 二极管半波整流峰值", net: "03_diode_rectifier.net", target: 5e-3, step: 1e-5,
			checks: []analogCheck{{node: 2, want: 4.307, tol: 0.05, desc: "5V−Vf（Vf≈0.693V）"}},
		},
		{
			// Ib=(5−Vbe)/10k≈0.43mA，β=100 → 42mA ≫ Vcc/Rc=5mA ⇒ 深饱和
			name: "04 NPN 开关饱和", net: "04_transistor_switch.net", target: 1e-4, step: 1e-6,
			checks: []analogCheck{{node: 3, want: 0.05, tol: 0.45, desc: "Vce≈0（饱和，实测 ~6mV）"}},
		},
		{
			name: "05 运放同相放大", net: "05_opamp_amplifier.net", target: 1e-4, step: 1e-6,
			checks: []analogCheck{{node: 3, want: 3.0, tol: 0.05, desc: "1V×(1+2k/1k)"}},
		},
		{
			name: "06 多波形源", net: "06_voltage_sources.net", target: 1e-4, step: 1e-6,
			checks: []analogCheck{
				{node: 1, want: 5.0, tol: 1e-3, desc: "DC 5V"},
				{node: 2, want: 0.1564344650, tol: 5e-3, desc: "50Hz 正弦 5·sin(2π·50·1e-4)"},
			},
		},
		{
			// R=100Ω/L=1mH/C=1µF：ω0=31623rad/s，ζ=(R/2)√(C/L)≈1.58 ⇒ 过阻尼
			name: "07 RLC 阶跃（过阻尼）", net: "07_rlc_circuit.net", target: 5e-3, step: 1e-5,
			checks: []analogCheck{{node: 3, want: 5.0, tol: 0.05, desc: "电容终值→5V"}},
		},
		{
			// R1-C-R2 级联：电容隔直 ⇒ 各级 out 的 DC 极限 = 0V；串联 mid 无直流压降 = 输入
			name: "08 三级子电路 RC（隔直）", net: "08_subcircuit.net", target: 0.5, step: 1e-4,
			checks: []analogCheck{
				{node: 1, want: 5.0, tol: 1e-3, desc: "输入恒 5V"},
				{node: 5, want: 5.0, tol: 1e-3, desc: "第一级 mid（无直流压降）"},
				{node: 4, want: 0.0, tol: 1e-3, desc: "末级输出（隔直→0）"},
			},
		},
		{
			name: "12 电流源负载", net: "12_current_source.net", target: 1e-4, step: 1e-6,
			checks: []analogCheck{{node: 1, want: 10.0, tol: 0.01, desc: "10mA×1kΩ"}},
		},
		{
			name: "13 受控源四种", net: "13_controlled_sources.net", target: 1e-4, step: 1e-6,
			checks: []analogCheck{
				{node: 2, want: 4.0, tol: 0.01, desc: "VCVS 2×2V"},
				{node: 8, want: 10.0, tol: 0.01, desc: "CCCS 2×5mA×1kΩ"},
				{node: 4, want: 1.0, tol: 0.01, desc: "VCCS 1mS×1V×1kΩ"},
			},
		},
		{
			// 指数软膝：6.9mA 下 Vout 比名义 Vz=5.1V 高约 21mV（动态电阻 ≈2Ω）
			name: "14 齐纳稳压（软膝）", net: "14_zener_regulator.net", target: 1e-4, step: 1e-6,
			checks: []analogCheck{{node: 2, want: 5.122, tol: 0.02, desc: "Vz+软膝(≈21mV)"}},
		},
		{
			// 行为判据：供电 12V→20V（电流 +8mA），输出变化必须 <50mV ⇒ 钳位成立
			name: "14b 齐纳稳压行为（供电 12V vs 20V）", target: 1e-4, step: 1e-6,
			inline: "V1 [1,-1] [0,0,0,0,20]\nR1 [1,2] [1000]\nZ1 [-1,2] [1e-14,5.1,1,0.1]\n",
			checks: nil,
		},
		{
			name: "16 LED 正向压降", net: "16_led_driver.net", target: 1e-4, step: 1e-6,
			checks: []analogCheck{{node: 2, want: 1.526, tol: 0.05, desc: "220Ω 限流，I≈15.8mA"}},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if c.name == "14b 齐纳稳压行为（供电 12V vs 20V）" {
				testZenerRegulation(t)
				return
			}
			nl := c.inline
			if nl == "" {
				nl = readNetlist(t, c.net)
			}
			con := runAnalog(t, nl, c.target, c.step)
			for _, ck := range c.checks {
				got := con.GetRawNodeVoltage(ck.node)
				if math.Abs(got-ck.want) > ck.tol {
					t.Errorf("node_%d = %.6g，期望 %.6g ± %g（%s）", ck.node, got, ck.want, ck.tol, ck.desc)
				}
			}
		})
	}
}

// testZenerRegulation 断言齐纳的钳位行为：供电从 12V 升到 20V（电流 +8mA）时，
// 输出变化必须 <50mV（指数软膝的动态电阻 ≈2Ω，而分压网络若有同样电流变化会有 8V 变化）。
func testZenerRegulation(t *testing.T) {
	t.Helper()
	circuit := func(v float64) string {
		return "V1 [1,-1] [0,0,0,0," + trimFloat(v) + "]\nR1 [1,2] [1000]\nZ1 [-1,2] [1e-14,5.1,1,0.1]\n"
	}
	con12 := runAnalog(t, circuit(12), 1e-4, 1e-6)
	v12 := con12.GetRawNodeVoltage(2)
	con20 := runAnalog(t, circuit(20), 1e-4, 1e-6)
	v20 := con20.GetRawNodeVoltage(2)
	if d := math.Abs(v20 - v12); d > 0.05 {
		t.Errorf("齐纳未钳位：12V→%.5fV, 20V→%.5fV（Δ=%.1fmV，应 <50mV）", v12, v20, d*1000)
	} else {
		t.Logf("齐纳钳位：12V→%.5fV, 20V→%.5fV（Δ=%.1fmV）", v12, v20, d*1000)
	}
}

// trimFloat 把电压值格式化为网表可用的最短十进制（12 → "12"，5.5 → "5.5"）。
func trimFloat(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
