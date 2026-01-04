package base

import (
	"circuit/element"
	"circuit/mna"
	"circuit/utils"
	"math"
)

// DiodeType 定义元件
var DiodeType element.NodeType = element.AddElement(2, &Diode{
	&element.Config{
		Name: "d",
		Pin:  []string{"anode", "cathode"},
		ValueInit: []any{
			float64(1e-14),    // 0: 反向饱和电流 Is (A)
			float64(0),        // 1: 齐纳击穿电压 Vz (V) (0表示无齐纳击穿)
			float64(1),        // 2: 发射系数 N
			float64(0.1),      // 3: 串联电阻 Rs (Ω)
			float64(300.15),   // 4: 温度 T (K)
			float64(0),        // 5: 上次电压差 V_old (V)
			float64(0),        // 6: 尺度电压 N*Vt (V)
			float64(0),        // 7: 1/(N*Vt) (1/V)
			float64(0.025865), // 8: 热电压 Vt = kT/q (V) (27°C时的默认值)
			float64(38.662),   // 9: 1/Vt (1/V)
			float64(0),        // 10: 齐纳偏移量 zoffset (V)
			float64(0),        // 11: 正向临界电压 vcrit (V)
			float64(0),        // 12: 齐纳临界电压 vzcrit (V)
			float64(0),        // 13: 漏电流 leakage (A)
			float64(0),        // 14: 最小电导 Gmin (S)
		},
		Current:   []int{0},
		OrigValue: []int{5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
	},
})

// Diode 二极管（基于CircuitJS1的精确实现）
type Diode struct{ *element.Config }

func (Diode) CirLoad(element.NodeFace, utils.NetList)  {}
func (Diode) CirExport(element.NodeFace) utils.NetList { return nil }

func (Diode) Reset(base element.NodeFace) {
	// 获取参数
	Is := base.GetFloat64(0)   // 饱和电流 (A)
	Vz := base.GetFloat64(1)   // 齐纳电压 (V)
	N := base.GetFloat64(2)    // 发射系数
	temp := base.GetFloat64(4) // 温度 (K)

	// 计算热电压 Vt = kT/q
	k := 1.380649e-23   // 玻尔兹曼常数 (J/K)
	q := 1.60217662e-19 // 电子电荷 (C)
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
	base.SetFloat64(13, Is)

	// 计算临界电压（基于CircuitJS1算法）
	// 正向临界电压 vcrit：电流为 vscale/sqrt(2) 时的电压
	var vcrit float64
	if vscale > 0 && Is > 0 {
		vcrit = vscale * math.Log(vscale/(math.Sqrt(2)*Is))
	} else {
		vcrit = 0.7 // 默认值
	}
	base.SetFloat64(11, vcrit)

	// 计算齐纳偏移量和临界电压
	var zoffset, vzcrit float64
	if Vz == 0 {
		zoffset = 0
		vzcrit = 0
	} else {
		// 计算偏移量，使得在Vz时有-5mA电流
		i := -0.005 // -5mA
		zoffset = Vz - math.Log(-(1+i/Is))/base.GetFloat64(9)

		// 齐纳临界电压
		vzcrit = Vt * math.Log(Vt/(math.Sqrt(2)*Is))
	}
	base.SetFloat64(10, zoffset)
	base.SetFloat64(12, vzcrit)

	// 计算最小电导防止奇异矩阵
	Gmin := Is * 0.01
	if Gmin < 1e-12 {
		Gmin = 1e-12
	}
	base.SetFloat64(14, Gmin)

	// 重置上次电压差
	base.SetFloat64(5, 0)
}

func (Diode) DoStep(mna mna.MNA, time mna.Time, value element.NodeFace) {
	// 获取节点电压
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	voltdiff := v1 - v2 // 阳极-阴极电压
	lastvoltdiff := value.GetFloat64(5)
	Vz := value.GetFloat64(1) // 齐纳电压

	// 检查收敛性（基于CircuitJS1算法）
	if math.Abs(voltdiff-lastvoltdiff) > 0.01 {
		time.Converged()
	}

	// 对于齐纳二极管，实现平滑的分段模型：
	// 1. 反向电压绝对值远小于齐纳电压：使用大电阻（近似开路）
	// 2. 反向电压在齐纳电压附近：混合模型，平滑过渡
	// 3. 反向电压远大于齐纳电压：使用完整的齐纳击穿模型
	// 4. 正向偏置：使用常规二极管模型
	if Vz > 0 && voltdiff < 0 {
		// 反向偏置齐纳二极管
		reverseVoltage := -voltdiff // 反向电压绝对值

		// 在齐纳电压附近实现平滑过渡（Vz ± 0.1V）
		if reverseVoltage < Vz-0.1 {
			// 反向电压明显小于齐纳电压：使用大电阻模拟反向漏电流
			// 使用非常大的电阻（100MΩ）模拟开路，但防止奇异矩阵
			mna.StampResistor(value.GetNodes(0), value.GetNodes(1), 1e8)
			// 不需要执行后续的复杂模型计算
			value.SetFloat64(5, voltdiff)
			return
		} else if reverseVoltage < Vz+0.1 {
			// 在齐纳电压附近：混合模型，平滑过渡
			// 计算混合权重：0到1之间
			weight := (reverseVoltage - (Vz - 0.1)) / 0.2
			if weight < 0 {
				weight = 0
			}
			if weight > 1 {
				weight = 1
			}

			// 大电阻模型
			mna.StampResistor(value.GetNodes(0), value.GetNodes(1), 1e8)

			// 齐纳模型贡献（按权重混合）
			voltdiff = limitDiodeStep(voltdiff, lastvoltdiff, time, value)
			value.SetFloat64(5, voltdiff)

			// 执行MNA建模，但按权重缩放贡献
			doDiodeStepWeighted(mna, value, voltdiff, weight)

			// 添加串联电阻贡献
			Rs := value.GetFloat64(3)
			if Rs > 0 {
				mna.StampResistor(value.GetNodes(0), value.GetNodes(1), Rs)
			}
			return
		}
		// 反向电压明显大于齐纳电压：继续执行完整的齐纳模型
	}

	// 限制电压步长
	voltdiff = limitDiodeStep(voltdiff, lastvoltdiff, time, value)
	value.SetFloat64(5, voltdiff)

	// 执行MNA建模
	doDiodeStep(mna, time, value, voltdiff)

	// 添加串联电阻贡献（使用用户设置的原始值，不额外增加）
	Rs := value.GetFloat64(3)
	if Rs > 0 {
		mna.StampResistor(value.GetNodes(0), value.GetNodes(1), Rs)
	}
}

func (Diode) CalculateCurrent(mna mna.MNA, time mna.Time, value element.NodeFace) {
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	voltdiff := v1 - v2
	current := calculateDiodeCurrent(voltdiff, value)
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), -current)
}

// limitDiodeStep 限制二极管电压步长（基于CircuitJS1算法）
func limitDiodeStep(vnew, vold float64, time mna.Time, value element.NodeFace) float64 {
	vscale := value.GetFloat64(6)
	vcrit := value.GetFloat64(11)
	Vt := value.GetFloat64(8)
	vzcrit := value.GetFloat64(12)
	zoffset := value.GetFloat64(10)

	// 检查新电压；电流是否变化了e^2因子？
	if vnew > vcrit && math.Abs(vnew-vold) > (vscale+vscale) {
		if vold > 0 {
			arg := 1 + (vnew-vold)/vscale
			if arg > 0 {
				// 调整vnew使得电流与上一次迭代的线性化模型相同
				vnew = vold + vscale*math.Log(arg)
			} else {
				vnew = vcrit
			}
		} else {
			// 调整vnew使得电流与上一次迭代的线性化模型相同
			// 防止vnew/vscale <= 0导致对数计算错误
			if vnew > 0 && vscale > 0 {
				ratio := vnew / vscale
				if ratio > 1e-10 { // 防止数值下溢
					vnew = vscale * math.Log(ratio)
				} else {
					vnew = vscale * math.Log(1e-10)
				}
			} else {
				// 如果vnew <= 0，使用一个小的正数
				vnew = vscale * math.Log(1e-10)
			}
		}
		time.Converged()
	} else if vnew < 0 && zoffset != 0 {
		// 对于齐纳击穿，使用相同的逻辑但平移值，
		// 并用齐纳特定值替换正常值以考虑齐纳击穿曲线的更陡指数
		vnewTrans := -vnew - zoffset
		voldTrans := -vold - zoffset

		if vnewTrans > vzcrit && math.Abs(vnewTrans-voldTrans) > (Vt+Vt) {
			if voldTrans > 0 {
				arg := 1 + (vnewTrans-voldTrans)/Vt
				if arg > 0 {
					vnewTrans = voldTrans + Vt*math.Log(arg)
				} else {
					vnewTrans = vzcrit
				}
			} else {
				// 防止vnewTrans/Vt <= 0导致对数计算错误
				if vnewTrans > 0 && Vt > 0 {
					ratio := vnewTrans / Vt
					if ratio > 1e-10 {
						vnewTrans = Vt * math.Log(ratio)
					} else {
						vnewTrans = Vt * math.Log(1e-10)
					}
				} else {
					vnewTrans = Vt * math.Log(1e-10)
				}
			}
			time.Converged()
		}
		vnew = -(vnewTrans + zoffset)
	}
	return vnew
}

// doDiodeStep 执行二极管MNA建模（基于CircuitJS1算法）
func doDiodeStep(mna mna.MNA, time mna.Time, value element.NodeFace, voltdiff float64) {
	leakage := value.GetFloat64(13) // 漏电流（饱和电流）
	vdcoef := value.GetFloat64(7)   // 1/(N*Vt)
	vzcoef := value.GetFloat64(9)   // 1/Vt
	zoffset := value.GetFloat64(10)
	Vz := value.GetFloat64(1)

	// 防止奇异矩阵或其他数值问题，在每个PN结上并联一个微小电导
	// 使用更小的默认gmin值，避免过度影响电路
	gmin := leakage * 0.01
	if gmin < 1e-12 {
		gmin = 1e-12
	}

	// 只有在收敛困难时才增加gmin，且增加幅度要小
	// 原始CircuitJS1代码中这个逻辑可能导致gmin过大
	// 这里使用更保守的值
	subIterations := time.GoodIterations()
	if subIterations > 100 {
		// 缓慢增加gmin，但最大值限制在1e-6
		extraGmin := math.Exp(-12 * math.Log(10) * float64(1-subIterations/1000.))
		if extraGmin > 1e-6 {
			extraGmin = 1e-6
		}
		gmin += extraGmin
	}

	if voltdiff >= 0 || Vz == 0 {
		// 常规二极管或正向偏置齐纳二极管
		eval := math.Exp(voltdiff * vdcoef)
		geq := vdcoef*leakage*eval + gmin
		nc := (eval-1)*leakage - geq*voltdiff
		mna.StampConductance(value.GetNodes(0), value.GetNodes(1), geq)
		mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), nc)
	} else {
		// 齐纳二极管

		// 对于反向偏置齐纳二极管，使用类似于理想肖克利曲线的指数来模拟齐纳击穿曲线。
		// （真实的击穿曲线不是简单的指数，但这个近似应该可以。）

		/*
		 * I(Vd) = Is * (exp[Vd*C] - exp[(-Vd-Vz)*Cz] - 1 )
		 *
		 * geq 是 I'(Vd)
		 * nc 是 I(Vd) + I'(Vd)*(-Vd)
		 */

		geq := leakage*(vdcoef*math.Exp(voltdiff*vdcoef)+vzcoef*math.Exp((-voltdiff-zoffset)*vzcoef)) + gmin

		nc := leakage*(math.Exp(voltdiff*vdcoef)-
			math.Exp((-voltdiff-zoffset)*vzcoef)-
			1) + geq*(-voltdiff)

		mna.StampConductance(value.GetNodes(0), value.GetNodes(1), geq)
		mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), nc)
	}
}

// doDiodeStepWeighted 执行加权二极管MNA建模
func doDiodeStepWeighted(mna mna.MNA, value element.NodeFace, voltdiff, weight float64) {
	leakage := value.GetFloat64(13) // 漏电流（饱和电流）
	vdcoef := value.GetFloat64(7)   // 1/(N*Vt)
	vzcoef := value.GetFloat64(9)   // 1/Vt
	zoffset := value.GetFloat64(10)
	Vz := value.GetFloat64(1)

	// 防止奇异矩阵或其他数值问题，在每个PN结上并联一个微小电导
	gmin := leakage * 0.01
	if gmin < 1e-12 {
		gmin = 1e-12
	}

	if voltdiff >= 0 || Vz == 0 {
		// 常规二极管或正向偏置齐纳二极管
		eval := math.Exp(voltdiff * vdcoef)
		geq := vdcoef*leakage*eval + gmin
		nc := (eval-1)*leakage - geq*voltdiff
		// 按权重缩放贡献
		mna.StampConductance(value.GetNodes(0), value.GetNodes(1), geq*weight)
		mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), nc*weight)
	} else {
		// 齐纳二极管
		geq := leakage*(vdcoef*math.Exp(voltdiff*vdcoef)+vzcoef*math.Exp((-voltdiff-zoffset)*vzcoef)) + gmin
		nc := leakage*(math.Exp(voltdiff*vdcoef)-
			math.Exp((-voltdiff-zoffset)*vzcoef)-
			1) + geq*(-voltdiff)
		// 按权重缩放贡献
		mna.StampConductance(value.GetNodes(0), value.GetNodes(1), geq*weight)
		mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), nc*weight)
	}
}

// calculateDiodeCurrent 计算二极管电流（基于CircuitJS1算法）
func calculateDiodeCurrent(voltdiff float64, value element.NodeFace) float64 {
	leakage := value.GetFloat64(13) // 漏电流（饱和电流）
	vdcoef := value.GetFloat64(7)   // 1/(N*Vt)
	vzcoef := value.GetFloat64(9)   // 1/Vt
	zoffset := value.GetFloat64(10)
	Vz := value.GetFloat64(1)

	if voltdiff >= 0 || Vz == 0 {
		return leakage * (math.Exp(voltdiff*vdcoef) - 1)
	}
	return leakage * (math.Exp(voltdiff*vdcoef) -
		math.Exp((-voltdiff-zoffset)*vzcoef) -
		1)
}
