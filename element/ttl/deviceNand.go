// deviceNand.go：TTLNAND —— 74LS00 系列二输入与非门（单门元件）。
//
// 引脚：a、b（输入，PinLowVoltage）、y（输出）。型号默认 74LS00，可由网表第 1 个参数覆盖
// （K4：型号只作参数值；参数不完整/未知型号时元件**不收敛**，由引擎报错，绝不静默建模）。
// 电气行为（延时 tpd、输入电流 IIH/IIL、输出 Ron 与 VOH/VOL 预算）全部来自 spec.go 的参数表。
package ttl

import (
	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
)

// TtlNandType 是 TTLNAND 注册后的类型号（NodeType 50，K4 号段）。
var TtlNandType = element.AddElement(50, &ttlNand{gateBase: gateBase{&element.Config{
	Name:      "TTLNAND",
	Pin:       element.SetPin(element.PinLowVoltage, "a", "b", "y"),
	ValueInit: gateValueInit("74LS00"),
	ValueName: gateValueNames(),
	OrigValue: gateRuntimeSlots,
	Voltage:   []string{"v_y"},
	Internal:  []string{"int_y"},
	Flags:     element.FlagNonlinear,
}}})

// ttlNand 是二输入与非门元件（TTL 电气级）。
type ttlNand struct{ gateBase }

// Base 按网表参数生成本实例配置（型号可覆盖，默认 74LS00）。
func (g *ttlNand) Base(elem ast.ElementNode) *element.Config {
	return gateConfigWithModel(g.Config, elem, "74LS00")
}

// DoStep 判决 y = NOT(a AND b)，其余交给门模型（步长自检、惯性延时、盖章）。
func (g *ttlNand) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	driveGate(m, t, value, 2, []int{0, 1}, func(in []bool) bool { return !(in[0] && in[1]) })
}

// Reset 复位运行态槽（C5）。
func (g *ttlNand) Reset(value element.NodeFace) { resetSlots(value, gateRuntimeSlots) }
