package motor

import (
	"circuit/element/inductor"
	"circuit/types"
)

// ACInductionMotorValue 元件值处理结构
type ACInductionMotorValue struct {
	types.ValueBase
	Type MotorType
	// 使用电感元件作为基础
	Inductor [3]*inductor.Base
	// 电机基础参数
	//@ 电压,电流,功率,级数
}

// Init 内部元件初始化
func (vlaue *ACInductionMotorValue) Init() {
	vlaue.Inductor[0] = types.NewNameType(inductor.Type).(*inductor.Base)
	vlaue.Inductor[1] = types.NewNameType(inductor.Type).(*inductor.Base)
	vlaue.Inductor[2] = types.NewNameType(inductor.Type).(*inductor.Base)
}

// Reset 元件值初始化
func (vlaue *ACInductionMotorValue) Reset() {
	vlaue.Inductor[0].Reset()
	vlaue.Inductor[1].Reset()
	vlaue.Inductor[2].Reset()
}

// CirLoad 网表文件写入值
func (vlaue *ACInductionMotorValue) CirLoad(value []string) {}

// CirExport 网表文件导出值
func (vlaue *ACInductionMotorValue) CirExport() []string { return []string{} }

// ACInductionMotorBase 元件实现
type ACInductionMotorBase struct {
	*types.ElementBase
	*ACInductionMotorValue
}

// Reset 重置
func (base *ACInductionMotorBase) Reset() {
	// 绑定引脚
	base.Inductor[0].Nodes[0], base.Inductor[0].Nodes[1] = base.Nodes[0], base.Nodes[3]
	base.Inductor[1].Nodes[0], base.Inductor[1].Nodes[1] = base.Nodes[1], base.Nodes[4]
	base.Inductor[2].Nodes[0], base.Inductor[2].Nodes[1] = base.Nodes[2], base.Nodes[5]
	base.Inductor[0].Reset()
	base.Inductor[1].Reset()
	base.Inductor[2].Reset()
	base.ElementBase.Reset()
}

// Type 类型
func (base *ACInductionMotorBase) Type() types.ElementType { return CompoundMotorType }

// StartIteration 迭代开始
func (base *ACInductionMotorBase) StartIteration(stamp types.Stamp) {
	base.Inductor[0].StartIteration(stamp)
	base.Inductor[1].StartIteration(stamp)
	base.Inductor[2].StartIteration(stamp)
}

// Stamp 更新线性贡献
func (base *ACInductionMotorBase) Stamp(stamp types.Stamp) {
	base.Inductor[0].Stamp(stamp)
	base.Inductor[1].Stamp(stamp)
	base.Inductor[2].Stamp(stamp)
}

// DoStep 执行元件仿真
func (base *ACInductionMotorBase) DoStep(stamp types.Stamp) {
	base.Inductor[0].DoStep(stamp)
	base.Inductor[1].DoStep(stamp)
	base.Inductor[2].DoStep(stamp)
}

// CalculateCurrent 电流计算
func (base *ACInductionMotorBase) CalculateCurrent(stamp types.Stamp) {
	base.Inductor[0].StepFinished(stamp)
	base.Inductor[1].StepFinished(stamp)
	base.Inductor[2].StepFinished(stamp)
}

// StepFinished 步长迭代结束
func (base *ACInductionMotorBase) StepFinished(stamp types.Stamp) {
	base.Inductor[0].StepFinished(stamp)
	base.Inductor[1].StepFinished(stamp)
	base.Inductor[2].StepFinished(stamp)
}

// Debug  调试
func (base *ACInductionMotorBase) Debug(stamp types.Stamp) string { return "" }
