package element

import (
	"circuit/mna"
	"circuit/utils"
	"fmt"
	"strings"
	"time"
)

// Pin 引脚。
type Pin struct {
	Name string  // 引脚名称。
	Type PinType // 节点索引类型。
}

// SetPin 设置引脚。
func SetPin(pinType PinType, pin ...string) (pins []Pin) {
	pins = make([]Pin, len(pin))
	for i, n := range pin {
		pins[i].Name = n
		pins[i].Type = pinType
	}
	return pins
}

// Config 元件配置结构体，存储元件的静态配置信息。
// 这些配置在元件创建时初始化，并在整个仿真过程中保持不变。
type Config struct {
	Name      string   // 元件名称（如 "r" 表示电阻）。
	Pin       []Pin    // 引脚列表，定义元件的外部连接点。
	ValueInit []any    // 初始化数据，存储元件的参数初始值（如电阻值、电压值等）。
	ValueName []string // 参数名称。
	Current   []int    // 电流数据索引，指向ValueInit中存储电流值的索引位置。
	OrigValue []int    // 数据备份索引，指向需要备份/恢复的参数在ValueInit中的索引。
	Voltage   []string // 电压源名称列表，定义元件内部的电压源标识。
	Internal  []string // 内部引脚名称列表，定义元件的内部节点标识。
}

// Base 默认配置信息。
func (config *Config) Base(netlist utils.NetList) (*Config, utils.NetList) { return config, netlist }

// GetName 元件名称。
func (config *Config) GetName() string {
	return strings.ToUpper(config.Name)
}

// Reset 重置元件状态到初始值。
// 将元件的当前值恢复为配置中的初始值，并更新备份数据。
// 参数base: 元件的节点接口，用于访问元件的底层数据。
func (Config) Reset(base NodeFace) {}

// CirLoad 加载元件值（不处理引脚）。
func (Config) CirLoad(node NodeFace, valueStrs utils.NetList) {
	base, config := node.Base(), node.Config()
	for i := 0; i < len(valueStrs) && i < len(config.ValueInit); i++ {
		switch v := config.ValueInit[i].(type) {
		case string:
			base.NodeValue[i] = valueStrs.ParseString(i, v)
		case bool:
			base.NodeValue[i] = valueStrs.ParseBool(i, v)
		case int:
			base.NodeValue[i] = valueStrs.ParseInt(i, v)
		case int8:
			base.NodeValue[i] = valueStrs.ParseInt8(i, v)
		case int16:
			base.NodeValue[i] = valueStrs.ParseInt16(i, v)
		case int32:
			base.NodeValue[i] = valueStrs.ParseInt32(i, v)
		case int64:
			base.NodeValue[i] = valueStrs.ParseInt64(i, v)
		case uint:
			base.NodeValue[i] = valueStrs.ParseUint(i, v)
		case uint16:
			base.NodeValue[i] = valueStrs.ParseUint16(i, v)
		case uint32:
			base.NodeValue[i] = valueStrs.ParseUint32(i, v)
		case uint64:
			base.NodeValue[i] = valueStrs.ParseUint64(i, v)
		case float32:
			base.NodeValue[i] = valueStrs.ParseFloat32(i, v)
		case float64:
			base.NodeValue[i] = valueStrs.ParseFloat64(i, v)
		case complex64:
			base.NodeValue[i] = valueStrs.ParseComplex64(i, v)
		case complex128:
			base.NodeValue[i] = valueStrs.ParseComplex128(i, v)
		case time.Duration:
			base.NodeValue[i] = valueStrs.ParseDuration(i, v)
		case fmt.Stringer:
			base.NodeValue[i] = valueStrs.ParseString(i, v.String())
		default:
			base.NodeValue[i] = valueStrs.ParseString(i, fmt.Sprint(v))
		}
	}
}

// CirExport 导出元件。
func (Config) CirExport(node NodeFace) utils.NetList {
	return utils.FromAnySlice(node.Base().NodeValue)
}

// PinNum 获取元件的外部引脚数量。
// 返回：引脚名称列表的长度。
func (config *Config) PinNum() int { return len(config.Pin) }

// VoltageNum 获取元件内部的电压源数量。
// 返回：电压源名称列表的长度。
func (config *Config) VoltageNum() int { return len(config.Voltage) }

// InternalNum 获取元件的内部节点数量。
// 返回：内部引脚名称列表的长度。
func (config *Config) InternalNum() int { return len(config.Internal) }

// ValueNum 获取元件的参数数量。
// 返回：初始化数据列表的长度，即元件需要存储的参数个数。
func (config *Config) ValueNum() int { return len(config.ValueInit) }

// 以下为空实现方法，为Config结构体提供默认的元件行为。
// 具体元件类型可以通过重写这些方法来实现自定义行为。

// StartIteration 步长迭代开始时的回调（空实现）。
func (Config) StartIteration(mna mna.Mna, time mna.Time, value NodeFace) {}

// Stamp 加盖线性贡献到MNA矩阵（空实现）。
func (Config) Stamp(mna mna.Mna, time mna.Time, value NodeFace) {}

// DoStep 执行仿真步长计算（空实现）。
func (Config) DoStep(mna mna.Mna, time mna.Time, value NodeFace) {}

// CalculateCurrent 计算元件电流（空实现）。
func (Config) CalculateCurrent(mna mna.Mna, time mna.Time, value NodeFace) {}

// StepFinished 步长迭代结束时的回调（空实现）。
func (Config) StepFinished(mna mna.Mna, time mna.Time, value NodeFace) {}
