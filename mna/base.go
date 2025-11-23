package mna

// Graph 解析用数据
type Graph struct {
	Nodes         []NodeID // 节点索引
	VoltSource    []NodeID // 电压索引
	NodesInternal []NodeID // 内部索引
}

// SetNodes 设置指定节点索引
func (graph *Graph) SetNodes(i int, n NodeID) {
	if i >= 0 && i < len(graph.Nodes) {
		graph.Nodes[i] = n
	}
}

// SetVoltSource 设置指定电压索引
func (graph *Graph) SetVoltSource(i int, n NodeID) {
	if i >= 0 && i < len(graph.VoltSource) {
		graph.VoltSource[i] = n
	}
}

// SetNodesInternal 设置指定内部索引
func (graph *Graph) SetNodesInternal(i int, n NodeID) {
	if i >= 0 && i < len(graph.NodesInternal) {
		graph.NodesInternal[i] = n
	}
}

// ElementBase 元件底层数据
type ElementBase struct {
	Graph
	TimeMNA               // 迭代时间
	NetList   *NetList    // 原始引用,网表的解析引用
	Pin       []string    // 引脚名称
	Value     []any       // 元件数据
	Current   []int       // 电流数据索引
	OrigValue map[int]any // 元件数据备份
	Voltage   []string    // 电压源名称
	Internal  []string    // 内部引脚名称
}

// Nodes 节点索引
func (base *ElementBase) Nodes(i int) NodeID {
	return base.Graph.Nodes[i]
}

// VoltSource 电压索引
func (base *ElementBase) VoltSource(i int) NodeID {
	return base.Graph.VoltSource[i]
}

// NodesInternal 内部索引
func (base *ElementBase) NodesInternal(i int) NodeID {
	return base.Graph.NodesInternal[i]
}

// Base 得到底层
func (base *ElementBase) Base() *ElementBase { return base }

// PinNum 引脚数量
func (base *ElementBase) PinNum() int { return len(base.Pin) }

// VoltageNum 电压源数量
func (base *ElementBase) VoltageNum() int { return len(base.Voltage) }

// InternalNum 内部数量
func (base *ElementBase) InternalNum() int { return len(base.Internal) }

// ValueNum 元件数据
func (base *ElementBase) ValueNum() int { return len(base.Value) }

// Update 更新操作
// 将当前值保存到原始值（更新备份）
func (base *ElementBase) Update() {
	for i := range base.OrigValue {
		base.OrigValue[i] = base.Value[i]
	}
}

// Rollback 回溯操作
// 将原始值恢复到当前值（回滚到备份）
func (base *ElementBase) Rollback() {
	for i := range base.OrigValue {
		base.Value[i] = base.OrigValue[i]
	}
}

// GetFloat64 获取浮点数
func (base *ElementBase) GetFloat64(i int) float64 {
	if i >= 0 && i < len(base.Value) {
		return base.Value[i].(float64)
	}
	return 0
}

// GetFloat64 设置浮点数
func (base *ElementBase) SetFloat64(i int, v float64) {
	if i >= 0 && i < len(base.Value) {
		base.Value[i] = v
	}
}

// GetInt 获取整数
func (base *ElementBase) GetInt(i int) int {
	if i >= 0 && i < len(base.Value) {
		return base.Value[i].(int)
	}
	return 0
}

// GetInt 设置整数
func (base *ElementBase) SetInt(i int, v int) {
	if i >= 0 && i < len(base.Value) {
		base.Value[i] = v
	}
}
