package element

import (
	"circuit/element/capacitor"
	"circuit/element/diode"
	"circuit/element/inductor"
	"circuit/element/opamp"
	"circuit/element/resistor"
	"circuit/element/transistor"
	"circuit/element/vcc"
	"circuit/types"
)

// init 初始化
func init() {
	isError(types.ElementRegister(capacitor.Type, "C", &capacitor.Config{}))
	isError(types.ElementRegister(inductor.Type, "L", &inductor.Config{}))
	isError(types.ElementRegister(opamp.Type, "OpAmp", &opamp.Config{}))
	isError(types.ElementRegister(resistor.Type, "R", &resistor.Config{}))
	isError(types.ElementRegister(vcc.Type, "V", &vcc.Config{}))
	isError(types.ElementRegister(diode.Type, "D", &diode.Config{}))
	isError(types.ElementRegister(transistor.Type, "T", &transistor.Config{}))
}
func isError(err error) {
	if err != nil {
		panic(err)
	}
}
