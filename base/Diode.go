package base

import (
	"circuit/mna"
	"fmt"
	"math"
)

// Diode 二极管（改进版本）
type Diode struct{ Base }

func (diode *Diode) New() {
	diode.ElementConfigBase = &mna.ElementConfigBase{
		Pin: []string{"d1", "d2"},
		ValueInit: []any{
			float64(1e-14),  // 0: 反向饱和电流 Is (A)
			float64(0),      // 1: 齐纳击穿电压 Vz (V) (0表示无齐纳击穿)
			float64(1),      // 2: 发射系数 N
			float64(0.1),    // 3: 串联电阻 Rs (Ω)
			float64(300.15), // 4: 温度 T (K)
			float64(0),      // 5: 上次电压差 V_old (V)
			float64(0),      // 6: 尺度电压 N*Vt (V)
			float64(0),      // 7: 1/(N*Vt) (1/V)
			float64(0),      // 8: 热电压 Vt = kT/q (V)
			float64(0),      // 9: 1/Vt (1/V)
			float64(0),      // 10: 齐纳动态电阻 Rz (Ω)
			float64(0),      // 11: 漏电流 = Is (A)
			float64(0),      // 12: 正向临界电压 Vcrit (V)
			float64(0),      // 13: 齐纳击穿临界电压 Vzcrit (V)
			float64(0),      // 14: 最小电导 Gmin (S)
		},
		Current:   []int{0},
		OrigValue: []int{5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
	}
}

func (diode *Diode) Init() mna.ValueMNA {
	return mna.NewElementBase(diode.ElementConfigBase)
}

func (Diode) Reset(base mna.ValueMNA) {
	// 常数
	k := 1.380649e-23   // 玻尔兹曼常数 (J/K)
	q := 1.60217662e-19 // 电子电荷 (C)

	// 获取参数
	temp := base.GetFloat64(4) // 温度 (K)
	Is := base.GetFloat64(0)   // 饱和电流 (A)
	N := base.GetFloat64(2)    // 发射系数
	Vz := base.GetFloat64(1)   // 齐纳电压 (V)

	// 计算热电压 Vt = kT/q
	Vt := k * temp / q
	base.SetFloat64(8, Vt)

	// 计算尺度电压和相关系数
	vscale := N * Vt
	base.SetFloat64(6, vscale)

	if vscale > 0 {
		base.SetFloat64(7, 1.0/vscale) // 1/(N*Vt)
	} else {
		base.SetFloat64(7, 0)
	}

	if Vt > 0 {
		base.SetFloat64(9, 1.0/Vt) // 1/Vt
	} else {
		base.SetFloat64(9, 0)
	}

	// 设置漏电流
	base.SetFloat64(11, Is)

	// 设置齐纳动态电阻（默认值，可根据需要调整）
	Rz := 0.1 // 默认动态电阻
	if Vz > 0 {
		// 齐纳动态电阻通常随Vz增加而增加
		Rz = 0.1 + 0.01*Vz
	}
	base.SetFloat64(10, Rz)

	// 计算临界电压
	// 正向临界电压 Vcrit：电流开始显著增加时的电压
	sqrt2 := 1.4142135623730951
	var Vcrit float64
	if vscale > 0 && Is > 0 {
		denominator := sqrt2 * Is
		if denominator > 0 {
			Vcrit = vscale * math.Log(vscale/denominator)
		} else {
			Vcrit = 0.7 // 典型硅二极管正向电压
		}
	} else {
		Vcrit = 0.7
	}
	base.SetFloat64(12, Vcrit)

	// 齐纳击穿临界电压 Vzcrit
	// 当反向电压超过Vzcrit时，认为进入击穿区
	var Vzcrit float64
	if Vz > 0 {
		// Vzcrit略小于Vz，表示开始进入击穿区
		// 使用Vz的95%作为临界点
		Vzcrit = Vz * 0.95
	} else {
		// 无齐纳击穿，设置大值
		Vzcrit = 1000.0
	}
	base.SetFloat64(13, Vzcrit)

	// 计算最小电导防止奇异矩阵
	Gmin := Is * 0.01
	if Gmin < 1e-12 {
		Gmin = 1e-12
	}
	base.SetFloat64(14, Gmin)

	fmt.Printf("二极管参数重置: Is=%.1eA, Vz=%.2fV, N=%.2f, T=%.1fK, Vt=%.6fV, Vcrit=%.3fV, Vzcrit=%.3fV\n",
		Is, Vz, N, temp, Vt, Vcrit, Vzcrit)
}

func (Diode) DoStep(mna mna.MNA, base mna.ValueMNA) {
	// 获取节点电压
	v1 := mna.GetNodeVoltage(base.Nodes(0))
	v2 := mna.GetNodeVoltage(base.Nodes(1))
	V := v1 - v2 // 阳极-阴极电压
	V_old := base.GetFloat64(5)

	// 检查收敛性
	if math.Abs(V-V_old) > 0.01 {
		base.Converged()
	}

	// 限制电压步长以保证数值稳定性
	V = limitDiodeStep(V, V_old, base)
	base.SetFloat64(5, V)

	// 获取最小电导
	Gmin := base.GetFloat64(14)

	// 根据工作区域选择模型
	Vz := base.GetFloat64(1)
	if V >= 0 || Vz == 0 {
		// 正向偏置或无齐纳击穿的二极管
		modelForward(mna, base, V, Gmin)
	} else {
		// 反向偏置，可能进入齐纳击穿
		modelReverse(mna, base, V, Gmin)
	}

	// 添加串联电阻贡献
	Rs := base.GetFloat64(3)
	if Rs > 0 {
		mna.StampResistor(base.Nodes(0), base.Nodes(1), Rs)
	}
}

func (Diode) CalculateCurrent(mna mna.MNA, base mna.ValueMNA) {
	v1 := mna.GetNodeVoltage(base.Nodes(0))
	v2 := mna.GetNodeVoltage(base.Nodes(1))
	V := v1 - v2
	current := calculateCurrent(V, base)
	mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), -current)
}

// 辅助函数

// limitDiodeStep 限制二极管电压步长
func limitDiodeStep(Vnew, Vold float64, base mna.ValueMNA) float64 {
	Vcrit := base.GetFloat64(12)
	Vzcrit := base.GetFloat64(13)
	vscale := base.GetFloat64(6)
	Vt := base.GetFloat64(8)
	Vz := base.GetFloat64(1)

	// 正向电压限制
	if Vnew > Vcrit && math.Abs(Vnew-Vold) > 2*vscale {
		if Vold > 0 {
			arg := 1 + (Vnew-Vold)/vscale
			if arg > 0 {
				Vnew = Vold + vscale*math.Log(arg)
			} else {
				Vnew = Vcrit
			}
		} else {
			Vnew = vscale * math.Log(Vnew/vscale+1)
		}
		base.Converged()
	}

	// 反向电压限制（齐纳击穿区）
	if Vz > 0 && Vnew < -Vzcrit && math.Abs(Vnew-Vold) > 2*Vt {
		// 在击穿区，限制电压变化
		maxChange := 0.1 * Vz // 最大变化为Vz的10%
		if Vnew < Vold-maxChange {
			Vnew = Vold - maxChange
			base.Converged()
		} else if Vnew > Vold+maxChange {
			Vnew = Vold + maxChange
			base.Converged()
		}
	}

	return Vnew
}

// modelForward 正向偏置模型
func modelForward(mna mna.MNA, base mna.ValueMNA, V, Gmin float64) {
	Is := base.GetFloat64(11)
	coef := base.GetFloat64(7) // 1/(N*Vt)

	// 标准二极管方程：I = Is * [exp(V/(N*Vt)) - 1]
	expV := math.Exp(V * coef)

	// 电导：dI/dV = Is * (1/(N*Vt)) * exp(V/(N*Vt))
	Geq := Is*coef*expV + Gmin

	// 等效电流源：Ieq = I(V) - Geq*V
	I_V := Is * (expV - 1)
	Ieq := I_V - Geq*V

	mna.StampConductance(base.Nodes(0), base.Nodes(1), Geq)
	mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), Ieq)
}

// modelReverse 反向偏置模型（包含齐纳击穿）
func modelReverse(mna mna.MNA, base mna.ValueMNA, V, Gmin float64) {
	Is := base.GetFloat64(11)
	Vz := base.GetFloat64(1)
	coef := base.GetFloat64(7) // 1/(N*Vt)
	Rz := base.GetFloat64(10)  // 齐纳动态电阻
	Vzcrit := base.GetFloat64(13)

	// 判断是否进入齐纳击穿
	if Vz > 0 && V < -Vzcrit {
		// 齐纳击穿区
		// 使用分段线性模型：I = -(V + Vz) / Rz  (当 V < -Vz)
		// 但为了平滑过渡，使用平滑函数

		// 平滑过渡参数
		Vsoft := 0.1 // 软化电压

		// 击穿电流
		Vbreak := -Vz
		I_break := -(V - Vbreak) / Rz

		// 反向饱和电流（未击穿时）
		I_rev := -Is * (1 - math.Exp(V*coef))

		// 平滑过渡函数
		// 当V远小于-Vz时，使用击穿模型
		// 当V接近-Vz时，混合两种模型
		transition := 1.0 / (1.0 + math.Exp((V+Vz)/Vsoft))
		I_V := transition*I_break + (1-transition)*I_rev

		// 计算电导（导数）
		// dI_break/dV = -1/Rz
		// dI_rev/dV = -Is*coef*exp(V*coef)
		dI_break_dV := -1.0 / Rz
		dI_rev_dV := -Is * coef * math.Exp(V*coef)

		// 过渡函数的导数
		s := math.Exp((V + Vz) / Vsoft)
		dtransition_dV := -s / (Vsoft * math.Pow(1+s, 2))

		Geq := transition*dI_break_dV + (1-transition)*dI_rev_dV +
			dtransition_dV*(I_break-I_rev) + Gmin

		// 等效电流源
		Ieq := I_V - Geq*V

		mna.StampConductance(base.Nodes(0), base.Nodes(1), Geq)
		mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), Ieq)

	} else {
		// 普通反向偏置（未击穿）
		// I = -Is * (1 - exp(V/(N*Vt)))
		expV := math.Exp(V * coef)
		I_V := -Is * (1 - expV)

		// 电导：dI/dV = -Is * coef * exp(V*coef)
		Geq := -Is*coef*expV + Gmin

		// 等效电流源
		Ieq := I_V - Geq*V

		mna.StampConductance(base.Nodes(0), base.Nodes(1), Geq)
		mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), Ieq)
	}
}

// calculateCurrent 计算二极管电流
func calculateCurrent(V float64, base mna.ValueMNA) float64 {
	Is := base.GetFloat64(11)
	Vz := base.GetFloat64(1)
	coef := base.GetFloat64(7)
	Rz := base.GetFloat64(10)
	Vzcrit := base.GetFloat64(13)

	if V >= 0 {
		// 正向偏置
		return Is * (math.Exp(V*coef) - 1)
	} else if Vz > 0 && V < -Vzcrit {
		// 齐纳击穿区
		Vsoft := 0.1
		Vbreak := -Vz
		I_break := -(V - Vbreak) / Rz
		I_rev := -Is * (1 - math.Exp(V*coef))
		transition := 1.0 / (1.0 + math.Exp((V+Vz)/Vsoft))
		return transition*I_break + (1-transition)*I_rev
	} else {
		// 反向偏置（未击穿）
		return -Is * (1 - math.Exp(V*coef))
	}
}
