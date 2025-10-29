package motor

import "circuit/types"

// CompoundMotorValue 元件值处理结构
type CompoundMotorValue struct {
	types.ValueBase
}

// Reset 元件值初始化
func (vlaue *CompoundMotorValue) Reset() {}

// CirLoad 网表文件写入值
func (vlaue *CompoundMotorValue) CirLoad(value types.LoadVlaue) {}

// CirExport 网表文件导出值
func (vlaue *CompoundMotorValue) CirExport() []string { return []string{} }

// CompoundMotorBase 元件实现
type CompoundMotorBase struct {
	*types.ElementBase
	*CompoundMotorValue
}

// Type 类型
func (base *CompoundMotorBase) Type() types.ElementType { return CompoundMotorType }

// StartIteration 迭代开始
func (base *CompoundMotorBase) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献
func (base *CompoundMotorBase) Stamp(stamp types.Stamp) {}

// DoStep 执行元件仿真
func (base *CompoundMotorBase) DoStep(stamp types.Stamp) {}

// CalculateCurrent 电流计算
func (base *CompoundMotorBase) CalculateCurrent(stamp types.Stamp) {}

// StepFinished 步长迭代结束
func (base *CompoundMotorBase) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *CompoundMotorBase) Debug(stamp types.Stamp) string { return "" }
