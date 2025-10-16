package motor

import "circuit/types"

// SeparatelyExcitedMotorValue 元件值处理结构
type SeparatelyExcitedMotorValue struct {
	types.ValueBase
	Type MotorType
}

// Reset 元件值初始化
func (vlaue *SeparatelyExcitedMotorValue) Reset() {}

// CirLoad 网表文件写入值
func (vlaue *SeparatelyExcitedMotorValue) CirLoad(value []string) {}

// CirExport 网表文件导出值
func (vlaue *SeparatelyExcitedMotorValue) CirExport() []string { return []string{} }

// SeparatelyExcitedMotorBase 元件实现
type SeparatelyExcitedMotorBase struct {
	*types.ElementBase
	*SeparatelyExcitedMotorValue
}

// Type 类型
func (base *SeparatelyExcitedMotorBase) Type() types.ElementType { return SeparatelyExcitedMotorType }

// StartIteration 迭代开始
func (base *SeparatelyExcitedMotorBase) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献
func (base *SeparatelyExcitedMotorBase) Stamp(stamp types.Stamp) {}

// DoStep 执行元件仿真
func (base *SeparatelyExcitedMotorBase) DoStep(stamp types.Stamp) {}

// CalculateCurrent 电流计算
func (base *SeparatelyExcitedMotorBase) CalculateCurrent(stamp types.Stamp) {}

// StepFinished 步长迭代结束
func (base *SeparatelyExcitedMotorBase) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *SeparatelyExcitedMotorBase) Debug(stamp types.Stamp) string { return "" }
