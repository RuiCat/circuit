package base

import (
	"circuit/mna"
	"math"
)

const OpAmpType ElementType = 5

// OpAmp 运算放大器（基于文章反正切非线性模型的实现）
// 引脚定义：0-Vp(同相输入)、1-Vn(反相输入)、2-Vout(输出)
// Value参数定义：
// 0: Vmax(正最大输出摆幅, V) 1: Vmin(负最大输出摆幅, V) 2: G(开环电压增益)
// 3: lastVD(上一次的输入电压差Vp-Vn, 用于收敛判断) 4: Iout(输出电流, A)
// 5: g(小信号增益, 动态计算) 6: VoutCalc(非线性模型计算的输出电压, V)
type OpAmp struct{ Base }

// New 初始化运放的配置参数
func (opamp *OpAmp) New() {
	opamp.ElementConfigBase = &mna.ElementConfigBase{
		Pin: []string{"Vp", "Vn", "Vout"}, // 引脚：同相、反相、输出
		ValueInit: []any{
			float64(15),  // 0: Vmax 正最大输出摆幅
			float64(-15), // 1: Vmin 负最大输出摆幅
			float64(1e5), // 2: G 开环增益（典型值）
			float64(0),   // 3: lastVD 上一次的输入电压差
			float64(0),   // 4: Iout 输出电流
			float64(0),   // 5: g 小信号增益
			float64(0),   // 6: VoutCalc 非线性模型计算的输出电压
		},
		Voltage:   []string{"ov"},    // 输出电压源标识
		Current:   []int{4},          // 输出电流的索引
		OrigValue: []int{3, 4, 5, 6}, // 需要保存的原始值索引
	}
}

// Init 初始化元件的MNA基础结构
func (opamp *OpAmp) Init() mna.ValueMNA {
	return mna.NewElementBase(opamp.ElementConfigBase)
}

// Stamp 构建运放的MNA矩阵基础约束
// 参考 v1 版本的简单实现：只建立输出节点与电压源的关系
func (OpAmp) Stamp(mna mna.MNA, base mna.ValueMNA) {
	mna.StampResistor(-1, base.Nodes(0), 1e16)
	mna.StampResistor(-1, base.Nodes(1), 1e16)
	mna.StampVoltageSource(base.Nodes(2), -1, base.VoltSource(0), 0)
}

// DoStep 执行非线性仿真步，优化收敛速度
func (OpAmp) DoStep(mna mna.MNA, base mna.ValueMNA) {
	// 获取节点电压
	vp := mna.GetNodeVoltage(base.Nodes(0)) // 同相输入电压 (Vp)
	vn := mna.GetNodeVoltage(base.Nodes(1)) // 反相输入电压 (Vn)
	out := base.GetFloat64(3)               // 输出电压
	// 计算输入电压差
	vd := vp - vn
	vdabs := math.Abs(vd)
	// 优化：动态计算增益，基于开环增益和电压差
	// 使用开环增益G，但限制最大增益以避免数值问题
	G := base.GetFloat64(2) // 开环增益

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
		base.Converged()
	}
	out -= vd * gain
	// 更新电压，限制在摆幅范围内
	out = math.Min(out, base.GetFloat64(0))
	out = math.Max(out, base.GetFloat64(1))
	mna.UpdateVoltageSource(base.VoltSource(0), out)
	base.SetFloat64(3, out)

	// 保存小信号增益用于调试
	base.SetFloat64(5, gain)
}

// CalculateCurrent 计算运放输出电流（从MNA解中获取电压源的支路电流）
func (OpAmp) CalculateCurrent(mna mna.MNA, base mna.ValueMNA) {
	// 电压源的支路电流即为运放输出电流
	iout := mna.GetVoltageSourceCurrent(base.VoltSource(0))
	base.SetFloat64(4, iout)
}
