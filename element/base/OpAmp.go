package base

import (
	"circuit/element"
	"circuit/mna"
	"math"
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
		Voltage:   []string{"ov"},
		Current:   []int{4},
		OrigValue: []int{3, 4, 5, 6},
	},
})

// OpAmp 运算放大器（基于文章反正切非线性模型的实现）
type OpAmp struct{ *element.Config }

func (OpAmp) Stamp(mna mna.MNA, time mna.Time, value element.NodeFace) {
	// 输入引脚连接到高阻抗（大电阻到地）
	mna.StampResistor(-1, value.GetNodes(0), 1e16)
	mna.StampResistor(-1, value.GetNodes(1), 1e16)
	// 输出引脚通过电压源连接到地
	mna.StampVoltageSource(value.GetNodes(2), -1, value.GetVoltSource(0), 0)
}

func (OpAmp) DoStep(mna mna.MNA, time mna.Time, value element.NodeFace) {
	// 获取节点电压
	vp := mna.GetNodeVoltage(value.GetNodes(0)) // 同相输入电压 (Vp)
	vn := mna.GetNodeVoltage(value.GetNodes(1)) // 反相输入电压 (Vn)
	out := value.GetFloat64(3)                  // 输出电压
	// 计算输入电压差
	vd := vp - vn
	vdabs := math.Abs(vd)
	// 优化：动态计算增益，基于开环增益和电压差
	// 使用开环增益G，但限制最大增益以避免数值问题
	G := value.GetFloat64(2) // 开环增益

	// 动态增益计算：当vd很小时使用较大增益，vd较大时使用较小增益
	// 这有助于快速收敛同时保持稳定性
	var gain float64
	if vdabs < 1e-6 {
		// 非常小的电压差，使用较大增益加速收敛
		gain = 0.1
	} else if vdabs < 0.01 {
		// 较小电压差，使用中等增益
		gain = 0.05
	} else {
		// 较大电压差，使用较小增益保持稳定
		gain = 0.01
	}

	// 进一步优化：根据开环增益调整
	// 但限制增益范围避免数值问题
	maxGain := 0.5
	if G > 0 {
		adjustedGain := math.Min(0.001*G, maxGain)
		gain = math.Max(gain, adjustedGain)
	}
	// 判断是否收敛
	if vdabs > 1e-9 {
		time.Converged() // 标记为未收敛
	}
	out += vd * gain
	// 更新电压，限制在摆幅范围内
	out = math.Min(out, value.GetFloat64(0))
	out = math.Max(out, value.GetFloat64(1))
	mna.UpdateVoltageSource(value.GetVoltSource(0), out)
	value.SetFloat64(3, out)

	// 保存小信号增益用于调试
	value.SetFloat64(5, gain)
}

func (OpAmp) CalculateCurrent(mna mna.MNA, time mna.Time, value element.NodeFace) {
	// 电压源的支路电流即为运放输出电流
	iout := mna.GetNodeCurrent(value.GetVoltSource(0))
	value.SetFloat64(4, iout)
}
