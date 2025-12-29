package base

import "circuit/mna"

// Inductor 电感器
type Inductor struct{ Base }

func (inductor *Inductor) New() {
	inductor.ElementConfigBase = &mna.ElementConfigBase{
		Pin: []string{"l1", "l2"},
		ValueInit: []any{
			float64(1e-3), // 0: 电感值(Henry)
			float64(0),    // 1: 初始电流(A)
			float64(0),    // 2: 补偿电阻
			float64(0),    // 3: 电流源值
		},
		Current:   []int{0},
		OrigValue: []int{2, 3},
	}
}
func (inductor *Inductor) Init() mna.ValueMNA {
	return mna.NewElementBase(inductor.ElementConfigBase)
}
func (Inductor) Reset(base mna.ValueMNA) {
	// 初始化补偿电阻和电流源值
	base.SetFloat64(2, 0)
	base.SetFloat64(3, 0)
}
func (Inductor) StartIteration(mna mna.MNA, base mna.ValueMNA) {
	// 如果是梯形积分法，计算电流源值
	if base.TimeStep() > 0 {
		compResistance := base.GetFloat64(2)
		if compResistance > 0 {
			v1 := mna.GetNodeVoltage(base.Nodes(0))
			v2 := mna.GetNodeVoltage(base.Nodes(1))
			voltdiff := v1 - v2
			current := mna.GetVoltageSourceCurrent(base.VoltSource(0))
			curSourceValue := voltdiff/compResistance + current
			base.SetFloat64(3, curSourceValue)
		}
	}
}
func (Inductor) Stamp(mna mna.MNA, base mna.ValueMNA) {
	inductance := base.GetFloat64(0)
	dt := base.TimeStep()
	if dt <= 0 || inductance <= 0 {
		return
	}

	// 计算补偿电阻
	var compResistance float64
	if base.GoodIterations() > 0 { // 使用梯形积分法
		compResistance = 2 * inductance / dt
	} else { // 使用后向欧拉法
		compResistance = inductance / dt
	}
	base.SetFloat64(2, compResistance)

	// 加盖电阻贡献
	mna.StampResistor(base.Nodes(0), base.Nodes(1), compResistance)
}
func (Inductor) DoStep(mna mna.MNA, base mna.ValueMNA) {
	curSourceValue := base.GetFloat64(3)
	mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), curSourceValue)
}
func (Inductor) CalculateCurrent(mna mna.MNA, base mna.ValueMNA) {
	compResistance := base.GetFloat64(2)
	if compResistance > 0 {
		v1 := mna.GetNodeVoltage(base.Nodes(0))
		v2 := mna.GetNodeVoltage(base.Nodes(1))
		voltdiff := v1 - v2
		curSourceValue := base.GetFloat64(3)
		current := voltdiff/compResistance + curSourceValue
		// 存储电流值
		mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), -current)
	}
}
