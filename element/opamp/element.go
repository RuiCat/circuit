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
		"GBW":       float64(1e6),    // 带宽增益积
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
	GBW             float64 // 带宽增益积
}

// GetVoltageSourceCnt 电压源数量
func (value *Value) GetVoltageSourceCnt() int { return 1 }

// GetInternalNodeCount 内部引脚数量
func (value *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (value *Value) Reset() {
	val := value.GetValue()
	value.MaxOutput = val["MaxOutput"].(float64)
	value.MinOutput = val["MinOutput"].(float64)
	value.Gain = val["Gain"].(float64)
	value.GBW = val["GBW"].(float64)
}

// CirLoad 网表文件写入值
func (value *Value) CirLoad(valueStr []string) {
	if len(valueStr) >= 1 {
		// 解析最大输出电压
		if maxOutput, err := strconv.ParseFloat(valueStr[0], 64); err == nil {
			value.MaxOutput = maxOutput
			value.SetKeyValue("MaxOutput", maxOutput)
		}
	}
	if len(valueStr) >= 2 {
		// 解析最小输出电压
		if minOutput, err := strconv.ParseFloat(valueStr[1], 64); err == nil {
			value.MinOutput = minOutput
			value.SetKeyValue("MinOutput", minOutput)
		}
	}
	if len(valueStr) >= 3 {
		// 解析带宽增益积
		if gbw, err := strconv.ParseFloat(valueStr[2], 64); err == nil {
			value.GBW = gbw
			value.SetKeyValue("GBW", gbw)
		}
	}
	if len(valueStr) >= 4 {
		// 解析开环增益
		if gain, err := strconv.ParseFloat(valueStr[3], 64); err == nil {
			value.Gain = gain
			value.SetKeyValue("Gain", gain)
		}
	}
}

// CirExport 网表文件导出值
func (value *Value) CirExport() []string {
	return []string{
		fmt.Sprintf("%.6g", value.MaxOutput),
		fmt.Sprintf("%.6g", value.MinOutput),
		fmt.Sprintf("%.6g", value.GBW),
		fmt.Sprintf("%.6g", value.Gain),
	}
}

// Base 元件实现
type Base struct {
	*types.ElementBase
	*Value
	lastVD float64 // 上一次的电压差
}

// Type 类型
func (base *Base) Type() types.ElementType { return Type }

// Reset 数据重置
func (base *Base) Reset() {
	base.Value.Reset()
	base.lastVD = 0
}

// StartIteration 迭代开始
func (base *Base) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献 - 实现运放约束建模
func (base *Base) Stamp(stamp types.Stamp) {}

// DoStep 执行元件仿真 - 实现完整的非线性求解
func (base *Base) DoStep(stamp types.Stamp) {
	// 获取输入电压
	inPlus, err1 := stamp.GetVoltage(base.Nodes[1])  // 正输入
	inMinus, err2 := stamp.GetVoltage(base.Nodes[0]) // 负输入
	if err1 != nil || err2 != nil {
		return
	}
	// 计算电压差
	vd := inPlus - inMinus
	if math.Abs(base.lastVD-vd) > 0.001 {
		stamp.SetConverged()
	}
	out, err3 := stamp.GetVoltage(base.Nodes[2])
	if err3 != nil {
		return
	}
	// 检查输出是否超出范围
	if out > base.MaxOutput+0.01 || out < base.MinOutput-0.01 {
		stamp.SetConverged()
	}
	var x float64
	var dx float64
	// 检查是否饱和
	if vd > (base.MaxOutput-base.MinOutput)/base.Gain {
		// 正饱和
		x = base.MaxOutput
		dx = 0
	} else if vd < (base.MinOutput-base.MaxOutput)/base.Gain {
		// 负饱和
		x = base.MinOutput
		dx = 0
	} else {
		// 线性区域
		x = vd * base.Gain
		dx = base.Gain
	}
	if len(base.VoltSource) == 0 {
		return
	}
	// 构建约束方程
	vn := stamp.GetNumNodes() + base.VoltSource[0]
	stamp.StampMatrix(base.Nodes[0], vn, -dx) // 负输入节点
	stamp.StampMatrix(base.Nodes[1], vn, dx)  // 正输入节点
	stamp.StampMatrix(base.Nodes[2], vn, -1)  // 输出节点
	stamp.StampMatrix(vn, base.Nodes[0], -dx) // 负输入节点
	stamp.StampMatrix(vn, base.Nodes[1], dx)  // 正输入节点
	stamp.StampMatrix(vn, base.Nodes[2], -1)  // 输出节点
	stamp.StampRightSide(vn, -x)
	base.lastVD = vd
}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	base.Current.SetVec(0, 0) // V+端电流
	base.Current.SetVec(1, 0) // V-端电流
	base.Current.SetVec(2, 0) // 输出端电流
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug 调试
func (base *Base) Debug(stamp types.Stamp) string {
	inPlus, _ := stamp.GetVoltage(base.Nodes[1])
	inMinus, _ := stamp.GetVoltage(base.Nodes[0])
	out, _ := stamp.GetVoltage(base.Nodes[2])
	return fmt.Sprintf("运放: V+=%+8.3f V-=%+8.3f Vout=%+8.3f Gain=%+8.0f Max=%+8.1f Min=%+8.1f",
		inPlus, inMinus, out, base.Gain, base.MaxOutput, base.MinOutput)
}
