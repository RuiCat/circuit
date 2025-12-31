package base

import "circuit/mna"

const SwitchType ElementType = 7

// Switch 开关
type Switch struct{ Base }

func (sw *Switch) New() {
	sw.ElementConfigBase = &mna.ElementConfigBase{
		Pin: []string{"sw1", "sw2"},
		ValueInit: []any{
			int(0),        // 0: 开关状态 (0=关, 1=开)
			float64(1e-6), // 1: 导通电阻
			float64(1e12), // 2: 关断电阻
		},
		Current: []int{0},
	}
}
func (sw *Switch) Init() mna.ValueMNA {
	return mna.NewElementBase(sw.ElementConfigBase)
}

func (Switch) Stamp(mna mna.MNA, base mna.ValueMNA) {
	state := base.GetInt(0)
	var resistance float64
	if state == 1 {
		resistance = base.GetFloat64(1) // 导通状态
	} else {
		resistance = base.GetFloat64(2) // 关断状态
	}
	mna.StampResistor(base.Nodes(0), base.Nodes(1), resistance)
}

func (Switch) CalculateCurrent(mna mna.MNA, base mna.ValueMNA) {
	state := base.GetInt(0)
	var resistance float64
	if state == 1 {
		resistance = base.GetFloat64(1)
	} else {
		resistance = base.GetFloat64(2)
	}

	// 计算电流（欧姆定律）
	v1 := mna.GetNodeVoltage(base.Nodes(0))
	v2 := mna.GetNodeVoltage(base.Nodes(1))
	if resistance > 0 {
		current := (v1 - v2) / resistance
		mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), -current)
	}
}
