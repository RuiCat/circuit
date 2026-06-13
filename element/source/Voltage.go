package source

import (
	"circuit/element"
	"circuit/mna"
	"math"
	"math/rand"
	"sync"
	"time"
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

var (
	rng   = rand.New(rand.NewSource(time.Now().UnixNano()))
	rngMu sync.Mutex
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
		ValueName: []string{"waveform", "bias", "frequency", "phase", "V_max", "duty_cycle", "t_zero", "noise"},
		Voltage:   []string{"v"},
		OrigValue: []int{7},
	},
})

// Voltage 电压源
type Voltage struct{ *element.Config }

// Reset 初始化电压源状态。噪声波形时，根据最大电压和偏置生成初始噪声值。
func (Voltage) Reset(base element.NodeFace) {
	// 初始化噪声值
	if base.GetInt(0) == WfNOISE {
		rngMu.Lock()
		v := rng.NormFloat64()*base.GetFloat64(4)+base.GetFloat64(1)
		rngMu.Unlock()
		base.SetFloat64(7, v)
	}
}

// Stamp 加盖电压源的 MNA 贡献。直流波形时直接加盖固定电压；其他波形先设零偏置，后续通过 UpdateVoltageSource 动态更新。
func (Voltage) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	waveform := value.GetInt(0)
	if waveform == WfDC {
		voltage := value.GetFloat64(4) + value.GetFloat64(1) // MaxVoltage + Bias
		mna.StampVoltageSource(value.GetNodes(0), value.GetNodes(1), value.GetVoltSource(0), voltage)
	} else {
		mna.StampVoltageSource(value.GetNodes(0), value.GetNodes(1), value.GetVoltSource(0), 0)
	}
}

// DoStep 在每个仿真步长中更新非直流波形的电压值。根据当前时间和波形参数（直流/正弦/脉冲/三角/噪声等）计算瞬时电压。
func (Voltage) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	waveform := value.GetInt(0)
	if waveform != WfDC {
		voltage := getVoltage(value, time)
		mna.UpdateVoltageSource(value.GetVoltSource(0), voltage)
	}
}

// StepFinished 步长结束时更新噪声值，为下一步生成新的随机噪声采样。
func (Voltage) StepFinished(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 更新噪声值
	if value.GetInt(0) == WfNOISE {
		rngMu.Lock()
		v := rng.NormFloat64()*value.GetFloat64(4)+value.GetFloat64(1)
		rngMu.Unlock()
		value.SetFloat64(7, v)
	}
}

// getVoltage 根据电源波形类型和当前时间计算输出电压。
// 支持直流、交流、方波、三角波、锯齿波、脉冲波和噪声七种波形，
// 其中方波和脉冲波的占空比参数 (dutyCycle) 被钳位在 [0, 1] 范围内。
func getVoltage(value element.NodeFace, time mna.Time) float64 {
	waveform := value.GetInt(0)
	bias := value.GetFloat64(1)
	frequency := value.GetFloat64(2)
	phaseShift := value.GetFloat64(3)
	maxVoltage := value.GetFloat64(4)
	dutyCycle := value.GetFloat64(5)
	if dutyCycle < 0 {
		dutyCycle = 0
	} else if dutyCycle > 1 {
		dutyCycle = 1
	}
	freqTimeZero := value.GetFloat64(6)
	noiseValue := value.GetFloat64(7)

	// 如果是直流分析，返回偏置电压
	// 注意：这里无法访问graph.IsDCAnalysis，简化处理
	if waveform == WfDC {
		return maxVoltage + bias
	}

	// 计算角度
	t := time.Time()
	// 使用模运算将时间限制在周期内，防止长时间仿真精度损失
	period := 1.0 / frequency
	t_mod := math.Mod(t-freqTimeZero, period)
	w := 2*math.Pi*t_mod*frequency + phaseShift

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

// triangleFunc 计算三角波函数值，输入为相位角度（0 到 2π），输出范围为 [-1, 1]。
func triangleFunc(x float64) float64 {
	if x < math.Pi {
		return x*(2/math.Pi) - 1
	}
	return 1 - (x-math.Pi)*(2/math.Pi)
}
