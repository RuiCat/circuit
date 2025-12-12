package base

import "circuit/mna"

// CurrentSource 电流源
type CurrentSource struct{ Base }

func (currentSource *CurrentSource) New() {
	currentSource.ElementConfigBase = &mna.ElementConfigBase{
		Pin:   []string{"i+", "i-"},
		Value: []any{float64(0.01)}, // 基础电流: 0:0.01A
	}
}
func (currentSource *CurrentSource) Init() mna.ValueMNA {
	return mna.NewElementBase(currentSource.ElementConfigBase)
}
func (CurrentSource) Stamp(mna mna.MNA, base mna.ValueMNA) {
	mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), base.GetFloat64(0))
}
