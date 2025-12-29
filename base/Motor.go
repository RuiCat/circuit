package base

import "circuit/mna"

// Motor 电机（直流电机实现）
type Motor struct{ Base }

func (motor *Motor) New() {
	motor.ElementConfigBase = &mna.ElementConfigBase{
		Pin: []string{"m+", "m-"}, // 电机正负极
		ValueInit: []any{
			float64(12),    // 0: 额定电压(V)
			float64(1000),  // 1: 额定转速(RPM)
			float64(0.1),   // 2: 电枢电阻(Ω)
			float64(0.01),  // 3: 电枢电感(H)
			float64(0.05),  // 4: 转矩常数(N·m/A)
			float64(0.001), // 5: 转动惯量(kg·m²)
			float64(0.01),  // 6: 阻尼系数(N·m·s/rad)
			float64(0),     // 7: 当前转速(rad/s)
			float64(0),     // 8: 当前电流(A)
			float64(0),     // 9: 当前转矩(N·m)
		},
		Current:   []int{0},
		OrigValue: []int{7, 8, 9},
	}
}
func (motor *Motor) Init() mna.ValueMNA {
	return mna.NewElementBase(motor.ElementConfigBase)
}
func (Motor) Reset(base mna.ValueMNA) {
	// 初始化状态变量
	base.SetFloat64(7, 0) // 当前转速
	base.SetFloat64(8, 0) // 当前电流
	base.SetFloat64(9, 0) // 当前转矩
}
func (Motor) StartIteration(mna mna.MNA, base mna.ValueMNA) {
	// 计算反电动势
	kt := base.GetFloat64(4)    // 转矩常数
	speed := base.GetFloat64(7) // 当前转速
	backEMF := kt * speed       // 反电动势 = kt * ω

	// 更新电压源值
	mna.UpdateVoltageSource(base.VoltSource(0), backEMF)
}
func (Motor) Stamp(mna mna.MNA, base mna.ValueMNA) {
	// 电枢电阻
	ra := base.GetFloat64(2)
	if ra > 0 {
		mna.StampResistor(base.Nodes(0), base.Nodes(1), ra)
	}

	// 电枢电感
	la := base.GetFloat64(3)
	dt := base.TimeStep()
	if dt > 0 && la > 0 {
		// 使用梯形积分法计算电感补偿
		var compResistance float64
		if base.GoodIterations() > 0 {
			compResistance = 2 * la / dt
		} else {
			compResistance = la / dt
		}
		mna.StampResistor(base.Nodes(0), base.Nodes(1), compResistance)
		base.SetFloat64(10, compResistance) // 存储补偿电阻
	}

	// 反电动势电压源
	mna.StampVoltageSource(base.Nodes(0), base.Nodes(1), base.VoltSource(0), 0)
}
func (Motor) DoStep(mna mna.MNA, base mna.ValueMNA) {
	// 获取电枢电压
	va := mna.GetNodeVoltage(base.Nodes(0)) - mna.GetNodeVoltage(base.Nodes(1))

	// 计算反电动势
	kt := base.GetFloat64(4)
	speed := base.GetFloat64(7)
	backEMF := kt * speed

	// 计算电枢电流
	ra := base.GetFloat64(2)
	var ia float64
	if ra > 0 {
		ia = (va - backEMF) / ra
	} else {
		ia = 0
	}
	base.SetFloat64(8, ia)

	// 计算电磁转矩
	torque := kt * ia
	base.SetFloat64(9, torque)

	// 机械方程：J*dω/dt + B*ω = T - T_load
	// 简化：使用欧拉法更新转速
	j := base.GetFloat64(5) // 转动惯量
	b := base.GetFloat64(6) // 阻尼系数
	dt := base.TimeStep()

	if dt > 0 && j > 0 {
		// 计算加速度
		acceleration := (torque - b*speed) / j
		// 更新转速
		newSpeed := speed + acceleration*dt
		base.SetFloat64(7, newSpeed)
	}

	// 更新电流源（如果需要）
	mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), ia)
}
func (Motor) CalculateCurrent(mna mna.MNA, base mna.ValueMNA) {
	// 电流已经在DoStep中计算
	ia := base.GetFloat64(8)
	mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), -ia)
}
func (Motor) StepFinished(mna mna.MNA, base mna.ValueMNA) {
	// 检查转速是否在合理范围内
	speed := base.GetFloat64(7)
	ratedSpeed := base.GetFloat64(1)               // RPM
	ratedSpeedRad := ratedSpeed * 2 * 3.14159 / 60 // 转换为rad/s

	if speed > 1.5*ratedSpeedRad {
		// 转速过高，限制
		base.SetFloat64(7, 1.5*ratedSpeedRad)
	}
}
