// VCCS（压控电流源）元件实现，提供电压到电流的线性跨导转换。
package controlled

import (
	"circuit/element"
	"circuit/mna"
)

// VCCSType 压控电流源类型标识，网表标识为 "g"，4引脚(cp/cn/op/on)。
var VCCSType element.NodeType = element.AddElement(21, &VCCS{
	&element.Config{
		Name:      "g",
		Pin:       element.SetPin(element.PinLowVoltage, "cp", "cn", "op", "on"),
		ValueInit: []any{float64(1e-3)}, // 默认跨导 1mS
		ValueName: []string{"gm"},
	},
})

// VCCS 压控电流源结构体，嵌入element.Config。
// 数学模型: I_out = gm * (V_cp - V_cn)，其中gm为跨导增益。
type VCCS struct{ *element.Config }

// Stamp 加盖VCCS的MNA贡献。
// 使用StampVCCS在MNA矩阵中建立跨导关系：输出电流I(out)=gm·V(cp,cn)。
func (VCCS) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	gm := value.GetFloat64(0)
	mna.StampVCCS(
		value.GetNodes(2), value.GetNodes(3), // 输出电流 op, on
		value.GetNodes(0), value.GetNodes(1), // 控制电压 cp, cn
		gm,
	)
}
