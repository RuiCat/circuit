// 压力传感器元件实现，将气路/液压压力信号转换为电压信号。
// 使用VCVS模式，输入端高阻抗隔离，输出端电压源驱动。
package sensor

import (
	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
	"strconv"
)

// PressureSensorType 压力传感器类型标识(NodeType=43)，网表"PSENS<name> <sense+,sense-> <out+,out-> [gain,offset,domain]"。
var PressureSensorType element.NodeType

// PressureSensor 压力传感器。通过Base()方法根据domain参数(0=气路/1=液压)动态切换引脚类型。
// VCVS模型: Vout = gain·(Psense+ - Psense-) + offset。
type PressureSensor struct{ *element.Config }

func init() {
	PressureSensorType = element.AddElement(43, &PressureSensor{
		&element.Config{
			Name: "psens",
			Pin: []element.Pin{
				{Name: "sense+", Type: element.PinPneumatic},
				{Name: "sense-", Type: element.PinPneumatic},
				{Name: "out+", Type: element.PinLowVoltage},
				{Name: "out-", Type: element.PinLowVoltage},
			},
			ValueInit: []any{
				float64(1e-5),
				float64(0),
				int(0),
			},
			ValueName: []string{"gain", "offset", "domain"},
			Voltage:   []string{"vout"},
		},
	})
}

// Base 根据网表domain参数动态配置引脚类型：0=气路(PinPneumatic)，1=液压(PinHydraulic)。
func (ps *PressureSensor) Base(elem ast.ElementNode) *element.Config {
	domain := 0
	if len(elem.Values) > 2 {
		if v, err := strconv.Atoi(elem.Values[2].Value); err == nil {
			domain = v
		}
	}
	pinType := element.PinPneumatic
	if domain == 1 {
		pinType = element.PinHydraulic
	}
	return &element.Config{
		Name: "psens",
		Pin: []element.Pin{
			{Name: "sense+", Type: pinType},
			{Name: "sense-", Type: pinType},
			{Name: "out+", Type: element.PinLowVoltage},
			{Name: "out-", Type: element.PinLowVoltage},
		},
		ValueInit: ps.Config.ValueInit,
		ValueName: ps.Config.ValueName,
		Voltage:   ps.Config.Voltage,
	}
}

// Stamp 压力传感器MNA矩阵加盖。输入端高阻抗(1e16Ω)隔离，
// VCVS将sense端压差×gain驱动到输出电压源。
func (ps *PressureSensor) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
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
func (ps *PressureSensor) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	offset := value.GetFloat64(1)
	if offset != 0 {
		mna.IncrementVoltageSource(value.GetVoltSource(0), offset)
	}
}
