package capacitor

import (
	"circuit/types"
	"fmt"
	"strconv"
)

// Type 元件类型
const Type types.ElementType = 4

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
		"Capacitance":    float64(1e-5), // 电容值(Farad)，默认10μF
		"InitialVoltage": float64(1e-3), // 初始电压，默认1mV
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 2 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase         // 基础创建
	Capacitance     float64 // 电容值(F)
	InitialVoltage  float64 // 初始电压(V)
}

// GetVoltageSourceCnt 电压源数量
func (value *Value) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内部节点数量
func (value *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (value *Value) Reset(stamp types.Stamp) {
	val := value.GetValue()
	value.Capacitance = val["Capacitance"].(float64)
	value.InitialVoltage = val["InitialVoltage"].(float64)
}

// CirLoad 网表文件写入值
func (value *Value) CirLoad(values types.LoadVlaue) {
	if len(values) >= 1 {
		// 解析电容值
		if capacitance, err := strconv.ParseFloat(values[0], 64); err == nil {
			value.SetKeyValue("Capacitance", capacitance)
		}
	}
	if len(values) >= 2 {
		// 解析初始电压值
		if initialVoltage, err := strconv.ParseFloat(values[1], 64); err == nil {
			value.SetKeyValue("InitialVoltage", initialVoltage)
		}
	}
}

// CirExport 网表文件导出值
func (value *Value) CirExport() []string {
	return []string{
		fmt.Sprintf("%.6g", value.Capacitance),
		fmt.Sprintf("%.6g", value.InitialVoltage),
	}
}

// Base 元件实现
type Base struct {
	*types.ElementBase
	*Value
	// 内部值
	compResistance float64
	curSourceValue float64
	voltdiff       float64
	Orig           [3]float64
}

func (base *Base) Reset(stamp types.Stamp) {
	base.Value.Reset(stamp)
	base.voltdiff = base.InitialVoltage
	base.curSourceValue = 0
	base.compResistance = 0
}

// Update 更新元件值
func (base *Base) Update() {
	base.Orig[0], base.Orig[1], base.Orig[2] = base.voltdiff, base.curSourceValue, base.compResistance
}

// Rollback 回溯
func (base *Base) Rollback() {
	base.voltdiff, base.curSourceValue, base.compResistance = base.Orig[0], base.Orig[1], base.Orig[2]
}

// Type 类型
func (base *Base) Type() types.ElementType { return Type }

// StartIteration 迭代开始
func (base *Base) StartIteration(stamp types.Stamp) {
	if stamp.GetGraph().IsTrapezoidal {
		base.curSourceValue = -base.voltdiff/base.compResistance - stamp.GetCurrent(0)
	} else {
		base.curSourceValue = -base.voltdiff / base.compResistance
	}
}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {
	graph := stamp.GetGraph()
	if graph.IsDCAnalysis {
		stamp.StampResistor(base.Nodes[0], base.Nodes[1], 1e8)
		base.curSourceValue = 0
		return
	}
	if graph.IsTrapezoidal {
		base.compResistance = graph.TimeStep / (2 * base.Capacitance)
	} else {
		base.compResistance = graph.TimeStep / (base.Capacitance)
	}
	stamp.StampResistor(base.Nodes[0], base.Nodes[1], base.compResistance)
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {
	if stamp.GetGraph().IsDCAnalysis {
		return
	}
	stamp.StampCurrentSource(base.Nodes[0], base.Nodes[1], base.curSourceValue)
}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	v1 := stamp.GetVoltage(base.Nodes[0])
	v2 := stamp.GetVoltage(base.Nodes[1])
	base.voltdiff = v1 - v2
	if stamp.GetGraph().IsDCAnalysis {
		stamp.SetCurrent(0, base.voltdiff/1e8)
		return
	}
	if base.compResistance > 0 {
		stamp.SetCurrent(0, base.voltdiff/base.compResistance+base.curSourceValue)
	}
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *Base) Debug(stamp types.Stamp) string {
	return fmt.Sprintf("容压:%+16f 电流:%+16f", base.voltdiff, stamp.GetCurrent(0))
}
