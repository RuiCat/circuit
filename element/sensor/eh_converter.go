// 电气-液压转换器(EH)元件实现，将电压信号转换为油路压力。
package sensor

import (
	"circuit/element"
	"circuit/mna"
)

// EHType 电气-液压转换器类型标识(NodeType=45)，网表"EH<name> <in+,in-> <pout+,pout-> [gain,offset]"。
var EHType element.NodeType

// EHConverter 电气-液压比例转换器。VCVS模型: Pout = gain·(Vin+ - Vin-) + offset。
// 默认gain=1e6Pa/V(1V→1MPa)，输入端高阻抗隔离。
type EHConverter struct{ *element.Config }

func init() {
	EHType = element.AddElement(45, &EHConverter{
		&element.Config{
			Name: "eh",
			Pin: []element.Pin{
				{Name: "in+", Type: element.PinLowVoltage},
				{Name: "in-", Type: element.PinLowVoltage},
				{Name: "pout+", Type: element.PinHydraulic},
				{Name: "pout-", Type: element.PinHydraulic},
			},
			ValueInit: []any{
				float64(1e6),
				float64(0),
			},
			ValueName: []string{"gain", "offset"},
			Voltage:   []string{"vout"},
		},
	})
}

// Stamp 电气-液压转换器MNA矩阵加盖。输入端高阻抗(1e16Ω)隔离，
// VCVS将输入端压差×gain驱动到输出端(pout+/pout-)电压源。
func (eh *EHConverter) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	mna.StampImpedance(-1, value.GetNodes(0), 1e16)
	mna.StampImpedance(-1, value.GetNodes(1), 1e16)
	gain := value.GetFloat64(0)
	mna.StampVCVS(
		value.GetNodes(2), value.GetNodes(3),
		value.GetNodes(0), value.GetNodes(1),
		value.GetVoltSource(0), gain,
	)
}

// DoStep 叠加offset偏置到输出电压源。
func (eh *EHConverter) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	offset := value.GetFloat64(1)
	if offset != 0 {
		mna.IncrementVoltageSource(value.GetVoltSource(0), offset)
	}
}
