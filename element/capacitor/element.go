package capacitor

import (
	"circuit/types"
	"fmt"
)

// Type 元件类型
const Type types.ElementType = 4

// init 初始化
func init() {
	types.ElementRegister(Type, "C", &config{})
}

// config 默认配置
type config struct{}

// Init 初始化
func (config) Init(value *types.ElementBase) types.ElementFace {
	return &Base{
		ElementBase: value,
		Value:       value.Value.(*Value),
	}
}

// InitValue 元件值
func (config) InitValue() types.Value {
	val := &Value{}
	val.ValueMap = types.ValueMap{
		"Capacitance":    float64(1e-5), // 电容值(Farad)，默认10μF
		"InitialVoltage": float64(1e-3), // 初始电压，默认1mV
	}
	return val
}

// GetPostCount 获取引脚数量
func (config) GetPostCount() int { return 2 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase         // 基础创建
	Capacitance     float64 // 电容值(F)
	InitialVoltage  float64 // 初始电压(V)

	compResistance float64
	curSourceValue float64
	voltdiff       float64
}

// GetVoltageSourceCnt 电压源数量
func (value *Value) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内部节点数量
func (value *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (value *Value) Reset() {
	val := value.GetValue()
	value.Capacitance = val["Capacitance"].(float64)
	value.InitialVoltage = val["InitialVoltage"].(float64)
	value.voltdiff = value.InitialVoltage
	value.curSourceValue = 0
	value.compResistance = 0
}

// CirLoad 网表文件写入值
func (value *Value) CirLoad(values []string) {}

// CirExport 网表文件导出值
func (value *Value) CirExport() []string { return []string{} }

// Base 元件实现
type Base struct {
	*types.ElementBase
	*Value
}

// Type 类型
func (base *Base) Type() types.ElementType { return Type }

// StartIteration 迭代开始
func (base *Base) StartIteration(stamp types.Stamp) {
	if stamp.GetConfig().IsTrapezoidal {
		base.curSourceValue = -base.voltdiff/base.compResistance - base.Current.AtVec(0)
	} else {
		base.curSourceValue = -base.voltdiff / base.compResistance
	}
}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {
	config := stamp.GetConfig()
	if config.IsDCAnalysis {
		stamp.StampResistor(base.Nodes[0], base.Nodes[1], 1e8)
		base.curSourceValue = 0
		return
	}
	timeStep := stamp.GetTime().TimeStep
	if config.IsTrapezoidal {
		base.compResistance = timeStep / (2 * base.Capacitance)
	} else {
		base.compResistance = timeStep / (base.Capacitance)
	}
	stamp.StampResistor(base.Nodes[0], base.Nodes[1], base.compResistance)
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {
	if stamp.GetConfig().IsDCAnalysis {
		return
	}
	stamp.StampCurrentSource(base.Nodes[0], base.Nodes[1], base.curSourceValue)
}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	base.voltdiff = stamp.GetVoltage(base.Nodes[0]) - stamp.GetVoltage(base.Nodes[1])
	if stamp.GetConfig().IsDCAnalysis {
		base.Current.SetVec(0, base.voltdiff/1e8)
		return
	}
	if base.compResistance > 0 {
		base.Current.SetVec(0, base.voltdiff/base.compResistance+base.curSourceValue)
	}
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {
}

// Debug  调试
func (base *Base) Debug(stamp types.Stamp) string {
	return fmt.Sprintf("容压:%+16f 电流:%+16f", base.voltdiff, base.Current.AtVec(0))
}
