// 气路气室(气容)元件实现，使用梯形积分伴随模型模拟气体可压缩性。
package pneumatic

import (
	"circuit/element"
	mn "circuit/mna"
	"log"
)

// PCType 气容元件类型标识(NodeType=25)，网表"PC<name> <n1> [C]"，单引脚+地。
var PCType element.NodeType = element.AddElement(25, &PC{
	&element.Config{
		Name:      "pc",
		Pin:       element.SetPin(element.PinPneumatic, "pc1"),
		ValueInit: []any{float64(1e-8), 0.0, 0.0, 0.0, 0.0},
		ValueName: []string{"C", "G_eq", "I_hist", "V_diff", "I_cap"},
		Current:   []int{4},
		OrigValue: []int{1, 2, 3, 4},
		Flags:     element.FlagReactive,
	},
})

// PCChamber 气室/气容元件。模型: C·dp/dt = qm，与电容(C·dV/dt=I)完全同构。
// 使用梯形积分伴随模型(G_eq=2C/dt + I_hist)，一端接大气压(GND)。
type PC struct{ *element.Config }

// StartIteration 气容的迭代初始化
// 计算历史质量流量 I_hist = (2C/dt) * v_diff + I_cap_prev
func (PC) StartIteration(mna mn.Mna, time mn.Time, value element.NodeFace) {
	dt := time.TimeStep()
	if dt <= 0 {
		log.Printf("警告: 气容 StartIteration 中 dt=%.6e <= 0，跳过历史流量计算", dt)
		return
	}
	c := value.GetFloat64(0)
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v_diff := v1
	I_hist := (2*c/dt)*v_diff + value.GetFloat64(4)
	value.SetFloat64(2, I_hist)
}

// Stamp 气容的MNA矩阵加盖操作
// 计算等效流导 G_eq = 2C/dt，并将其添加到MNA矩阵中（一端接地）
func (PC) Stamp(mna mn.Mna, time mn.Time, value element.NodeFace) {
	dt := time.TimeStep()
	c := value.GetFloat64(0)
	if dt <= 0 {
		log.Printf("警告: 气容 Stamp 中 dt=%.6e <= 0，跳过等效流导计算", dt)
		return
	}
	if c <= 0 {
		log.Printf("警告: 气容 %s 值为 %.6e <= 0，将被忽略", value.Config().ValueName[0], c)
		return
	}
	G_eq := 2 * c / dt
	value.SetFloat64(1, G_eq)
	mna.StampAdmittance(value.GetNodes(0), mn.Gnd, G_eq)
}

// DoStep 气容的步进计算
// 将历史质量流量 I_hist 加盖到MNA右侧向量中
func (PC) DoStep(mna mn.Mna, time mn.Time, value element.NodeFace) {
	I_hist := value.GetFloat64(2)
	mna.StampCurrentSource(mn.Gnd, value.GetNodes(0), I_hist)
}

// CalculateCurrent 计算流入气容的质量流量
// 使用 I_cap = G_eq * v_diff - I_hist
func (PC) CalculateCurrent(mna mn.Mna, time mn.Time, value element.NodeFace) {
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v_diff := v1
	value.SetFloat64(3, v_diff)
	G_eq := value.GetFloat64(1)
	I_hist := value.GetFloat64(2)
	I_cap := G_eq*v_diff - I_hist
	value.SetFloat64(4, I_cap)
}

// AddDerivative 向导数向量 der 累加气容的 dp/dt 贡献：dp/dt = I_cap / C
func (PC) AddDerivative(mna mn.Mna, time mn.Time, value element.NodeFace, der []float64) {
	C := value.GetFloat64(0)
	iCap := value.GetFloat64(4)
	if C > 0 {
		n1 := int(value.GetNodes(0))
		if n1 >= 0 && n1 < mna.GetNodeNum() {
			der[n1] += -iCap / C
		}
	}
}
