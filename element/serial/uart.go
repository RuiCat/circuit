// uart.go：UART 元件的注册、按引脚数选择 3 针/13 针实例配置（Base）与输出引脚电压源加盖（Stamp）。
package serial

import (
	"strings"

	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
)

// uartNodeType 是 UART 的 NodeType（K4 号段：48）。
const uartNodeType element.NodeType = 48

// UART 的引脚下标。3 针形态只用 pinTx..pinIntr；13 针形态按 K1 固定追加
// D0..D7、RDY、ACK：0=tx、1=rx、2=intr、3..10=D0..D7、11=RDY、12=ACK。
const (
	pinTx   = 0
	pinRx   = 1
	pinIntr = 2
	pinD0   = 3
	pinD7   = 10
	pinRdy  = 11
	pinAck  = 12
)

// uartTapPinNum 是 13 针字节口形态的引脚数（K1 冻结）。
const uartTapPinNum = pinAck + 1

// UART 的电压源下标（Voltage 声明顺序）。3 针形态只有 volTx/volIntr；
// 13 针形态追加 D0..D7 与 RDY（共 11 个）。ACK 是输入脚，不配电压源。
const (
	volTx   = 0
	volIntr = 1
	volD0   = 2
	volD7   = 9
	volRdy  = 10
)

// uartPins3 与 uartVoltages3 是 3 针形态的引脚/电压源表（68/69 网表沿用此形态）。
var (
	uartPins3     = element.SetPin(element.PinBoolean, "tx", "rx", "intr")
	uartVoltages3 = []string{"tx", "intr"}
)

// uartPins13 与 uartVoltages13 是 13 针字节口形态的引脚/电压源表（K1 固定顺序）。
// 成对切换是 C2 的硬要求：只扩 Pin 或只扩 Voltage 都会让求解矩阵与引脚错位。
var (
	uartPins13 = element.SetPin(element.PinBoolean,
		"tx", "rx", "intr", "D0", "D1", "D2", "D3", "D4", "D5", "D6", "D7", "RDY", "ACK")
	uartVoltages13 = []string{"tx", "intr", "D0", "D1", "D2", "D3", "D4", "D5", "D6", "D7", "RDY"}
)

// UartType 是 UART 元件注册后的类型号（0 值以外的调用方请用本变量，不硬编码 48）。
// 注册表里的 Pin/Voltage 是 3 针缺省形态；13 针由 Base 按实例引脚数切换。
var UartType = element.AddElement(uartNodeType, &Uart{Config: &element.Config{
	Name:       "UART",
	Pin:        uartPins3,
	ValueInit:  uartValueInit(),
	ValueName:  uartValueNames,
	OrigValue:  uartRuntimeSlots,
	Voltage:    uartVoltages3,
	EventSlots: uartEventSlots,
}})

// Uart 是 TTL 电平 UART 元件（8N1，网表类型名 UART）。
type Uart struct{ *element.Config }

// Base 按实例生成配置：3 针形态为 tx/rx/intr + 2 个电压源；
// 网表给出 13 个及以上引脚时切换为 tx/rx/intr、D0..D7、RDY、ACK + 11 个电压源（K1）。
// 两个事件名槽按实例名填充（网表未给 ackEvent 时用 "<实例名>.ACK"；
// 入站字节事件名恒为 "<实例名>.TX"）。
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
	pins, voltages := uartPins3, uartVoltages3
	if len(elem.Pins) >= uartTapPinNum {
		// 13 针脚位固定，多余的引脚由 load 的 PinNum 校验忽略（不会移位）。
		pins, voltages = uartPins13, uartVoltages13
	}
	return &element.Config{
		Name:       cfg.Name,
		Pin:        pins,
		ValueInit:  values,
		ValueName:  cfg.ValueName,
		OrigValue:  cfg.OrigValue,
		Voltage:    voltages,
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

// Stamp 为输出引脚建立电压源约束（元件唯一允许的输出方式）：
// 3 针形态盖 tx/intr；13 针形态追加 D0..D7 与 RDY（ACK 是输入脚，不盖）。
// 初值取节点当前电压（与 element/logic.Gate 同口径，避免 t=0 的虚假瞬态）；
// 真实电平由 DoStep 每步经 digital.DriveNow 写入。
func (u *Uart) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	if m == nil || value == nil {
		return
	}
	stampUartOutput(m, value, pinTx, volTx)
	stampUartOutput(m, value, pinIntr, volIntr)
	if !uartIsByteTap(value) {
		return
	}
	for i := 0; i < 8; i++ {
		stampUartOutput(m, value, pinD0+i, volD0+i)
	}
	stampUartOutput(m, value, pinRdy, volRdy)
}

// stampUartOutput 为一根输出引脚与其电压源加盖（引脚节点，Gnd，电压源 ID，节点初值）。
func stampUartOutput(m mna.Mna, value element.NodeFace, pin, voltSource int) {
	node := value.GetNodes(pin)
	m.StampVoltageSource(node, mna.Gnd, value.GetVoltSource(voltSource), m.GetNodeVoltage(node))
}
