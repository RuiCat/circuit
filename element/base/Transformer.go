package base

import (
	"circuit/element"
	"circuit/mna"
	"math"
)

// TransformerType 定义元件
var TransformerType element.NodeType = element.AddElement(8, &Transformer{
	&element.Config{
		Name: "xfmr",
		Pin:  []string{"p1", "p2", "s1", "s2"}, // 初级: p1-p2, 次级: s1-s2
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
	},
})

// Transformer 变压器
type Transformer struct{ *element.Config }

func (Transformer) Reset(base element.NodeFace) {
	// 初始化内部参数
	base.SetFloat64(3, 0) // a1
	base.SetFloat64(4, 0) // a2
	base.SetFloat64(5, 0) // a3
	base.SetFloat64(6, 0) // a4
	base.SetFloat64(7, 0) // curSourceValue1
	base.SetFloat64(8, 0) // curSourceValue2
}

func (Transformer) StartIteration(mna mna.MNA, time mna.Time, value element.NodeFace) {
	voltdiff1 := mna.GetNodeVoltage(value.GetNodes(0)) - mna.GetNodeVoltage(value.GetNodes(1))
	voltdiff2 := mna.GetNodeVoltage(value.GetNodes(2)) - mna.GetNodeVoltage(value.GetNodes(3))

	// 如果是梯形积分法
	if time.GoodIterations() > 0 {
		a1 := value.GetFloat64(3)
		a2 := value.GetFloat64(4)
		a3 := value.GetFloat64(5)
		a4 := value.GetFloat64(6)
		curSourceValue1 := voltdiff1*a1 + voltdiff2*a2 + mna.GetNodeCurrent(value.GetVoltSource(0))
		curSourceValue2 := voltdiff1*a3 + voltdiff2*a4 + mna.GetNodeCurrent(value.GetVoltSource(1))
		value.SetFloat64(7, curSourceValue1)
		value.SetFloat64(8, curSourceValue2)
	} else {
		// 后向欧拉法
		curSourceValue1 := mna.GetNodeCurrent(value.GetVoltSource(0))
		curSourceValue2 := mna.GetNodeCurrent(value.GetVoltSource(1))
		value.SetFloat64(7, curSourceValue1)
		value.SetFloat64(8, curSourceValue2)
	}
}

func (Transformer) Stamp(mna mna.MNA, time mna.Time, value element.NodeFace) {
	// equations for transformer:
	// v1 = L1 di1/dt + M di2/dt
	// v2 = M di1/dt + L2 di2/dt
	// we invert that to get:
	// di1/dt = a1 v1 + a2 v2
	// di2/dt = a3 v1 + a4 v2

	l1 := value.GetFloat64(0) // 初级电感
	ratio := value.GetFloat64(1)
	couplingCoef := value.GetFloat64(2)

	// 计算次级电感
	l2 := l1 * ratio * ratio
	// 计算互感
	m := couplingCoef * math.Sqrt(l1*l2)

	// 构建逆矩阵
	deti := 1.0 / (l1*l2 - m*m)

	// 时间步长
	ts := time.TimeStep()
	if time.GoodIterations() > 0 { // 梯形积分法
		ts = ts / 2
	}

	// 计算a1-a4系数
	a1 := l2 * deti * ts
	a2 := -m * deti * ts
	a3 := -m * deti * ts
	a4 := l1 * deti * ts

	value.SetFloat64(3, a1)
	value.SetFloat64(4, a2)
	value.SetFloat64(5, a3)
	value.SetFloat64(6, a4)

	// 设置矩阵值
	mna.StampConductance(value.GetNodes(0), value.GetNodes(1), a1)
	mna.StampVCCS(value.GetNodes(0), value.GetNodes(1), value.GetNodes(2), value.GetNodes(3), a2)
	mna.StampVCCS(value.GetNodes(2), value.GetNodes(3), value.GetNodes(0), value.GetNodes(1), a3)
	mna.StampConductance(value.GetNodes(2), value.GetNodes(3), a4)

	// 加盖虚拟电阻避免奇异矩阵
	mna.StampResistor(-1, value.GetNodes(0), 1e9)
	mna.StampResistor(-1, value.GetNodes(1), 1e9)
	mna.StampResistor(-1, value.GetNodes(2), 1e9)
	mna.StampResistor(-1, value.GetNodes(3), 1e9)
}

func (Transformer) DoStep(mna mna.MNA, time mna.Time, value element.NodeFace) {
	curSourceValue1 := value.GetFloat64(7)
	curSourceValue2 := value.GetFloat64(8)
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), curSourceValue1)
	mna.StampCurrentSource(value.GetNodes(2), value.GetNodes(3), curSourceValue2)
}

func (Transformer) CalculateCurrent(mna mna.MNA, time mna.Time, value element.NodeFace) {
	voltdiff1 := mna.GetNodeVoltage(value.GetNodes(0)) - mna.GetNodeVoltage(value.GetNodes(1))
	voltdiff2 := mna.GetNodeVoltage(value.GetNodes(2)) - mna.GetNodeVoltage(value.GetNodes(3))

	a1 := value.GetFloat64(3)
	a2 := value.GetFloat64(4)
	a3 := value.GetFloat64(5)
	a4 := value.GetFloat64(6)
	curSourceValue1 := value.GetFloat64(7)
	curSourceValue2 := value.GetFloat64(8)

	current1 := voltdiff1*a1 + voltdiff2*a2 + curSourceValue1
	current2 := voltdiff1*a3 + voltdiff2*a4 + curSourceValue2

	// 存储电流值
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), -current1)
	mna.StampCurrentSource(value.GetNodes(2), value.GetNodes(3), -current2)
}
