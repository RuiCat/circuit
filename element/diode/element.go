package diode

import (
	"circuit/types"
	"fmt"
)

// Type 元件类型
const Type types.ElementType = 7

// Config 默认配置
type Config struct{}

// Init 初始化
func (Config) Init(value *types.ElementBase) types.ElementFace {
	return &Base{
		ElementBase: value,
		Value:       value.Value.(*Value),
		Resistance:  1e9,
	}
}

// InitValue 元件值
func (Config) InitValue() types.Value {
	val := &Value{}
	val.ValueMap = types.ValueMap{
		"IsolateVoltage": float64(0),   // 截止电压
		"ForwardVoltage": float64(0.7), // 正向导通电压
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 2 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase         // 基础创建
	IsolateVoltage  float64 // 截止电压
	ForwardVoltage  float64 // 正向导通电压
}

// GetVoltageSourceCnt 电压源数量
func (vlaue *Value) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内壁引脚数量
func (vlaue *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (vlaue *Value) Reset() {
	val := vlaue.GetValue()
	vlaue.IsolateVoltage = val["IsolateVoltage"].(float64)
	vlaue.ForwardVoltage = val["ForwardVoltage"].(float64)
}

// CirLoad 网表文件写入值
func (vlaue *Value) CirLoad(value []string) {}

// CirExport 网表文件导出值
func (vlaue *Value) CirExport() []string { return []string{} }

// Base 元件实现
type Base struct {
	*types.ElementBase
	*Value
	Resistance float64 // 当前内阻
	Conduction bool    // 导通状态
}

// Reset 元件值初始化
func (base *Base) Reset() {
	base.Resistance = 1e9
	base.Conduction = false
	base.Value.Reset()
}

// Type 类型
func (base *Base) Type() types.ElementType { return Type }

// StartIteration 迭代开始
func (base *Base) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {
	// 得到当前电压差
	v1, _ := stamp.GetVoltage(base.Nodes[0])
	v2, _ := stamp.GetVoltage(base.Nodes[1])
	vDiff := v1 - v2
	switch {
	case vDiff < base.IsolateVoltage:
		base.Resistance = 1e9
		base.Conduction = false
	case vDiff > base.ForwardVoltage:
		base.Conduction = true
	}
	// 超出数据
	if base.Conduction {
		base.Resistance = (base.ForwardVoltage / (vDiff)) * base.Resistance
	}
	stamp.StampResistor(base.Nodes[0], base.Nodes[1], base.Resistance)
}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	v1, _ := stamp.GetVoltage(base.Nodes[0])
	v2, _ := stamp.GetVoltage(base.Nodes[1])
	base.Current.SetVec(0, (v1-v2)/base.Resistance)
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug 调试
func (base *Base) Debug(stamp types.Stamp) string {
	v1, _ := stamp.GetVoltage(base.Nodes[0])
	v2, _ := stamp.GetVoltage(base.Nodes[1])
	return "二极管: " + "电压差=" + fmt.Sprintf("%+16f", v1-v2) + " 电流=" + fmt.Sprintf("%+16f", base.Current.AtVec(0))
}
