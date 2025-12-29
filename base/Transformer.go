package base

import (
	"circuit/mna"
	"math"
)

// Transformer 变压器
type Transformer struct{ Base }

func (transformer *Transformer) New() {
	transformer.ElementConfigBase = &mna.ElementConfigBase{
		Pin: []string{"p1", "p2", "s1", "s2"}, // 初级: p1-p2, 次级: s1-s2
		ValueInit: []any{
			float64(4),     // 0: 电感值(Henry)
			float64(1),     // 1: 匝数比
			float64(0.999), // 2: 耦合系数
			float64(0),     // 3: a1
			float64(0),     // 4: a2
			float64(0),     // 5: a3
			float64(0),     // 6: a4
			float64(0),     // 7: curSourceValue1
			float64(0),     // 8: curSourceValue2
		},
		Current:   []int{0, 1},
		OrigValue: []int{3, 4, 5, 6, 7, 8},
	}
}
func (transformer *Transformer) Init() mna.ValueMNA {
	return mna.NewElementBase(transformer.ElementConfigBase)
}
func (Transformer) Reset(base mna.ValueMNA) {
	// 初始化内部参数
	base.SetFloat64(3, 0) // a1
	base.SetFloat64(4, 0) // a2
	base.SetFloat64(5, 0) // a3
	base.SetFloat64(6, 0) // a4
	base.SetFloat64(7, 0) // curSourceValue1
	base.SetFloat64(8, 0) // curSourceValue2
}
func (Transformer) StartIteration(mna mna.MNA, base mna.ValueMNA) {
	voltdiff1 := mna.GetNodeVoltage(base.Nodes(0)) - mna.GetNodeVoltage(base.Nodes(1))
	voltdiff2 := mna.GetNodeVoltage(base.Nodes(2)) - mna.GetNodeVoltage(base.Nodes(3))

	// 如果是梯形积分法
	if base.GoodIterations() > 0 {
		a1 := base.GetFloat64(3)
		a2 := base.GetFloat64(4)
		a3 := base.GetFloat64(5)
		a4 := base.GetFloat64(6)
		curSourceValue1 := voltdiff1*a1 + voltdiff2*a2 + mna.GetVoltageSourceCurrent(base.VoltSource(0))
		curSourceValue2 := voltdiff1*a3 + voltdiff2*a4 + mna.GetVoltageSourceCurrent(base.VoltSource(1))
		base.SetFloat64(7, curSourceValue1)
		base.SetFloat64(8, curSourceValue2)
	} else {
		// 后向欧拉法
		curSourceValue1 := mna.GetVoltageSourceCurrent(base.VoltSource(0))
		curSourceValue2 := mna.GetVoltageSourceCurrent(base.VoltSource(1))
		base.SetFloat64(7, curSourceValue1)
		base.SetFloat64(8, curSourceValue2)
	}
}
func (Transformer) Stamp(mna mna.MNA, base mna.ValueMNA) {
	// equations for transformer:
	// v1 = L1 di1/dt + M di2/dt
	// v2 = M di1/dt + L2 di2/dt
	// we invert that to get:
	// di1/dt = a1 v1 + a2 v2
	// di2/dt = a3 v1 + a4 v2

	l1 := base.GetFloat64(0) // 初级电感
	ratio := base.GetFloat64(1)
	couplingCoef := base.GetFloat64(2)

	// 计算次级电感
	l2 := l1 * ratio * ratio
	// 计算互感
	m := couplingCoef * math.Sqrt(l1*l2)

	// 构建逆矩阵
	deti := 1.0 / (l1*l2 - m*m)

	// 时间步长
	ts := base.TimeStep()
	if base.GoodIterations() > 0 { // 梯形积分法
		ts = ts / 2
	}

	// 计算a1-a4系数
	a1 := l2 * deti * ts
	a2 := -m * deti * ts
	a3 := -m * deti * ts
	a4 := l1 * deti * ts

	base.SetFloat64(3, a1)
	base.SetFloat64(4, a2)
	base.SetFloat64(5, a3)
	base.SetFloat64(6, a4)

	// 设置矩阵值
	mna.StampConductance(base.Nodes(0), base.Nodes(1), a1)
	mna.StampVCCS(base.Nodes(0), base.Nodes(1), base.Nodes(2), base.Nodes(3), a2)
	mna.StampVCCS(base.Nodes(2), base.Nodes(3), base.Nodes(0), base.Nodes(1), a3)
	mna.StampConductance(base.Nodes(2), base.Nodes(3), a4)

	// 加盖虚拟电阻避免奇异矩阵
	mna.StampResistor(-1, base.Nodes(0), 1e9)
	mna.StampResistor(-1, base.Nodes(1), 1e9)
	mna.StampResistor(-1, base.Nodes(2), 1e9)
	mna.StampResistor(-1, base.Nodes(3), 1e9)
}
func (Transformer) DoStep(mna mna.MNA, base mna.ValueMNA) {
	curSourceValue1 := base.GetFloat64(7)
	curSourceValue2 := base.GetFloat64(8)
	mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), curSourceValue1)
	mna.StampCurrentSource(base.Nodes(2), base.Nodes(3), curSourceValue2)
}
func (Transformer) CalculateCurrent(mna mna.MNA, base mna.ValueMNA) {
	voltdiff1 := mna.GetNodeVoltage(base.Nodes(0)) - mna.GetNodeVoltage(base.Nodes(1))
	voltdiff2 := mna.GetNodeVoltage(base.Nodes(2)) - mna.GetNodeVoltage(base.Nodes(3))

	a1 := base.GetFloat64(3)
	a2 := base.GetFloat64(4)
	a3 := base.GetFloat64(5)
	a4 := base.GetFloat64(6)
	curSourceValue1 := base.GetFloat64(7)
	curSourceValue2 := base.GetFloat64(8)

	current1 := voltdiff1*a1 + voltdiff2*a2 + curSourceValue1
	current2 := voltdiff1*a3 + voltdiff2*a4 + curSourceValue2

	// 存储电流值
	mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), -current1)
	mna.StampCurrentSource(base.Nodes(2), base.Nodes(3), -current2)
}
