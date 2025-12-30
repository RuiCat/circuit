package element

import (
	"circuit/mna"
	"circuit/utils"
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
	EleType utils.EventType
	Config  mna.ElementConfig
	Element mna.Element
}

// NewElement 创建
func NewElement(t utils.EventType, ele interface {
	mna.ElementConfig
	mna.Element
}) *Element {
	ele.New()
	return &Element{EleType: t, Config: ele, Element: ele}
}

// Type 元件类型
func (ele *Element) Type() utils.EventType {
	return ele.EleType
}

// Callback 转发事件
func (ele *Element) Callback(event utils.EventValue) {
	if val, ok := event.Get().(ElementValue); ok {
		switch event.Mark() {
		case utils.MarkStartIteration:
			ele.Element.StartIteration(val.MNA, val.Base)
		case utils.MarkStamp:
			ele.Element.Stamp(val.MNA, val.Base)
		case utils.MarkDoStep:
			ele.Element.DoStep(val.MNA, val.Base)
		case utils.MarkCalculateCurrent:
			ele.Element.CalculateCurrent(val.MNA, val.Base)
		case utils.MarkStepFinished:
			ele.Element.StepFinished(val.MNA, val.Base)
		case utils.MarkReset:
			ele.Config.Reset(val.Base)
		case utils.MarkCirLoad:
			ele.Config.CirLoad(val.Base)
		case utils.MarkCirExport:
			ele.Config.CirExport(val.Base)
		}
	}
}

// EventValue 创建事件传递值
func (ele *Element) EventValue() utils.EventValue {
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
	types     utils.EventType
	Value     ElementValue
	EventMark utils.EventMark
}

// Type 元件类型
func (val *EventValue) Type() utils.EventType {
	return val.types
}

// Mark 事件标记
func (val *EventValue) Mark() utils.EventMark {
	return val.EventMark
}

// SetMark 事件标记
func (val *EventValue) SetMark(mark utils.EventMark) {
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
