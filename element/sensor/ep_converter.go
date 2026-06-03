// 电气-气动转换器(EP)元件实现，将电压信号转换为气路压力。
package sensor

import (
	"circuit/element"
	"circuit/mna"
)

// EPType 电气-气动转换器类型标识(NodeType=44)，网表"EP<name> <in+,in-> <pout+,pout-> [gain,offset]"。
var EPType element.NodeType

// EPConverter 电气-气动比例转换器。VCVS模型: Pout = gain·(Vin+ - Vin-) + offset。
// 默认gain=100000Pa/V(1V→100kPa≈1atm)，输入端高阻抗隔离。
type EPConverter struct{ *element.Config }

func init() {
	EPType = element.AddElement(44, &EPConverter{
		&element.Config{
			Name: "ep",
			Pin: []element.Pin{
				{Name: "in+", Type: element.PinLowVoltage},
				{Name: "in-", Type: element.PinLowVoltage},
				{Name: "pout+", Type: element.PinPneumatic},
				{Name: "pout-", Type: element.PinPneumatic},
			},
			ValueInit: []any{
				float64(100000),
				float64(0),
			},
			ValueName: []string{"gain", "offset"},
			Voltage:   []string{"vout"},
		},
	})
}

// Stamp 电气-气动转换器MNA矩阵加盖。输入端高阻抗(1e16Ω)隔离，
// VCVS将输入端压差×gain驱动到输出端(pout+/pout-)电压源。
func (ep *EPConverter) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
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
func (ep *EPConverter) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	offset := value.GetFloat64(1)
	if offset != 0 {
		mna.IncrementVoltageSource(value.GetVoltSource(0), offset)
	}
}
