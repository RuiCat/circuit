package mna

import (
	"circuit/graph"
	"circuit/mna/mat"
	"circuit/types"
)

// Value 元件内部索引
type Value struct {
	Current     mat.UpdateVector // 电流
	CurrentList [][2]int         // 下标索引
	Value       mat.UpdateVector // 内部数据
	ValueBase   mat.Vector       // 内部数据
	ValueList   [][2]int         // 下标索引
}

// NewValue 创建
func NewValue(graph *graph.Graph) *Value {
	value := new(Value)
	value.CurrentList = make([][2]int, len(graph.ElementList))
	value.ValueList = make([][2]int, len(graph.ElementList))
	m := len(graph.ElementList)
	n, v := 0, 0
	for i := range m {
		ele := graph.ElementList[i]
		value.CurrentList[i][0] = n
		value.CurrentList[i][1] = len(ele.Nodes)
		n += value.CurrentList[i][1]
		value.ValueList[i][0] = v
		value.ValueList[i][1] = ele.GetInternalValueCount()
		v += value.ValueList[i][1]
	}
	value.ValueBase = mat.NewDenseVector(v)
	value.Value = mat.NewUpdateVector(value.ValueBase)
	value.Current = mat.NewUpdateVector(mat.NewDenseVector(n))
	return value
}

// GetPinCurrent 返回引脚电流
func (value *Value) GetPinCurrent(id types.ElementID, pin int) float64 {
	if id >= 0 && id < len(value.CurrentList) && pin < value.CurrentList[id][1] {
		return value.Current.Get(value.CurrentList[id][0] + pin)
	}
	return 0
}

// SetPinCurrent 设置引脚电流
func (value *Value) SetPinCurrent(id types.ElementID, pin int, i float64) {
	if id >= 0 && id < len(value.CurrentList) && pin < value.CurrentList[id][1] {
		value.Current.Set(value.CurrentList[id][0]+pin, i)
	}
}

// GetValue 返回内部数据
func (value *Value) GetValue(id types.ElementID, n int) float64 {
	if id >= 0 && id < len(value.ValueList) && n < value.ValueList[id][1] {
		return value.Value.Get(value.ValueList[id][0] + n)
	}
	return 0
}

// SetValue 设置内部数据
func (value *Value) SetValue(id types.ElementID, n int, v float64) {
	if id >= 0 && id < len(value.ValueList) && n < value.ValueList[id][1] {
		value.Value.Set(value.ValueList[id][0]+n, v)
	}
}

// SetValueBase 设置内部底层
func (value *Value) SetValueBase(id types.ElementID, n int, v float64) {
	if id >= 0 && id < len(value.ValueList) && n < value.ValueList[id][1] {
		value.ValueBase.Set(value.ValueList[id][0]+n, v)
	}
}

// Reset 重置
func (value *Value) Reset() {
	value.Value.Clear()
	value.Current.Clear()
}

// Update 更新
func (value *Value) Update() {
	value.Value.Update()
	value.Current.Update()
}

// Rollback 回溯
func (value *Value) Rollback() {
	value.Value.Rollback()
	value.Current.Rollback()
}

// 所有元件引脚数量
func (value *Value) GetNumPin() int {
	return value.Current.Length()
}
