package base

import "circuit/mna"

const VCVSType ElementType = 10

// VCVS 电压控制电压源
type VCVS struct{ Base }

func (vcvs *VCVS) New() {
	vcvs.ElementConfigBase = &mna.ElementConfigBase{
		Pin: []string{"cp", "cn", "op", "on"}, // 控制正、控制负、输出正、输出负
		ValueInit: []any{
			float64(1), // 0: 增益系数
		},
		Voltage: []string{"v"}, // 电压源
	}
}
func (vcvs *VCVS) Init() mna.ValueMNA {
	return mna.NewElementBase(vcvs.ElementConfigBase)
}

func (VCVS) Stamp(mna mna.MNA, base mna.ValueMNA) {
	// VCVS: V_out = Gain * V_in
	// 控制节点: base.Nodes[0], base.Nodes[1] (输入)
	// 输出节点: base.Nodes[2], base.Nodes[3] (输出)
	gain := base.GetFloat64(0)

	// 使用StampVCVS方法，参数为: 输出正, 输出负, 控制正, 控制负, 电压源ID, 增益
	mna.StampVCVS(base.Nodes(2), base.Nodes(3), base.Nodes(0), base.Nodes(1), base.VoltSource(0), gain)

	// 电压源连接在输出节点之间
	mna.StampVoltageSource(base.Nodes(2), base.Nodes(3), base.VoltSource(0), 0)
}
