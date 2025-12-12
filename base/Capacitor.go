package base

import "circuit/mna"

// Capacitor 电容
type Capacitor struct{ Base }

func (capacitor *Capacitor) New() {
	capacitor.ElementConfigBase = &mna.ElementConfigBase{
		Pin:       []string{"c1", "c2"},
		Value:     []any{float64(1e-6), 0.0, 0.0, 0.0, 0.0}, // 0:C 1:G_eq 2:I_hist 3:V_diff 4:I_cap
		Current:   []int{4},
		OrigValue: []int{1, 2, 3, 4},
	}
}
func (capacitor *Capacitor) Init() mna.ValueMNA {
	return mna.NewElementBase(capacitor.ElementConfigBase)
}
func (Capacitor) StartIteration(mna mna.MNA, base mna.ValueMNA) {
	dt := base.TimeStep()
	c := base.GetFloat64(0)
	v_prev := base.GetFloat64(3)
	I_hist := (2*c/dt)*v_prev + base.GetFloat64(4)
	base.SetFloat64(2, I_hist)
}
func (Capacitor) Stamp(mna mna.MNA, base mna.ValueMNA) {
	dt := base.TimeStep()
	c := base.GetFloat64(0)
	if dt <= 0 || c <= 0 {
		return
	}
	G_eq := 2 * c / dt
	base.SetFloat64(1, G_eq)
	mna.StampConductance(base.Nodes(0), base.Nodes(1), G_eq)
}
func (Capacitor) DoStep(mna mna.MNA, base mna.ValueMNA) {
	I_hist := base.GetFloat64(2)
	mna.StampCurrentSource(base.Nodes(1), base.Nodes(0), I_hist)
}
func (Capacitor) CalculateCurrent(mna mna.MNA, base mna.ValueMNA) {
	v1 := mna.GetNodeVoltage(base.Nodes(0))
	v2 := mna.GetNodeVoltage(base.Nodes(1))
	v_diff := v1 - v2
	base.SetFloat64(3, v_diff)
	G_eq := base.GetFloat64(1)
	I_hist := base.GetFloat64(2)
	I_cap := G_eq*v_diff - I_hist
	base.SetFloat64(4, I_cap)
}
