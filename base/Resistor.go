package base

import (
	"circuit/mna"
)

// Resistor 电阻
type Resistor struct{ Base }

func (resistor *Resistor) New() {
	resistor.ElementConfigBase = &mna.ElementConfigBase{
		Pin:       []string{"r1", "r2"},
		ValueInit: []any{float64(10000)}, // 基础电阻: 0:10kΩ
	}
}
func (resistor *Resistor) Init() mna.ValueMNA {
	return mna.NewElementBase(resistor.ElementConfigBase)
}
func (Resistor) Stamp(mna mna.MNA, base mna.ValueMNA) {
	r := base.GetFloat64(0)
	if r > 0 {
		mna.StampResistor(base.Nodes(0), base.Nodes(1), r)
	}
}
