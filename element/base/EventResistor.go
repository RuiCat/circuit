package base

import (
	"circuit/element"
	"circuit/mna"
	"math"
)

// EventResistorType 事件驱动可变电阻元件类型 (NodeType=14)
// 网表语法: VR<name> <n1, n2> [eventName, min_R, max_R]
// 例: VR1 [0, 1] [POT1, 100, 10000]
// 事件值由 Context.PushEvents 自动同步到 NodeValue[4]
var EventResistorType element.NodeType = element.AddElement(14, &EventResistor{
	&element.Config{
		Name: "VR",
		Pin:  element.SetPin(element.PinLowVoltage, "r1", "r2"),
		ValueInit: []any{
			"POT1",       // 0: eventName — 事件名称 (string，永不覆盖)
			float64(100), // 1: min_R — 最小电阻 (Ω)
			float64(1e4), // 2: max_R — 最大电阻 (Ω)
			float64(100), // 3: lastR — 上次电阻值
			float64(0),   // 4: eventValue — 事件值 (由 PushEvents 更新)
		},
		ValueName:  []string{"eventName", "min_R", "max_R", "lastR", "eventValue"},
		OrigValue:  []int{3},
		EventSlots: map[int]int{0: 4}, // nameIndex(0) → valueIndex(4): 事件名"POT1"→值写入索引4
	},
})

// EventResistor 事件驱动可变电阻
// 阻值 = min_R + eventValue*(max_R - min_R)
type EventResistor struct{ *element.Config }

// calcEventResistance 根据事件值和阻值范围计算当前电阻
// 事件值 0.0~1.0 线性映射到 minR~maxR，超出范围自动夹紧
func calcEventResistance(eventVal float64, minR float64, maxR float64) float64 {
	R := minR + eventVal*(maxR-minR)
	if R < minR {
		R = minR
	}
	if R > maxR {
		R = maxR
	}
	return R
}

// Stamp 读取事件值计算电阻并加盖
func (er *EventResistor) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	R := calcEventResistance(value.GetFloat64(4), value.GetFloat64(1), value.GetFloat64(2))
	value.SetFloat64(3, R)
	mna.StampImpedance(value.GetNodes(0), value.GetNodes(1), R)
}

// DoStep 检测阻值变化
func (er *EventResistor) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	newR := calcEventResistance(value.GetFloat64(4), value.GetFloat64(1), value.GetFloat64(2))
	if math.Abs(newR-value.GetFloat64(3)) > 1e-12 {
		time.NoConverged()
	}
}

// CalculateCurrent 根据当前阻值计算电流
func (er *EventResistor) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	R := value.GetFloat64(3)
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	if R > 0 {
		current := (v1 - v2) / R
		mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), -current)
	}
}
