package transformer

import (
	"circuit/types"
	"fmt"
	"math"
	"strconv"
)

// Type 元件类型
const Type types.ElementType = 9

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
		"Inductance":   float64(4),     // 电感值(Henry)，默认4H
		"Ratio":        float64(1),     // 匝数比，默认1:1
		"CouplingCoef": float64(0.999), // 耦合系数，默认0.999
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 4 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase // 基础创建
	// 参数
	Inductance   float64
	Ratio        float64
	CouplingCoef float64
	// 内部参数
	a1, a2, a3, a4                   float64
	curSourceValue1, curSourceValue2 float64
}

// GetVoltageSourceCnt 电压源数量
func (value *Value) GetVoltageSourceCnt() int { return 0 }

// GetInternalNodeCount 内部节点数量
func (value *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (value *Value) Reset() {
	val := value.GetValue()
	value.Inductance = val["Inductance"].(float64)
	value.Ratio = val["Ratio"].(float64)
	value.CouplingCoef = val["CouplingCoef"].(float64)
	value.a1, value.a2, value.a3, value.a4 = 0, 0, 0, 0
	value.curSourceValue1, value.curSourceValue2 = 0, 0
}

// CirLoad 网表文件写入值
func (value *Value) CirLoad(values []string) {
	if len(values) >= 1 {
		if inductance, err := strconv.ParseFloat(values[0], 64); err == nil {
			value.Inductance = inductance
		}
	}
	if len(values) >= 2 {
		if ratio, err := strconv.ParseFloat(values[1], 64); err == nil {
			value.Ratio = ratio
		}
	}
	if len(values) >= 3 {
		if couplingCoef, err := strconv.ParseFloat(values[2], 64); err == nil {
			value.CouplingCoef = couplingCoef
		}
	}
}

// CirExport 网表文件导出值
func (value *Value) CirExport() []string {
	return []string{
		fmt.Sprintf("%.6g", value.Inductance),
		fmt.Sprintf("%.6g", value.Ratio),
		fmt.Sprintf("%.6g", value.CouplingCoef),
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
	base.Current.SetVec(0, 0) // 初级电流
	base.Current.SetVec(1, 0) // 次级电流
}

// StartIteration 迭代开始
func (base *Base) StartIteration(stamp types.Stamp) {
	voltdiff1 := stamp.GetVoltage(base.Nodes[0]) - stamp.GetVoltage(base.Nodes[1])
	voltdiff2 := stamp.GetVoltage(base.Nodes[2]) - stamp.GetVoltage(base.Nodes[3])
	if stamp.GetConfig().IsTrapezoidal {
		base.curSourceValue1 = voltdiff1*base.a1 + voltdiff2*base.a2 + base.Current.AtVec(0)
		base.curSourceValue2 = voltdiff1*base.a3 + voltdiff2*base.a4 + base.Current.AtVec(1)
	} else {
		base.curSourceValue1 = base.Current.AtVec(0)
		base.curSourceValue2 = base.Current.AtVec(1)
	}
}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {
	// equations for transformer:
	// v1 = L1 di1/dt + M di2/dt
	// v2 = M di1/dt + L2 di2/dt
	// we invert that to get:
	// di1/dt = a1 v1 + a2 v2
	// di2/dt = a3 v1 + a4 v2
	// integrate di1/dt using trapezoidal approx and we get:
	// i1(t2) = i1(t1) + dt/2 (i1(t1) + i1(t2))
	// = i1(t1) + a1 dt/2 v1(t1) + a2 dt/2 v2(t1) +
	// a1 dt/2 v1(t2) + a2 dt/2 v2(t2)
	// the norton equivalent of this for i1 is:
	// a. current source, I = i1(t1) + a1 dt/2 v1(t1) + a2 dt/2 v2(t1)
	// b. resistor, G = a1 dt/2
	// c. current source controlled by voltage v2, G = a2 dt/2
	// and for i2:
	// a. current source, I = i2(t1) + a3 dt/2 v1(t1) + a4 dt/2 v2(t1)
	// b. resistor, G = a3 dt/2
	// c. current source controlled by voltage v2, G = a4 dt/2
	//
	// For backward euler,
	//
	// i1(t2) = i1(t1) + a1 dt v1(t2) + a2 dt v2(t2)
	//
	// So the current source value is just i1(t1) and we use
	// dt instead of dt/2 for the resistor and VCCS.
	//
	// first winding goes from node 0 to 2, second is from 1 to 3
	l1 := base.Inductance
	l2 := base.Inductance * base.Ratio * base.Ratio
	// double l1 = inductance;
	// double l2 = inductance * ratio * ratio;
	m := base.CouplingCoef * math.Sqrt(l1*l2)
	// double m = couplingCoef * Math.sqrt(l1 * l2);
	// build inverted matrix
	deti := 1 / (l1*l2 - m*m)
	// double deti = 1 / (l1 * l2 - m * m);
	ts := stamp.GetTime().TimeStep
	if stamp.GetConfig().IsTrapezoidal {
		ts = ts / 2
	}
	// double ts = isTrapezoidal() ? sim.timeStep / 2 : sim.timeStep;
	base.a1 = l2 * deti * ts // we multiply dt/2 into a1..a4 here
	base.a2 = -m * deti * ts
	base.a3 = -m * deti * ts
	base.a4 = l1 * deti * ts
	// 设置矩阵值
	stamp.StampConductance(base.Nodes[0], base.Nodes[1], base.a1)
	stamp.StampVCCurrentSource(base.Nodes[0], base.Nodes[1], base.Nodes[2], base.Nodes[3], base.a2)
	stamp.StampVCCurrentSource(base.Nodes[2], base.Nodes[3], base.Nodes[0], base.Nodes[1], base.a3)
	stamp.StampConductance(base.Nodes[2], base.Nodes[3], base.a4)
	// 加盖虚拟电阻避免奇异矩阵
	stamp.StampResistor(-1, base.Nodes[0], 1e9)
	stamp.StampResistor(-1, base.Nodes[1], 1e9)
	stamp.StampResistor(-1, base.Nodes[2], 1e9)
	stamp.StampResistor(-1, base.Nodes[3], 1e9)
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {
	stamp.StampCurrentSource(base.Nodes[0], base.Nodes[1], base.curSourceValue1)
	stamp.StampCurrentSource(base.Nodes[2], base.Nodes[3], base.curSourceValue2)
}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	voltdiff1 := stamp.GetVoltage(base.Nodes[0]) - stamp.GetVoltage(base.Nodes[1])
	voltdiff2 := stamp.GetVoltage(base.Nodes[2]) - stamp.GetVoltage(base.Nodes[3])
	base.Current.SetVec(0, voltdiff1*base.a1+voltdiff2*base.a2+base.curSourceValue1)
	base.Current.SetVec(1, voltdiff1*base.a3+voltdiff2*base.a4+base.curSourceValue2)
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug 调试
func (base *Base) Debug(stamp types.Stamp) string {
	primaryVoltage := stamp.GetVoltage(base.Nodes[0]) - stamp.GetVoltage(base.Nodes[1])
	secondaryVoltage := stamp.GetVoltage(base.Nodes[2]) - stamp.GetVoltage(base.Nodes[3])
	return fmt.Sprintf("变压器 电流1:%+16f 电流2:%+16f 初级压差:%+16f 次级压差:%+16f",
		base.Current.AtVec(0), base.Current.AtVec(1), primaryVoltage, secondaryVoltage)
}
