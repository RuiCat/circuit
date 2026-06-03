// MOSFET场效应管元件实现，基于Shichman-Hodges LEVEL=1模型。
// 支持NMOS/PMOS，3引脚(drain/gate/source)，Newton-Raphson线性化加盖。
package semiconductor

import (
	"circuit/element"
	"circuit/mna"
	"math"
)

// MOSFETType 场效应管类型标识(NodeType=36)，网表"M<name> <drain,gate,source> [type,VTO,KP,LAMBDA]"。
var MOSFETType element.NodeType = element.AddElement(36, &MOSFET{
	&element.Config{
		Name: "m",
		Pin:  element.SetPin(element.PinLowVoltage, "drain", "gate", "source"),
		ValueInit: []any{
			int(0),          // 0: 类型 type (0=NMOS, 1=PMOS)
			float64(1.5),    // 1: 阈值电压 VTO (V)
			float64(2e-5),   // 2: 跨导参数 KP (A/V²)
			float64(0.01),   // 3: 沟道长度调制系数 LAMBDA (1/V)
			float64(0),      // 4: Vgs (V)
			float64(0),      // 5: Vds (V)
			float64(0),      // 6: Ids (A)
			float64(0),      // 7: gm (S)
			float64(0),      // 8: gds (S)
			int(0),          // 9: lastMode (0=截止/1=线性/2=饱和)
		},
		ValueName: []string{"type", "VTO", "KP", "LAMBDA", "Vgs", "Vds", "Ids", "gm", "gds", "lastMode"},
		Current:   []int{6},
		OrigValue: []int{4, 5, 6, 7, 8, 9},
		Flags:     element.FlagNonlinear,
	},
})

// MOSFET 场效应管。Shichman-Hodges模型三区：截止(Vgs≤VTO)、线性(Vds<Vgs-VTO)、饱和。
// DoStep中每Newton迭代重新计算Ids/gm/gds，用StampAdmittance+StampVCCS+StampCurrentSource线性化加盖。
type MOSFET struct{ *element.Config }

// Reset 初始化MOSFET内部状态，清零Vgs/Vds/Ids/gm/gds和模式标志。
func (MOSFET) Reset(base element.NodeFace) {
	mosType := base.GetInt(0)
	if mosType == 1 {
		// PMOS: VTO默认为绝对值1.5，实际阈值电压为-1.5
		if base.GetFloat64(1) == 1.5 {
			base.SetFloat64(1, 1.5)
		}
	}
	base.SetFloat64(4, 0)
	base.SetFloat64(5, 0)
	base.SetFloat64(6, 0)
	base.SetFloat64(7, 0)
	base.SetFloat64(8, 0)
	base.SetInt(9, 0)
}

// DoStep MOSFET每步仿真。根据Vgs/Vds判断工作区，计算Ids/gm/gds后用
// Newton线性化加盖MNA矩阵（导纳+VCCS+电流源）。
func (MOSFET) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	vD := mna.GetNodeVoltage(value.GetNodes(0))
	vG := mna.GetNodeVoltage(value.GetNodes(1))
	vS := mna.GetNodeVoltage(value.GetNodes(2))

	mosType := value.GetInt(0)
	vto := value.GetFloat64(1)
	kp := value.GetFloat64(2)
	lambda := value.GetFloat64(3)

	vgs := vG - vS
	vds := vD - vS

	// PMOS: 翻转符号后使用NMOS方程
	if mosType == 1 {
		vgs = -vgs
		vds = -vds
	}

	vgsPrev := value.GetFloat64(4)
	vdsPrev := value.GetFloat64(5)

	// 收敛检查
	if math.Abs(vgs-vgsPrev) > 0.01 || math.Abs(vds-vdsPrev) > 0.01 {
		time.NoConverged()
	}

	value.SetFloat64(4, vgs)
	value.SetFloat64(5, vds)

	var ids, gm, gds float64
	var mode int

	if vgs <= vto {
		// 截止区: Vgs不超过阈值，沟道未形成，Ids≈0
		ids = 0
		gm = 0
		gds = 0
		mode = 0
	} else if vds < vgs-vto {
		// 线性区(三极管区): Vds<Vgs-Vto，Ids随Vds线性增长
		mode = 1
		ids = kp * (vgs - vto - vds/2) * vds * (1 + lambda*vds)
		gm = kp * vds * (1 + lambda*vds)
		gds = kp * (vgs - vto - vds) * (1 + 2*lambda*vds)
	} else {
		// 饱和区(恒流区): Vds≥Vgs-Vto，Ids近似恒定仅受λ调制
		mode = 2
		vgsMinusVto := vgs - vto
		ids = kp / 2 * vgsMinusVto * vgsMinusVto * (1 + lambda*vds)
		gm = kp * vgsMinusVto * (1 + lambda*vds)
		gds = kp / 2 * vgsMinusVto * vgsMinusVto * lambda
	}

	if mosType == 1 {
		ids = -ids
	}

	value.SetFloat64(6, ids)
	value.SetFloat64(7, gm)
	value.SetFloat64(8, gds)
	value.SetInt(9, mode)

	var iconst float64
	if mosType == 1 {
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

// CalculateCurrent 将DoStep中保存的Ids加盖为MOSFET支路电流源。
func (MOSFET) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	ids := value.GetFloat64(6)
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(2), -ids)
}
