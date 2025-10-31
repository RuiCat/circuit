package vcvs

import (
	"circuit/types"
	"fmt"
	"strconv"
)

// Type 元件类型
const Type types.ElementType = 13

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
		"Gain": float64(1),
	}
	return val
}

// GetPostCount 获取引脚数量
func (Config) GetPostCount() int { return 4 }

// Value 元件值处理结构
type Value struct {
	types.ValueBase         // 基础创建
	Gain            float64 // 增益系数
}

// GetVoltageSourceCnt 电压源数量
func (vlaue *Value) GetVoltageSourceCnt() int { return 1 }

// GetInternalNodeCount 内壁引脚数量
func (vlaue *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
func (vlaue *Value) Reset(stamp types.Stamp) {
	val := vlaue.GetValue()
	vlaue.Gain = val["Gain"].(float64)
}

// CirLoad 网表文件写入值
func (vlaue *Value) CirLoad(values types.LoadVlaue) {
	if len(values) >= 1 {
		if gain, err := strconv.ParseFloat(values[0], 64); err == nil {
			vlaue.SetKeyValue("Gain", gain)
		}
	}
}

// CirExport 网表文件导出值
func (vlaue *Value) CirExport() []string {
	return []string{fmt.Sprintf("%.6g", vlaue.Gain)}
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
	// VCVS: V_out = Gain * V_in
	// 控制节点: base.Nodes[0], base.Nodes[1] (输入)
	// 输出节点: base.Nodes[2], base.Nodes[3] (输出)

	// 使用StampVCVS方法，参数为: 控制节点+, 控制节点-, 电压源ID, 增益
	stamp.StampVCVS(base.Nodes[0], base.Nodes[1], base.VoltSource[0], base.Gain)

	// 电压源连接在输出节点之间
	stamp.StampVoltageSource(base.Nodes[2], base.Nodes[3], base.VoltSource[0], 0)
}

// DoStep 执行元件仿真
func (base *Base) DoStep(stamp types.Stamp) {}

// CalculateCurrent 电流计算
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	// 对于VCVS，电流计算需要从矩阵求解结果中获取
	// 这里暂时设置为0，实际电流会在MNA求解过程中计算
	stamp.SetCurrent(0, 0)
}

// StepFinished 步长迭代结束
func (base *Base) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *Base) Debug(stamp types.Stamp) string {
	inputVoltage := stamp.GetVoltage(base.Nodes[0]) - stamp.GetVoltage(base.Nodes[1])
	outputVoltage := stamp.GetVoltage(base.Nodes[2]) - stamp.GetVoltage(base.Nodes[3])
	current := stamp.GetCurrent(0)
	return fmt.Sprintf("增益:%.3f 输入电压:%+16f 输出电压:%+16f 电流:%+16f",
		base.Gain, inputVoltage, outputVoltage, current)
}
