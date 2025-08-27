package inductor

import (
	"circuit/types"
	"fmt"
	"strconv"
)

// Type 元件类型
const Type types.ElementType = 6 // 使用新类型ID

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
		"Inductance":     float64(1e-3), // 电感值(Henry)，默认1mH
		"InitialCurrent": float64(0),    // 初始电流，默认0A
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 2 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase         // 基础创建
	Inductance      float64 // 电感值(H)
	InitialCurrent  float64 // 初始电流(A)

	compResistance float64
	curSourceValue float64
}

// GetVoltageSourceCnt 电压源数量
func (value *Value) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内部节点数量
func (value *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (value *Value) Reset() {
	val := value.GetValue()
	value.Inductance = val["Inductance"].(float64)
	value.InitialCurrent = val["InitialCurrent"].(float64)
	value.compResistance = 0
	value.curSourceValue = 0
}

// CirLoad 网表文件写入值
func (value *Value) CirLoad(values []string) {
	if len(values) >= 1 {
		// 解析电感值
		if inductance, err := strconv.ParseFloat(values[0], 64); err == nil {
			value.Inductance = inductance
			value.SetKeyValue("Inductance", inductance)
		}
	}
	if len(values) >= 2 {
		// 解析初始电流值
		if initialCurrent, err := strconv.ParseFloat(values[1], 64); err == nil {
			value.InitialCurrent = initialCurrent
			value.SetKeyValue("InitialCurrent", initialCurrent)
		}
	}
}

// CirExport 网表文件导出值
func (value *Value) CirExport() []string {
	return []string{
		fmt.Sprintf("%.6g", value.Inductance),
		fmt.Sprintf("%.6g", value.InitialCurrent),
	}
}

// Base 元件实现
type Base struct {
	*types.ElementBase
	*Value
}

// Type 类型
func (base *Base) Type() types.ElementType { return Type }

// Reset 元件值初始化
func (base *Base) Reset() {
	base.Value.Reset()
	base.Current.SetVec(0, base.InitialCurrent)
}

// StartIteration 迭代开始
func (base *Base) StartIteration(stamp types.Stamp) {
	config := stamp.GetConfig()
	if config.IsTrapezoidal {
		v1, _ := stamp.GetVoltage(base.Nodes[0])
		v2, _ := stamp.GetVoltage(base.Nodes[1])
		voltdiff := v1 - v2
		base.curSourceValue = voltdiff/base.compResistance + base.Current.AtVec(0)
	} else {
		base.curSourceValue = base.Current.AtVec(0)
	}
}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {
	config := stamp.GetConfig()
	timeStep := stamp.GetTime().TimeStep
	if config.IsTrapezoidal {
		base.compResistance = 2 * base.Inductance / timeStep
	} else {
		base.compResistance = base.Inductance / timeStep
	}
	stamp.StampResistor(base.Nodes[0], base.Nodes[1], base.compResistance)
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {
	stamp.StampCurrentSource(base.Nodes[0], base.Nodes[1], base.curSourceValue)
}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	if base.compResistance > 0 {
		v1, _ := stamp.GetVoltage(base.Nodes[0])
		v2, _ := stamp.GetVoltage(base.Nodes[1])
		voltdiff := v1 - v2
		base.Current.SetVec(0, voltdiff/base.compResistance+base.curSourceValue)
	}
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *Base) Debug(stamp types.Stamp) string {
	return fmt.Sprintf("电感电流:%+16f", base.Current.AtVec(0))
}
