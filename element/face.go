package element

import (
	"circuit/load/ast"
	"circuit/mna"
	"log"
)

// PinType 引脚类型。
type PinType uint8

const (
	PinLowVoltage  PinType = (1 << iota) // 弱电引脚。
	PinHighVoltage                       // 强电引脚。
	PinBoolean                           // 布尔引脚。
	PinPneumatic                         // 气路引脚。
	PinHydraulic                         // 油路引脚。
)

// ElementFaceList 元件接口，组合了配置接口和元件实现接口。
// 这是内部使用的接口类型，用于统一管理元件的配置和行为。
type ElementFaceList interface {
	ConfigFace  // 元件配置接口，提供元件的静态配置信息。
	ElementFace // 元件实现接口，提供元件的动态行为实现。
}

// ElementList 元件类型注册表，全局映射表。
// 键：NodeType（元件类型标识）。
// 值：elementFace（元件接口实现）。
// 用于存储所有已注册的元件类型及其实现。
var ElementList = map[NodeType]ElementFaceList{}

// ElementListName 元件名称注册表，全局映射表。
// 键：string 元件标识名称。
// 值：NodeType 元件类型。
// 用于存储所有已注册的元件类型及其实现。
var ElementListName = map[string]NodeType{}

// AddElement 注册元件类型到全局元件列表。
// 参数eleType: 元件类型标识，必须是唯一的。
// 参数face: 元件接口实现，包含配置和行为的完整实现。
// 返回：注册成功的元件类型标识。
// 注意：如果元件类型已注册，会触发致命错误并终止程序。
func AddElement(eleType NodeType, face ElementFaceList) NodeType {
	if _, ok := ElementList[eleType]; ok {
		log.Fatalf("元件重复注册: %d", eleType)
	}
	ElementList[eleType] = face
	ElementListName[face.GetName()] = eleType
	return eleType
}

// NodeType 元件类型标识，使用无符号整数表示。
// 每个元件类型都有一个唯一的NodeType值，用于在ElementLitt中标识和查找。
type NodeType uint

// NodeFace 元件节点接口，提供对元件动态数据的访问和操作。
// 这是仿真过程中元件实例的主要接口，用于访问和修改元件的状态。
type NodeFace interface {
	Type() NodeType                                  // 获取元件类型标识。
	Base() *Node                                     // 获取底层节点结构体指针。
	Config() *Config                                 // 配置信息。
	Update()                                         // 更新操作：将当前值保存到备份。
	Rollback()                                       // 回溯操作：将备份值恢复到当前值。
	GetFloat64(i int) float64                        // 获取第i个浮点数值参数。
	GetInt(i int) int                                // 获取第i个整数值参数。
	GetBool(i int) bool                              // 获取第i个逻辑值参数。
	GetString(i int) string                          // 获取第i个逻辑值参数。
	GetNodes(i int) mna.NodeID                       // 获取第i个引脚对应的MNA节点索引。
	GetVoltSource(i int) mna.VoltageID               // 获取第i个电压源对应的MNA节点索引。
	GetVoltSourceNodeID(m mna.Mna, i int) mna.NodeID // 获取第i个电压源对应的MNA节点索引。
	GetNodesInternal(i int) mna.NodeID               // 获取第i个内部节点对应的MNA节点索引。
	SetFloat64(i int, v float64)                     // 设置第i个浮点数值参数。
	SetInt(i int, v int)                             // 设置第i个整数值参数。
	SetBool(i int, v bool)                           // 设置第i个逻辑值参数。
	SetString(i int, v string)                       // 设置第i个逻辑值参数。
	SetNodePin(i int, n mna.NodeID)                  // 设置指定引脚对应的MNA节点索引。
	SetNodePins(n ...mna.NodeID)                     // 设置引脚节点索引。
	SetNodesInternal(i int, n mna.NodeID)            // 设置指定内部节点对应的MNA节点索引。
	SetVoltSource(i int, n mna.VoltageID)            // 设置指定电压源对应的MNA节点索引。
}

// ConfigFace 元件配置接口，提供元件的静态配置信息。
// 所有元件类型都必须实现此接口，以提供其配置信息。
type ConfigFace interface {
	Base(ast.ElementNode) *Config // 初始化配置信息。
	InternalNum() int             // 获取内部节点数量。
	GetName() string              // 元件名称。
	PinNum() int                  // 获取外部引脚数量。
	ValueNum() int                // 获取元件参数数量。
	VoltageNum() int              // 获取电压源数量。
	Reset(base NodeFace)          // 重置元件状态到初始值。
}

// ElementFace 元件实现接口，提供元件的动态行为实现。
// 所有元件类型都必须实现此接口，以定义其在仿真过程中的行为。
type ElementFace interface {
	StartIteration(mna mna.Mna, time mna.Time, value NodeFace)   // 步长迭代开始时的回调。
	Stamp(mna mna.Mna, time mna.Time, value NodeFace)            // 加盖线性贡献到MNA矩阵。
	DoStep(mna mna.Mna, time mna.Time, value NodeFace)           // 执行仿真步长计算。
	CalculateCurrent(mna mna.Mna, time mna.Time, value NodeFace) // 计算元件电流。
	StepFinished(mna mna.Mna, time mna.Time, value NodeFace)     // 步长迭代结束时的回调。
}
