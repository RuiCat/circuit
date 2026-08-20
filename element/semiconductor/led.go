// 发光二极管(LED)元件实现，基于Diode模型的低电流变体。
// 典型参数: Is=1e-18A, N=1.5, Rs=5Ω, Vf≈2.0V(参考)。
package semiconductor

import (
	"circuit/element"
	"circuit/mna"
	"math"
)

// LEDType 发光二极管类型标识(NodeType=35)，网表"LED<name> <anode,cathode> [Is,N,Rs,Vf]"。
var LEDType element.NodeType = element.AddElement(35, &LED{
	&element.Config{
		Name:     "led",
		Pin:      element.SetPin(element.PinLowVoltage, "anode", "cathode"),
		Internal: []string{"int_anode"},
		ValueInit: []any{
			float64(1e-18),    // 0: 反向饱和电流 Is (A)
			float64(0),        // 1: 齐纳击穿电压 Vz (V) (0表示无齐纳)
			float64(1.5),      // 2: 发射系数 N
			float64(5),        // 3: 串联电阻 Rs (Ω)
			float64(300.15),   // 4: 温度 T (K)
			float64(0),        // 5: 上次电压差 V_old (V)
			float64(0),        // 6: 尺度电压 N*Vt (V)
			float64(0),        // 7: 1/(N*Vt) (1/V)
			float64(0.025865), // 8: 热电压 Vt = kT/q (V)
			float64(38.662),   // 9: 1/Vt (1/V)
			float64(0),        // 10: 齐纳偏移量 zoffset (V) (始终0)
			float64(0),        // 11: 正向临界电压 vcrit (V)
			float64(0),        // 12: 齐纳临界电压 vzcrit (V) (始终0)
			float64(0),        // 13: 漏电流 leakage (A)
			float64(0),        // 14: 最小电导 Gmin (S)
			float64(2.0),      // 15: 典型正向压降 Vf (V) (仅作参考/显示)
			float64(0),        // 16: 电流 I (A)（CalculateCurrent 写入）
		},
		ValueName: []string{"Is", "Vz", "N", "Rs", "T", "V_old", "NVt", "invNVt", "Vt", "invVt", "zoffset", "vcrit", "vzcrit", "leakage", "Gmin", "Vf", "I"},
		Current:   []int{16},
		OrigValue: []int{5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
		Flags:     element.FlagNonlinear | element.FlagCacheStamp,
	},
})

// LED 发光二极管元件。使用更小的饱和电流(1e-18A)和更大的发射系数(1.5)
// 来模拟LED的I-V特性曲线，发光强度可通过正向电流线性估算。
type LED struct{ *element.Config }

// Reset 初始化LED热电压、尺度电压等内部状态。
// 无齐纳模式：zoffset与vzcrit固定为0。
func (LED) Reset(base element.NodeFace) {
	Is := base.GetFloat64(0)
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

	// LED不做齐纳模式：zoffset和vzcrit始终为0
	base.SetFloat64(10, 0)
	base.SetFloat64(12, 0)

	Gmin := Is * 0.01
	if Gmin < 1e-12 {
		Gmin = 1e-12
	}
	base.SetFloat64(14, Gmin)

	base.SetFloat64(5, 0)

	// 无独立亮度状态：发光强度与正向电流成正比（见类型文档）
}

// DoStep LED每步仿真。限制正向电压步长后进行Newton线性化加盖。
func (LED) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	vint := mna.GetNodeVoltage(value.GetNodesInternal(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	voltdiff := vint - v2
	lastvoltdiff := value.GetFloat64(5)

	if math.Abs(voltdiff-lastvoltdiff) > 0.01 {
		time.NoConverged()
	}

	voltdiff = ledLimitStep(voltdiff, lastvoltdiff, time, value)
	value.SetFloat64(5, voltdiff)

	ledDoStep(mna, time, value, voltdiff)

	Rs := value.GetFloat64(3)
	if Rs > 0 {
		mna.StampImpedance(value.GetNodes(0), value.GetNodesInternal(0), Rs)
	}
}

// CalculateCurrent 计算并加盖LED支路电流。正向电流与发光强度成正比。
func (LED) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	vint := mna.GetNodeVoltage(value.GetNodesInternal(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	voltdiff := vint - v2
	current := ledCalculateCurrent(voltdiff, value)
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), -current)
	value.SetFloat64(16, current) // 记录支路电流供输出
}

func ledSafeExp(x float64) float64 {
	const maxExpArg = 700.0
	if x > maxExpArg {
		x = maxExpArg
	} else if x < -maxExpArg {
		x = -maxExpArg
	}
	return math.Exp(x)
}

func ledLimitStep(vnew, vold float64, time mna.Time, value element.NodeFace) float64 {
	vscale := value.GetFloat64(6)
	if vscale == 0 {
		return vnew
	}
	vcrit := value.GetFloat64(11)

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
	}
	return vnew
}

func ledDoStep(mna mna.Mna, time mna.Time, value element.NodeFace, voltdiff float64) {
	leakage := value.GetFloat64(13)
	vdcoef := value.GetFloat64(7)

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

	eval := ledSafeExp(voltdiff * vdcoef)
	geq := vdcoef*leakage*eval + gmin
	nc := (eval-1)*leakage - geq*voltdiff
	mna.StampAdmittance(value.GetNodesInternal(0), value.GetNodes(1), geq)
	mna.StampCurrentSource(value.GetNodesInternal(0), value.GetNodes(1), nc)
}

func ledCalculateCurrent(voltdiff float64, value element.NodeFace) float64 {
	leakage := value.GetFloat64(13)
	vdcoef := value.GetFloat64(7)
	return leakage * (ledSafeExp(voltdiff*vdcoef) - 1)
}
