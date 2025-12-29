package base

import (
	"circuit/mna"
	"math"
)

// Transistor 晶体管
type Transistor struct{ Base }

func (transistor *Transistor) New() {
	transistor.ElementConfigBase = &mna.ElementConfigBase{
		Pin: []string{"b", "c", "e"}, // 基极、集电极、发射极
		ValueInit: []any{
			bool(false),       // 0: PNP标志 (false=NPN, true=PNP)
			float64(100),      // 1: 电流增益(hFE)
			string("default"), // 2: 模型名称
			float64(0),        // 3: 上次基极-集电极电压
			float64(0),        // 4: 上次基极-发射极电压
			float64(0),        // 5: 临界电压
			float64(0),        // 6: 集电极电流
			float64(0),        // 7: 发射极电流
			float64(0),        // 8: 基极电流
			float64(0),        // 9: 最小电导
		},
		Voltage:   []string{"v1", "v2", "v3"}, // 三个电压源
		Current:   []int{0, 1, 2},             // 基极、集电极、发射极电流
		OrigValue: []int{3, 4, 6, 7, 8},
	}
}
func (transistor *Transistor) Init() mna.ValueMNA {
	return mna.NewElementBase(transistor.ElementConfigBase)
}
func (Transistor) Reset(base mna.ValueMNA) {
	// 初始化状态变量
	base.SetFloat64(3, 0) // lastvbc
	base.SetFloat64(4, 0) // lastvbe
	base.SetFloat64(6, 0) // ic
	base.SetFloat64(7, 0) // ie
	base.SetFloat64(8, 0) // ib

	// 计算临界电压
	thermalVoltage := 0.025865 // 电子热电压 (27°C = 300.15K)
	vcrit := thermalVoltage * math.Log(thermalVoltage/(math.Sqrt(2)*1e-13))
	base.SetFloat64(5, vcrit)

	// 设置最小电导
	base.SetFloat64(9, 1e-12)
}

func (Transistor) DoStep(mna mna.MNA, base mna.ValueMNA) {
	// 从电路节点获取电压
	v1 := mna.GetNodeVoltage(base.Nodes(0)) // 基极
	v2 := mna.GetNodeVoltage(base.Nodes(1)) // 集电极
	v3 := mna.GetNodeVoltage(base.Nodes(2)) // 发射极

	// 确定极性因子
	pnp := base.GetFloat64(0) > 0
	pnpFactor := 1.0
	if pnp {
		pnpFactor = -1.0
	}

	// 计算电压差
	vbc := pnpFactor * (v1 - v2) // 基极-集电极电压
	vbe := pnpFactor * (v1 - v3) // 基极-发射极电压

	// 检查收敛性
	lastvbc := base.GetFloat64(3)
	lastvbe := base.GetFloat64(4)
	if math.Abs(vbc-lastvbc) > 0.01 || math.Abs(vbe-lastvbe) > 0.01 {
		base.Converged()
	}

	// 限制电压步长以保证数值稳定性
	vbc = limitStepTransistor(vbc, lastvbc, base)
	vbe = limitStepTransistor(vbe, lastvbe, base)
	base.SetFloat64(3, vbc)
	base.SetFloat64(4, vbe)

	// SPICE BJT模型参数
	csat := 1e-13   // 默认饱和电流
	vtn := 0.025865 // 热电压

	// 计算发射结电流
	var cbe, gbe float64
	if vbe > -5*vtn {
		evbe := math.Exp(vbe / vtn)
		cbe = csat*(evbe-1) + base.GetFloat64(9)*vbe
		gbe = csat*evbe/vtn + base.GetFloat64(9)
	} else {
		gbe = -csat/vbe + base.GetFloat64(9)
		cbe = gbe * vbe
	}

	// 计算集电结电流
	var cbc, gbc float64
	if vbc > -5*vtn {
		evbc := math.Exp(vbc / vtn)
		cbc = csat*(evbc-1) + base.GetFloat64(9)*vbc
		gbc = csat*evbc/vtn + base.GetFloat64(9)
	} else {
		gbc = -csat/vbc + base.GetFloat64(9)
		cbc = gbc * vbc
	}

	// 计算电流
	beta := base.GetFloat64(1)
	cc := (cbe - cbc) / 1.0    // 简化模型，忽略基区电荷
	cb := cbe/beta + cbc/100.0 // 默认反向beta=100

	// 计算最终电流
	ic := pnpFactor * cc
	ib := pnpFactor * cb
	ie := pnpFactor * (-cc - cb)

	base.SetFloat64(6, ic)
	base.SetFloat64(7, ie)
	base.SetFloat64(8, ib)

	// 计算电导
	gpi := gbe / beta
	gmu := gbc / 100.0
	go_ := gbc
	gm := gbe - go_

	// 计算等效电流源
	ceqbe := pnpFactor * (cc + cb - vbe*(gm+go_+gpi) + vbc*go_)
	ceqbc := pnpFactor * (-cc + vbe*(gm+go_) - vbc*(gmu+go_))

	// 矩阵加盖
	// Node 0 is the base, node 1 the collector, node 2 the emitter.
	mna.StampMatrix(base.Nodes(1), base.Nodes(1), gmu+go_)
	mna.StampMatrix(base.Nodes(1), base.Nodes(0), -gmu+gm)
	mna.StampMatrix(base.Nodes(1), base.Nodes(2), -gm-go_)
	mna.StampMatrix(base.Nodes(0), base.Nodes(0), gpi+gmu)
	mna.StampMatrix(base.Nodes(0), base.Nodes(2), -gpi)
	mna.StampMatrix(base.Nodes(0), base.Nodes(1), -gmu)
	mna.StampMatrix(base.Nodes(2), base.Nodes(0), -gpi-gm)
	mna.StampMatrix(base.Nodes(2), base.Nodes(1), -go_)
	mna.StampMatrix(base.Nodes(2), base.Nodes(2), gpi+gm+go_)

	// 加盖电流源
	mna.StampRightSide(base.VoltSource(0), -ceqbe-ceqbc) // 第一个电压源
	mna.StampRightSide(base.VoltSource(1), ceqbc)        // 第二个电压源
	mna.StampRightSide(base.VoltSource(2), ceqbe)        // 第三个电压源
}
func (Transistor) CalculateCurrent(mna mna.MNA, base mna.ValueMNA) {
	// 存储所有三个端子的电流
	ic := base.GetFloat64(6)
	ib := base.GetFloat64(8)
	ie := base.GetFloat64(7)

	mna.StampCurrentSource(base.Nodes(0), base.Nodes(1), -ib) // 基极电流
	mna.StampCurrentSource(base.Nodes(1), base.Nodes(2), -ic) // 集电极电流
	mna.StampCurrentSource(base.Nodes(2), base.Nodes(0), -ie) // 发射极电流
}
func (Transistor) StepFinished(mna mna.MNA, base mna.ValueMNA) {
	// 检查巨大电流
	ic := base.GetFloat64(6)
	ib := base.GetFloat64(8)
	if math.Abs(ic) > 1e12 || math.Abs(ib) > 1e12 {
		// 电流过大，可能有问题
	}
}

// 辅助函数
func limitStepTransistor(vnew, vold float64, base mna.ValueMNA) float64 {
	vt := 0.025865 // 热电压
	vcrit := base.GetFloat64(5)

	// 应用步长限制以获得数值稳定性
	if vnew > vcrit && math.Abs(vnew-vold) > (vt+vt) {
		if vold > 0 {
			arg := 1 + (vnew-vold)/vt
			if arg > 0 {
				vnew = vold + vt*math.Log(arg)
			} else {
				vnew = vcrit
			}
		} else {
			vnew = vt * math.Log(vnew/vt)
		}
	}
	return vnew
}
