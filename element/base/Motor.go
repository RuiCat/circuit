package base

import (
	"circuit/element"
	"circuit/mna"
	"math"
)

// MotorType 定义元件
var MotorType element.NodeType = element.AddElement(4, &Motor{
	&element.Config{
		Name: "motor",
		Pin:  element.SetPin(element.PinLowVoltage, "m+", "m-"),
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
			float64(0),     // 10: 电感补偿电阻
			float64(0),     // 11: 电感电流源值
		},
		ValueName: []string{"V_rated", "RPM_rated", "Ra", "La", "Kt", "J", "B", "speed", "current", "torque", "comp_R", "comp_I"},
		Current:   []int{0},
		OrigValue: []int{7, 8, 9, 10, 11},
	},
})

// Motor 电机（直流电机实现）
type Motor struct{ *element.Config }

func (Motor) Reset(base element.NodeFace) {
	// 初始化状态变量
	base.SetFloat64(7, 0)  // 当前转速
	base.SetFloat64(8, 0)  // 当前电流
	base.SetFloat64(9, 0)  // 当前转矩
	base.SetFloat64(10, 0) // 电感补偿电阻
	base.SetFloat64(11, 0) // 电感电流源值
}

func (Motor) StartIteration(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 计算反电动势
	kt := value.GetFloat64(4)    // 转矩常数
	speed := value.GetFloat64(7) // 当前转速
	backEMF := kt * speed        // 反电动势 = kt * ω

	// 更新电压源值
	mna.UpdateVoltageSource(value.GetVoltSource(0), backEMF)

	// 如果是梯形积分法，计算电感电流源值
	dt := time.TimeStep()
	if dt > 0 {
		compResistance := value.GetFloat64(10)
		if compResistance > 0 {
			v1 := mna.GetNodeVoltage(value.GetNodes(0))
			v2 := mna.GetNodeVoltage(value.GetNodes(1))
			voltdiff := v1 - v2
			current := mna.GetVoltageSourceCurrent(value.GetVoltSource(0))
			curSourceValue := voltdiff/compResistance + current
			value.SetFloat64(11, curSourceValue)
		}
	}
}

func (Motor) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 电枢电阻
	ra := value.GetFloat64(2)
	if ra > 0 {
		mna.StampImpedance(value.GetNodes(0), value.GetNodes(1), ra)
	}

	// 电枢电感
	la := value.GetFloat64(3)
	dt := time.TimeStep()
	if dt > 0 && la > 0 {
		// 计算补偿电阻
		var compResistance float64
		if time.GoodIterations() > 0 { // 使用梯形积分法
			compResistance = 2 * la / dt
		} else { // 使用后向欧拉法
			compResistance = la / dt
		}
		value.SetFloat64(10, compResistance)
		mna.StampImpedance(value.GetNodes(0), value.GetNodes(1), compResistance)
	}

	// 反电动势电压源
	mna.StampVoltageSource(value.GetNodes(0), value.GetNodes(1), value.GetVoltSource(0), 0)
}

func (Motor) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 电感电流源（伴随模型）
	curSourceValue := value.GetFloat64(11)
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), curSourceValue)
}

func (Motor) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 获取电枢电压
	va := mna.GetNodeVoltage(value.GetNodes(0)) - mna.GetNodeVoltage(value.GetNodes(1))

	// 反电动势
	kt := value.GetFloat64(4)
	speed := value.GetFloat64(7)
	backEMF := kt * speed

	// 电枢电流
	ra := value.GetFloat64(2)
	var ia float64
	if ra > 0 {
		ia = (va - backEMF) / ra
	}
	value.SetFloat64(8, ia) // 存储电流

	// 电感电流
	compResistance := value.GetFloat64(10)
	curSourceValue := value.GetFloat64(11)
	var iInductor float64
	if compResistance > 0 {
		voltdiff := va
		iInductor = voltdiff/compResistance + curSourceValue
	}

	// 加盖总电流（仅一次）
	totalCurrent := ia + iInductor
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), -totalCurrent)
}

func (Motor) StepFinished(mna mna.Mna, time mna.Time, value element.NodeFace) {
	kt := value.GetFloat64(4)
	j := value.GetFloat64(5)
	b := value.GetFloat64(6)
	dt := time.TimeStep()

	// 使用 CalculateCurrent 中已计算的电枢电流
	ia := value.GetFloat64(8)

	// 计算转矩
	torque := kt * ia
	value.SetFloat64(9, torque)

	// 更新速度（显式欧拉法）
	speed := value.GetFloat64(7)
	if dt > 0 && j > 0 {
		acceleration := (torque - b*speed) / j
		newSpeed := speed + acceleration*dt
		value.SetFloat64(7, newSpeed)
	}

	// 限制转速
	finalSpeed := value.GetFloat64(7)
	ratedSpeed := value.GetFloat64(1)
	ratedSpeedRad := ratedSpeed * 2 * math.Pi / 60
	if finalSpeed > 1.5*ratedSpeedRad {
		value.SetFloat64(7, 1.5*ratedSpeedRad)
	}
}
