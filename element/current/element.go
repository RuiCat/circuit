package current

import (
	"circuit/types"
	"fmt"
	"math"
	"math/rand"
	"strconv"
)

// 电流源波形类型
const (
	WfDC       = 0 // 直流波形
	WfAC       = 1 // 交流波形
	WfSQUARE   = 2 // 方波
	WfTRIANGLE = 3 // 三角波
	WfSAWTOOTH = 4 // 锯齿波
	WfPULSE    = 5 // 脉冲波
	WfNOISE    = 6 // 噪声波
)

// Type 元件类型
const Type types.ElementType = 11

// Config 默认配置
type Config struct{}

// Init 初始化
func (Config) Init(value *types.ElementBase) types.ElementFace {
	return &Base{
		ElementBase: value,
		Value:       value.Value.(*Value),
	}
}

// InitValue 元件值
func (Config) InitValue() types.Value {
	val := &Value{}
	val.ValueMap = types.ValueMap{
		"Waveform":     int(WfDC),
		"Bias":         float64(0),
		"Frequency":    float64(0),
		"PhaseShift":   float64(0),
		"MaxCurrent":   float64(0),
		"DutyCycle":    float64(0),
		"FreqTimeZero": float64(0),
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 2 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase         // 基础创建
	Waveform        int     // 波形类型，定义电流源的波形形状（DC、AC、方波等）
	Bias            float64 // 偏置电流，直流偏置值
	FreqTimeZero    float64 // 频率时间零点，频率计算的时间参考点
	Frequency       float64 // 频率，波形的频率值
	PhaseShift      float64 // 相位偏移，波形的相位角度偏移
	MaxCurrent      float64 // 最大电流，波形的最大电流幅度
	DutyCycle       float64 // 占空比，方波和脉冲波形的占空比值
}

// GetVoltageSourceCnt 电压源数量
func (vlaue *Value) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内壁引脚数量
func (vlaue *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (vlaue *Value) Reset(stamp types.Stamp) {
	val := vlaue.GetValue()
	vlaue.Waveform = val["Waveform"].(int)
	vlaue.Bias = val["Bias"].(float64)
	vlaue.Frequency = val["Frequency"].(float64)
	vlaue.PhaseShift = val["PhaseShift"].(float64)
	vlaue.MaxCurrent = val["MaxCurrent"].(float64)
	vlaue.DutyCycle = val["DutyCycle"].(float64)
	vlaue.FreqTimeZero = val["FreqTimeZero"].(float64)
}

// CirLoad 网表文件写入值
func (vlaue *Value) CirLoad(values types.LoadVlaue) {
	if len(values) == 0 {
		return
	}
	if waveform, err := strconv.Atoi(values[0]); err == nil {
		switch waveform {
		case WfDC, WfNOISE: // 直流波形,噪声波
			if bias, err := strconv.ParseFloat(values[1], 64); err == nil {
				vlaue.SetKeyValue("Bias", bias)
			}
		case WfAC, WfTRIANGLE, WfSAWTOOTH: // 交流波形,三角波,锯齿波
			if len(values) < 5 {
				return
			}
			if bias, err := strconv.ParseFloat(values[1], 64); err == nil {
				vlaue.SetKeyValue("Bias", bias)
			}
			if maxCurrent, err := strconv.ParseFloat(values[2], 64); err == nil {
				vlaue.SetKeyValue("MaxCurrent", maxCurrent)
			}
			if frequency, err := strconv.ParseFloat(values[3], 64); err == nil {
				vlaue.SetKeyValue("Frequency", frequency)
			}
			if phaseShift, err := strconv.ParseFloat(values[4], 64); err == nil {
				vlaue.SetKeyValue("PhaseShift", phaseShift)
			}
			if freqTimeZero, err := strconv.ParseFloat(values[5], 64); err == nil {
				vlaue.SetKeyValue("FreqTimeZero", freqTimeZero)
			}
		case WfSQUARE, WfPULSE:
			if len(values) < 6 {
				return
			}
			if bias, err := strconv.ParseFloat(values[1], 64); err == nil {
				vlaue.SetKeyValue("Bias", bias)
			}
			if maxCurrent, err := strconv.ParseFloat(values[2], 64); err == nil {
				vlaue.SetKeyValue("MaxCurrent", maxCurrent)
			}
			if frequency, err := strconv.ParseFloat(values[3], 64); err == nil {
				vlaue.SetKeyValue("Frequency", frequency)
			}
			if phaseShift, err := strconv.ParseFloat(values[4], 64); err == nil {
				vlaue.SetKeyValue("PhaseShift", phaseShift)
			}
			if dutyCycle, err := strconv.ParseFloat(values[5], 64); err == nil {
				vlaue.SetKeyValue("DutyCycle", dutyCycle)
			}
			if freqTimeZero, err := strconv.ParseFloat(values[6], 64); err == nil {
				vlaue.SetKeyValue("FreqTimeZero", freqTimeZero)
			}
		default:
			return
		}
		vlaue.SetKeyValue("Waveform", waveform)
	}
}

// CirExport 网表文件导出值
func (vlaue *Value) CirExport() []string {
	switch vlaue.Waveform {
	case WfDC, WfNOISE: // 直流波形,噪声波
		return []string{
			fmt.Sprintf("%d", vlaue.Waveform),
			fmt.Sprintf("%.6g", vlaue.Bias),
		}
	case WfAC, WfTRIANGLE, WfSAWTOOTH: // 交流波形,三角波,锯齿波
		return []string{
			fmt.Sprintf("%d", vlaue.Waveform),
			fmt.Sprintf("%.6g", vlaue.Bias),
			fmt.Sprintf("%.6g", vlaue.MaxCurrent),
			fmt.Sprintf("%.6g", vlaue.Frequency),
			fmt.Sprintf("%.6g", vlaue.PhaseShift),
			fmt.Sprintf("%.6g", vlaue.FreqTimeZero),
		}
	case WfSQUARE, WfPULSE:
		return []string{
			fmt.Sprintf("%d", vlaue.Waveform),
			fmt.Sprintf("%.6g", vlaue.Bias),
			fmt.Sprintf("%.6g", vlaue.MaxCurrent),
			fmt.Sprintf("%.6g", vlaue.Frequency),
			fmt.Sprintf("%.6g", vlaue.PhaseShift),
			fmt.Sprintf("%.6g", vlaue.DutyCycle),
			fmt.Sprintf("%.6g", vlaue.FreqTimeZero),
		}
	default:
		return []string{}
	}
}

// Base 元件实现
type Base struct {
	*types.ElementBase
	*Value
	NoiseValue float64 // 噪声值，噪声波形的当前值
}

// Type 类型
func (base *Base) Type() types.ElementType { return Type }

// StartIteration 迭代开始
func (base *Base) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {
	current := base.getCurrent(stamp)
	stamp.StampCurrentSource(base.Nodes[0], base.Nodes[1], current)
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {}

// getCurrent 得到电流值
func (base *Base) getCurrent(stamp types.Stamp) float64 {
	graph := stamp.GetGraph()
	if base.Waveform != WfDC && graph.IsDCAnalysis {
		return base.Bias
	}
	w := (2*math.Pi)*(graph.Time-base.FreqTimeZero)*base.Frequency + base.PhaseShift
	switch base.Waveform {
	case WfDC:
		return base.MaxCurrent + base.Bias
	case WfAC:
		return math.Sin(w)*base.MaxCurrent + base.Bias
	case WfSQUARE:
		if math.Mod(w, 2*math.Pi) > (2 * math.Pi * base.DutyCycle) {
			return base.Bias - base.MaxCurrent
		} else {
			return base.Bias + base.MaxCurrent
		}
	case WfTRIANGLE:
		return base.Bias + triangleFunc(math.Mod(w, 2*math.Pi))*base.MaxCurrent
	case WfSAWTOOTH:
		return base.Bias + math.Mod(w, 2*math.Pi)*(base.MaxCurrent/math.Pi) - base.MaxCurrent
	case WfPULSE:
		if math.Mod(w, 2*math.Pi) < (2 * math.Pi * base.DutyCycle) {
			return base.MaxCurrent + base.Bias
		} else {
			return base.Bias
		}
	case WfNOISE:
		return base.NoiseValue
	default:
		return 0
	}
}

// triangleFunc 三角波函数计算
func triangleFunc(x float64) float64 {
	if x < math.Pi {
		return x*(2/math.Pi) - 1
	}
	return 1 - (x-math.Pi)*(2/math.Pi)
}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	// 电流源的电流是已知的，直接设置
	stamp.SetCurrent(0, base.getCurrent(stamp))
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {
	if base.Waveform == WfNOISE {
		base.NoiseValue = (rand.NormFloat64()*2-1)*base.MaxCurrent + base.Bias
	}
}

var waveformMap = map[int]string{
	WfDC:       "DC",
	WfAC:       "AC",
	WfSQUARE:   "SQUARE",
	WfTRIANGLE: "TRIANGLE",
	WfSAWTOOTH: "SAWTOOTH",
	WfPULSE:    "PULSE",
	WfNOISE:    "NOISE",
}

// Debug  调试
func (base *Base) Debug(stamp types.Stamp) string {
	return fmt.Sprintf("类型:%s 电流:%+16f", waveformMap[base.Waveform], base.getCurrent(stamp))
}
