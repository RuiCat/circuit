package base

import (
	"circuit/mna"
	"math"
	"math/rand"
	"strconv"
)

// 电源类型
const (
	WfDC       = 0 // 直流波形
	WfAC       = 1 // 交流波形
	WfSQUARE   = 2 // 方波
	WfTRIANGLE = 3 // 三角波
	WfSAWTOOTH = 4 // 锯齿波
	WfPULSE    = 5 // 脉冲波
	WfNOISE    = 6 // 噪声波
)

// Voltage 电压源
type Voltage struct{ Base }

func (voltage *Voltage) New() {
	voltage.ElementConfigBase = &mna.ElementConfigBase{
		Pin: []string{"v+", "v-"},
		ValueInit: []any{
			int(WfDC),  // 0: 波形类型
			float64(0), // 1: 偏置电压
			float64(0), // 2: 频率
			float64(0), // 3: 相位偏移
			float64(5), // 4: 最大电压
			float64(0), // 5: 占空比
			float64(0), // 6: 频率时间零点
			float64(0), // 7: 噪声值
		},
		Voltage:   []string{"v"},
		OrigValue: []int{7},
	}
}
func (voltage *Voltage) Init() mna.ValueMNA {
	return mna.NewElementBase(voltage.ElementConfigBase)
}
func (Voltage) Reset(base mna.ValueMNA) {
	// 初始化噪声值
	if base.GetInt(0) == WfNOISE {
		base.SetFloat64(7, (rand.NormFloat64()*2-1)*base.GetFloat64(4)+base.GetFloat64(1))
	}
}
func (Voltage) StartIteration(mna mna.MNA, base mna.ValueMNA) {}
func (Voltage) Stamp(mna mna.MNA, base mna.ValueMNA) {
	waveform := base.GetInt(0)
	if waveform == WfDC {
		voltage := base.GetFloat64(4) + base.GetFloat64(1) // MaxVoltage + Bias
		mna.StampVoltageSource(base.Nodes(0), base.Nodes(1), base.VoltSource(0), voltage)
	} else {
		mna.StampVoltageSource(base.Nodes(0), base.Nodes(1), base.VoltSource(0), 0)
	}
}
func (Voltage) DoStep(mna mna.MNA, base mna.ValueMNA) {
	waveform := base.GetInt(0)
	if waveform != WfDC {
		voltage := getVoltage(base)
		mna.UpdateVoltageSource(base.VoltSource(0), voltage)
	}
}
func (Voltage) CalculateCurrent(mna mna.MNA, base mna.ValueMNA) {
	// 电流计算由MNA系统自动处理
}
func (Voltage) StepFinished(mna mna.MNA, base mna.ValueMNA) {
	// 更新噪声值
	if base.GetInt(0) == WfNOISE {
		base.SetFloat64(7, (rand.NormFloat64()*2-1)*base.GetFloat64(4)+base.GetFloat64(1))
	}
}
func (Voltage) CirLoad(base mna.ValueMNA) {
	// 网表文件写入值
	// 这里简化实现，实际应该从base中解析参数
}
func (Voltage) CirExport(base mna.ValueMNA) {
	// 网表文件导出值
	// 这里简化实现，实际应该导出参数
}

// 辅助函数
func getVoltage(base mna.ValueMNA) float64 {
	waveform := base.GetInt(0)
	bias := base.GetFloat64(1)
	frequency := base.GetFloat64(2)
	phaseShift := base.GetFloat64(3)
	maxVoltage := base.GetFloat64(4)
	dutyCycle := base.GetFloat64(5)
	freqTimeZero := base.GetFloat64(6)
	noiseValue := base.GetFloat64(7)

	// 如果是直流分析，返回偏置电压
	// 注意：这里无法访问graph.IsDCAnalysis，简化处理
	if waveform == WfDC {
		return maxVoltage + bias
	}

	// 计算角度
	time := base.Time()
	w := (2*math.Pi)*(time-freqTimeZero)*frequency + phaseShift

	switch waveform {
	case WfAC:
		return math.Sin(w)*maxVoltage + bias
	case WfSQUARE:
		if math.Mod(w, 2*math.Pi) > (2 * math.Pi * dutyCycle) {
			return bias - maxVoltage
		} else {
			return bias + maxVoltage
		}
	case WfTRIANGLE:
		return bias + triangleFunc(math.Mod(w, 2*math.Pi))*maxVoltage
	case WfSAWTOOTH:
		return bias + math.Mod(w, 2*math.Pi)*(maxVoltage/math.Pi) - maxVoltage
	case WfPULSE:
		if math.Mod(w, 2*math.Pi) < (2 * math.Pi * dutyCycle) {
			return maxVoltage + bias
		} else {
			return bias
		}
	case WfNOISE:
		return noiseValue
	default:
		return 0
	}
}

func triangleFunc(x float64) float64 {
	if x < math.Pi {
		return x*(2/math.Pi) - 1
	}
	return 1 - (x-math.Pi)*(2/math.Pi)
}

// 网表加载函数（简化版）
func parseVoltageParams(values []string, base mna.ValueMNA) {
	if len(values) == 0 {
		return
	}
	if waveform, err := strconv.Atoi(values[0]); err == nil {
		base.SetInt(0, waveform)

		switch waveform {
		case WfDC, WfNOISE:
			if len(values) >= 2 {
				if bias, err := strconv.ParseFloat(values[1], 64); err == nil {
					base.SetFloat64(1, bias)
				}
			}
		case WfAC, WfTRIANGLE, WfSAWTOOTH:
			if len(values) >= 6 {
				if bias, err := strconv.ParseFloat(values[1], 64); err == nil {
					base.SetFloat64(1, bias)
				}
				if maxVoltage, err := strconv.ParseFloat(values[2], 64); err == nil {
					base.SetFloat64(4, maxVoltage)
				}
				if frequency, err := strconv.ParseFloat(values[3], 64); err == nil {
					base.SetFloat64(2, frequency)
				}
				if phaseShift, err := strconv.ParseFloat(values[4], 64); err == nil {
					base.SetFloat64(3, phaseShift*math.Pi/180.0) // 度转弧度
				}
				if freqTimeZero, err := strconv.ParseFloat(values[5], 64); err == nil {
					base.SetFloat64(6, freqTimeZero)
				}
			}
		case WfSQUARE, WfPULSE:
			if len(values) >= 7 {
				if bias, err := strconv.ParseFloat(values[1], 64); err == nil {
					base.SetFloat64(1, bias)
				}
				if maxVoltage, err := strconv.ParseFloat(values[2], 64); err == nil {
					base.SetFloat64(4, maxVoltage)
				}
				if frequency, err := strconv.ParseFloat(values[3], 64); err == nil {
					base.SetFloat64(2, frequency)
				}
				if phaseShift, err := strconv.ParseFloat(values[4], 64); err == nil {
					base.SetFloat64(3, phaseShift*math.Pi/180.0) // 度转弧度
				}
				if dutyCycle, err := strconv.ParseFloat(values[5], 64); err == nil {
					base.SetFloat64(5, dutyCycle)
				}
				if freqTimeZero, err := strconv.ParseFloat(values[6], 64); err == nil {
					base.SetFloat64(6, freqTimeZero)
				}
			}
		}
	}
}
