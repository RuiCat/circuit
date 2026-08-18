// 油路惯性元件(液感)实现，模拟油液惯性效应。
package hydraulic

import (
	"circuit/element"
	"circuit/mna"
)

// HLType 液感元件类型标识(NodeType=31)，网表"HL<name> <n1,n2> [L]"。
var HLType element.NodeType = element.AddElement(31, &HL{
	&element.Config{
		Name:      "hl",
		Pin:       element.SetPin(element.PinHydraulic, "hl1", "hl2"),
		ValueInit: []any{float64(1e8), 0.0, 0.0, 0.0},
		ValueName: []string{"L", "I_init", "G_eq", "I_hist"},
		Current:   []int{1},
		OrigValue: []int{2, 3},
		Flags:     element.FlagReactive,
	},
})

// HLInertia 惯性/液感元件。模型: Δp = L·dQ/dt，与电感同构。
type HL struct{ *element.Config }

// StartIteration 液感的迭代初始化
// 计算历史流量 I_hist = v_diff * G_eq + I_hist_prev
func (HL) StartIteration(mna mna.Mna, time mna.Time, value element.NodeFace) {
	dt := time.TimeStep()
	if dt <= 0 {
		return
	}
	G_eq := value.GetFloat64(2)
	if G_eq > 0 {
		v1 := mna.GetNodeVoltage(value.GetNodes(0))
		v2 := mna.GetNodeVoltage(value.GetNodes(1))
		voltdiff := v1 - v2
		I_hist := voltdiff*G_eq + value.GetFloat64(3)
		value.SetFloat64(3, I_hist)
	}
}

// Stamp 液感的MNA矩阵加盖操作
// 计算等效流导 G_eq，根据迭代收敛情况选择梯形积分法或后向欧拉法
func (HL) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	inductance := value.GetFloat64(0)
	dt := time.TimeStep()
	if dt <= 0 || inductance <= 0 {
		return
	}
	var G_eq float64
	if time.GoodIterations() > 0 {
		G_eq = dt / (2 * inductance)
	} else {
		G_eq = dt / inductance
	}
	value.SetFloat64(2, G_eq)
	mna.StampAdmittance(value.GetNodes(0), value.GetNodes(1), G_eq)
}

// DoStep 液感的步进计算
// 将历史流量 I_hist 加盖到MNA右侧向量中
func (HL) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	I_hist := value.GetFloat64(3)
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), I_hist)
}

// CalculateCurrent 计算液感的流量
// 使用 I = G_eq * v_diff - I_hist
func (HL) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	G_eq := value.GetFloat64(2)
	if G_eq > 0 {
		v1 := mna.GetNodeVoltage(value.GetNodes(0))
		v2 := mna.GetNodeVoltage(value.GetNodes(1))
		voltdiff := v1 - v2
		I_hist := value.GetFloat64(3)
		current := G_eq*voltdiff - I_hist
		value.SetFloat64(1, current)
	}
}

// AddDerivative 向导数向量 der 累加液感的状态导数贡献
// 液感储存的"磁通量"导数 dI_L/dt = V_diff / L，通过伴模型影响节点压力导数
func (HL) AddDerivative(mna mna.Mna, time mna.Time, value element.NodeFace, der []float64) {
	L := value.GetFloat64(0)
	if L <= 0 {
		return
	}
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	v_diff := v1 - v2
	G_eq := value.GetFloat64(2)
	if G_eq > 0 {
		n1 := int(value.GetNodes(0))
		n2 := int(value.GetNodes(1))
		dIdt := v_diff / L
		dVdt_contrib := dIdt / G_eq
		if n1 >= 0 && n1 < mna.GetNodeNum() {
			der[n1] += -dVdt_contrib
		}
		if n2 >= 0 && n2 < mna.GetNodeNum() {
			der[n2] += dVdt_contrib
		}
	}
}
