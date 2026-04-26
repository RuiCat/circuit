package base

import (
	"circuit/element"
	"circuit/mna"
)

// CurrentSourceType 定义元件
var CurrentSourceType element.NodeType = element.AddElement(1, &CurrentSource{
	&element.Config{
		Name:      "i",
		Pin:       element.SetPin(element.PinLowVoltage, "i+", "i-"),
		ValueInit: []any{float64(0.01)}, // 基础电流: 0:0.01A
		ValueName: []string{"I"},
		Current:   []int{0},
	},
})

// CurrentSource 独立电流源元件结构体，继承element.Config
// 实现独立电流源的MNA加盖行为
type CurrentSource struct{ *element.Config }

// Stamp 电流源的MNA矩阵加盖操作
// 将电流值 I 直接加盖到MNA右侧向量中，方向从 n1 流向 n2
func (CurrentSource) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), value.GetFloat64(0))
}
