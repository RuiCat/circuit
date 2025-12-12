package base

import "circuit/mna"

// Voltage 电压源设置
type Voltage struct{ Base }

func (voltage *Voltage) New() {
	voltage.ElementConfigBase = &mna.ElementConfigBase{
		Pin:     []string{"v+", "v-"},
		Value:   []any{float64(5)}, // 基础电压: 0:v5
		Voltage: []string{"v"},
	}
}
func (voltage *Voltage) Init() mna.ValueMNA {
	return mna.NewElementBase(voltage.ElementConfigBase)
}
func (Voltage) Stamp(mna mna.MNA, base mna.ValueMNA) {
	mna.StampVoltageSource(base.Nodes(0), base.Nodes(1), base.VoltSource(0), base.GetFloat64(0))
}
