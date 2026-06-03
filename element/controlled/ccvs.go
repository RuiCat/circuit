// CCVS（流控电压源）元件实现，将控制支路电流转换为输出电压。
package controlled

import (
	"circuit/element"
	"circuit/mna"
)

// CCVSType 流控电压源类型标识，网表标识为 "h"，4引脚，含vmeas和vout两个内部电压源。
var CCVSType element.NodeType = element.AddElement(23, &CCVS{
	&element.Config{
		Name:      "h",
		Pin:       element.SetPin(element.PinLowVoltage, "cp", "cn", "op", "on"),
		ValueInit: []any{float64(1)}, // 默认跨阻 1Ω
		ValueName: []string{"rm"},
		Voltage:   []string{"vmeas", "vout"}, // vmeas=测量电压源, vout=输出CCVS
	},
})

// CCVS 流控电压源结构体。
// 数学模型: V_out = rm * I_control，通过0V测量电压源检测控制电流，CCVS约束方程驱动输出。
type CCVS struct{ *element.Config }

// Stamp 加盖CCVS的MNA贡献。
// 第一步：在cp-cn间加盖0V测量电压源(vmeas)；第二步：在op-on间调用StampCCVS。
func (CCVS) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 在 cp-cn 之间加盖 0V 电压源用于测量控制电流
	mna.StampVoltageSource(value.GetNodes(0), value.GetNodes(1), value.GetVoltSource(0), 0)
	// 在 op-on 之间加盖 CCVS
	rm := value.GetFloat64(0)
	mna.StampCCVS(value.GetNodes(2), value.GetNodes(3), value.GetVoltSource(0), value.GetVoltSource(1), rm)
}
