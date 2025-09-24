package transformer

import (
	"circuit/types"
	"fmt"
	"math"
	"strconv"
)

// Type 元件类型
const Type types.ElementType = 9 // 使用新类型ID

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
		"PrimaryInductance":   float64(1e-3),  // 初级电感值(Henry)，默认1mH
		"SecondaryInductance": float64(1e-3),  // 次级电感值(Henry)，默认1mH
		"TurnsRatio":          float64(1.0),   // 匝数比
		"CouplingCoefficient": float64(0.999), // 耦合系数
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 4 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase             // 基础创建
	PrimaryInductance   float64 // 初级电感值(H)
	SecondaryInductance float64 // 次级电感值(H)
	TurnsRatio          float64 // 匝数比
	CouplingCoefficient float64 // 耦合系数

	// 用于仿真计算的内部变量
	a1, a2, a3, a4 float64    // 系数矩阵元素
	current        [2]float64 // 线圈电流
	curSourceValue [2]float64 // 当前源值
}

// GetVoltageSourceCnt 电压源数量
func (value *Value) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内部节点数量
func (value *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (value *Value) Reset() {
	val := value.GetValue()
	value.PrimaryInductance = val["PrimaryInductance"].(float64)
	value.SecondaryInductance = val["SecondaryInductance"].(float64)
	value.TurnsRatio = val["TurnsRatio"].(float64)
	value.CouplingCoefficient = val["CouplingCoefficient"].(float64)

	// 重置仿真相关变量
	value.a1 = 0
	value.a2 = 0
	value.a3 = 0
	value.a4 = 0
	value.current[0] = 0
	value.current[1] = 0
	value.curSourceValue[0] = 0
	value.curSourceValue[1] = 0
}

// CirLoad 网表文件写入值
func (value *Value) CirLoad(values []string) {
	if len(values) >= 1 {
		// 解析初级电感值
		if primaryInductance, err := strconv.ParseFloat(values[0], 64); err == nil {
			value.SetKeyValue("PrimaryInductance", primaryInductance)
		}
	}
	if len(values) >= 2 {
		// 解析次级电感值
		if secondaryInductance, err := strconv.ParseFloat(values[1], 64); err == nil {
			value.SetKeyValue("SecondaryInductance", secondaryInductance)
		}
	}
	if len(values) >= 3 {
		// 解析匝数比
		if turnsRatio, err := strconv.ParseFloat(values[2], 64); err == nil {
			value.SetKeyValue("TurnsRatio", turnsRatio)
		}
	}
	if len(values) >= 4 {
		// 解析耦合系数
		if couplingCoefficient, err := strconv.ParseFloat(values[3], 64); err == nil {
			value.SetKeyValue("CouplingCoefficient", couplingCoefficient)
		}
	}
}

// CirExport 网表文件导出值
func (value *Value) CirExport() []string {
	return []string{
		fmt.Sprintf("%.6g", value.PrimaryInductance),
		fmt.Sprintf("%.6g", value.SecondaryInductance),
		fmt.Sprintf("%.6g", value.TurnsRatio),
		fmt.Sprintf("%.6g", value.CouplingCoefficient),
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
func (base *Base) StartIteration(stamp types.Stamp) {
	// 获取节点电压差
	voltdiff1 := stamp.GetVoltage(base.Nodes[0]) - stamp.GetVoltage(base.Nodes[2])
	voltdiff2 := stamp.GetVoltage(base.Nodes[1]) - stamp.GetVoltage(base.Nodes[3])

	config := stamp.GetConfig()
	if config.IsTrapezoidal {
		base.curSourceValue[0] = voltdiff1*base.a1 + voltdiff2*base.a2 + base.current[0]
		base.curSourceValue[1] = voltdiff1*base.a3 + voltdiff2*base.a4 + base.current[1]
	} else {
		base.curSourceValue[0] = base.current[0]
		base.curSourceValue[1] = base.current[1]
	}
}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {
	// 从匝数比计算次级电感
	l1 := base.PrimaryInductance
	l2 := base.SecondaryInductance * base.TurnsRatio * base.TurnsRatio

	// 计算互感
	m := base.CouplingCoefficient * base.Sqrt(l1*l2)

	// 构建逆矩阵
	deti := 1 / (l1*l2 - m*m)
	ts := stamp.GetTime().TimeStep
	if stamp.GetConfig().IsTrapezoidal {
		ts = ts / 2
	}

	base.a1 = l2 * deti * ts
	base.a2 = -m * deti * ts
	base.a3 = -m * deti * ts
	base.a4 = l1 * deti * ts

	// 添加到矩阵中
	stamp.StampConductance(base.Nodes[0], base.Nodes[2], base.a1)
	stamp.StampVCCurrentSource(base.Nodes[0], base.Nodes[2], base.Nodes[1], base.Nodes[3], base.a2)
	stamp.StampVCCurrentSource(base.Nodes[1], base.Nodes[3], base.Nodes[0], base.Nodes[2], base.a3)
	stamp.StampConductance(base.Nodes[1], base.Nodes[3], base.a4)

	// 右边向量
	stamp.StampRightSide(base.Nodes[0], 0)
	stamp.StampRightSide(base.Nodes[1], 0)
	stamp.StampRightSide(base.Nodes[2], 0)
	stamp.StampRightSide(base.Nodes[3], 0)
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {
	stamp.StampCurrentSource(base.Nodes[0], base.Nodes[2], base.curSourceValue[0])
	stamp.StampCurrentSource(base.Nodes[1], base.Nodes[3], base.curSourceValue[1])
}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	// 获取节点电压差
	voltdiff1 := stamp.GetVoltage(base.Nodes[0]) - stamp.GetVoltage(base.Nodes[2])
	voltdiff2 := stamp.GetVoltage(base.Nodes[1]) - stamp.GetVoltage(base.Nodes[3])

	base.current[0] = voltdiff1*base.a1 + voltdiff2*base.a2 + base.curSourceValue[0]
	base.current[1] = voltdiff1*base.a3 + voltdiff2*base.a4 + base.curSourceValue[1]
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug 调试
func (base *Base) Debug(stamp types.Stamp) string {
	return fmt.Sprintf("变压器 电流1:%+16f 电流2:%+16f", base.current[0], base.current[1])
}

// Sqrt 平方根函数
func (base *Base) Sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}
	return math.Sqrt(x)
}
