// 稳压管(Zener Diode)元件实现，基于Diode模型的齐纳击穿封装。
// 默认Vz=5.1V，固定工作在反偏齐纳击穿模式，提供稳定的参考电压。
package semiconductor

import (
	"circuit/element"
	"circuit/mna"
	"math"
)

// ZenerType 稳压管类型标识(NodeType=34)，网表"Z<name> <anode,cathode> [Vz,Is,N,Rs]"。
var ZenerType element.NodeType = element.AddElement(34, &Zener{
	&element.Config{
		Name:     "z",
		Pin:      element.SetPin(element.PinLowVoltage, "anode", "cathode"),
		Internal: []string{"int_anode"},
		ValueInit: []any{
			float64(1e-14),    // 0: 反向饱和电流 Is (A)
			float64(5.1),      // 1: 齐纳击穿电压 Vz (V) (默认>0，固定开启齐纳模式)
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
		ValueName: []string{"Is", "Vz", "N", "Rs", "T", "V_old", "NVt", "invNVt", "Vt", "invVt", "zoffset", "vcrit", "vzcrit", "leakage", "Gmin"},
		Current:   []int{0},
		OrigValue: []int{5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
		Flags:     element.FlagNonlinear | element.FlagCacheStamp,
	},
})

// Zener 稳压管元件。基于CircuitJS1二极管模型的齐纳击穿分支，
// Vz>0默认开启齐纳模式，Is/N/Rs控制二极管特性。
type Zener struct{ *element.Config }

// Reset 初始化稳压管热电压、尺度电压、齐纳偏移量等内部状态。
// 计算Vt=kT/q、NVt=N·Vt、vcrit、zoffset、vzcrit及最小电导Gmin。
func (Zener) Reset(base element.NodeFace) {
	Is := base.GetFloat64(0)
	Vz := base.GetFloat64(1)
	N := base.GetFloat64(2)
	temp := base.GetFloat64(4)

	k := 1.380649e-23
	q := 1.60217662e-19
	Vt := k * temp / q
	base.SetFloat64(8, Vt)

	vscale := N * Vt
	base.SetFloat64(6, vscale)

	if vscale > 0 {
		base.SetFloat64(7, 1.0/vscale)
	} else {
		base.SetFloat64(7, 0)
	}

	if Vt > 0 {
		base.SetFloat64(9, 1.0/Vt)
	} else {
		base.SetFloat64(9, 0)
	}

	base.SetFloat64(13, Is)

	var vcrit float64
	if vscale > 0 && Is > 0 {
		vcrit = vscale * math.Log(vscale/(math.Sqrt(2)*Is))
	} else {
		vcrit = 0.7
	}
	base.SetFloat64(11, vcrit)

	var zoffset, vzcrit float64
	if Vz == 0 || Vt <= 0 || Is <= 0 {
		zoffset = 0
		vzcrit = 0
	} else {
		i := -0.005
		// 对数参数 -(1+i/Is) 必须为正；Is 过大（>=5mA）时为非正，回退到 0 避免 NaN 污染矩阵。
		if logArg := -(1 + i/Is); logArg > 0 {
			zoffset = Vz - math.Log(logArg)/base.GetFloat64(9)
		}
		vzcrit = Vt * math.Log(Vt/(math.Sqrt(2)*Is))
	}
	base.SetFloat64(10, zoffset)
	base.SetFloat64(12, vzcrit)

	Gmin := Is * 0.01
	if Gmin < 1e-12 {
		Gmin = 1e-12
	}
	base.SetFloat64(14, Gmin)

	base.SetFloat64(5, 0)
}

// DoStep 执行稳压管每步仿真。检测齐纳击穿区(<Vz-0.1为高阻，Vz±0.1为过渡区加权混叠)，
// 限制电压步长后用Newton线性化加盖MNA矩阵。
func (Zener) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	vint := mna.GetNodeVoltage(value.GetNodesInternal(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	voltdiff := vint - v2
	lastvoltdiff := value.GetFloat64(5)
	Vz := value.GetFloat64(1)

	if math.Abs(voltdiff-lastvoltdiff) > 0.01 {
		time.NoConverged()
	}

	// 固定开启齐纳模式，Vz默认>0
	if Vz > 0 && voltdiff < 0 {
		reverseVoltage := -voltdiff
		if reverseVoltage < Vz-0.1 {
			mna.StampImpedance(value.GetNodesInternal(0), value.GetNodes(1), 1e8)
			value.SetFloat64(5, voltdiff)
			Rs := value.GetFloat64(3)
			if Rs > 0 {
				mna.StampImpedance(value.GetNodes(0), value.GetNodesInternal(0), Rs)
			}
			return
		} else if reverseVoltage < Vz+0.1 {
			weight := (reverseVoltage - (Vz - 0.1)) / 0.2
			if weight < 0 {
				weight = 0
			}
			if weight > 1 {
				weight = 1
			}
			mna.StampImpedance(value.GetNodesInternal(0), value.GetNodes(1), 1e8)
			voltdiff = zenerLimitStep(voltdiff, lastvoltdiff, time, value)
			value.SetFloat64(5, voltdiff)
			zenerDoStepWeighted(mna, value, voltdiff, weight)
			Rs := value.GetFloat64(3)
			if Rs > 0 {
				mna.StampImpedance(value.GetNodes(0), value.GetNodesInternal(0), Rs)
			}
			return
		}
	}

	voltdiff = zenerLimitStep(voltdiff, lastvoltdiff, time, value)
	value.SetFloat64(5, voltdiff)

	zenerDoStep(mna, time, value, voltdiff)

	Rs := value.GetFloat64(3)
	if Rs > 0 {
		mna.StampImpedance(value.GetNodes(0), value.GetNodesInternal(0), Rs)
	}
}

// CalculateCurrent 计算并加盖稳压管支路电流。
// 正向用二极管方程，反偏使用齐纳击穿分支计算电流。
func (Zener) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	vint := mna.GetNodeVoltage(value.GetNodesInternal(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	voltdiff := vint - v2
	current := zenerCalculateCurrent(voltdiff, value)
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), -current)
}

func zenerSafeExp(x float64) float64 {
	const maxExpArg = 700.0
	if x > maxExpArg {
		x = maxExpArg
	} else if x < -maxExpArg {
		x = -maxExpArg
	}
	return math.Exp(x)
}

func zenerLimitStep(vnew, vold float64, time mna.Time, value element.NodeFace) float64 {
	vscale := value.GetFloat64(6)
	if vscale == 0 {
		return vnew
	}
	vcrit := value.GetFloat64(11)
	Vt := value.GetFloat64(8)
	vzcrit := value.GetFloat64(12)
	zoffset := value.GetFloat64(10)

	if vnew > vcrit && math.Abs(vnew-vold) > (vscale+vscale) {
		if vold > 0 {
			arg := 1 + (vnew-vold)/vscale
			if arg > 0 {
				vnew = vold + vscale*math.Log(arg)
			} else {
				vnew = vcrit
			}
		} else {
			vnew = vcrit
		}
		time.NoConverged()
	} else if vnew < 0 && zoffset != 0 {
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
			time.NoConverged()
		}
		vnew = -(vnewTrans + zoffset)
	}
	return vnew
}

func zenerDoStep(mna mna.Mna, time mna.Time, value element.NodeFace, voltdiff float64) {
	leakage := value.GetFloat64(13)
	vdcoef := value.GetFloat64(7)
	vzcoef := value.GetFloat64(9)
	zoffset := value.GetFloat64(10)
	Vz := value.GetFloat64(1)

	gmin := leakage * 0.01
	if gmin < 1e-12 {
		gmin = 1e-12
	}

	subIterations := time.GoodIterations()
	if subIterations > 100 {
		ratio := 1.0 - math.Min(float64(subIterations)/1000.0, 1.0)
		extraGmin := math.Exp(-12 * math.Ln10 * ratio)
		if extraGmin < 1e-15 {
			extraGmin = 0
		}
		if extraGmin > 1e-6 {
			extraGmin = 1e-6
		}
		gmin += extraGmin
	}

	if voltdiff >= 0 || Vz == 0 {
		eval := zenerSafeExp(voltdiff * vdcoef)
		geq := vdcoef*leakage*eval + gmin
		nc := (eval-1)*leakage - geq*voltdiff
		mna.StampAdmittance(value.GetNodesInternal(0), value.GetNodes(1), geq)
		mna.StampCurrentSource(value.GetNodesInternal(0), value.GetNodes(1), nc)
	} else {
		geq := leakage*(vdcoef*zenerSafeExp(voltdiff*vdcoef)+vzcoef*zenerSafeExp((-voltdiff-zoffset)*vzcoef)) + gmin
		nc := leakage*(zenerSafeExp(voltdiff*vdcoef)-
			zenerSafeExp((-voltdiff-zoffset)*vzcoef)-
			1) + geq*(-voltdiff)
		mna.StampAdmittance(value.GetNodesInternal(0), value.GetNodes(1), geq)
		mna.StampCurrentSource(value.GetNodesInternal(0), value.GetNodes(1), nc)
	}
}

func zenerDoStepWeighted(mna mna.Mna, value element.NodeFace, voltdiff, weight float64) {
	leakage := value.GetFloat64(13)
	vdcoef := value.GetFloat64(7)
	vzcoef := value.GetFloat64(9)
	zoffset := value.GetFloat64(10)
	Vz := value.GetFloat64(1)

	gmin := leakage * 0.01
	if gmin < 1e-12 {
		gmin = 1e-12
	}

	if voltdiff >= 0 || Vz == 0 {
		eval := zenerSafeExp(voltdiff * vdcoef)
		geq := vdcoef*leakage*eval + gmin
		nc := (eval-1)*leakage - geq*voltdiff
		mna.StampAdmittance(value.GetNodesInternal(0), value.GetNodes(1), geq*weight)
		mna.StampCurrentSource(value.GetNodesInternal(0), value.GetNodes(1), nc*weight)
	} else {
		geq := leakage*(vdcoef*zenerSafeExp(voltdiff*vdcoef)+vzcoef*zenerSafeExp((-voltdiff-zoffset)*vzcoef)) + gmin
		nc := leakage*(zenerSafeExp(voltdiff*vdcoef)-
			zenerSafeExp((-voltdiff-zoffset)*vzcoef)-
			1) + geq*(-voltdiff)
		mna.StampAdmittance(value.GetNodesInternal(0), value.GetNodes(1), geq*weight)
		mna.StampCurrentSource(value.GetNodesInternal(0), value.GetNodes(1), nc*weight)
	}
}

func zenerCalculateCurrent(voltdiff float64, value element.NodeFace) float64 {
	leakage := value.GetFloat64(13)
	vdcoef := value.GetFloat64(7)
	vzcoef := value.GetFloat64(9)
	zoffset := value.GetFloat64(10)
	Vz := value.GetFloat64(1)

	if voltdiff >= 0 || Vz == 0 {
		return leakage * (zenerSafeExp(voltdiff*vdcoef) - 1)
	}
	return leakage * (zenerSafeExp(voltdiff*vdcoef) -
		zenerSafeExp((-voltdiff-zoffset)*vzcoef) -
		1)
}
