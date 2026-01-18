package base

import (
	"circuit/element"
	"circuit/mna"
	"math"
	"math/rand"
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

// VoltageType 定义元件
var VoltageType element.NodeType = element.AddElement(11, &Voltage{
	&element.Config{
		Name: "v",
		Pin:  element.SetPin(element.PinLowVoltage, "v+", "v-"),
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
	},
})

// Voltage 电压源
type Voltage struct{ *element.Config }

func (Voltage) Reset(base element.NodeFace) {
	// 初始化噪声值
	if base.GetInt(0) == WfNOISE {
		base.SetFloat64(7, (rand.NormFloat64()*2-1)*base.GetFloat64(4)+base.GetFloat64(1))
	}
}

func (Voltage) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	waveform := value.GetInt(0)
	if waveform == WfDC {
		voltage := value.GetFloat64(4) + value.GetFloat64(1) // MaxVoltage + Bias
		mna.StampVoltageSource(value.GetNodes(0), value.GetNodes(1), value.GetVoltSource(0), voltage)
	} else {
		mna.StampVoltageSource(value.GetNodes(0), value.GetNodes(1), value.GetVoltSource(0), 0)
	}
}

func (Voltage) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	waveform := value.GetInt(0)
	if waveform != WfDC {
		voltage := getVoltage(value, time)
		mna.UpdateVoltageSource(value.GetVoltSource(0), voltage)
	}
}

func (Voltage) StepFinished(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 更新噪声值
	if value.GetInt(0) == WfNOISE {
		value.SetFloat64(7, (rand.NormFloat64()*2-1)*value.GetFloat64(4)+value.GetFloat64(1))
	}
}

// 辅助函数
func getVoltage(value element.NodeFace, time mna.Time) float64 {
	waveform := value.GetInt(0)
	bias := value.GetFloat64(1)
	frequency := value.GetFloat64(2)
	phaseShift := value.GetFloat64(3)
	maxVoltage := value.GetFloat64(4)
	dutyCycle := value.GetFloat64(5)
	freqTimeZero := value.GetFloat64(6)
	noiseValue := value.GetFloat64(7)

	// 如果是直流分析，返回偏置电压
	// 注意：这里无法访问graph.IsDCAnalysis，简化处理
	if waveform == WfDC {
		return maxVoltage + bias
	}

	// 计算角度
	t := time.Time()
	w := (2*math.Pi)*(t-freqTimeZero)*frequency + phaseShift

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
