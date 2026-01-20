package base

import (
	"circuit/element"
	"circuit/mna"
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

func (OpAmp) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 输入引脚连接到高阻抗（大电阻到地）
	mna.StampImpedance(-1, value.GetNodes(0), 1e16)
	mna.StampImpedance(-1, value.GetNodes(1), 1e16)
	// 输出引脚通过压控电压源连接
	gain := value.GetFloat64(2)
	mna.StampVCVS(
		value.GetNodes(2), -1, // Vout to Gnd
		value.GetNodes(0), value.GetNodes(1), // controlled by Vp - Vn
		value.GetVoltSource(0), gain,
	)
}

func (OpAmp) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 获取节点电压
	vp := mna.GetNodeVoltage(value.GetNodes(0)) // 同相输入电压 (Vp)
	vn := mna.GetNodeVoltage(value.GetNodes(1)) // 反相输入电压 (Vn)

	// 计算输入电压差
	vd := vp - vn

	// 更新内部状态值
	// 改为使用VCVS模型后，DoStep不再需要计算和更新电压源
	// MNA求解器会处理，我们只需在这里读取最终状态即可
	vout := mna.GetNodeVoltage(value.GetNodes(2))
	value.SetFloat64(3, vd)
	value.SetFloat64(6, vout)
}

func (OpAmp) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 电压源的支路电流即为运放输出电流
	iout := mna.GetVoltageSourceCurrent(value.GetVoltSource(0))
	value.SetFloat64(4, iout)
}
