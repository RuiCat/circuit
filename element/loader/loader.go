// loader.go：LOADER 元件的注册、28 引脚/19 输出源表与输出引脚电压源加盖（Stamp）。
package loader

import (
	"circuit/element"
	"circuit/mna"
)

// loaderNodeType 是 LOADER 的 NodeType（K6：49，未被 0–48、50、52、54 占用）。
const loaderNodeType element.NodeType = 49

// LOADER 的引脚下标（K2 冻结顺序：D0..D7, RDY, LA0..LA7, LD0..LD7, LOAD, LWE, ACK）。
const (
	pinD0 = iota
	pinD1
	pinD2
	pinD3
	pinD4
	pinD5
	pinD6
	pinD7
	pinRDY
	pinLA0
	pinLA1
	pinLA2
	pinLA3
	pinLA4
	pinLA5
	pinLA6
	pinLA7
	pinLD0
	pinLD1
	pinLD2
	pinLD3
	pinLD4
	pinLD5
	pinLD6
	pinLD7
	pinLOAD
	pinLWE
	pinACK
)

// loaderPinNum 是引脚总数（28），K2 冻结。
const loaderPinNum = pinACK + 1

// LOADER 的电压源下标（LA/LD/LOAD/LWE/ACK 都是输出，只能经电压源钳位）。
const (
	volLA0 = iota
	volLA1
	volLA2
	volLA3
	volLA4
	volLA5
	volLA6
	volLA7
	volLD0
	volLD1
	volLD2
	volLD3
	volLD4
	volLD5
	volLD6
	volLD7
	volLOAD
	volLWE
	volACK
)

// loaderVoltNum 是电压源总数（8 地址 + 8 数据 + LOAD/LWE/ACK = 19）。
const loaderVoltNum = volACK + 1

// loaderVoltageNames 是输出电压源的调试名（与槽位/引脚表同序）。
var loaderVoltageNames = []string{
	"la0", "la1", "la2", "la3", "la4", "la5", "la6", "la7",
	"ld0", "ld1", "ld2", "ld3", "ld4", "ld5", "ld6", "ld7",
	"LOAD", "LWE", "ACK",
}

// loaderVoltPin 返回输出电压源下标对应的引脚下标（两者布局不同：电压源按 LA/LD/LOAD/LWE/ACK 紧凑排列）。
func loaderVoltPin(voltIndex int) int {
	if voltIndex < 8 {
		return pinLA0 + voltIndex
	}
	if voltIndex < 16 {
		return pinLD0 + (voltIndex - 8)
	}
	switch voltIndex {
	case volLOAD:
		return pinLOAD
	case volLWE:
		return pinLWE
	default:
		return pinACK
	}
}

// LoaderType 是 LOADER 注册后的类型号（调用方用本变量，不硬编码 49）。
// 网表类型名 LOADER；K6 要求注册行在包内只出现一次。
var LoaderType = element.AddElement(loaderNodeType, &Loader{Config: &element.Config{
	Name:      "LOADER",
	Pin:       loaderPins(),
	ValueInit: loaderValueInit(),
	ValueName: loaderValueNames(),
	OrigValue: loaderRuntimeSlots,
	Voltage:   loaderVoltageNames,
}})

// Loader 是串口加载控制器：把 C1 的字节握手解码成 CPU8 的 LA/LD/LOAD/LWE 加载时序。
// 一个模型按 CpuMode 参数化 v3（55）与 v2（54 改造网表）两种时序。
type Loader struct{ *element.Config }

// loaderPins 生成 K2 冻结的 28 引脚表（全部为布尔电平引脚）。
func loaderPins() []element.Pin {
	return element.SetPin(element.PinBoolean,
		"D0", "D1", "D2", "D3", "D4", "D5", "D6", "D7", "RDY",
		"LA0", "LA1", "LA2", "LA3", "LA4", "LA5", "LA6", "LA7",
		"LD0", "LD1", "LD2", "LD3", "LD4", "LD5", "LD6", "LD7",
		"LOAD", "LWE", "ACK")
}

// Stamp 为 19 根输出引脚建立电压源约束（元件唯一允许的输出方式）。
// 初值取节点当前电压（与 UART/logic.Gate 同口径，避免 t=0 的虚假瞬态）；
// 真实电平由 DoStep 每步经 digital.DriveNow 写入。
func (l *Loader) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	if m == nil || value == nil {
		return
	}
	for v := 0; v < loaderVoltNum; v++ {
		node := value.GetNodes(loaderVoltPin(v))
		m.StampVoltageSource(node, mna.Gnd, value.GetVoltSource(v), m.GetNodeVoltage(node))
	}
}
