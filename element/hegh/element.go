package hegh

import "circuit/types"

// Type 元件类型
const Type types.ElementType = 2

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
	val.ValueMap = types.ValueMap{}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 1 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase // 基础创建
}

// GetVoltageSourceCnt 电压源数量
func (vlaue *Value) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内壁引脚数量
func (vlaue *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (vlaue *Value) Reset() {}

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
func (base *Base) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {
	node := base.Nodes[0]
	if node < 0 {
		return
	}
	stamp.StampResistor(node, node, 1e12)
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}
