package base

import (
	"circuit/element"
	"circuit/mna"
)

// CapacitorType 定义元件
var CapacitorType element.NodeType = element.AddElement(0, &Capacitor{
	&element.Config{
		Name:      "c",
		Pin:       []string{"c1", "c2"},
		ValueInit: []any{float64(1e-6), 0.0, 0.0, 0.0, 0.0}, // 0:C 1:G_eq 2:I_hist 3:V_diff 4:I_cap
		Current:   []int{4},
		OrigValue: []int{1, 2, 3, 4},
	},
})

// Capacitor 电容
type Capacitor struct{ *element.Config }

func (Capacitor) StartIteration(mna mna.MNA, time mna.Time, value element.NodeFace) {
	dt := time.TimeStep()
	c := value.GetFloat64(0)
	v_prev := value.GetFloat64(3)
	I_hist := (2*c/dt)*v_prev + value.GetFloat64(4)
	value.SetFloat64(2, I_hist)
}

func (Capacitor) Stamp(mna mna.MNA, time mna.Time, value element.NodeFace) {
	dt := time.TimeStep()
	c := value.GetFloat64(0)
	if dt <= 0 || c <= 0 {
		return
	}
	G_eq := 2 * c / dt
	value.SetFloat64(1, G_eq)
	mna.StampConductance(value.GetNodes(0), value.GetNodes(1), G_eq)
}

func (Capacitor) DoStep(mna mna.MNA, time mna.Time, value element.NodeFace) {
	I_hist := value.GetFloat64(2)
	mna.StampCurrentSource(value.GetNodes(1), value.GetNodes(0), I_hist)
}

func (Capacitor) CalculateCurrent(mna mna.MNA, time mna.Time, value element.NodeFace) {
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	v_diff := v1 - v2
	value.SetFloat64(3, v_diff)
	G_eq := value.GetFloat64(1)
	I_hist := value.GetFloat64(2)
	I_cap := G_eq*v_diff - I_hist
	value.SetFloat64(4, I_cap)
}
