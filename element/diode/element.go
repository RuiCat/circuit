package diode

import (
	"circuit/types"
	"fmt"
	"math"
	"strconv"
)

// Type 元件类型
const Type types.ElementType = 7

// 物理常数
const (
	ThermalVoltage = 0.025865 // 电子热电压 (27°C = 300.15K)
	Sqrt2          = 1.4142135623730951
)

// Config 默认配置
type Config struct{}

// Init 初始化
func (Config) Init(value *types.ElementBase) types.ElementFace {
	return &Base{
		ElementBase:  value,
		Value:        value.Value.(*Value),
		lastvoltdiff: 0,
		converged:    true,
	}
}

// InitValue 元件值
func (Config) InitValue() types.Value {
	val := &Value{}
	val.ValueMap = types.ValueMap{
		"SaturationCurrent":   1e-14,  // 反向饱和电流 (Is) - 默认硅二极管
		"BreakdownVoltage":    0.0,    // 击穿电压 (齐纳电压) - 0表示无击穿
		"EmissionCoefficient": 1.0,    // 发射系数 (N) - 理想因子
		"SeriesResistance":    0.0,    // 串联电阻
		"Temperature":         300.15, // 温度 (K)
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 2 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase             // 基础创建
	SaturationCurrent   float64 // 反向饱和电流 (Is)
	BreakdownVoltage    float64 // 击穿电压 (齐纳电压)
	EmissionCoefficient float64 // 发射系数 (N)
	SeriesResistance    float64 // 串联电阻
	Temperature         float64 // 温度 (K)
}

// GetVoltageSourceCnt 电压源数量
func (v *Value) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内壁引脚数量
func (v *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (v *Value) Reset() {
	val := v.GetValue()
	v.SaturationCurrent = val["SaturationCurrent"].(float64)
	v.BreakdownVoltage = val["BreakdownVoltage"].(float64)
	v.EmissionCoefficient = val["EmissionCoefficient"].(float64)
	v.SeriesResistance = val["SeriesResistance"].(float64)
	v.Temperature = val["Temperature"].(float64)
}

// CirLoad 网表文件写入值
func (v *Value) CirLoad(values []string) {
	if len(values) >= 1 {
		// 解析反向饱和电流
		if saturationCurrent, err := strconv.ParseFloat(values[0], 64); err == nil {
			v.SaturationCurrent = saturationCurrent
			v.SetKeyValue("SaturationCurrent", saturationCurrent)
		}
	}
	if len(values) >= 2 {
		// 解析击穿电压
		if breakdownVoltage, err := strconv.ParseFloat(values[1], 64); err == nil {
			v.BreakdownVoltage = breakdownVoltage
			v.SetKeyValue("BreakdownVoltage", breakdownVoltage)
		}
	}
	if len(values) >= 3 {
		// 解析发射系数
		if emissionCoefficient, err := strconv.ParseFloat(values[2], 64); err == nil {
			v.EmissionCoefficient = emissionCoefficient
			v.SetKeyValue("EmissionCoefficient", emissionCoefficient)
		}
	}
	if len(values) >= 4 {
		// 解析串联电阻
		if seriesResistance, err := strconv.ParseFloat(values[3], 64); err == nil {
			v.SeriesResistance = seriesResistance
			v.SetKeyValue("SeriesResistance", seriesResistance)
		}
	}
	if len(values) >= 5 {
		// 解析温度
		if temperature, err := strconv.ParseFloat(values[4], 64); err == nil {
			v.Temperature = temperature
			v.SetKeyValue("Temperature", temperature)
		}
	}
}

// CirExport 网表文件导出值
func (v *Value) CirExport() []string {
	return []string{
		fmt.Sprintf("%.6g", v.SaturationCurrent),
		fmt.Sprintf("%.6g", v.BreakdownVoltage),
		fmt.Sprintf("%.6g", v.EmissionCoefficient),
		fmt.Sprintf("%.6g", v.SeriesResistance),
		fmt.Sprintf("%.6g", v.Temperature),
	}
}

// Base 元件实现
type Base struct {
	*types.ElementBase
	*Value
	// SPICE模型计算参数
	vscale   float64 // 尺度电压 = N * Vt
	vdcoef   float64 // 1 / vscale (用于加速计算)
	vt       float64 // 热电压 = kT/q
	vzcoef   float64 // 1 / vt (用于加速计算)
	zvoltage float64 // 齐纳电压
	leakage  float64 // 漏电流 = SaturationCurrent
	zoffset  float64 // 齐纳击穿偏移
	vcrit    float64 // 正向临界电压
	vzcrit   float64 // 齐纳击穿临界电压
	// 仿真状态
	lastvoltdiff float64 // 上次电压差
	converged    bool    // 收敛状态
}

// Reset 元件值初始化
func (base *Base) Reset() {
	base.Value.Reset()
	base.lastvoltdiff = 0
	base.converged = true
	// 计算温度相关的热电压
	base.vt = ThermalVoltage * (base.Temperature / 300.15)
	base.leakage = base.SaturationCurrent
	base.zvoltage = base.BreakdownVoltage
	// 计算尺度电压和系数
	base.vscale = base.EmissionCoefficient * base.vt
	if base.vscale > 0 {
		base.vdcoef = 1.0 / base.vscale
	} else {
		base.vdcoef = 0
	}
	base.vzcoef = 1.0 / base.vt
	// 计算临界电压用于数值稳定性
	base.calculateCriticalVoltages()
	// 计算齐纳击穿偏移
	base.calculateZenerOffset()
}

// calculateCriticalVoltages 计算临界电压
func (base *Base) calculateCriticalVoltages() {
	// 正向临界电压：电流为 vscale/sqrt(2) 时的电压
	if base.vscale > 0 && base.leakage > 0 {
		denominator := Sqrt2 * base.leakage
		if denominator > 0 {
			base.vcrit = base.vscale * math.Log(base.vscale/denominator)
		} else {
			base.vcrit = 10.0
		}
	} else {
		base.vcrit = 10.0
	}
	// 齐纳击穿临界电压
	if base.leakage > 0 {
		denominator := Sqrt2 * base.leakage
		if denominator > 0 {
			base.vzcrit = base.vt * math.Log(base.vt/denominator)
		} else {
			base.vzcrit = 10.0
		}
	} else {
		base.vzcrit = 10.0
	}
}

// calculateZenerOffset 计算齐纳击穿偏移
func (base *Base) calculateZenerOffset() {
	if base.zvoltage == 0 {
		base.zoffset = 0
	} else {
		// 计算偏移量，使在zvoltage处得到5mA电流
		i := -0.005 // -5mA
		if base.leakage != 0 {
			logArg := -(1 + i/base.leakage)
			if logArg > 0 {
				base.zoffset = base.zvoltage - math.Log(logArg)/base.vzcoef
			} else {
				base.zoffset = base.zvoltage
			}
		} else {
			base.zoffset = base.zvoltage
		}
	}
}

// Type 类型
func (base *Base) Type() types.ElementType { return Type }

// StartIteration 迭代开始
func (base *Base) StartIteration(stamp types.Stamp) {}

// limitStep 电压步长限制 - 防止数值不稳定
func (base *Base) limitStep(vnew, vold float64) float64 {
	// 检查正向电压：电流是否变化了e^2倍？
	if vnew > base.vcrit && math.Abs(vnew-vold) > (base.vscale+base.vscale) {
		if vold > 0 {
			arg := 1 + (vnew-vold)/base.vscale
			if arg > 0 {
				// 调整vnew使电流与前一次迭代的线性化模型相同
				vnew = vold + base.vscale*math.Log(arg)
			} else {
				vnew = base.vcrit
			}
		} else {
			// 调整vnew使电流与前一次迭代的线性化模型相同
			if base.vscale > 0 && vnew > 0 {
				vnew = base.vscale * math.Log(vnew/base.vscale)
			}
		}
		base.converged = false
	} else if vnew < 0 && base.zoffset != 0 {
		// 对于齐纳击穿，使用相同的逻辑但转换值
		vnewTranslated := -vnew - base.zoffset
		voldTranslated := -vold - base.zoffset
		if vnewTranslated > base.vzcrit && math.Abs(vnewTranslated-voldTranslated) > (base.vt+base.vt) {
			if voldTranslated > 0 {
				arg := 1 + (vnewTranslated-voldTranslated)/base.vt
				if arg > 0 {
					vnewTranslated = voldTranslated + base.vt*math.Log(arg)
				} else {
					vnewTranslated = base.vzcrit
				}
			} else {
				if base.vt > 0 && vnewTranslated > 0 {
					vnewTranslated = base.vt * math.Log(vnewTranslated/base.vt)
				}
			}
			base.converged = false
		}
		vnew = -(vnewTranslated + base.zoffset)
	}
	return vnew
}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {
	// 获取电压差
	v1, _ := stamp.GetVoltage(base.Nodes[0])
	v2, _ := stamp.GetVoltage(base.Nodes[1])
	voltdiff := v1 - v2
	// 检查电压变化是否足够大以影响收敛
	if math.Abs(voltdiff-base.lastvoltdiff) > 0.01 {
		base.converged = false
	}
	// 限制电压步长以保证数值稳定性
	voltdiff = base.limitStep(voltdiff, base.lastvoltdiff)
	base.lastvoltdiff = voltdiff
	// 计算最小电导防止奇异矩阵
	// 基础最小电导
	gmin := base.leakage * 0.01
	// 获取当前迭代次数
	iter := stamp.GetGraph().Iter
	// 根据迭代次数动态调整gmin以提高收敛性
	// 随着迭代次数增加，逐渐增大gmin来帮助收敛
	if iter > 10 {
		// 迭代次数较多时，增加gmin
		gmin *= float64(iter-10) * 0.1
	}
	// 确保gmin不会过小导致数值问题
	if gmin < 1e-12 {
		gmin = 1e-12
	}
	// 根据工作区域选择模型
	if voltdiff >= 0 || base.zvoltage == 0 {
		// 正向偏置或无齐纳击穿的二极管
		base.modelForwardBias(stamp, voltdiff, gmin)
	} else {
		// 反向偏置的齐纳二极管
		base.modelReverseBias(stamp, voltdiff, gmin)
	}
	// 添加串联电阻贡献
	if base.SeriesResistance > 0 {
		stamp.StampResistor(base.Nodes[0], base.Nodes[1], base.SeriesResistance)
	}
}

// modelForwardBias 正向偏置模型
func (base *Base) modelForwardBias(stamp types.Stamp, voltdiff, gmin float64) {
	eval := math.Exp(voltdiff * base.vdcoef)
	geq := base.vdcoef*base.leakage*eval + gmin
	nc := (eval-1)*base.leakage - geq*voltdiff
	stamp.StampConductance(base.Nodes[0], base.Nodes[1], geq)
	stamp.StampCurrentSource(base.Nodes[0], base.Nodes[1], nc)
}

// modelReverseBias 反向偏置模型（齐纳击穿）
func (base *Base) modelReverseBias(stamp types.Stamp, voltdiff, gmin float64) {
	// 齐纳二极管使用更陡峭的指数模型
	geq := base.leakage*(base.vdcoef*math.Exp(voltdiff*base.vdcoef)+
		base.vzcoef*math.Exp((-voltdiff-base.zoffset)*base.vzcoef)) + gmin
	nc := base.leakage*(math.Exp(voltdiff*base.vdcoef)-
		math.Exp((-voltdiff-base.zoffset)*base.vzcoef)-1) + geq*(-voltdiff)
	stamp.StampConductance(base.Nodes[0], base.Nodes[1], geq)
	stamp.StampCurrentSource(base.Nodes[0], base.Nodes[1], nc)
}

// calculateCurrent 计算二极管电流
func (base *Base) calculateCurrent(voltdiff float64) float64 {
	if voltdiff >= 0 || base.zvoltage == 0 {
		// 正向偏置或无齐纳击穿
		return base.leakage * (math.Exp(voltdiff*base.vdcoef) - 1)
	}
	// 齐纳击穿区域
	return base.leakage * (math.Exp(voltdiff*base.vdcoef) -
		math.Exp((-voltdiff-base.zoffset)*base.vzcoef) - 1)
}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	v1, _ := stamp.GetVoltage(base.Nodes[0])
	v2, _ := stamp.GetVoltage(base.Nodes[1])
	voltdiff := v1 - v2
	current := base.calculateCurrent(voltdiff)
	base.Current.SetVec(0, current)
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug 调试
func (base *Base) Debug(stamp types.Stamp) string {
	v1, _ := stamp.GetVoltage(base.Nodes[0])
	v2, _ := stamp.GetVoltage(base.Nodes[1])
	voltdiff := v1 - v2
	current := base.calculateCurrent(voltdiff)
	return fmt.Sprintf("二极管: 电压差=%+12.6fV 电流=%+12.6fA", voltdiff, current)
}
