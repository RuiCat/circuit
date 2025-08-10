package vcc

import (
	"circuit/types"
	"fmt"
	"math"
)

// Type 元件类型
const Type types.ElementType = 5

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
		"Voltage": float64(0),
		"IsAC":    false,
		"Freq":    float64(0),
		"Phase":   float64(0),
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 2 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase         // 基础创建
	Voltage         float64 // 源电压(V) - 对AC表示峰值电压
	IsAC            bool    // 是否为交流电压源
	Freq            float64 // 频率(Hz)，仅AC有效
	Phase           float64 // 相位(弧度)，仅AC有效
}

// GetVoltageSourceCnt 电压源数量
func (vlaue *Value) GetVoltageSourceCnt() int { return 1 }

// GetInternalNodeCount 内壁引脚数量
func (vlaue *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (vlaue *Value) Reset() {
	val := vlaue.GetValue()
	vlaue.Voltage = val["Voltage"].(float64)
	vlaue.IsAC = val["IsAC"].(bool)
	vlaue.Freq = val["Freq"].(float64)
	vlaue.Phase = val["Phase"].(float64)
}

// CirLoad 网表文件写入值
func (vlaue *Value) CirLoad(value []string) {}

// CirExport 网表文件导出值
func (vlaue *Value) CirExport() []string { return []string{} }

// Base 元件实现
type Base struct {
	*types.ElementBase
	*Value
}

// Type 类型
func (base *Base) Type() types.ElementType { return Type }

// StartIteration 迭代开始
func (base *Base) StartIteration(stamp types.Stamp) {
	if base.IsAC {
		// 计算瞬时交流电压值: V(t) = Vpeak * sin(2πft + φ)
		time := stamp.GetTime().Time
		base.Voltage = base.Voltage * math.Sin(2*math.Pi*base.Freq*time+base.Phase)
	}
}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {
	if !base.IsAC {
		// 直流电压源
		stamp.StampVoltageSource(base.Nodes[0], base.Nodes[1], base.VoltSource[0], base.Voltage)
	}
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {
	if base.IsAC {
		stamp.StampVoltageSource(base.Nodes[0], base.Nodes[1], base.VoltSource[0], base.Voltage)
	}
}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *Base) Debug(stamp types.Stamp) string {
	return fmt.Sprintf("电压:%+16f", base.Voltage)
}
