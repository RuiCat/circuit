package element

import (
	"circuit/element/capacitor"
	"circuit/element/gnd"
	"circuit/element/hegh"
	"circuit/element/inductor"
	"circuit/element/resistor"
	"circuit/element/vcc"
	"circuit/types"
)

// init 初始化
func init() {
	isError(types.ElementRegister(capacitor.Type, "C", &capacitor.Config{}))
	isError(types.ElementRegister(gnd.Type, "G", &gnd.Config{}))
	isError(types.ElementRegister(hegh.Type, "Hegh", &hegh.Config{}))
	isError(types.ElementRegister(inductor.Type, "L", &inductor.Config{}))
	isError(types.ElementRegister(resistor.Type, "R", &resistor.Config{}))
	isError(types.ElementRegister(vcc.Type, "V", &vcc.Config{}))
}
func isError(err error) {
	if err != nil {
		panic(err)
	}
}
