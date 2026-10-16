// deviceAnd.go：TTLAND —— 74LS08 系列二输入与门（单门元件）。
//
// 引脚：a、b（输入，PinLowVoltage）、y（输出）。型号默认 74LS08，可由网表第 1 个参数覆盖。
// 与 deviceNand.go 的唯一差别是判决函数；电气行为共用 spec.go 的参数表与 gate.go 的模型。
package ttl

import (
	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
)

// TtlAndType 是 TTLAND 注册后的类型号（NodeType 52，K4 号段）。
var TtlAndType = element.AddElement(52, &ttlAnd{gateBase: gateBase{&element.Config{
	Name:      "TTLAND",
	Pin:       element.SetPin(element.PinLowVoltage, "a", "b", "y"),
	ValueInit: gateValueInit("74LS08"),
	ValueName: gateValueNames(),
	OrigValue: gateRuntimeSlots,
	Voltage:   []string{"v_y"},
	Internal:  []string{"int_y"},
	Flags:     element.FlagNonlinear,
}}})

// ttlAnd 是二输入与门元件（TTL 电气级）。
type ttlAnd struct{ gateBase }

// Base 按网表参数生成本实例配置（型号可覆盖，默认 74LS08）。
func (g *ttlAnd) Base(elem ast.ElementNode) *element.Config {
	return gateConfigWithModel(g.Config, elem, "74LS08")
}

// DoStep 判决 y = a AND b，其余交给门模型。
func (g *ttlAnd) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	driveGate(m, t, value, 2, []int{0, 1}, func(in []bool) bool { return in[0] && in[1] })
}

// Reset 复位运行态槽（C5）。
func (g *ttlAnd) Reset(value element.NodeFace) { resetSlots(value, gateRuntimeSlots) }
