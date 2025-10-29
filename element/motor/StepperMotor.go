package motor

import "circuit/types"

// StepperMotorValue 元件值处理结构
type StepperMotorValue struct {
	types.ValueBase
}

// Reset 元件值初始化
func (vlaue *StepperMotorValue) Reset() {}

// CirLoad 网表文件写入值
func (vlaue *StepperMotorValue) CirLoad(value types.LoadVlaue) {}

// CirExport 网表文件导出值
func (vlaue *StepperMotorValue) CirExport() []string { return []string{} }

// StepperMotorBase 元件实现
type StepperMotorBase struct {
	*types.ElementBase
	*StepperMotorValue
}

// Type 类型
func (base *StepperMotorBase) Type() types.ElementType { return StepperMotorType }

// StartIteration 迭代开始
func (base *StepperMotorBase) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献
func (base *StepperMotorBase) Stamp(stamp types.Stamp) {}

// DoStep 执行元件仿真
func (base *StepperMotorBase) DoStep(stamp types.Stamp) {}

// CalculateCurrent 电流计算
func (base *StepperMotorBase) CalculateCurrent(stamp types.Stamp) {}

// StepFinished 步长迭代结束
func (base *StepperMotorBase) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *StepperMotorBase) Debug(stamp types.Stamp) string { return "" }
