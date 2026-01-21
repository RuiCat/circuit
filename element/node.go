package element

import "circuit/mna"

// Node 元件节点结构体，存储元件的动态数据和连接信息。
// 这些数据在仿真过程中会不断更新，反映元件的当前状态。
type Node struct {
	ConfigPtr    *Config         // 配置项指针。
	NodeType     NodeType        // 元件类型标识，对应ElementLitt中的注册类型。
	NodeValue    []any           // 元件当前数据，存储仿真过程中变化的参数值。
	OrigValue    map[int]any     // 元件数据备份，用于支持回滚操作。
	VoltSource   []mna.VoltageID // 电压索引列表，存储元件内部电压源对应的MNA节点ID。
	Nodes        []mna.NodeID    // 节点索引列表，存储元件引脚对应的MNA节点ID。
	NodeInternal []mna.NodeID    // 内部索引列表，存储元件内部节点对应的MNA节点ID。
}

// Base 获取元件的底层节点结构体指针。
// 返回：指向当前Node结构体的指针，用于访问元件的底层数据。
func (node *Node) Base() *Node {
	return node
}

// Config 获取元件配置信息。
// 返回：元件配置信息。
func (node *Node) Config() *Config {
	return node.ConfigPtr
}

// Type 获取元件的类型标识。
// 返回：元件的NodeType类型值，用于识别元件的具体类型。
func (node *Node) Type() NodeType {
	return node.NodeType
}

// GetNodes 获取指定引脚对应的MNA节点索引。
// 参数i: 引脚索引（0-based）。
// 返回：对应引脚的MNA节点ID，如果索引无效则返回-1。
func (node *Node) GetNodes(i int) mna.NodeID {
	if i >= 0 && i < len(node.Nodes) {
		return node.Nodes[i]
	}
	return -1
}

// GetVoltSource 获取指定电压源对应的MNA节点索引。
// 参数i: 电压源索引（0-based）。
// 返回：对应电压源的MNA节点ID，如果索引无效则返回-1。
func (node *Node) GetVoltSource(i int) mna.VoltageID {
	if i >= 0 && i < len(node.VoltSource) {
		return node.VoltSource[i]
	}
	return -1
}

// GetVoltSourceNodeID 获取指定电压源对应的MNA节点索引。
// 参数i: 电压源索引（0-based）。
// 返回：对应电压源的MNA节点ID，如果索引无效则返回-1。
func (node *Node) GetVoltSourceNodeID(m mna.Mna, i int) mna.NodeID {
	if i >= 0 && i < len(node.VoltSource) {
		return mna.NodeID(m.GetNodeNum()) + mna.NodeID(node.VoltSource[i])
	}
	return -1
}

// GetNodesInternal 获取指定内部节点对应的MNA节点索引。
// 参数i: 内部节点索引（0-based）。
// 返回：对应内部节点的MNA节点ID，如果索引无效则返回-1。
func (node *Node) GetNodesInternal(i int) mna.NodeID {
	if i >= 0 && i < len(node.NodeInternal) {
		return node.NodeInternal[i]
	}
	return -1
}

// SetNodePin 设置指定引脚对应的MNA节点索引。
// 参数i: 引脚索引（0-based）。
// 参数n: 要设置的MNA节点ID。
func (node *Node) SetNodePin(i int, n mna.NodeID) {
	if i >= 0 && i < len(node.Nodes) {
		node.Nodes[i] = n
	}
}

// SetNodePins 设置引脚对应的MNA节点索引。
// 参数n: 引脚连接节点列表。
func (node *Node) SetNodePins(n ...mna.NodeID) {
	if len(n) <= len(node.Nodes) {
		copy(node.Nodes, n)
	}
}

// SetVoltSource 设置指定电压源对应的MNA节点索引。
// 参数i: 电压源索引（0-based）。
// 参数n: 要设置的MNA节点ID。
func (node *Node) SetVoltSource(i int, n mna.VoltageID) {
	if i >= 0 && i < len(node.VoltSource) {
		node.VoltSource[i] = n
	}
}

// SetNodesInternal 设置指定内部节点对应的MNA节点索引。
// 参数i: 内部节点索引（0-based）。
// 参数n: 要设置的MNA节点ID。
func (node *Node) SetNodesInternal(i int, n mna.NodeID) {
	if i >= 0 && i < len(node.NodeInternal) {
		node.NodeInternal[i] = n
	}
}

// Update 更新操作，将当前参数值保存到备份中。
// 用于在仿真迭代中保存当前状态，以便在需要时进行回滚。
func (node *Node) Update() {
	for i := range node.OrigValue {
		node.OrigValue[i] = node.NodeValue[i]
	}
}

// Rollback 回溯操作，将备份的参数值恢复到当前值。
// 用于在仿真迭代失败时回滚到之前保存的状态。
func (node *Node) Rollback() {
	for i := range node.OrigValue {
		node.NodeValue[i] = node.OrigValue[i]
	}
}

// GetInt 获取指定索引处的整数值参数。
// 参数i: 参数索引（0-based）。
// 返回：对应位置的整数值，如果索引无效则返回0。
func (node *Node) GetInt(i int) int {
	if i >= 0 && i < len(node.NodeValue) {
		return node.NodeValue[i].(int)
	}
	return 0
}

// GetBool 获取指定索引处的逻辑值参数。
// 参数i: 参数索引（0-based）。
// 返回：对应位置的逻辑值，如果索引无效则返回false。
func (node *Node) GetBool(i int) bool {
	if i >= 0 && i < len(node.NodeValue) {
		return node.NodeValue[i].(bool)
	}
	return false
}

// GetString 获取指定索引处的字符串参数。
// 参数i: 参数索引（0-based）。
// 返回：对应位置的字符串，如果索引无效则返回空字符串。
func (node *Node) GetString(i int) string {
	if i >= 0 && i < len(node.NodeValue) {
		return node.NodeValue[i].(string)
	}
	return ""
}

// GetFloat64 获取指定索引处的浮点数值参数。
// 参数i: 参数索引（0-based）。
// 返回：对应位置的浮点数值，如果索引无效则返回0。
func (node *Node) GetFloat64(i int) float64 {
	if i >= 0 && i < len(node.NodeValue) {
		return node.NodeValue[i].(float64)
	}
	return 0
}

// SetInt 设置指定索引处的整数值参数。
// 参数i: 参数索引（0-based）。
// 参数v: 要设置的整数值。
func (node *Node) SetInt(i int, v int) {
	if i >= 0 && i < len(node.NodeValue) {
		node.NodeValue[i] = v
	}
}

// SetBool 设置指定索引处的逻辑值参数。
// 参数i: 参数索引（0-based）。
// 参数v: 要设置的逻辑值。
func (node *Node) SetBool(i int, v bool) {
	if i >= 0 && i < len(node.NodeValue) {
		node.NodeValue[i] = v
	}
}

// SetString 设置指定索引处的字符串值参数。
// 参数i: 参数索引（0-based）。
// 参数v: 要设置的字符串值。
func (node *Node) SetString(i int, v string) {
	if i >= 0 && i < len(node.NodeValue) {
		node.NodeValue[i] = v
	}
}

// SetFloat64 设置指定索引处的浮点数值参数。
// 参数i: 参数索引（0-based）。
// 参数v: 要设置的浮点数值。
func (node *Node) SetFloat64(i int, v float64) {
	if i >= 0 && i < len(node.NodeValue) {
		node.NodeValue[i] = v
	}
}
