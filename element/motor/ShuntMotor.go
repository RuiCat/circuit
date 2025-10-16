package motor

import "circuit/types"

// ShuntMotorValue 元件值处理结构
type ShuntMotorValue struct {
	types.ValueBase
	Type MotorType
}

// Reset 元件值初始化
func (vlaue *ShuntMotorValue) Reset() {}

// CirLoad 网表文件写入值
func (vlaue *ShuntMotorValue) CirLoad(value []string) {}

// CirExport 网表文件导出值
func (vlaue *ShuntMotorValue) CirExport() []string { return []string{} }

// ShuntMotorBase 元件实现
type ShuntMotorBase struct {
	*types.ElementBase
	*ShuntMotorValue
}

// Type 类型
func (base *ShuntMotorBase) Type() types.ElementType { return ShuntMotorType }

// StartIteration 迭代开始
func (base *ShuntMotorBase) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献
func (base *ShuntMotorBase) Stamp(stamp types.Stamp) {}

// DoStep 执行元件仿真
func (base *ShuntMotorBase) DoStep(stamp types.Stamp) {}

// CalculateCurrent 电流计算
func (base *ShuntMotorBase) CalculateCurrent(stamp types.Stamp) {}

// StepFinished 步长迭代结束
func (base *ShuntMotorBase) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *ShuntMotorBase) Debug(stamp types.Stamp) string { return "" }
