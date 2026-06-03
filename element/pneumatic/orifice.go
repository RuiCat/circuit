// 气路节流孔(气阻)元件实现，基于压力-流量线性关系。
package pneumatic

import (
	"circuit/element"
	"circuit/mna"
	"log"
	"math"
)

// PRType 气阻元件类型标识(NodeType=24)，网表"PR<name> <n1,n2> [R]"。
var PRType element.NodeType = element.AddElement(24, &PR{
	&element.Config{
		Name:      "pr",
		Pin:       element.SetPin(element.PinPneumatic, "pr1", "pr2"),
		ValueInit: []any{float64(1e9)},
		ValueName: []string{"R"},
	},
})

// PROrifice 气阻/节流孔元件。模型: Δp = R * qm，与电阻元件(V=R·I)完全同构。
type PR struct{ *element.Config }

// Stamp 加盖气阻的MNA贡献，调用StampImpedance将气导(G=1/R)添加到矩阵。
func (PR) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	r := value.GetFloat64(0)
	if math.Abs(r) <= 1e-9 {
		log.Printf("PR: 气阻值 %.3e 近似为零，将被视为 1n Pa·s/kg 短路路径", r)
	}
	mna.StampImpedance(value.GetNodes(0), value.GetNodes(1), r)
}
