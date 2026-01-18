package base

import (
	"circuit/element"
	"circuit/mna"
	"math"
)

// TransistorType 定义元件
var TransistorType element.NodeType = element.AddElement(9, &Transistor{
	&element.Config{
		Name: "q",
		Pin:  element.SetPin(element.PinLowVoltage, "b", "c", "e"), // 基极、集电极、发射极
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
			float64(0),        // 电流记录
			float64(0),        // 电流记录
			float64(0),        // 电流记录
		},
		OrigValue: []int{3, 4, 6, 7, 8},
	},
})

// Transistor 晶体管
type Transistor struct{ *element.Config }

func (Transistor) Stamp(m mna.Mna, time mna.Time, value element.NodeFace) {
	// 为结添加最小电导，以确保矩阵在数值上稳定
	gmin := 1e-12
	nodeB := value.GetNodes(0)
	nodeC := value.GetNodes(1)
	nodeE := value.GetNodes(2)

	// 为基极-发射极结加盖gmin
	m.StampMatrix(nodeB, nodeB, gmin)
	m.StampMatrix(nodeE, nodeE, gmin)
	m.StampMatrix(nodeB, nodeE, -gmin)
	m.StampMatrix(nodeE, nodeB, -gmin)

	// 为基极-集电极结加盖gmin
	m.StampMatrix(nodeB, nodeB, gmin)
	m.StampMatrix(nodeC, nodeC, gmin)
	m.StampMatrix(nodeB, nodeC, -gmin)
	m.StampMatrix(nodeC, nodeB, -gmin)
}

func (Transistor) Reset(base element.NodeFace) {
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

func (Transistor) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 从电路节点获取电压
	v1 := mna.GetNodeVoltage(value.GetNodes(0)) // 基极
	v2 := mna.GetNodeVoltage(value.GetNodes(1)) // 集电极
	v3 := mna.GetNodeVoltage(value.GetNodes(2)) // 发射极

	// 确定极性因子
	pnpFactor := 1.0
	if value.GetBool(0) {
		pnpFactor = -1.0
	}

	// 计算电压差
	vbc := pnpFactor * (v1 - v2) // 基极-集电极电压
	vbe := pnpFactor * (v1 - v3) // 基极-发射极电压

	// 检查收敛性
	lastvbc := value.GetFloat64(3)
	lastvbe := value.GetFloat64(4)

	if math.Abs(vbc-lastvbc) > 0.01 || math.Abs(vbe-lastvbe) > 0.01 {
		time.NoConverged()
	}

	// 限制电压步长以保证数值稳定性
	vbc = limitStepTransistor(vbc, lastvbc, value)
	vbe = limitStepTransistor(vbe, lastvbe, value)
	value.SetFloat64(3, vbc)
	value.SetFloat64(4, vbe)

	// SPICE BJT模型参数
	csat := 1e-13   // 默认饱和电流
	vtn := 0.025865 // 热电压

	// 计算发射结电流
	var cbe, gbe float64
	if vbe > -5*vtn {
		evbe := math.Exp(vbe / vtn)
		cbe = csat*(evbe-1) + value.GetFloat64(9)*vbe
		gbe = csat*evbe/vtn + value.GetFloat64(9)
	} else {
		gbe = value.GetFloat64(9)
		cbe = -csat + gbe*vbe
	}

	// 计算集电结电流
	var cbc, gbc float64
	if vbc > -5*vtn {
		evbc := math.Exp(vbc / vtn)
		cbc = csat*(evbc-1) + value.GetFloat64(9)*vbc
		gbc = csat*evbc/vtn + value.GetFloat64(9)
	} else {
		gbc = value.GetFloat64(9)
		cbc = -csat + gbc*vbc
	}

	// 计算电流
	beta := value.GetFloat64(1)
	cc := (cbe - cbc) / 1.0    // 简化模型，忽略基区电荷
	cb := cbe/beta + cbc/100.0 // 默认反向beta=100

	// 计算最终电流
	// 集电极电流是传输电流减去基极-集电极二极管电流。
	ic := pnpFactor * (cc - (cbc / 100.0))
	ib := pnpFactor * cb
	ie := -(ic + ib) // 为保证数值稳定性，强制执行KCL

	value.SetFloat64(6, ic)
	value.SetFloat64(7, ie)
	value.SetFloat64(8, ib)

	// 计算电导
	gpi := gbe / beta
	gmu := gbc / 100.0
	go_ := gbc
	gm := gbe - go_

	// 计算线性化模型的诺顿等效电流源
	// I_eq = I_nonlinear(V_old) - G_linearized * V_old
	ieq_be := cbe - gbe*vbe
	ieq_bc := cbc - gbc*vbc

	// 合并结电流以获得终端等效电流
	// 等效电流计算必须与最终终端电流计算一致
	ic_eq_final := ieq_be - ieq_bc - (ieq_bc / 100.0)
	ib_eq_base := ieq_be/beta + ieq_bc/100.0

	// 应用PNP因子
	ic_eq := pnpFactor * ic_eq_final
	ib_eq := pnpFactor * ib_eq_base
	ie_eq := -(ic_eq + ib_eq) // 强制执行KCL

	// 矩阵加盖
	// 节点0是基极，节点1是集电极，节点2是发射极。
	mna.StampMatrix(value.GetNodes(1), value.GetNodes(1), gmu+go_)
	mna.StampMatrix(value.GetNodes(1), value.GetNodes(0), -gmu+gm)
	mna.StampMatrix(value.GetNodes(1), value.GetNodes(2), -gm-go_)
	mna.StampMatrix(value.GetNodes(0), value.GetNodes(0), gpi+gmu)
	mna.StampMatrix(value.GetNodes(0), value.GetNodes(2), -gpi)
	mna.StampMatrix(value.GetNodes(0), value.GetNodes(1), -gmu)
	mna.StampMatrix(value.GetNodes(2), value.GetNodes(0), -gpi-gm)
	mna.StampMatrix(value.GetNodes(2), value.GetNodes(1), -go_)
	mna.StampMatrix(value.GetNodes(2), value.GetNodes(2), gpi+gm+go_)

	// 将诺顿等效电流源加盖在MNA矩阵的右侧。
	// KCL约定是离开节点的电流总和=0。
	// 流入节点的独立源`I_src`对KCL总和的贡献为`-I_src`。
	// 对于系统A*V = b，此项移至右侧，变为`+I_src`。
	// 我们的`ib_eq`、`ic_eq`定义为流入器件端子。
	// MNA实现似乎期望右侧为负值。
	mna.StampRightSide(value.GetNodes(0), -ib_eq)
	mna.StampRightSide(value.GetNodes(1), -ic_eq)
	mna.StampRightSide(value.GetNodes(2), -ie_eq)
}

// 辅助函数
func limitStepTransistor(vnew, vold float64, value element.NodeFace) float64 {
	vt := 0.025865 // 热电压
	vcrit := value.GetFloat64(5)

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
			// 当电压从非正值大幅跳跃到正值时，
			// 将其钳位在临界电压以开始导通。
			vnew = vcrit
		}
	}
	return vnew
}
