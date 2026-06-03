// JFET结型场效应管元件实现，基于平方律模型。支持NJF/PJF。
package semiconductor

import (
	"circuit/element"
	"circuit/mna"
	"math"
)

// JFETType 结型场效应管类型标识(NodeType=37)，网表"J<name> <drain,gate,source> [type,VTO,BETA,LAMBDA]"。
var JFETType element.NodeType = element.AddElement(37, &JFET{
	&element.Config{
		Name: "j",
		Pin:  element.SetPin(element.PinLowVoltage, "drain", "gate", "source"),
		ValueInit: []any{
			int(0),          // 0: 类型 type (0=NJF, 1=PJF)
			float64(2.0),    // 1: 夹断电压绝对值 VTO (V)
			float64(1e-4),   // 2: 跨导参数 BETA (A/V²)
			float64(0.02),   // 3: 沟道长度调制系数 LAMBDA (1/V)
			float64(0),      // 4: Vgs (V) (内部存储，带符号)
			float64(0),      // 5: Vds (V) (内部存储，带符号)
			float64(0),      // 6: Ids (A)
			float64(0),      // 7: gm (S)
			float64(0),      // 8: gds (S)
			int(0),          // 9: lastMode (0=截止/1=线性/2=饱和)
		},
		ValueName: []string{"type", "VTO", "BETA", "LAMBDA", "Vgs", "Vds", "Ids", "gm", "gds", "lastMode"},
		Current:   []int{6},
		OrigValue: []int{4, 5, 6, 7, 8, 9},
		Flags:     element.FlagNonlinear,
	},
})

// JFET 结型场效应管。平方律模型：Ids=BETA·(Vgs-VTO)²·(1+λ·Vds)。
// G-S间反偏PN结用大电阻(1e12Ω)近似，DoStep中Newton线性化加盖。
type JFET struct{ *element.Config }

// Stamp 在Gate-Source节点间加盖1e12Ω大电阻模拟反偏PN结。
func (JFET) Stamp(m mna.Mna, time mna.Time, value element.NodeFace) {
	// G-S间加大电阻模拟反偏PN结
	m.StampImpedance(value.GetNodes(1), value.GetNodes(2), 1e12)
}

// Reset 初始化JFET内部状态，清零Vgs/Vds/Ids/gm/gds和模式标志。
func (JFET) Reset(base element.NodeFace) {
	base.SetFloat64(4, 0)
	base.SetFloat64(5, 0)
	base.SetFloat64(6, 0)
	base.SetFloat64(7, 0)
	base.SetFloat64(8, 0)
	base.SetInt(9, 0)
}

// DoStep JFET每步仿真。根据Vgs/Vds判断工作区(截止/线性/饱和)，
// 平方律计算Ids/gm/gds后用Newton线性化加盖MNA矩阵。
func (JFET) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	vD := mna.GetNodeVoltage(value.GetNodes(0))
	vG := mna.GetNodeVoltage(value.GetNodes(1))
	vS := mna.GetNodeVoltage(value.GetNodes(2))

	jfetType := value.GetInt(0)
	vtoAbs := value.GetFloat64(1)
	beta := value.GetFloat64(2)
	lambda := value.GetFloat64(3)

	vgs := vG - vS
	vds := vD - vS

	// PJF: 翻转符号后使用NJF方程
	if jfetType == 1 {
		vgs = -vgs
		vds = -vds
	}

	vgsPrev := value.GetFloat64(4)
	vdsPrev := value.GetFloat64(5)

	if math.Abs(vgs-vgsPrev) > 0.01 || math.Abs(vds-vdsPrev) > 0.01 {
		time.NoConverged()
	}

	value.SetFloat64(4, vgs)
	value.SetFloat64(5, vds)

	var ids, gm, gds float64
	var mode int

	// NJF/PJF统一：使用翻转后的vgs,vds和绝对值VTO
	if vgs <= vtoAbs {
		// 截止区
		ids = 0
		gm = 0
		gds = 0
		mode = 0
	} else {
		vgsMinusVto := vgs - vtoAbs
		if vds < vgsMinusVto {
			// 线性区
			mode = 1
			ids = beta * (2*vgsMinusVto*vds - vds*vds) * (1 + lambda*vds)
			gm = beta * 2 * vds * (1 + lambda*vds)
			gds = beta * (2*vgsMinusVto - 2*vds) * (1+lambda*vds) + beta*(2*vgsMinusVto*vds-vds*vds)*lambda
		} else {
			// 饱和区
			mode = 2
			ids = beta * vgsMinusVto * vgsMinusVto * (1 + lambda*vds)
			gm = beta * 2 * vgsMinusVto * (1 + lambda*vds)
			gds = beta * vgsMinusVto * vgsMinusVto * lambda
		}
	}

	// PJF: Ids取反（电流方向从s→d变为d→s）
	if jfetType == 1 {
		ids = -ids
	}

	value.SetFloat64(6, ids)
	value.SetFloat64(7, gm)
	value.SetFloat64(8, gds)
	value.SetInt(9, mode)

	var iconst float64
	if jfetType == 1 {
		iconst = ids + gm*vgsPrev + gds*vdsPrev
	} else {
		iconst = ids - gm*vgsPrev - gds*vdsPrev
	}

	nodeDrain := value.GetNodes(0)
	nodeGate := value.GetNodes(1)
	nodeSource := value.GetNodes(2)

	mna.StampAdmittance(nodeDrain, nodeSource, gds)
	mna.StampVCCS(nodeDrain, nodeSource, nodeGate, nodeSource, gm)
	mna.StampCurrentSource(nodeSource, nodeDrain, -iconst)
}

// CalculateCurrent 将DoStep中保存的Ids加盖为JFET支路电流源。
func (JFET) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	ids := value.GetFloat64(6)
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(2), -ids)
}
