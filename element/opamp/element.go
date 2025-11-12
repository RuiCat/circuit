package opamp

import (
	"circuit/types"
	"fmt"
	"math"
	"strconv"
)

// Type 元件类型
const Type types.ElementType = 10

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
		"MaxOutput": float64(15),     // 最大输出电压
		"MinOutput": float64(-15),    // 最小输出电压
		"Gain":      float64(100000), // 开环增益
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 3 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase         // 基础创建
	MaxOutput       float64 // 最大输出电压
	MinOutput       float64 // 最小输出电压
	Gain            float64 // 开环增益
}

// GetVoltageSourceCnt 电压源数量
func (value *Value) GetVoltageSourceCnt() int { return 1 }

// GetInternalNodeCount 内部引脚数量
func (value *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (value *Value) Reset(stamp types.Stamp) {
	val := value.GetValue()
	value.MaxOutput = val["MaxOutput"].(float64)
	value.MinOutput = val["MinOutput"].(float64)
	value.Gain = val["Gain"].(float64)
}

// CirLoad 网表文件写入值
func (value *Value) CirLoad(values types.LoadVlaue) {
	if len(values) >= 1 {
		// 解析最大输出电压
		if maxOutput, err := strconv.ParseFloat(values[0], 64); err == nil {
			value.SetKeyValue("MaxOutput", maxOutput)
		}
	}
	if len(values) >= 2 {
		// 解析最小输出电压
		if minOutput, err := strconv.ParseFloat(values[1], 64); err == nil {
			value.SetKeyValue("MinOutput", minOutput)
		}
	}
	if len(values) >= 4 {
		// 解析开环增益
		if gain, err := strconv.ParseFloat(values[3], 64); err == nil {
			value.SetKeyValue("Gain", gain)
		}
	}
}

// CirExport 网表文件导出值
func (value *Value) CirExport() []string {
	return []string{
		fmt.Sprintf("%.6g", value.MaxOutput),
		fmt.Sprintf("%.6g", value.MinOutput),
		fmt.Sprintf("%.6g", value.Gain),
	}
}

// Base 元件实现
type Base struct {
	*types.ElementBase
	*Value
	lastVD float64 // 上一次的电压差
	Orig   float64
}

// Update 更新元件值
func (base *Base) Update() {
	base.Orig = base.lastVD
}

// Rollback 回溯
func (base *Base) Rollback() {
	base.lastVD = base.Orig
}

// Type 类型
func (base *Base) Type() types.ElementType { return Type }

// Reset 数据重置
func (base *Base) Reset(stamp types.Stamp) {
	base.Value.Reset(stamp)
	base.lastVD = 0
}

// StartIteration 迭代开始
func (base *Base) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {
	vn := stamp.GetGraph().NumNodes + base.VoltSource[0]
	stamp.StampMatrix(base.Nodes[2], vn, 1)
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {
	// 获取输入电压
	volts0 := stamp.GetVoltage(base.Nodes[0]) // 负输入
	volts1 := stamp.GetVoltage(base.Nodes[1]) // 正输入
	volts2 := stamp.GetVoltage(base.Nodes[2]) // 输出
	// 计算电压差
	var x float64
	vd, dx := volts1-volts0, 1e-4
	// 收敛判断
	switch {
	case math.Abs(base.lastVD-vd) > types.Tolerance:
		stamp.SetConverged()
	case volts2 > base.MaxOutput+types.Tolerance || volts2 < base.MinOutput-types.Tolerance:
		stamp.SetConverged()
	}
	switch {
	case vd >= base.MaxOutput/base.Gain && (base.lastVD >= 0):
		x = base.MaxOutput - dx*base.MaxOutput/base.Gain
	case vd <= base.MinOutput/base.Gain && (base.lastVD <= 0):
		x = base.MinOutput - dx*base.MinOutput/base.Gain
	default:
		dx = base.Gain
	}
	// 通过设置电压源右侧向量来实现约束
	vn := stamp.GetGraph().NumNodes + base.VoltSource[0]
	// 建立完整的运放约束方程
	stamp.StampMatrix(vn, base.Nodes[0], dx)
	stamp.StampMatrix(vn, base.Nodes[1], -dx)
	stamp.StampMatrix(vn, base.Nodes[2], 1)
	stamp.StampRightSide(vn, x)
	base.lastVD = vd
}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug 调试
func (base *Base) Debug(stamp types.Stamp) string {
	inPlus := stamp.GetVoltage(base.Nodes[1])
	inMinus := stamp.GetVoltage(base.Nodes[0])
	out := stamp.GetVoltage(base.Nodes[2])
	return fmt.Sprintf("运放: V+=%+8.3f V-=%+8.3f Vout=%+8.3f Gain=%+8.0f Max=%+8.1f Min=%+8.1f",
		inPlus, inMinus, out, base.Gain, base.MaxOutput, base.MinOutput)
}
