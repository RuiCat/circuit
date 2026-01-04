package base

import (
	"circuit/element"
	"circuit/mna"
)

// CurrentSourceType 定义元件
var CurrentSourceType element.NodeType = element.AddElement(1, &CurrentSource{
	&element.Config{
		Name:      "i",
		Pin:       []string{"i+", "i-"},
		ValueInit: []any{float64(0.01)}, // 基础电流: 0:0.01A
	},
})

// CurrentSource 电流源
type CurrentSource struct{ *element.Config }

func (CurrentSource) Stamp(mna mna.MNA, time mna.Time, value element.NodeFace) {
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), value.GetFloat64(0))
}
