package motor

import (
	"circuit/element/inductor"
	"circuit/types"
	"fmt"
)

// ACInductionMotorValue 元件值处理结构
type ACInductionMotorValue struct {
	types.ValueBase
	Type MotorType
	// 使用电感元件作为基础
	Inductor [3]*inductor.Base
	// 电机基础参数
	StatorRes  float64 // 定子电阻(Ω)
	StatorInd  float64 // 定子电感(H)
	RotorRes   float64 // 转子电阻(Ω)
	RotorInd   float64 // 转子电感(H)
	MutualInd  float64 // 互感(H)
	Slip       float64 // 转差率
	Frequency  float64 // 频率(Hz)
	PolePairs  int     // 极对数
	Inertia    float64 // 转动惯量(kg·m²)
	Damping    float64 // 阻尼系数(N·m·s/rad)
	LoadTorque float64 // 负载转矩(N·m)
}

// Init 内部元件初始化
func (vlaue *ACInductionMotorValue) Init() {
	vlaue.Inductor[0] = types.NewNameType(inductor.Type).(*inductor.Base)
	vlaue.Inductor[1] = types.NewNameType(inductor.Type).(*inductor.Base)
	vlaue.Inductor[2] = types.NewNameType(inductor.Type).(*inductor.Base)
}

// Reset 元件值初始化
func (vlaue *ACInductionMotorValue) Reset() {
	// 从ValueMap中获取参数值
	val := vlaue.GetValue()
	vlaue.StatorRes = val["StatorRes"].(float64)
	vlaue.StatorInd = val["StatorInd"].(float64)
	vlaue.RotorRes = val["RotorRes"].(float64)
	vlaue.RotorInd = val["RotorInd"].(float64)
	vlaue.MutualInd = val["MutualInd"].(float64)
	vlaue.Slip = val["Slip"].(float64)
	vlaue.Frequency = val["Frequency"].(float64)
	vlaue.PolePairs = val["PolePairs"].(int)
	vlaue.Inertia = val["Inertia"].(float64)
	vlaue.Damping = val["Damping"].(float64)
	vlaue.LoadTorque = val["LoadTorque"].(float64)
	// 设置电感器的电感值
	vlaue.Inductor[0].Value.SetKeyValue("Inductance", vlaue.StatorInd)
	vlaue.Inductor[1].Value.SetKeyValue("Inductance", vlaue.StatorInd)
	vlaue.Inductor[2].Value.SetKeyValue("Inductance", vlaue.StatorInd)
	// 初始化其他参数
	vlaue.Inductor[0].Reset()
	vlaue.Inductor[1].Reset()
	vlaue.Inductor[2].Reset()
}

// CirLoad 网表文件写入值
func (vlaue *ACInductionMotorValue) CirLoad(values types.LoadVlaue) {
	if len(values) >= 11 {
		// 解析电机参数
		vlaue.SetKeyValue("StatorRes", values.ParseFloat(0, 0.1))
		vlaue.SetKeyValue("StatorInd", values.ParseFloat(1, 0.015))
		vlaue.SetKeyValue("RotorRes", values.ParseFloat(2, 0.15))
		vlaue.SetKeyValue("RotorInd", values.ParseFloat(3, 0.008))
		vlaue.SetKeyValue("MutualInd", values.ParseFloat(4, 0.03))
		vlaue.SetKeyValue("Slip", values.ParseFloat(5, 0.03))
		vlaue.SetKeyValue("Frequency", values.ParseFloat(6, 50.0))
		vlaue.SetKeyValue("PolePairs", values.ParseInt(7, 4))
		vlaue.SetKeyValue("Inertia", values.ParseFloat(8, 0.1))
		vlaue.SetKeyValue("Damping", values.ParseFloat(9, 0.01))
		vlaue.SetKeyValue("LoadTorque", values.ParseFloat(10, 0.1))
	}
}

// CirExport 网表文件导出值
func (vlaue *ACInductionMotorValue) CirExport() []string {
	return []string{
		fmt.Sprintf("%.6g", vlaue.StatorRes),
		fmt.Sprintf("%.6g", vlaue.StatorInd),
		fmt.Sprintf("%.6g", vlaue.RotorRes),
		fmt.Sprintf("%.6g", vlaue.RotorInd),
		fmt.Sprintf("%.6g", vlaue.MutualInd),
		fmt.Sprintf("%.6g", vlaue.Slip),
		fmt.Sprintf("%.6g", vlaue.Frequency),
		fmt.Sprintf("%d", vlaue.PolePairs),
		fmt.Sprintf("%.6g", vlaue.Inertia),
		fmt.Sprintf("%.6g", vlaue.Damping),
		fmt.Sprintf("%.6g", vlaue.LoadTorque),
	}
}

// ACInductionMotorBase 元件实现
type ACInductionMotorBase struct {
	*types.ElementBase
	*ACInductionMotorValue
	// 电机状态
	RotorPosition float64 // 转子位置(rad)
	RotorSpeed    float64 // 转子速度(rad/s)
	ElectroTorque float64 // 电磁转矩(N·m)
}

// Reset 重置
func (base *ACInductionMotorBase) Reset() {
	// 绑定引脚
	base.Inductor[0].Nodes[0], base.Inductor[0].Nodes[1] = base.Nodes[0], base.Nodes[3]
	base.Inductor[1].Nodes[0], base.Inductor[1].Nodes[1] = base.Nodes[1], base.Nodes[4]
	base.Inductor[2].Nodes[0], base.Inductor[2].Nodes[1] = base.Nodes[2], base.Nodes[5]

	// 初始化电机状态
	base.RotorPosition = 0
	base.RotorSpeed = 0
	base.ElectroTorque = 0

	// 初始化
	base.Inductor[0].Reset()
	base.Inductor[1].Reset()
	base.Inductor[2].Reset()
	base.ElementBase.Reset()
}

// Type 类型
func (base *ACInductionMotorBase) Type() types.ElementType { return ACInductionMotorType }

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

	// 计算转子的旋转位置和转速
	graph := stamp.GetGraph()
	dt := graph.TimeStep

	// 计算同步转速 (rad/s)
	syncSpeed := 2 * 3.14159 * base.Frequency / float64(base.PolePairs)

	// 计算实际转速 (基于转差率)
	base.RotorSpeed = syncSpeed * (1 - base.Slip)

	// 更新转子位置 (积分计算)
	base.RotorPosition += base.RotorSpeed * dt

	// 归一化转子位置到 [0, 2π)
	for base.RotorPosition >= 2*3.14159 {
		base.RotorPosition -= 2 * 3.14159
	}
	for base.RotorPosition < 0 {
		base.RotorPosition += 2 * 3.14159
	}

	// 简化电磁转矩计算 (基于电流和转差率)
	ia := base.Inductor[0].Current.AtVec(0)
	ib := base.Inductor[1].Current.AtVec(0)
	ic := base.Inductor[2].Current.AtVec(0)
	currentMagnitude := (ia*ia + ib*ib + ic*ic) / 3.0
	base.ElectroTorque = currentMagnitude * base.MutualInd * base.Slip * float64(base.PolePairs)
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
func (base *ACInductionMotorBase) Debug(stamp types.Stamp) string {
	// 计算转子角度（度）
	return fmt.Sprintf("交流感应电机 - 转速: %.1f rad/s | 转矩: %.3fN·m | 转子角度: %.1f°",
		base.RotorSpeed, base.ElectroTorque, base.RotorPosition*180/3.14159)
}
