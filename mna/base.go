package mna

// ElementConfigBase 基础配置
type ElementConfigBase struct {
	Pin       []string // 引脚名称
	Value     []any    // 初始化数据
	Current   []int    // 电流数据索引
	OrigValue []int    // 数据备份索引
	Voltage   []string // 电压源名称
	Internal  []string // 内部引脚名称
}

// PinNum 引脚数量
func (base *ElementConfigBase) PinNum() int { return len(base.Pin) }

// VoltageNum 电压源数量
func (base *ElementConfigBase) VoltageNum() int { return len(base.Voltage) }

// InternalNum 内部数量
func (base *ElementConfigBase) InternalNum() int { return len(base.Internal) }

// ValueNum 元件数据
func (base *ElementConfigBase) ValueNum() int { return len(base.Value) }

// Graph 解析用数据
type Graph struct {
	NetList       *NetList    // 原始引用,网表的解析引用
	Value         []any       // 元件数据
	OrigValue     map[int]any // 元件数据备份
	Nodes         []NodeID    // 节点索引
	VoltSource    []NodeID    // 电压索引
	NodesInternal []NodeID    // 内部索引
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

// Update 更新操作
// 将当前值保存到原始值（更新备份）
func (graph *Graph) Update() {
	for i := range graph.OrigValue {
		graph.OrigValue[i] = graph.Value[i]
	}
}

// Rollback 回溯操作
// 将原始值恢复到当前值（回滚到备份）
func (graph *Graph) Rollback() {
	for i := range graph.OrigValue {
		graph.Value[i] = graph.OrigValue[i]
	}
}

// GetFloat64 获取浮点数
func (graph *Graph) GetFloat64(i int) float64 {
	if i >= 0 && i < len(graph.Value) {
		return graph.Value[i].(float64)
	}
	return 0
}

// GetFloat64 设置浮点数
func (graph *Graph) SetFloat64(i int, v float64) {
	if i >= 0 && i < len(graph.Value) {
		graph.Value[i] = v
	}
}

// GetInt 获取整数
func (graph *Graph) GetInt(i int) int {
	if i >= 0 && i < len(graph.Value) {
		return graph.Value[i].(int)
	}
	return 0
}

// GetInt 设置整数
func (graph *Graph) SetInt(i int, v int) {
	if i >= 0 && i < len(graph.Value) {
		graph.Value[i] = v
	}
}

// ElementBase 元件底层数据
type ElementBase struct {
	*ElementConfigBase // 元件定义
	Graph              // 元件数据
	TimeMNA            // 迭代时间
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
