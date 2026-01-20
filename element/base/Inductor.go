package base

import (
	"circuit/element"
	"circuit/mna"
)

// InductorType 定义元件
var InductorType element.NodeType = element.AddElement(3, &Inductor{
	&element.Config{
		Name:      "l",
		Pin:       element.SetPin(element.PinLowVoltage, "l1", "l2"),
		ValueInit: []any{float64(1e-3), 0.0, 0.0, 0.0},
		ValueName: []string{"L", "I_init", "G_eq", "I_hist"},
		Current:   []int{0},
		OrigValue: []int{2, 3},
	},
})

// Inductor 电感器
type Inductor struct{ *element.Config }

func (Inductor) StartIteration(mna mna.Mna, time mna.Time, value element.NodeFace) {
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

func (Inductor) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	inductance := value.GetFloat64(0)
	dt := time.TimeStep()
	if dt <= 0 || inductance <= 0 {
		return
	}

	// 计算等效电导
	var G_eq float64
	if time.GoodIterations() > 0 { // 使用梯形积分法
		G_eq = dt / (2 * inductance)
	} else { // 使用后向欧拉法
		G_eq = dt / inductance
	}
	value.SetFloat64(2, G_eq)

	// 加盖电导贡献
	mna.StampAdmittance(value.GetNodes(0), value.GetNodes(1), G_eq)
}

func (Inductor) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	I_hist := value.GetFloat64(3)
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), I_hist)
}

func (Inductor) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
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
