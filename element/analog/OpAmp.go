package analog

import (
	"circuit/element"
	MNA "circuit/mna"
)

// OpAmpType 定义元件
var OpAmpType element.NodeType = element.AddElement(5, &OpAmp{
	&element.Config{
		Name: "opamp",
		Pin:  element.SetPin(element.PinLowVoltage, "Vp", "Vn", "Vout"),
		ValueInit: []any{
			float64(15),  // 0: Vmax 正最大输出摆幅
			float64(-15), // 1: Vmin 负最大输出摆幅
			float64(1e5), // 2: G 开环增益（典型值）
			float64(0),   // 3: lastVD 上一次的输入电压差
			float64(0),   // 4: Iout 输出电流
			float64(0),   // 5: g 小信号增益
			float64(0),   // 6: VoutCalc 非线性模型计算的输出电压
		},
		ValueName: []string{"Vmax", "Vmin", "G", "lastVD", "Iout", "g", "VoutCalc"},
		Voltage:   []string{"ov"},
		Current:   []int{4},
		OrigValue: []int{3, 4, 5, 6},
	},
})

// OpAmp 运算放大器（基于文章反正切非线性模型的实现）
type OpAmp struct{ *element.Config }

func (OpAmp) Stamp(mna MNA.Mna, time MNA.Time, value element.NodeFace) {
	// 输入引脚连接到高阻抗（大电阻到地）
	mna.StampImpedance(-1, value.GetNodes(0), 1e16)
	mna.StampImpedance(-1, value.GetNodes(1), 1e16)
	// 输出引脚通过压控电压源连接
	// 注意：当前模型为理想线性VCVS，未实现输出饱和限制（Vmax/Vmin）
	// 在负反馈电路中会正确收敛，开环大信号时输出电压可能无界
	gain := value.GetFloat64(2)
	mna.StampVCVS(
		value.GetNodes(2), -1, // Vout to Gnd
		value.GetNodes(0), value.GetNodes(1), // controlled by Vp - Vn
		value.GetVoltSource(0), gain,
	)
}

func (OpAmp) DoStep(mna MNA.Mna, time MNA.Time, value element.NodeFace) {
	// 获取节点电压
	vp := mna.GetNodeVoltage(value.GetNodes(0)) // 同相输入电压 (Vp)
	vn := mna.GetNodeVoltage(value.GetNodes(1)) // 反相输入电压 (Vn)

	// 计算输入电压差
	vd := vp - vn
	value.SetFloat64(3, vd)

	// 计算理想输出电压并进行饱和钳位
	gain := value.GetFloat64(2)
	vMax := value.GetFloat64(0)
	vMin := value.GetFloat64(1)

	vOutIdeal := gain * vd
	vOut := vOutIdeal
	if vOut > vMax {
		vOut = vMax
	} else if vOut < vMin {
		vOut = vMin
	}
	value.SetFloat64(6, vOut)

	// 如果在饱和区，将 VCVS 方程（Vout = gain*Vd）修改为钳位电压源（Vout = Vclamped）
	if vOut != vOutIdeal {
		// 防御性重建 VCVS 方程恢复正确增益系数，确保退出饱和时矩阵正确（不依赖 MarkStamp）
		mna.StampVCVS(
			value.GetNodes(2), -1,
			value.GetNodes(0), value.GetNodes(1),
			value.GetVoltSource(0), gain,
		)
		// 计算电压源在 MNA 扩展矩阵中的行号
		vsRow := MNA.NodeID(int(value.GetVoltSource(0)) + mna.GetNodeNum())
		// 清零 VCVS 方程中控制电压节点的系数
		mna.StampMatrixSet(vsRow, value.GetNodes(0), 0) // 清零 Vp 的 -gain 项
		mna.StampMatrixSet(vsRow, value.GetNodes(1), 0) // 清零 Vn 的 +gain 项
		// 将方程右侧设置为钳位电压
		mna.UpdateVoltageSource(value.GetVoltSource(0), vOut)
	}
}

func (OpAmp) CalculateCurrent(mna MNA.Mna, time MNA.Time, value element.NodeFace) {
	// 电压源的支路电流即为运放输出电流
	iout := mna.GetVoltageSourceCurrent(value.GetVoltSource(0))
	value.SetFloat64(4, iout)
}
