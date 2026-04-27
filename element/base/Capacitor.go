package base

import (
	"circuit/element"
	"circuit/mna"
)

// CapacitorType 定义元件
var CapacitorType element.NodeType = element.AddElement(0, &Capacitor{
	&element.Config{
		Name:      "c",
		Pin:       element.SetPin(element.PinLowVoltage, "c1", "c2"),
		ValueInit: []any{float64(1e-6), 0.0, 0.0, 0.0, 0.0},
		ValueName: []string{"C", "G_eq", "I_hist", "V_diff", "I_cap"},
		Current:   []int{4},
		OrigValue: []int{1, 2, 3, 4},
		Flags:     element.FlagReactive,
	},
})

// Capacitor 电容元件结构体，继承element.Config
// 实现电容的伴随模型（companion model），使用梯形积分法进行暂态仿真
type Capacitor struct{ *element.Config }

// StartIteration 电容的迭代初始化
// 计算历史电流源 I_hist = (2C/dt) * v_prev + I_cap_prev，用于伴随模型的电流源贡献
func (Capacitor) StartIteration(mna mna.Mna, time mna.Time, value element.NodeFace) {
	dt := time.TimeStep()
	c := value.GetFloat64(0)
	v_prev := value.GetFloat64(3)
	I_hist := (2*c/dt)*v_prev + value.GetFloat64(4)
	value.SetFloat64(2, I_hist)
}

// Stamp 电容的MNA矩阵加盖操作
// 计算等效电导 G_eq = 2C/dt，并将其添加到MNA矩阵中
func (Capacitor) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	dt := time.TimeStep()
	c := value.GetFloat64(0)
	if dt <= 0 || c <= 0 {
		return
	}
	G_eq := 2 * c / dt
	value.SetFloat64(1, G_eq)
	mna.StampAdmittance(value.GetNodes(0), value.GetNodes(1), G_eq)
}

// DoStep 电容的步进计算
// 将历史电流源 I_hist 加盖到MNA右侧向量中，实现伴随模型的电流源贡献
func (Capacitor) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	I_hist := value.GetFloat64(2)
	mna.StampCurrentSource(value.GetNodes(1), value.GetNodes(0), I_hist)
}

// CalculateCurrent 计算电容的电流
// 使用 I_cap = G_eq * v_diff - I_hist 计算流经电容的电流，其中 v_diff 为两端电压差
func (Capacitor) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	v_diff := v1 - v2
	value.SetFloat64(3, v_diff)
	G_eq := value.GetFloat64(1)
	I_hist := value.GetFloat64(2)
	I_cap := G_eq*v_diff - I_hist
	value.SetFloat64(4, I_cap)
}

// AddDerivative 向导数向量 der 累加电容的 dv/dt 贡献：dv/dt = I_cap / C
func (Capacitor) AddDerivative(mna mna.Mna, time mna.Time, value element.NodeFace, der []float64) {
	C := value.GetFloat64(0)
	iCap := value.GetFloat64(4)
	if C > 0 {
		n1 := int(value.GetNodes(0))
		n2 := int(value.GetNodes(1))
		if n1 >= 0 && n1 < mna.GetNodeNum() {
			der[n1] += -iCap / C
		}
		if n2 >= 0 && n2 < mna.GetNodeNum() {
			der[n2] += iCap / C
		}
	}
}
