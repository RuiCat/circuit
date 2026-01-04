package element

import (
	"circuit/mna"
	"circuit/utils"
	"fmt"
	"time"
)

// Config 元件配置结构体，存储元件的静态配置信息
// 这些配置在元件创建时初始化，并在整个仿真过程中保持不变
type Config struct {
	Name      string   // 元件名称（如 "r" 表示电阻）
	Pin       []string // 引脚名称列表，定义元件的外部连接点
	ValueInit []any    // 初始化数据，存储元件的参数初始值（如电阻值、电压值等）
	Current   []int    // 电流数据索引，指向ValueInit中存储电流值的索引位置
	OrigValue []int    // 数据备份索引，指向需要备份/恢复的参数在ValueInit中的索引
	Voltage   []string // 电压源名称列表，定义元件内部的电压源标识
	Internal  []string // 内部引脚名称列表，定义元件的内部节点标识
}

// GetConfig 获取元件配置的指针
// 返回：指向当前Config结构体的指针，用于访问元件的配置信息
func (config *Config) GetConfig() *Config {
	return config
}

// Reset 重置元件状态到初始值
// 将元件的当前值恢复为配置中的初始值，并更新备份数据
// 参数base: 元件的节点接口，用于访问元件的底层数据
func (config *Config) Reset(base NodeFace) {}

// CirLoad 加载元件
func (config *Config) CirLoad(node NodeFace, val utils.NetList) {
	base := node.Base()
	for i := range len(val) {
		switch v := config.ValueInit[i].(type) {
		case string:
			base.NodeValue[i] = val.ParseString(i, v)
		case bool:
			base.NodeValue[i] = val.ParseBool(i, v)
		case int:
			base.NodeValue[i] = val.ParseInt(i, v)
		case int8:
			base.NodeValue[i] = val.ParseInt8(i, v)
		case int16:
			base.NodeValue[i] = val.ParseInt16(i, v)
		case int32:
			base.NodeValue[i] = val.ParseInt32(i, v)
		case int64:
			base.NodeValue[i] = val.ParseInt64(i, v)
		case uint:
			base.NodeValue[i] = val.ParseUint(i, v)
		case uint16:
			base.NodeValue[i] = val.ParseUint16(i, v)
		case uint32:
			base.NodeValue[i] = val.ParseUint32(i, v)
		case uint64:
			base.NodeValue[i] = val.ParseUint64(i, v)
		case float32:
			base.NodeValue[i] = val.ParseFloat32(i, v)
		case float64:
			base.NodeValue[i] = val.ParseFloat64(i, v)
		case time.Duration:
			base.NodeValue[i] = val.ParseDuration(i, v)
		case fmt.Stringer:
			base.NodeValue[i] = val.ParseString(i, v.String())
		default:
			base.NodeValue[i] = val.ParseString(i, fmt.Sprint(v))
		}
	}
}

// CirExport 导出元件
func (Config) CirExport(node NodeFace) utils.NetList {
	return utils.FromAnySlice(node.Base().NodeValue)
}

// PinNum 获取元件的外部引脚数量
// 返回：引脚名称列表的长度
func (config *Config) PinNum() int { return len(config.Pin) }

// VoltageNum 获取元件内部的电压源数量
// 返回：电压源名称列表的长度
func (config *Config) VoltageNum() int { return len(config.Voltage) }

// InternalNum 获取元件的内部节点数量
// 返回：内部引脚名称列表的长度
func (config *Config) InternalNum() int { return len(config.Internal) }

// ValueNum 获取元件的参数数量
// 返回：初始化数据列表的长度，即元件需要存储的参数个数
func (config *Config) ValueNum() int { return len(config.ValueInit) }

// 以下为空实现方法，为Config结构体提供默认的元件行为
// 具体元件类型可以通过重写这些方法来实现自定义行为

// StartIteration 步长迭代开始时的回调（空实现）
func (Config) StartIteration(mna mna.MNA, time mna.Time, value NodeFace) {}

// Stamp 加盖线性贡献到MNA矩阵（空实现）
func (Config) Stamp(mna mna.MNA, time mna.Time, value NodeFace) {}

// DoStep 执行仿真步长计算（空实现）
func (Config) DoStep(mna mna.MNA, time mna.Time, value NodeFace) {}

// CalculateCurrent 计算元件电流（空实现）
func (Config) CalculateCurrent(mna mna.MNA, time mna.Time, value NodeFace) {}

// StepFinished 步长迭代结束时的回调（空实现）
func (Config) StepFinished(mna mna.MNA, time mna.Time, value NodeFace) {}

// Node 元件节点结构体，存储元件的动态数据和连接信息
// 这些数据在仿真过程中会不断更新，反映元件的当前状态
type Node struct {
	NdoeType      NodeType        // 元件类型标识，对应ElementLitt中的注册类型
	NodeValue     []any           // 元件当前数据，存储仿真过程中变化的参数值
	OrigValue     map[int]any     // 元件数据备份，用于支持回滚操作
	Nodes         []mna.NodeID    // 节点索引列表，存储元件引脚对应的MNA节点ID
	VoltSource    []mna.VoltageID // 电压索引列表，存储元件内部电压源对应的MNA节点ID
	NodesInternal []mna.NodeID    // 内部索引列表，存储元件内部节点对应的MNA节点ID
}

// Base 获取元件的底层节点结构体指针
// 返回：指向当前Node结构体的指针，用于访问元件的底层数据
func (ndoe *Node) Base() *Node {
	return ndoe
}

// Type 获取元件的类型标识
// 返回：元件的NodeType类型值，用于识别元件的具体类型
func (ndoe *Node) Type() NodeType {
	return ndoe.NdoeType
}

// GetNodes 获取指定引脚对应的MNA节点索引
// 参数i: 引脚索引（0-based）
// 返回：对应引脚的MNA节点ID，如果索引无效则返回-1
func (ndoe *Node) GetNodes(i int) mna.NodeID {
	if i >= 0 && i < len(ndoe.Nodes) {
		return ndoe.Nodes[i]
	}
	return -1
}

// GetVoltSource 获取指定电压源对应的MNA节点索引
// 参数i: 电压源索引（0-based）
// 返回：对应电压源的MNA节点ID，如果索引无效则返回-1
func (ndoe *Node) GetVoltSource(i int) mna.VoltageID {
	if i >= 0 && i < len(ndoe.VoltSource) {
		return ndoe.VoltSource[i]
	}
	return -1
}

// GetVoltSourceNodeID 获取指定电压源对应的MNA节点索引
// 参数i: 电压源索引（0-based）
// 返回：对应电压源的MNA节点ID，如果索引无效则返回-1
func (ndoe *Node) GetVoltSourceNodeID(m mna.MNA, i int) mna.NodeID {
	if i >= 0 && i < len(ndoe.VoltSource) {
		return mna.NodeID(m.GetNodeNum()) + mna.NodeID(ndoe.VoltSource[i])
	}
	return -1
}

// GetNodesInternal 获取指定内部节点对应的MNA节点索引
// 参数i: 内部节点索引（0-based）
// 返回：对应内部节点的MNA节点ID，如果索引无效则返回-1
func (ndoe *Node) GetNodesInternal(i int) mna.NodeID {
	if i >= 0 && i < len(ndoe.NodesInternal) {
		return ndoe.NodesInternal[i]
	}
	return -1
}

// SetNodePin 设置指定引脚对应的MNA节点索引
// 参数i: 引脚索引（0-based）
// 参数n: 要设置的MNA节点ID
func (ndoe *Node) SetNodePin(i int, n mna.NodeID) {
	if i >= 0 && i < len(ndoe.Nodes) {
		ndoe.Nodes[i] = n
	}
}

// SetNodePins 设置引脚对应的MNA节点索引
// 参数n: 引脚连接节点列表
func (ndoe *Node) SetNodePins(n ...mna.NodeID) {
	if len(n) <= len(ndoe.Nodes) {
		copy(ndoe.Nodes, n)
	}
}

// SetVoltSource 设置指定电压源对应的MNA节点索引
// 参数i: 电压源索引（0-based）
// 参数n: 要设置的MNA节点ID
func (ndoe *Node) SetVoltSource(i int, n mna.VoltageID) {
	if i >= 0 && i < len(ndoe.VoltSource) {
		ndoe.VoltSource[i] = n
	}
}

// SetNodesInternal 设置指定内部节点对应的MNA节点索引
// 参数i: 内部节点索引（0-based）
// 参数n: 要设置的MNA节点ID
func (ndoe *Node) SetNodesInternal(i int, n mna.NodeID) {
	if i >= 0 && i < len(ndoe.NodesInternal) {
		ndoe.NodesInternal[i] = n
	}
}

// Update 更新操作，将当前参数值保存到备份中
// 用于在仿真迭代中保存当前状态，以便在需要时进行回滚
func (ndoe *Node) Update() {
	for i := range ndoe.OrigValue {
		ndoe.OrigValue[i] = ndoe.NodeValue[i]
	}
}

// Rollback 回溯操作，将备份的参数值恢复到当前值
// 用于在仿真迭代失败时回滚到之前保存的状态
func (ndoe *Node) Rollback() {
	for i := range ndoe.OrigValue {
		ndoe.NodeValue[i] = ndoe.OrigValue[i]
	}
}

// GetInt 获取指定索引处的整数值参数
// 参数i: 参数索引（0-based）
// 返回：对应位置的整数值，如果索引无效则返回0
func (ndoe *Node) GetInt(i int) int {
	if i >= 0 && i < len(ndoe.NodeValue) {
		return ndoe.NodeValue[i].(int)
	}
	return 0
}

// GetBool 获取指定索引处的逻辑值参数
// 参数i: 参数索引（0-based）
// 返回：对应位置的逻辑值，如果索引无效则返回 false
func (ndoe *Node) GetBool(i int) bool {
	if i >= 0 && i < len(ndoe.NodeValue) {
		return ndoe.NodeValue[i].(bool)
	}
	return false
}

// GetBool 获取指定索引处的字符串参数
// 参数i: 参数索引（0-based）
// 返回：对应位置的字符串，如果索引无效则返回空字符串
func (ndoe *Node) GetString(i int) string {
	if i >= 0 && i < len(ndoe.NodeValue) {
		return ndoe.NodeValue[i].(string)
	}
	return ""
}

// GetFloat64 获取指定索引处的浮点数值参数
// 参数i: 参数索引（0-based）
// 返回：对应位置的浮点数值，如果索引无效则返回0
func (ndoe *Node) GetFloat64(i int) float64 {
	if i >= 0 && i < len(ndoe.NodeValue) {
		return ndoe.NodeValue[i].(float64)
	}
	return 0
}

// SetInt 设置指定索引处的整数值参数
// 参数i: 参数索引（0-based）
// 参数v: 要设置的整数值
func (ndoe *Node) SetInt(i int, v int) {
	if i >= 0 && i < len(ndoe.NodeValue) {
		ndoe.NodeValue[i] = v
	}
}

// SetBool 设置指定索引处的逻辑值参数
// 参数i: 参数索引（0-based）
// 参数v: 要设置的整数值
func (ndoe *Node) SetBool(i int, v bool) {
	if i >= 0 && i < len(ndoe.NodeValue) {
		ndoe.NodeValue[i] = v
	}
}

// SetString 设置指定索引处的字符串值参数
// 参数i: 参数索引（0-based）
// 参数v: 要设置的整数值
func (ndoe *Node) SetString(i int, v string) {
	if i >= 0 && i < len(ndoe.NodeValue) {
		ndoe.NodeValue[i] = v
	}
}

// SetFloat64 设置指定索引处的浮点数值参数
// 参数i: 参数索引（0-based）
// 参数v: 要设置的浮点数值
func (ndoe *Node) SetFloat64(i int, v float64) {
	if i >= 0 && i < len(ndoe.NodeValue) {
		ndoe.NodeValue[i] = v
	}
}
