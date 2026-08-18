package magnetic

import (
	"circuit/element"
	"circuit/mna"
	"math"
)

// TransformerType 定义
var TransformerType element.NodeType = element.AddElement(8, &Transformer{
	&element.Config{
		Name: "xfmr",
		Pin:  element.SetPin(element.PinLowVoltage, "p1", "p2", "s1", "s2"),
		ValueInit: []any{
			float64(4),     // 0: L1
			float64(1),     // 1: Ratio (N2/N1)
			float64(0.999), // 2: k
			float64(0),     // 3: G11 (a1)
			float64(0),     // 4: G12 (a2)
			float64(0),     // 5: G21 (a3)
			float64(0),     // 6: G22 (a4)
			float64(0),     // 7: I_hist1
			float64(0),     // 8: I_hist2
			float64(0),     // 9: I1 一次侧电流（CalculateCurrent 写入）
			float64(0),     // 10: I2 二次侧电流（CalculateCurrent 写入）
		},
		ValueName: []string{"L1", "Ratio", "k", "G11", "G12", "G21", "G22", "I_hist1", "I_hist2", "I1", "I2"},
		Current:   []int{9, 10},
	},
})

// Transformer 变压器元件，使用梯形积分法建模。
// 通过耦合系数 k 计算互感 M=k*sqrt(L1*L2)，将一次侧和二次侧建模为耦合电感对，
// 构建 4x4 的伴随导纳矩阵 G=[G11 G12; G21 G22] 和 Norton 等效历史电流源。
type Transformer struct{ *element.Config }

// Reset 初始化变压器状态：清零所有历史电流和电导系数。
func (Transformer) Reset(base element.NodeFace) {
	for i := 3; i <= 8; i++ {
		base.SetFloat64(i, 0)
	}
}

// Stamp 加盖变压器的 MNA 贡献。根据 L1/Ratio/k 计算等效电导矩阵，填充自导纳和互导纳项。
func (Transformer) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	l1 := value.GetFloat64(0)
	ratio := value.GetFloat64(1)
	k := value.GetFloat64(2)
	dt := time.TimeStep()

	if l1 < 0 {
		l1 = -l1
	}
	if k > 1.0 {
		k = 1.0
	} else if k < 0 {
		k = 0
	}

	l2 := l1 * ratio * ratio
	m := k * math.Sqrt(l1*l2)
	det := l1*l2 - m*m
	if det < 1e-30 {
		det = 1e-30
	}

	// 梯形积分法系数: G = dt / (2 * L_eq)
	// 后向欧拉法则去掉分母的 2
	factor := dt / (2.0 * det)

	g11 := l2 * factor
	g12 := -m * factor
	g21 := -m * factor
	g22 := l1 * factor

	// 存储电导系数供 StartIteration 使用
	value.SetFloat64(3, g11)
	value.SetFloat64(4, g12)
	value.SetFloat64(5, g21)
	value.SetFloat64(6, g22)

	// 填充 MNA 矩阵 (等效电导)
	mna.StampAdmittance(value.GetNodes(0), value.GetNodes(1), g11)
	mna.StampVCCS(value.GetNodes(0), value.GetNodes(1), value.GetNodes(2), value.GetNodes(3), g12)
	mna.StampVCCS(value.GetNodes(2), value.GetNodes(3), value.GetNodes(0), value.GetNodes(1), g21)
	mna.StampAdmittance(value.GetNodes(2), value.GetNodes(3), g22)
}

// StartIteration 迭代开始时更新历史电流源。根据梯形法用当前电压和电流更新 I_hist 供下一步使用。
func (t Transformer) StartIteration(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 获取当前电压差
	v1 := mna.GetNodeVoltage(value.GetNodes(0)) - mna.GetNodeVoltage(value.GetNodes(1))
	v2 := mna.GetNodeVoltage(value.GetNodes(2)) - mna.GetNodeVoltage(value.GetNodes(3))

	g11 := value.GetFloat64(3)
	g12 := value.GetFloat64(4)
	g21 := value.GetFloat64(5)
	g22 := value.GetFloat64(6)

	// 计算当前支路电流: i = G*v + I_hist
	i1 := g11*v1 + g12*v2 + value.GetFloat64(7)
	i2 := g21*v1 + g22*v2 + value.GetFloat64(8)

	// 更新下一时刻的历史电流源 (梯形法: I_hist_next = I_current + G*V_current)
	value.SetFloat64(7, i1+g11*v1+g12*v2)
	value.SetFloat64(8, i2+g21*v1+g22*v2)
}

// CalculateCurrent 计算一次/二次侧实时支路电流：i = G·v + I_hist。
func (Transformer) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	v1 := mna.GetNodeVoltage(value.GetNodes(0)) - mna.GetNodeVoltage(value.GetNodes(1))
	v2 := mna.GetNodeVoltage(value.GetNodes(2)) - mna.GetNodeVoltage(value.GetNodes(3))
	g11 := value.GetFloat64(3)
	g12 := value.GetFloat64(4)
	g21 := value.GetFloat64(5)
	g22 := value.GetFloat64(6)
	value.SetFloat64(9, g11*v1+g12*v2+value.GetFloat64(7)) // 一次侧电流
	value.SetFloat64(10, g21*v1+g22*v2+value.GetFloat64(8)) // 二次侧电流
}

// DoStep 将历史电流项加盖到 RHS 向量中，完成 Norton 等效电流源注入。
func (Transformer) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 将历史电流项加盖到 RHS 向量中
	mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), value.GetFloat64(7))
	mna.StampCurrentSource(value.GetNodes(2), value.GetNodes(3), value.GetFloat64(8))
}
