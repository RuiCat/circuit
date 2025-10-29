package motor

import (
	"circuit/types"
	"fmt"
	"math"
)

// ACInductionMotorValue 元件值处理结构
type ACInductionMotorValue struct {
	types.ValueBase
	Type MotorType
	// 电机基础参数
	Rs float64 // 定子电阻 (ohm)
	Rr float64 // 转子电阻 (ohm)
	Ls float64 // 定子漏感 (H)
	Lr float64 // 转子漏感 (H)
	Lm float64 // 磁化电感 (H)
	P  int     // 极对数
	J  float64 // 转动惯量 (kg·m^2)
	B  float64 // 阻尼系数 (N·m·s/rad)

	// 计算得到的参数
	Lss float64 // 定子自感 Ls + Lm
	Lrr float64 // 转子自感 Lr + Lm
}

// Init 内部元件初始化
func (value *ACInductionMotorValue) Init() {
	// 计算自感参数
	value.Lss = value.Ls + value.Lm
	value.Lrr = value.Lr + value.Lm
}

// Reset 元件值初始化
func (value *ACInductionMotorValue) Reset() {
	// 从ValueMap中获取参数值
	val := value.GetValue()
	value.Rs = val["Rs"].(float64)
	value.Rr = val["Rr"].(float64)
	value.Ls = val["Ls"].(float64)
	value.Lr = val["Lr"].(float64)
	value.Lm = val["Lm"].(float64)
	value.P = val["P"].(int)
	value.J = val["J"].(float64)
	value.B = val["B"].(float64)
	// 重新计算自感参数
	value.Lss = value.Ls + value.Lm
	value.Lrr = value.Lr + value.Lm
}

// CirLoad 网表文件写入值
func (value *ACInductionMotorValue) CirLoad(values types.LoadVlaue) {
	// 解析电机参数
	value.SetKeyValue("Rs", values.ParseFloat(0, 0.435))
	value.SetKeyValue("Rr", values.ParseFloat(1, 0.816))
	value.SetKeyValue("Ls", values.ParseFloat(2, 0.002))
	value.SetKeyValue("Lr", values.ParseFloat(3, 0.002))
	value.SetKeyValue("Lm", values.ParseFloat(4, 0.0015))
	value.SetKeyValue("P", values.ParseInt(5, 4))
	value.SetKeyValue("J", values.ParseFloat(6, 0.01))
	value.SetKeyValue("B", values.ParseFloat(7, 0.001))

	// 计算自感参数
	value.Lss = value.Ls + value.Lm
	value.Lrr = value.Lr + value.Lm
}

// CirExport 网表文件导出值
func (value *ACInductionMotorValue) CirExport() []string {
	return []string{
		fmt.Sprintf("%.6g", value.Rs),
		fmt.Sprintf("%.6g", value.Rr),
		fmt.Sprintf("%.6g", value.Ls),
		fmt.Sprintf("%.6g", value.Lr),
		fmt.Sprintf("%.6g", value.Lm),
		fmt.Sprintf("%d", value.P),
		fmt.Sprintf("%.6g", value.J),
		fmt.Sprintf("%.6g", value.B),
	}
}

// ACInductionMotorBase 元件实现
type ACInductionMotorBase struct {
	*types.ElementBase
	*ACInductionMotorValue
	// 初始条件
	Theta    float64 // 转子初始角度 (rad)
	Omega    float64 // 转子初始速度 (rad/s)
	ThetaCos float64 // 转子角度余弦值
	ThetaSin float64 // 转子角度正弦值
	// 状态变量
	Psi_ds float64 // d轴定子磁链 (Wb)
	Psi_qs float64 // q轴定子磁链 (Wb)
	Psi_dr float64 // d轴转子磁链 (Wb)
	Psi_qr float64 // q轴转子磁链 (Wb)
	// 电流变量
	I_ds          float64 // d轴定子电流 (A)
	I_qs          float64 // q轴定子电流 (A)
	I_dr          float64 // d轴转子电流 (A)
	I_qr          float64 // q轴转子电流 (A)
	I_u, I_v, I_w float64 // 定子线圈电流 (A)
	// 电气角速度
	Omega_e float64 // 同步角速度 (rad/s)
	// 计算得到的参数
	Lss float64 // 定子自感 Ls + Lm (H)
	Lrr float64 // 转子自感 Lr + Lm (H)
	det float64 // 电感矩阵行列式 Lss*Lrr - Lm^2
}

// Reset 重置
func (base *ACInductionMotorBase) Reset() {
	base.ElementBase.Reset()
	// 初始条件
	base.Theta = 0 // 转子初始角度
	base.Omega = 0 // 转子初始速度
	base.ThetaCos = 1.0
	base.ThetaSin = 0.0

	// 初始化磁链状态
	base.Psi_ds = 0
	base.Psi_qs = 0
	base.Psi_dr = 0
	base.Psi_qr = 0
	base.I_u, base.I_v, base.I_w = 0, 0, 0

	// 计算电感参数
	base.Lss = base.Ls + base.Lm
	base.Lrr = base.Lr + base.Lm
	base.det = base.Lss*base.Lrr - base.Lm*base.Lm
}

// Type 类型
func (base *ACInductionMotorBase) Type() types.ElementType { return ACInductionMotorType }

// StartIteration 迭代开始
func (base *ACInductionMotorBase) StartIteration(stamp types.Stamp) {}

var (
	sqrt2_3 = math.Sqrt(2.0 / 3.0)
	sqrt3_2 = math.Sqrt(3.0) / 2.0
)

// Stamp 更新线性贡献
func (base *ACInductionMotorBase) Stamp(stamp types.Stamp) {
	stamp.StampResistor(-1, base.Nodes[0], 1e15)
	stamp.StampResistor(-1, base.Nodes[1], 1e15)
	stamp.StampResistor(-1, base.Nodes[2], 1e15)
	stamp.StampResistor(-1, base.Nodes[3], 1e15)
	stamp.StampResistor(-1, base.Nodes[4], 1e15)
	stamp.StampResistor(-1, base.Nodes[5], 1e15)
}

// DoStep 执行元件仿真 - 基于严格的数学物理模型
func (base *ACInductionMotorBase) DoStep(stamp types.Stamp) {
	dt := stamp.GetGraph().TimeStep

	// 获取定子电压 - 严格的三相电压测量
	Vu := stamp.GetVoltage(base.Nodes[0]) - stamp.GetVoltage(base.Nodes[3])
	Vv := stamp.GetVoltage(base.Nodes[1]) - stamp.GetVoltage(base.Nodes[4])
	Vw := stamp.GetVoltage(base.Nodes[2]) - stamp.GetVoltage(base.Nodes[5])

	// Clarke变换电压：abc -> αβ (功率不变变换)
	// 使用严格的数学变换矩阵
	V_alpha := sqrt2_3 * (Vu - 0.5*Vv - 0.5*Vw)
	V_beta := sqrt2_3 * (sqrt3_2 * (Vv - Vw))

	// Park变换电压：αβ -> dq (转子参考坐标系)
	V_ds := V_alpha*base.ThetaCos + V_beta*base.ThetaSin
	V_qs := -V_alpha*base.ThetaSin + V_beta*base.ThetaCos

	// 计算同步角速度 (电气角速度)
	base.Omega_e = float64(base.P) * base.Omega

	// 感应电机电压方程 (基于磁链状态空间模型)
	// dPsi/dt = V - R*I + ω × Ψ
	dPsi_ds_dt := V_ds - base.Rs*base.I_ds + base.Omega_e*base.Psi_qs
	dPsi_qs_dt := V_qs - base.Rs*base.I_qs - base.Omega_e*base.Psi_ds
	dPsi_dr_dt := -base.Rr*base.I_dr + (base.Omega_e-base.Omega)*base.Psi_qr
	dPsi_qr_dt := -base.Rr*base.I_qr - (base.Omega_e-base.Omega)*base.Psi_dr

	// 改进的数值积分方法 - 半隐式欧拉法
	// 使用预计算的电感参数
	if base.det > 1e-12 { // 避免数值不稳定
		// 从磁链计算电流 (使用自感参数)
		base.I_ds = (base.Lrr*base.Psi_ds - base.Lm*base.Psi_dr) / base.det
		base.I_qs = (base.Lrr*base.Psi_qs - base.Lm*base.Psi_qr) / base.det
		base.I_dr = (base.Lss*base.Psi_dr - base.Lm*base.Psi_ds) / base.det
		base.I_qr = (base.Lss*base.Psi_qr - base.Lm*base.Psi_qs) / base.det
	}

	// 数值积分：更新磁链状态 (前向欧拉法)
	base.Psi_ds += dt * dPsi_ds_dt
	base.Psi_qs += dt * dPsi_qs_dt
	base.Psi_dr += dt * dPsi_dr_dt
	base.Psi_qr += dt * dPsi_qr_dt

	// 计算电磁转矩 - 基于严格的物理公式
	// Te = (3/2) * (P/2) * (Ψ_ds * I_qs - Ψ_qs * I_ds)
	Te := (3.0 / 2.0) * (float64(base.P) / 2.0) * (base.Psi_ds*base.I_qs - base.Psi_qs*base.I_ds)

	// 机械运动方程 - 牛顿第二定律
	// J * dΩ/dt = Te - B*Ω
	dOmega_dt := (Te - base.B*base.Omega) / base.J
	base.Omega += dt * dOmega_dt
	base.Theta += dt * base.Omega

	// 更新角度三角函数值
	base.ThetaCos = math.Cos(base.Theta)
	base.ThetaSin = math.Sin(base.Theta)

	// 从 dq 电流计算 abc 相电流 (逆Park + 逆Clarke变换)
	I_alpha := base.I_ds*base.ThetaCos - base.I_qs*base.ThetaSin
	I_beta := base.I_ds*base.ThetaSin + base.I_qs*base.ThetaCos

	// 逆Clarke变换：αβ -> abc
	base.I_u = sqrt2_3 * I_alpha
	base.I_v = sqrt2_3 * (-0.5*I_alpha + sqrt3_2*I_beta)
	base.I_w = sqrt2_3 * (-0.5*I_alpha - sqrt3_2*I_beta)

	// 加盖电流源到电路
	stamp.StampCurrentSource(base.Nodes[0], base.Nodes[3], base.I_u)
	stamp.StampCurrentSource(base.Nodes[1], base.Nodes[4], base.I_v)
	stamp.StampCurrentSource(base.Nodes[2], base.Nodes[5], base.I_w)
}

// CalculateCurrent 电流计算
func (base *ACInductionMotorBase) CalculateCurrent(stamp types.Stamp) {
	// 设置三相电流
	base.Current.SetVec(0, base.I_u)
	base.Current.SetVec(1, base.I_v)
	base.Current.SetVec(2, base.I_w)
}

// StepFinished 步长迭代结束
func (base *ACInductionMotorBase) StepFinished(stamp types.Stamp) {}

// AngleDeg 转子角度
func (base *ACInductionMotorBase) AngleDeg() float64 {
	angle_deg := math.Mod(base.Theta*180.0/math.Pi, 360.0)
	if angle_deg < 0 {
		angle_deg += 360.0
	}
	return angle_deg
}

// Torque 电磁转矩计算
func (base *ACInductionMotorBase) Torque() float64 {
	return (3.0 / 2.0) * (float64(base.P) / 2.0) * (base.Psi_ds*base.I_qs - base.Psi_qs*base.I_ds)
}

// GetSlip 获取转差率
func (base *ACInductionMotorBase) GetSlip() float64 {
	if base.Omega_e == 0 {
		return 0
	}
	return (base.Omega_e - base.Omega) / base.Omega_e
}

// EstimateParametersFromBasic 从基础参数估算仿真参数
// 输入: 额定电压(V), 额定功率(W), 额定频率(Hz), 极对数, 额定转速(rpm), 效率(0-1), 功率因数(0-1)
// 输出: 仿真参数结构体
func EstimateParametersFromBasic(V_rated, P_rated, f_rated float64, polePairs int, n_rated, efficiency, powerFactor float64) *ACInductionMotorValue {
	// 数学常数
	pi := math.Pi
	sqrt3 := math.Sqrt(3.0)

	// 计算同步转速 (rpm)
	n_sync := 120.0 * f_rated / float64(polePairs)

	// 计算转差率
	s_rated := (n_sync - n_rated) / n_sync
	if s_rated < 0.01 {
		s_rated = 0.02 // 最小转差率
	}
	if s_rated > 0.1 {
		s_rated = 0.05 // 最大转差率限制
	}

	// 计算额定电流 (A)
	I_rated := P_rated / (sqrt3 * V_rated * efficiency * powerFactor)

	// 计算总阻抗 (Ω)
	Z_total := V_rated / (sqrt3 * I_rated)

	// 基于典型电机参数比例估算参数
	// 电阻部分 (占总阻抗的2-5%)
	Rs := 0.035 * Z_total // 定子电阻
	Rr := 0.035 * Z_total // 转子电阻

	// 电抗部分
	X_total := math.Sqrt(Z_total*Z_total - (Rs+Rr)*(Rs+Rr))
	if X_total <= 0 {
		X_total = 0.95 * Z_total // 默认电抗比例
	}

	// 分解电抗为漏感和磁化电抗
	// 漏感电抗占总电抗的15-25%
	X_leakage := 0.2 * X_total
	// 磁化电抗占总电抗的75-85%
	X_magnetizing := 0.8 * X_total

	// 转换为电感值 (H)
	omega_e := 2 * pi * f_rated
	Ls := X_leakage / (2 * omega_e) // 定子漏感
	Lr := X_leakage / (2 * omega_e) // 转子漏感
	Lm := X_magnetizing / omega_e   // 磁化电感

	// 估算机械参数
	// 转动惯量 (kg·m²) - 基于功率和转速的经验公式
	J := 0.02 * P_rated / (n_rated * n_rated)
	if J < 1e-6 {
		J = 1e-6 // 最小转动惯量
	}

	// 阻尼系数 (N·m·s/rad) - 基于功率和转速
	B := 0.002 * P_rated / n_rated

	// 创建并返回参数结构体
	params := &ACInductionMotorValue{
		Rs: Rs,
		Rr: Rr,
		Ls: Ls,
		Lr: Lr,
		Lm: Lm,
		P:  polePairs,
		J:  J,
		B:  B,
	}

	// 计算自感参数
	params.Lss = params.Ls + params.Lm
	params.Lrr = params.Lr + params.Lm

	return params
}

// Debug 调试
func (base *ACInductionMotorBase) Debug(stamp types.Stamp) string {
	return fmt.Sprintf("转速:%.3f rad/s 转矩:%.3f N·m 角度:%.3f deg 转差率:%.3f",
		base.Omega,
		base.Torque(),
		base.AngleDeg(),
		base.GetSlip())
}
