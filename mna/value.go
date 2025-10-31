package mna

import (
	"circuit/graph"
	"circuit/mna/mat"
	"circuit/types"
)

// Value 元件内部索引
type Value struct {
	List    [][2]int         // 下标索引
	Current mat.UpdateVector // 电流
}

// NewValue 创建
func NewValue(graph *graph.Graph) *Value {
	n := 0
	value := new(Value)
	value.List = make([][2]int, len(graph.ElementList))
	m := len(graph.ElementList)
	for i := range m {
		ele := graph.ElementList[i]
		value.List[i][0] = n
		value.List[i][1] = len(ele.Nodes)
		n += len(ele.Nodes)
	}
	value.Current = mat.NewUpdateVector(mat.NewDenseVector(n))
	return value
}

// GetPinCurrent 返回引脚电流
func (value *Value) GetPinCurrent(id types.ElementID, pin int) float64 {
	if id >= 0 && id < len(value.List) && pin < value.List[id][1] {
		return value.Current.Get(value.List[id][0] + pin)
	}
	return 0
}

// SetPinCurrent 设置引脚电流
func (value *Value) SetPinCurrent(id types.ElementID, pin int, i float64) {
	if id >= 0 && id < len(value.List) && pin < value.List[id][1] {
		value.Current.Set(value.List[id][0]+pin, i)
	}
}

// Update 更新
func (value *Value) Update() {
	value.Current.Update()
}

// Rollback 回溯
func (value *Value) Rollback() {
	value.Current.Rollback()
}

// 所有元件引脚数量
func (value *Value) GetNumPin() int {
	return value.Current.Length()
}
