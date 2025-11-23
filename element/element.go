package element

import (
	"circuit/mna"
	"circuit/utils"
)

// 接口回调类型
const (
	MarkReset            utils.EventMark = iota // 元件重置
	MarkStartIteration                          // 步长迭代开始
	MarkStamp                                   // 加盖线性贡献
	MarkDoStep                                  // 执行仿真
	MarkCalculateCurrent                        // 电流计算
	MarkStepFinished                            // 步长迭代结束
	MarkCirLoad                                 // 网表文件写入值
	MarkCirExport                               // 网表文件导出值
)

// ElementValue 元件值
type ElementValue struct {
	MNA  mna.MNA
	Base mna.ValueMNA
}

// Element 元件实现接口
type Element struct {
	EleType utils.EventType
	Config  mna.ElementConfig
	Element mna.Element
}

// Type 元件类型
func (ele *Element) Type() utils.EventType {
	return ele.EleType
}

// Callback 转发事件
func (ele *Element) Callback(event utils.EventValue) {
	if val, ok := event.Get().(ElementValue); ok {
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
		case MarkReset:
			ele.Config.Reset(val.Base)
		case MarkCirLoad:
			ele.Config.CirLoad(val.Base)
		case MarkCirExport:
			ele.Config.CirExport(val.Base)
		}
	}
}

// EventValue 创建事件传递值
func (ele *Element) EventValue() utils.EventValue {
	// 初始化
	event := &EventValue{
		types: ele.EleType,
		Value: ElementValue{
			Base: ele.Config.Init(),
		},
	}
	// 初始化引脚
	base := event.Value.Base.Base()
	base.Graph.Nodes = make([]mna.NodeID, base.PinNum())
	base.Graph.VoltSource = make([]mna.NodeID, base.VoltageNum())
	base.Graph.NodesInternal = make([]mna.NodeID, base.InternalNum())
	return event
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
