package sw

import (
	"circuit/types"
	"fmt"
	"strconv"
)

// Type 元件类型
const Type types.ElementType = 12

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
		"State":         int(0),        // 0=关, 1=开
		"OnResistance":  float64(1e-6), // 导通电阻
		"OffResistance": float64(1e12), // 关断电阻
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 2 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase         // 基础创建
	State           int     // 开关状态: 0=关, 1=开
	OnResistance    float64 // 导通电阻
	OffResistance   float64 // 关断电阻
}

// GetVoltageSourceCnt 电压源数量
func (vlaue *Value) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内壁引脚数量
func (vlaue *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (vlaue *Value) Reset() {
	val := vlaue.GetValue()
	vlaue.State = val["State"].(int)
	vlaue.OnResistance = val["OnResistance"].(float64)
	vlaue.OffResistance = val["OffResistance"].(float64)
}

// CirLoad 网表文件写入值
func (vlaue *Value) CirLoad(values types.LoadVlaue) {
	if len(values) >= 1 {
		if state, err := strconv.Atoi(values[0]); err == nil {
			vlaue.SetKeyValue("State", state)
		}
	}
	if len(values) >= 2 {
		if onResistance, err := strconv.ParseFloat(values[1], 64); err == nil {
			vlaue.SetKeyValue("OnResistance", onResistance)
		}
	}
	if len(values) >= 3 {
		if offResistance, err := strconv.ParseFloat(values[2], 64); err == nil {
			vlaue.SetKeyValue("OffResistance", offResistance)
		}
	}
}

// CirExport 网表文件导出值
func (vlaue *Value) CirExport() []string {
	return []string{
		fmt.Sprintf("%d", vlaue.State),
		fmt.Sprintf("%.6g", vlaue.OnResistance),
		fmt.Sprintf("%.6g", vlaue.OffResistance),
	}
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
	var resistance float64
	if base.State == 1 {
		resistance = base.OnResistance // 导通状态
	} else {
		resistance = base.OffResistance // 关断状态
	}
	stamp.StampResistor(base.Nodes[0], base.Nodes[1], resistance)
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	var resistance float64
	if base.State == 1 {
		resistance = base.OnResistance
	} else {
		resistance = base.OffResistance
	}

	// 计算电流（欧姆定律）
	v1 := stamp.GetVoltage(base.Nodes[0])
	v2 := stamp.GetVoltage(base.Nodes[1])
	if resistance > 0 {
		current := (v1 - v2) / resistance
		base.Current.SetVec(0, current)
	} else {
		base.Current.SetVec(0, 0)
	}
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *Base) Debug(stamp types.Stamp) string {
	state := "关"
	if base.State == 1 {
		state = "开"
	}
	current := base.Current.AtVec(0)
	return fmt.Sprintf("状态:%s 电流:%+16f", state, current)
}
