package element

import (
	"circuit/mna"
	"circuit/utils/element/event"
)

// ElementValue 元件值
type ElementValue struct {
	MNA  mna.MNA
	Base mna.ValueMNA
}

// ElementBase 得到底层
func (ele *ElementValue) ElementBase() *mna.ElementBase {
	return ele.Base.Base()
}

// Element 元件实现接口
type Element struct {
	EleType event.EventType
	Config  mna.ElementConfig
	Element mna.Element
}

// NewElement 创建
func NewElement(t event.EventType, ele interface {
	mna.ElementConfig
	mna.Element
}) *Element {
	ele.New()
	return &Element{EleType: t, Config: ele, Element: ele}
}

// Type 元件类型
func (ele *Element) Type() event.EventType {
	return ele.EleType
}

// Callback 转发事件
func (ele *Element) Callback(eve event.EventValue) {
	if val, ok := eve.Get().(ElementValue); ok {
		switch eve.Mark() {
		case event.MarkStartIteration:
			ele.Element.StartIteration(val.MNA, val.Base)
		case event.MarkStamp:
			ele.Element.Stamp(val.MNA, val.Base)
		case event.MarkDoStep:
			ele.Element.DoStep(val.MNA, val.Base)
		case event.MarkCalculateCurrent:
			ele.Element.CalculateCurrent(val.MNA, val.Base)
		case event.MarkStepFinished:
			ele.Element.StepFinished(val.MNA, val.Base)
		case event.MarkReset:
			ele.Config.Reset(val.Base)
		case event.MarkCirLoad:
			ele.Config.CirLoad(val.Base)
		case event.MarkCirExport:
			ele.Config.CirExport(val.Base)
		}
	}
}

// EventValue 创建事件传递值
func (ele *Element) EventValue() event.EventValue {
	// 初始化
	return &EventValue{
		types: ele.EleType,
		Value: ElementValue{
			Base: ele.Config.Init(),
		},
	}
}

// EventValue 封装事件传递过程值
type EventValue struct {
	types     event.EventType
	Value     ElementValue
	EventMark event.EventMark
}

// Type 元件类型
func (val *EventValue) Type() event.EventType {
	return val.types
}

// Mark 事件标记
func (val *EventValue) Mark() event.EventMark {
	return val.EventMark
}

// SetMark 事件标记
func (val *EventValue) SetMark(mark event.EventMark) {
	val.EventMark = mark
}

// Get 得到值
func (val *EventValue) Get() (value any) {
	return val.Value
}

// Set 设置值
func (val *EventValue) Set(value any) {
	val.Value = value.(ElementValue)
}
