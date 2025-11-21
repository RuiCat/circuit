package element

import (
	"circuit/mna"
	"circuit/utils"
)

const (
	MarkStartIteration utils.EventMark = iota
	MarkStamp
	MarkDoStep
	MarkCalculateCurrent
	MarkStepFinished
)

// ElementValue 元件值
type ElementValue struct {
	MNA  mna.MNA
	Base *mna.ElementBase
}

// Element 元件实现接口
type Element struct {
	Config    mna.ElementConfig
	Element   mna.Element
	EventType utils.EventType
	utils.Event
}

func (ele *Element) Type() utils.EventType {
	return ele.EventType
}
func (ele *Element) Callback(event utils.EventValue) {
	val := event.Get().(ElementValue)
	switch event.Mark() {
	case MarkStartIteration:
		ele.Element.StartIteration(val.MNA, val.Base)
	case MarkStamp:
		ele.Element.Stamp(val.MNA, val.Base)
	case MarkDoStep:
		ele.Element.DoStep(val.MNA, val.Base)
	case MarkCalculateCurrent:
		ele.Element.CalculateCurrent(val.MNA, val.Base)
	case MarkStepFinished:
		ele.Element.StepFinished(val.MNA, val.Base)
	}
}
func (ele *Element) EventValue() utils.EventValue {
	return &EventValue{
		types: ele.EventType,
	}
}

type EventValue struct {
	types     utils.EventType
	Value     ElementValue
	EventMark utils.EventMark
}

func (val *EventValue) Get() (value any) {
	return val.Value
}
func (val *EventValue) Set(value any) {
	val.Value = value.(ElementValue)
}
func (val *EventValue) Mark() utils.EventMark {
	return val.EventMark
}

func (val *EventValue) Type() utils.EventType {
	return val.types
}
