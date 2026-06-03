// 油路蓄能器(液容)元件实现，使用梯形积分伴随模型。
package hydraulic

import (
	"circuit/element"
	mn "circuit/mna"
	"log"
)

// HAType 蓄能器元件类型标识(NodeType=30)，网表"HA<name> <n1> [C]"，单引脚+地(油箱)。
var HAType element.NodeType = element.AddElement(30, &HA{
	&element.Config{
		Name:      "ha",
		Pin:       element.SetPin(element.PinHydraulic, "ha1"),
		ValueInit: []any{float64(1e-10), 0.0, 0.0, 0.0, 0.0},
		ValueName: []string{"C", "G_eq", "I_hist", "V_diff", "I_cap"},
		Current:   []int{4},
		OrigValue: []int{1, 2, 3, 4},
		Flags:     element.FlagReactive,
	},
})

// HAAccumulator 蓄能器/液容元件。模型: C·dp/dt = Q，与电容同构。
// 一端接油箱(GND)，使用梯形积分伴随模型。
type HA struct{ *element.Config }

// StartIteration 蓄能器的迭代初始化
// 计算历史流量 I_hist = (2C/dt) * v_diff + I_cap_prev
func (HA) StartIteration(mna mn.Mna, time mn.Time, value element.NodeFace) {
	dt := time.TimeStep()
	if dt <= 0 {
		log.Printf("警告: 蓄能器 StartIteration 中 dt=%.6e <= 0，跳过历史流量计算", dt)
		return
	}
	c := value.GetFloat64(0)
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v_diff := v1
	I_hist := (2*c/dt)*v_diff + value.GetFloat64(4)
	value.SetFloat64(2, I_hist)
}

// Stamp 蓄能器的MNA矩阵加盖操作
// 计算等效流导 G_eq = 2C/dt，并将其添加到MNA矩阵中（一端接地）
func (HA) Stamp(mna mn.Mna, time mn.Time, value element.NodeFace) {
	dt := time.TimeStep()
	c := value.GetFloat64(0)
	if dt <= 0 {
		log.Printf("警告: 蓄能器 Stamp 中 dt=%.6e <= 0，跳过等效流导计算", dt)
		return
	}
	if c <= 0 {
		log.Printf("警告: 蓄能器 %s 值为 %.6e <= 0，将被忽略", value.Config().ValueName[0], c)
		return
	}
	G_eq := 2 * c / dt
	value.SetFloat64(1, G_eq)
	mna.StampAdmittance(value.GetNodes(0), mn.Gnd, G_eq)
}

// DoStep 蓄能器的步进计算
// 将历史流量 I_hist 加盖到MNA右侧向量中
func (HA) DoStep(mna mn.Mna, time mn.Time, value element.NodeFace) {
	I_hist := value.GetFloat64(2)
	mna.StampCurrentSource(mn.Gnd, value.GetNodes(0), I_hist)
}

// CalculateCurrent 计算流入蓄能器的流量
// 使用 I_cap = G_eq * v_diff - I_hist
func (HA) CalculateCurrent(mna mn.Mna, time mn.Time, value element.NodeFace) {
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v_diff := v1
	value.SetFloat64(3, v_diff)
	G_eq := value.GetFloat64(1)
	I_hist := value.GetFloat64(2)
	I_cap := G_eq*v_diff - I_hist
	value.SetFloat64(4, I_cap)
}

// AddDerivative 向导数向量 der 累加蓄能器的 dp/dt 贡献：dp/dt = I_cap / C
func (HA) AddDerivative(mna mn.Mna, time mn.Time, value element.NodeFace, der []float64) {
	C := value.GetFloat64(0)
	iCap := value.GetFloat64(4)
	if C > 0 {
		n1 := int(value.GetNodes(0))
		if n1 >= 0 && n1 < mna.GetNodeNum() {
			der[n1] += -iCap / C
		}
	}
}
