// 油路节流孔(液阻)元件实现，基于压力-流量线性关系。
package hydraulic

import (
	"circuit/element"
	"circuit/mna"
	"log"
	"math"
)

// HRType 液阻元件类型标识(NodeType=29)，网表"HR<name> <n1,n2> [R]"。
var HRType element.NodeType = element.AddElement(29, &HR{
	&element.Config{
		Name:      "hr",
		Pin:       element.SetPin(element.PinHydraulic, "hr1", "hr2"),
		ValueInit: []any{float64(1e10), 0.0},
		ValueName: []string{"R", "Q"},
		Current:   []int{1},
	},
})

// HROrifice 液阻/节流孔元件。模型: Δp = R * Q，与电阻同构。
type HR struct{ *element.Config }

// Stamp 加盖液阻的MNA贡献，调用StampImpedance将液导添加到矩阵。
func (HR) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	r := value.GetFloat64(0)
	if math.Abs(r) <= 1e-9 {
		log.Printf("HR: 液阻值 %.3e 近似为零，将被视为 1n Pa·s/m³ 短路路径", r)
	}
	mna.StampImpedance(value.GetNodes(0), value.GetNodes(1), r)
}

// CalculateCurrent 按 Δp=R·Q 计算流经液阻的体积流量（正方向为引脚0→引脚1）。
func (HR) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	r := value.GetFloat64(0)
	if math.Abs(r) <= 1e-9 {
		value.SetFloat64(1, 0)
		return
	}
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	value.SetFloat64(1, (v1-v2)/r)
}
