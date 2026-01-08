package base

import (
	"circuit/element"
	"circuit/mna"
)

// ResistorType 定义元件
var ResistorType element.NodeType = element.AddElement(6, &Resistor{
	&element.Config{
		Name:      "r",                                               // 元件名称，网表文件中使用的标识符
		Pin:       element.SetPin(element.PinLowVoltage, "r1", "r2"), // 引脚名称，电阻有两个引脚
		ValueInit: []any{float64(10000)},                             // 初始化数据：默认电阻值为10kΩ
	},
})

// Resistor 电阻元件结构体，继承element.Config
// 实现电阻元件的配置和行为，包括MNA矩阵加盖操作
type Resistor struct{ *element.Config }

// Stamp 电阻元件的MNA矩阵加盖操作
// 将电阻的电导贡献添加到MNA矩阵中，实现电阻的线性模型
// 参数mna: MNA求解器接口，用于访问和修改MNA矩阵
// 参数time: 仿真时间接口，当前未使用（电阻是线性时不变元件）
// 参数value: 电阻元件节点接口，用于获取电阻值和节点连接信息
func (Resistor) Stamp(mna mna.MNA, time mna.Time, value element.NodeFace) {
	mna.StampImpedance(value.GetNodes(0), value.GetNodes(1), value.GetFloat64(0))
}
