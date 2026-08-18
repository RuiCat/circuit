// Package base 提供电路仿真所有基础元件的实现。包括无源元件(电阻、电容、电感)、有源元件(二极管、三极管、运放)、电源(直流/交流/脉冲/噪声)、电机、变压器、逻辑门、触发器和交互式元件等。
package passive

import (
	"circuit/element"
	"circuit/mna"
	"log"
	"math"
)

// ResistorType 定义元件
var ResistorType element.NodeType = element.AddElement(6, &Resistor{
	&element.Config{
		Name:      "r",                                               // 元件名称，网表文件中使用的标识符
		Pin:       element.SetPin(element.PinLowVoltage, "r1", "r2"), // 引脚名称，电阻有两个引脚
		ValueInit: []any{float64(10000), 0.0},                        // 初始化数据：默认电阻值为10kΩ；索引1为电流槽
		ValueName: []string{"R", "I"},
		Current:   []int{1},
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
func (Resistor) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	r := value.GetFloat64(0)
	if math.Abs(r) <= 1e-9 {
		log.Printf("Resistor: 阻值 %.3e 近似为零，将被视为 1nΩ 短路路径", r)
	}
	mna.StampImpedance(value.GetNodes(0), value.GetNodes(1), r)
}

// CalculateCurrent 按欧姆定律 I=(V1-V2)/R 计算流经电阻的电流（正方向为引脚0→引脚1）。
func (Resistor) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	r := value.GetFloat64(0)
	if math.Abs(r) <= 1e-9 {
		value.SetFloat64(1, 0)
		return
	}
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	value.SetFloat64(1, (v1-v2)/r)
}
