// uart.go：UART 元件的注册、实例配置（Base）与输出引脚电压源加盖（Stamp）。
package serial

import (
	"strings"

	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
)

// uartNodeType 是 UART 的 NodeType（K4 号段：48）。
const uartNodeType element.NodeType = 48

// UART 的引脚下标（K2：tx = 输出、rx = 输入、intr = 输出，均 PinBoolean）。
const (
	pinTx = iota
	pinRx
	pinIntr
)

// UART 的电压源下标（Voltage 声明顺序："tx"、"intr"）。
const (
	volTx = iota
	volIntr
)

// UartType 是 UART 元件注册后的类型号（0 值以外的调用方请用本变量，不硬编码 48）。
var UartType = element.AddElement(uartNodeType, &Uart{Config: &element.Config{
	Name:       "UART",
	Pin:        element.SetPin(element.PinBoolean, "tx", "rx", "intr"),
	ValueInit:  uartValueInit(),
	ValueName:  uartValueNames,
	OrigValue:  uartRuntimeSlots,
	Voltage:    []string{"tx", "intr"},
	EventSlots: uartEventSlots,
}})

// Uart 是 TTL 电平 UART 元件（8N1，网表类型名 UART）。
type Uart struct{ *element.Config }

// Base 按实例生成配置：引脚固定 tx/rx/intr，两个事件名槽按实例名填充
// （网表未给 ackEvent 时用 "<实例名>.ACK"；入站字节事件名恒为 "<实例名>.TX"）。
func (u *Uart) Base(elem ast.ElementNode) *element.Config {
	name := elem.InstanceName
	if name == "" {
		name = "UART" + elem.ID
	}
	cfg := u.Config
	values := make([]any, len(cfg.ValueInit))
	copy(values, cfg.ValueInit)
	values[slotAckEvent] = ackEventOf(elem, name)
	values[slotRxEventName] = name + ".TX"
	return &element.Config{
		Name:       cfg.Name,
		Pin:        element.SetPin(element.PinBoolean, "tx", "rx", "intr"),
		ValueInit:  values,
		ValueName:  cfg.ValueName,
		OrigValue:  cfg.OrigValue,
		Voltage:    cfg.Voltage,
		EventSlots: cfg.EventSlots,
	}
}

// ackEventOf 返回 ACK 事件名：网表第 2 个参数非空时用它，否则用 "<实例名>.ACK"。
func ackEventOf(elem ast.ElementNode, name string) string {
	if len(elem.Values) > 1 {
		if s := strings.TrimSpace(elem.Values[1].Value); s != "" {
			return s
		}
	}
	return name + ".ACK"
}

// Stamp 为 tx/intr 两根输出引脚建立电压源约束（元件唯一允许的输出方式）。
// 初值取节点当前电压（与 element/logic.Gate 同口径，避免 t=0 的虚假瞬态）；
// 真实电平由 DoStep 每步经 digital.DriveNow 写入。
func (u *Uart) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	if m == nil || value == nil {
		return
	}
	txNode := value.GetNodes(pinTx)
	intrNode := value.GetNodes(pinIntr)
	m.StampVoltageSource(txNode, mna.Gnd, value.GetVoltSource(volTx), m.GetNodeVoltage(txNode))
	m.StampVoltageSource(intrNode, mna.Gnd, value.GetVoltSource(volIntr), m.GetNodeVoltage(intrNode))
}
