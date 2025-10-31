package resistor

import (
	"circuit/types"
	"fmt"
	"strconv"
)

// Type 元件类型
const Type types.ElementType = 3

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
		"Resistance": float64(0),
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 2 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase // 基础创建
	Resistance      float64
}

// GetVoltageSourceCnt 电压源数量
func (value *Value) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内壁引脚数量
func (value *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (value *Value) Reset(stamp types.Stamp) {
	val := value.GetValue()
	value.Resistance = val["Resistance"].(float64)
}

// CirLoad 网表文件写入值
func (vlaue *Value) CirLoad(values types.LoadVlaue) {
	if len(values) >= 1 {
		// 解析电阻值
		if resistance, err := strconv.ParseFloat(values[0], 64); err == nil {
			vlaue.SetKeyValue("Resistance", resistance)
		}
	}
}

// CirExport 网表文件导出值
func (value *Value) CirExport() []string {
	return []string{fmt.Sprintf("%.6g", value.Resistance)}
}

// Base 元件实现
type Base struct {
	*types.ElementBase
	*Value
}

// Type 类型
func (base *Base) Type() types.ElementType { return Type }

// StartIteration 迭代开始
func (base *Base) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {
	stamp.StampResistor(base.Nodes[0], base.Nodes[1], base.Resistance)
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	// 计算电流（欧姆定律）
	v1 := stamp.GetVoltage(base.Nodes[0])
	v2 := stamp.GetVoltage(base.Nodes[1])
	current := (v1 - v2) / base.Resistance
	// 储电流值
	stamp.SetCurrent(0, current)
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *Base) Debug(stamp types.Stamp) string {
	return fmt.Sprintf("电阻:%+16f 电流:%+16f", base.Resistance, stamp.GetCurrent(0))
}
