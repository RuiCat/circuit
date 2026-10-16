// Package ttl 提供 74LS 系列 TTL 器件的电气参数表与查表函数。
//
// 数据与 element/ttl/specTable.md 同源：表中每个型号一行，与此文件 deviceSpecs 一一对应。
// 未取证的字段两侧都标为未知（Go 侧为 0，md 侧为「（未取证）」），不做任何默认回退。
package ttl

// DeviceSpec 描述一个 74LS 器件的电气参数（手册典型值，TA = 25 °C，VCC = 5 V）。
type DeviceSpec struct {
	Model        string  // 型号，如 "74LS00"
	TpdLH, TpdHL float64 // 传播延迟（秒），手册表格以 ns 给出、此处已换算
	IIH, IIL     float64 // 输入高/低电平电流（安）
	IOH, IOL     float64 // 输出高/低电平电流（安）
	VOH, VOL     float64 // 输出高/低电平电压（伏）
	Source       string  // 手册名 + 文档号 + 表号/页码，或 URL 锚点；未取证时为空
}

// nsToSecond 把手册中的 ns 数值换算为秒：本文件所有 Tpd 字段都以秒存储。
const nsToSecond = 1e-9

// deviceSpecs 是与 specTable.md 同源的参数表；未取证字段写 0，不许填推测值。
var deviceSpecs = []DeviceSpec{
	{
		Model:  "74LS00",
		TpdLH:  9 * nsToSecond,
		TpdHL:  10 * nsToSecond,
		IIH:    20e-6,
		IIL:    -0.4e-3,
		IOH:    -0.4e-3,
		IOL:    8e-3,
		VOH:    3.4,
		VOL:    0.35,
		Source: "TI SN74LS00, SDLS025D, §6.3/§6.6/§6.9 (RL=2k, CL=15pF)",
	},
	{
		Model:  "74LS04",
		TpdLH:  9 * nsToSecond,
		TpdHL:  10 * nsToSecond,
		IOH:    -0.4e-3,
		IOL:    8e-3,
		VOH:    3.4,
		VOL:    0.25,
		Source: "TI SN74LS04, SDLS029C, \"54LS/74LS\" 各表 (RL=2k, CL=15pF)",
	},
	{
		Model:  "74LS08",
		TpdLH:  8 * nsToSecond,
		TpdHL:  10 * nsToSecond,
		IIH:    20e-6,
		IIL:    -0.4e-3,
		IOH:    -0.4e-3,
		IOL:    8e-3,
		VOH:    3.4,
		VOL:    0.35,
		Source: "TI SN74LS08, SDLS033, p.4 各表 (RL=2k, CL=15pF)",
	},
	{
		Model:  "74LS32",
		TpdLH:  14 * nsToSecond,
		IIH:    20e-6,
		IIL:    -0.4e-3,
		IOH:    -0.4e-3,
		IOL:    8e-3,
		VOH:    2.7,
		VOL:    0.35,
		Source: "TI SN54LS32/SN74LS32, p.4 各表 (RL=2k, CL=15pF)",
	},
	{
		Model:  "74LS74",
		TpdLH:  13 * nsToSecond,
		TpdHL:  25 * nsToSecond,
		IIH:    20e-6,
		IIL:    -0.4e-3,
		IOH:    -0.4e-3,
		IOL:    8e-3,
		VOH:    3.4,
		VOL:    0.35,
		Source: "TI SN74LS74A, SDLS119, p.5 各表 (CLR/PRE/CLK→Q, RL=2k, CL=15pF)",
	},
	{
		Model:  "74LS86",
		IOH:    -0.4e-3,
		IOL:    8e-3,
		VOH:    3.4,
		VOL:    0.25,
		Source: "TI SN74LS86A, SDLS124, p.4 各表",
	},
	{
		Model:  "74LS138",
		IOH:    -0.4e-3,
		IOL:    8e-3,
		VOH:    3.4,
		Source: "TI SN54LS138/SN74LS138, p.4 各表",
	},
	{
		Model:  "74LS164",
		IIH:    20e-6,
		IIL:    -0.4e-3,
		IOH:    -0.4e-3,
		IOL:    8e-3,
		VOH:    3.5,
		VOL:    0.35,
		Source: "TI SN54LS164/SN74LS164, p.5 各表",
	},
	{
		Model:  "74LS245",
		TpdLH:  8 * nsToSecond,
		TpdHL:  8 * nsToSecond,
		IIH:    20e-6,
		IIL:    -0.2e-3,
		IOH:    -15e-3,
		IOL:    24e-3,
		VOH:    3.4,
		Source: "TI SN74LS245, SDLS146B, §7.3/§7.5/§7.7 (CL=45pF, RL=667Ω)",
	},
	{
		Model:  "74LS373",
		TpdLH:  12 * nsToSecond,
		TpdHL:  12 * nsToSecond,
		IOH:    -2.6e-3,
		IOL:    24e-3,
		Source: "TI SN74LS373, SDLS165B, \"54LS/74LS\" 表 (Data→Q, RL=667Ω, CL=45pF)",
	},
	{
		Model:  "74LS574",
		IIH:    20e-6,
		IIL:    -0.4e-3,
		IOH:    -2.6e-3,
		IOL:    24e-3,
		VOH:    3.3,
		VOL:    0.35,
		Source: "Fairchild DM74LS574, Farnell 18717.pdf, Recommended/DC/Switching 表",
	},
}

// SpecOf 按型号返回电气参数；未登记的型号返回 false，不回退任何默认值。
func SpecOf(model string) (DeviceSpec, bool) {
	for _, spec := range deviceSpecs {
		if spec.Model == model {
			return spec, true
		}
	}
	return DeviceSpec{}, false
}
