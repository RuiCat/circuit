package base

import (
	"circuit/mna"
	"log"
)

// init 初始化
func init() {
	AddElement(CapacitorType, &Capacitor{})
	AddElement(CurrentSourceType, &CurrentSource{})
	AddElement(DiodeType, &Diode{})
	AddElement(InductorType, &Inductor{})
	AddElement(MotorType, &Motor{})
	AddElement(OpAmpType, &OpAmp{})
	AddElement(ResistorType, &Resistor{})
	AddElement(SwitchType, &Switch{})
	AddElement(TransformerType, &Transformer{})
	AddElement(TransistorType, &Transistor{})
	AddElement(VCVSType, &VCVS{})
	AddElement(VoltageType, &Voltage{})
}

// EventType 注册的事件类型标记
type ElementType uint

// ElementFace 元件接口
type ElementFace interface {
	mna.ElementConfig
	mna.Element
}

// ElementLitt 元件列表
var ElementLitt = map[ElementType]ElementFace{}

// AddElement 注册元件
func AddElement(eleType ElementType, face ElementFace) {
	if _, ok := ElementLitt[eleType]; ok {
		log.Fatalf("元件重复注册: %d", eleType)
	}
	face.New()
	ElementLitt[eleType] = face
}
