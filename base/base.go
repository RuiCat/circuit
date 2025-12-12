package base

import (
	"circuit/mna"
)

// Base 底层结构
type Base struct {
	*mna.ElementConfigBase
}

func (Base) StartIteration(mna mna.MNA, base mna.ValueMNA)   {}
func (Base) Stamp(mna mna.MNA, base mna.ValueMNA)            {}
func (Base) DoStep(mna mna.MNA, base mna.ValueMNA)           {}
func (Base) CalculateCurrent(mna mna.MNA, base mna.ValueMNA) {}
func (Base) StepFinished(mna mna.MNA, base mna.ValueMNA)     {}
func (Base) Init() mna.ValueMNA                              { return nil }
func (Base) Reset(base mna.ValueMNA)                         {}
func (Base) CirLoad(mna.ValueMNA)                            {}
func (Base) CirExport(mna.ValueMNA)                          {}
