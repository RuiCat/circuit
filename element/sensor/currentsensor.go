// 电流传感器元件实现，测量支路电流并转换为电压信号。
// 内部使用0V测量电压源+CCVS模式。
package sensor

import (
	"circuit/element"
	"circuit/mna"
)

// CSType 电流传感器类型标识(NodeType=46)，网表"CS<name> <in+,in-> <out+,out-> [gain,offset]"。
var CurrentSensorType element.NodeType

// CurrentSensor 电流传感器。内部vmeas(0V电压源)串入待测支路测量电流，
// 通过CCVS输出: Vout = gain·I_measured + offset。默认gain=1V/A。
type CurrentSensor struct{ *element.Config }

func init() {
	CurrentSensorType = element.AddElement(46, &CurrentSensor{
		&element.Config{
			Name: "cs",
			Pin:  element.SetPin(element.PinLowVoltage, "in+", "in-", "out+", "out-"),
			ValueInit: []any{
				float64(1),
				float64(0),
			},
			ValueName: []string{"gain", "offset"},
			Voltage:   []string{"vmeas", "vout"},
		},
	})
}

// Stamp 电流传感器MNA矩阵加盖。0V电压源(vmeas)串入测量支路，
// CCVS将测量电流×gain驱动到输出电压源(vout)。
func (cs *CurrentSensor) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	mna.StampVoltageSource(value.GetNodes(0), value.GetNodes(1), value.GetVoltSource(0), 0)
	gain := value.GetFloat64(0)
	mna.StampCCVS(value.GetNodes(2), value.GetNodes(3), value.GetVoltSource(0), value.GetVoltSource(1), gain)
}

// DoStep 叠加offset偏置到输出电压源。
func (cs *CurrentSensor) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	offset := value.GetFloat64(1)
	if offset != 0 {
		mna.IncrementVoltageSource(value.GetVoltSource(1), offset)
	}
}
