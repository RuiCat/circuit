package element

import (
	"circuit/element/capacitor"
	"circuit/element/current"
	"circuit/element/diode"
	"circuit/element/inductor"
	"circuit/element/motor"
	"circuit/element/opamp"
	"circuit/element/resistor"
	sw "circuit/element/switch"
	"circuit/element/transformer"
	"circuit/element/transistor"
	"circuit/element/vcc"
	"circuit/element/vcvs"
	"circuit/types"
)

// init 初始化
func init() {
	isError(types.ElementRegister(capacitor.Type, "C", &capacitor.Config{}))
	isError(types.ElementRegister(current.Type, "I", &current.Config{}))
	isError(types.ElementRegister(inductor.Type, "L", &inductor.Config{}))
	isError(types.ElementRegister(opamp.Type, "OpAmp", &opamp.Config{}))
	isError(types.ElementRegister(resistor.Type, "R", &resistor.Config{}))
	isError(types.ElementRegister(sw.Type, "SW", &sw.Config{}))
	isError(types.ElementRegister(vcc.Type, "V", &vcc.Config{}))
	isError(types.ElementRegister(vcvs.Type, "VCVS", &vcvs.Config{}))
	isError(types.ElementRegister(diode.Type, "D", &diode.Config{}))
	isError(types.ElementRegister(transistor.Type, "T", &transistor.Config{}))
	isError(types.ElementRegister(transformer.Type, "Transformer", &transformer.Config{}))
	// 不同的电机
	isError(types.ElementRegister(motor.DCMotorType, "DCMotor", &motor.Config{Type: motor.DCMotor}))
	isError(types.ElementRegister(motor.ACInductionMotorType, "ACInductionMotor", &motor.Config{Type: motor.ACInductionMotor}))
	isError(types.ElementRegister(motor.PMSMType, "PMSM", &motor.Config{Type: motor.PMSM}))
	isError(types.ElementRegister(motor.StepperMotorType, "StepperMotor", &motor.Config{Type: motor.StepperMotor}))
	isError(types.ElementRegister(motor.SeparatelyExcitedMotorType, "SeparatelyExcitedMotor", &motor.Config{Type: motor.SeparatelyExcitedMotor}))
	isError(types.ElementRegister(motor.ShuntMotorType, "ShuntMotor", &motor.Config{Type: motor.ShuntMotor}))
	isError(types.ElementRegister(motor.SeriesMotorType, "SeriesMotor", &motor.Config{Type: motor.SeriesMotor}))
	isError(types.ElementRegister(motor.CompoundMotorType, "CompoundMotor", &motor.Config{Type: motor.CompoundMotor}))
}
func isError(err error) {
	if err != nil {
		panic(err)
	}
}
