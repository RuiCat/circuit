package motor

import "circuit/types"

// PMSMValue 元件值处理结构
type PMSMValue struct {
	types.ValueBase
	Type MotorType
}

// Reset 元件值初始化
func (vlaue *PMSMValue) Reset() {}

// CirLoad 网表文件写入值
func (vlaue *PMSMValue) CirLoad(value []string) {}

// CirExport 网表文件导出值
func (vlaue *PMSMValue) CirExport() []string { return []string{} }

// PMSMBase 元件实现
type PMSMBase struct {
	*types.ElementBase
	*PMSMValue
}

// Type 类型
func (base *PMSMBase) Type() types.ElementType { return PMSMType }

// StartIteration 迭代开始
func (base *PMSMBase) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献
func (base *PMSMBase) Stamp(stamp types.Stamp) {}

// DoStep 执行元件仿真
func (base *PMSMBase) DoStep(stamp types.Stamp) {}

// CalculateCurrent 电流计算
func (base *PMSMBase) CalculateCurrent(stamp types.Stamp) {}

// StepFinished 步长迭代结束
func (base *PMSMBase) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *PMSMBase) Debug(stamp types.Stamp) string { return "" }
