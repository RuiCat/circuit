package motor

import "circuit/types"

// SeriesMotorValue 元件值处理结构
type SeriesMotorValue struct {
	types.ValueBase
}

// Reset 元件值初始化
func (vlaue *SeriesMotorValue) Reset(stamp types.Stamp) {}

// CirLoad 网表文件写入值
func (vlaue *SeriesMotorValue) CirLoad(value types.LoadVlaue) {}

// CirExport 网表文件导出值
func (vlaue *SeriesMotorValue) CirExport() []string { return []string{} }

// SeriesMotorBase 元件实现
type SeriesMotorBase struct {
	*types.ElementBase
	*SeriesMotorValue
}

// Type 类型
func (base *SeriesMotorBase) Type() types.ElementType { return SeriesMotorType }

// StartIteration 迭代开始
func (base *SeriesMotorBase) StartIteration(stamp types.Stamp) {}

// Stamp 更新线性贡献
func (base *SeriesMotorBase) Stamp(stamp types.Stamp) {}

// DoStep 执行元件仿真
func (base *SeriesMotorBase) DoStep(stamp types.Stamp) {}

// CalculateCurrent 电流计算
func (base *SeriesMotorBase) CalculateCurrent(stamp types.Stamp) {}

// StepFinished 步长迭代结束
func (base *SeriesMotorBase) StepFinished(stamp types.Stamp) {}

// Debug  调试
func (base *SeriesMotorBase) Debug(stamp types.Stamp) string { return "" }
