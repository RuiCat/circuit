package motor

import (
	"circuit/element/inductor"
	"circuit/types"
	"fmt"
	"math"
)

// ACInductionMotorValue 元件值处理结构
type ACInductionMotorValue struct {
	types.ValueBase
	Type MotorType
	// 使用电感元件作为基础
	Inductor [3]*inductor.Base
	// 电机基础参数
	StatorInd float64 // 定子电感(H)
	PolePairs int     // 极对数
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
	vlaue.StatorInd = val["StatorInd"].(float64)
	vlaue.PolePairs = val["PolePairs"].(int)
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
	if len(values) >= 2 {
		// 解析电机参数
		vlaue.SetKeyValue("StatorInd", values.ParseFloat(0, 0.015))
		vlaue.SetKeyValue("PolePairs", values.ParseInt(1, 4))
	}
}

// CirExport 网表文件导出值
func (vlaue *ACInductionMotorValue) CirExport() []string {
	return []string{
		fmt.Sprintf("%.6g", vlaue.StatorInd),
		fmt.Sprintf("%d", vlaue.PolePairs),
	}
}

// ACInductionMotorBase 元件实现
type ACInductionMotorBase struct {
	*types.ElementBase
	*ACInductionMotorValue
	// 电机参数
	prevPhi       float64 // 相位
	rotorPosition float64 // 转子角度
}

// Reset 重置
func (base *ACInductionMotorBase) Reset() {
	// 绑定引脚
	base.Inductor[0].Nodes[0], base.Inductor[0].Nodes[1] = base.Nodes[0], base.Nodes[3]
	base.Inductor[1].Nodes[0], base.Inductor[1].Nodes[1] = base.Nodes[1], base.Nodes[4]
	base.Inductor[2].Nodes[0], base.Inductor[2].Nodes[1] = base.Nodes[2], base.Nodes[5]
	// 初始化
	base.Inductor[0].Reset()
	base.Inductor[1].Reset()
	base.Inductor[2].Reset()
	base.ElementBase.Reset()
	// 计算电机采样点
	base.prevPhi = 0
	base.rotorPosition = 0
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

// DoStep 执行元件仿真 - 基于三个线圈电感的磁场计算
func (base *ACInductionMotorBase) DoStep(stamp types.Stamp) {
	base.Inductor[0].DoStep(stamp)
	base.Inductor[1].DoStep(stamp)
	base.Inductor[2].DoStep(stamp)
}

// CalculateCurrent 电流计算
func (base *ACInductionMotorBase) CalculateCurrent(stamp types.Stamp) {
	// 计算电感电流
	base.Inductor[0].CalculateCurrent(stamp)
	base.Inductor[1].CalculateCurrent(stamp)
	base.Inductor[2].CalculateCurrent(stamp)
	// 获取电感的磁场强度
	ja := base.Inductor[0].GetMagneticFieldEnergy()
	jb := base.Inductor[1].GetMagneticFieldEnergy()
	jc := base.Inductor[2].GetMagneticFieldEnergy()
	// 计算合成磁场矢量
	alpha := ja - 0.5*(jb+jc)
	beta := math.Sqrt(3) / 2.0 * (jb - jc)
	// 计算磁场幅值和相位
	phi := math.Atan2(beta, alpha)
	// 计算磁场相位变化
	dphi := phi - base.prevPhi
	// 处理相位跳变（当变化超过π时）
	if dphi > math.Pi {
		dphi -= 2 * math.Pi
	} else if dphi < -math.Pi {
		dphi += 2 * math.Pi
	}
	// 更新转子角度（考虑极对数，转换为机械角度）
	base.rotorPosition += dphi / float64(base.PolePairs)
	// 归一化角度到0-2π范围
	base.rotorPosition = math.Mod(base.rotorPosition, 2*math.Pi)
	if base.rotorPosition < 0 {
		base.rotorPosition += 2 * math.Pi
	}
	// 保存当前相位用于下一次计算
	base.prevPhi = phi
}

// StepFinished 步长迭代结束
func (base *ACInductionMotorBase) StepFinished(stamp types.Stamp) {
	base.Inductor[0].StepFinished(stamp)
	base.Inductor[1].StepFinished(stamp)
	base.Inductor[2].StepFinished(stamp)
}

// Debug  调试
func (base *ACInductionMotorBase) Debug(stamp types.Stamp) string {
	// 获取电感的磁场强度
	ja := base.Inductor[0].GetMagneticFieldEnergy()
	jb := base.Inductor[1].GetMagneticFieldEnergy()
	jc := base.Inductor[2].GetMagneticFieldEnergy()
	// 计算合成磁场矢量
	alpha := ja - 0.5*(jb+jc)
	beta := math.Sqrt(3) / 2.0 * (jb - jc)
	// 计算磁场幅值和相位
	Bm := math.Sqrt(alpha*alpha + beta*beta)
	phi := math.Atan2(beta, alpha)

	return fmt.Sprintf("磁场能量: A=%.6f B=%.6f C=%.6f | 磁场: Bm=%.6f φ=%.6f | 转子角度: %.6f rad (%.2f°)",
		ja, jb, jc, Bm, phi, base.rotorPosition, base.rotorPosition*180/math.Pi)
}
