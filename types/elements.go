package types

import (
	"fmt"
	"strconv"
)

func init() {
	if err := ElementRegister(GndType, "G", &GndConfig{}); err != nil {
		panic(err)
	}
	if err := ElementRegister(HeghType, "Hegh", &HeghConfig{}); err != nil {
		panic(err)
	}
}

// HeghType 元件类型
const (
	GndType  ElementType = 1
	HeghType ElementType = 2
)

// GndConfig 默认配置
type GndConfig struct{}

// Init 初始化
func (GndConfig) Init(value *ElementBase) ElementFace {
	return &GndBase{
		ElementBase: value,
		GndValue:    value.Value.(*GndValue),
	}
}

// InitValue 元件值
func (GndConfig) InitValue() Value {
	val := &GndValue{}
	val.ValueMap = ValueMap{}
	return val
}

// GetPostCount 获取引脚数量
func (GndConfig) GetPostCount() int { return 1 }

// GndValue 元件值处理结构
type GndValue struct {
	ValueBase // 基础创建
}

// GetVoltageSourceCnt 电压源数量
func (vlaue *GndValue) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内壁引脚数量
func (vlaue *GndValue) GetInternalNodeCount() int { return 0 }

// GetInternalValueCount 内部数据数量
func (vlaue *GndValue) GetInternalValueCount() int { return 0 }

// Reset 元件值初始化
func (vlaue *GndValue) Reset(stamp Stamp) {}

// CirLoad 网表文件写入值
func (vlaue *GndValue) CirLoad(value LoadVlaue) {}

// CirExport 网表文件导出值
func (vlaue *GndValue) CirExport() []string { return []string{} }

// GndBase 元件实现
type GndBase struct {
	*ElementBase
	*GndValue
}

// Type 类型
func (base *GndBase) Type() ElementType { return GndType }

// Reset 重置
func (base *GndBase) Reset(stamp Stamp) {}

// StartIteration 迭代开始
func (base *GndBase) StartIteration(stamp Stamp) {}

// Stamp 更新线性贡献
func (base *GndBase) Stamp(stamp Stamp) {
	panic(fmt.Errorf("接地节点不应该参与矩阵解析"))
}

// DoStep 执行元件仿真
func (base *GndBase) DoStep(stamp Stamp) {}

// CalculateCurrent 电流计算
func (base *GndBase) CalculateCurrent(stamp Stamp) {}

// StepFinished 步长迭代结束
func (base *GndBase) StepFinished(stamp Stamp) {}

// HeghConfig 默认配置
type HeghConfig struct{}

// Init 初始化
func (HeghConfig) Init(value *ElementBase) ElementFace {
	return &HeghBase{
		ElementBase: value,
		HeghValue:   value.Value.(*HeghValue),
	}
}

// InitValue 元件值
func (HeghConfig) InitValue() Value {
	val := &HeghValue{}
	val.ValueMap = ValueMap{}
	return val
}

// GetPostCount 获取引脚数量
func (HeghConfig) GetPostCount() int { return 1 }

// HeghValue 元件值处理结构
type HeghValue struct {
	ValueBase // 基础创建
}

// GetVoltageSourceCnt 电压源数量
func (vlaue *HeghValue) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内壁引脚数量
func (vlaue *HeghValue) GetInternalNodeCount() int { return 0 }

// GetInternalValueCount 内部数据数量
func (vlaue *HeghValue) GetInternalValueCount() int { return 0 }

// Reset 元件值初始化
func (vlaue *HeghValue) Reset(stamp Stamp) {}

// CirLoad 网表文件写入值
func (vlaue *HeghValue) CirLoad(value LoadVlaue) {}

// CirExport 网表文件导出值
func (vlaue *HeghValue) CirExport() []string { return []string{} }

// LoadVlaue 加载值
type LoadVlaue []string

func (vlaue LoadVlaue) ParseFloat(i int, defaultValue float64) float64 {
	if i < len(vlaue) {
		if val, err := strconv.ParseFloat(vlaue[i], 64); err == nil {
			return val
		}
	}
	return defaultValue
}

func (vlaue LoadVlaue) ParseInt(i int, defaultValue int) int {
	if i < len(vlaue) {
		if val, err := strconv.Atoi(vlaue[i]); err == nil {
			return val
		}
	}
	return defaultValue
}

// HeghBase 元件实现
type HeghBase struct {
	*ElementBase
	*HeghValue
}

// Type 类型
func (base *HeghBase) Type() ElementType { return HeghType }

// StartIteration 迭代开始
func (base *HeghBase) StartIteration(stamp Stamp) {}

// Stamp 更新线性贡献
func (base *HeghBase) Stamp(stamp Stamp) {
	node := base.Nodes[0]
	if node < 0 {
		return
	}
	stamp.StampResistor(node, node, 1e12)
}

// DoStep 执行元件仿真
func (base *HeghBase) DoStep(stamp Stamp) {}

// CalculateCurrent 电流计算
func (base *HeghBase) CalculateCurrent(stamp Stamp) {}

// StepFinished 步长迭代结束
func (base *HeghBase) StepFinished(stamp Stamp) {}
