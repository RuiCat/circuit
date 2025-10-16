package types

import (
	"encoding/json"
	"image"
	"reflect"

	"gioui.org/layout"
	"gonum.org/v1/gonum/mat"
)

// ValueMap 元件值列表
type ValueMap map[string]any

// Init 初始化
func (value *ValueMap) Init(uesr any) {
	val := reflect.TypeOf(uesr)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		v, n := reflect.New(field.Type), field.Tag.Get("value")
		if n != "" {
			if json.Unmarshal([]byte(n), v.Interface()) == nil {
				(*value)[field.Name] = v.Elem().Interface()
			}
		}
	}
}

// Reset 元件值初始化
func (value *ValueMap) Reset(uesr any) {
	val := reflect.ValueOf(uesr)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}
	f := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if v, ok := (*value)[f.Field(i).Name]; ok {
			field.Set(reflect.ValueOf(v))
		}
	}
}

// ValueBase 元件属性
type ValueBase struct {
	ValueMap             // 底层值
	Size     image.Point // 元件大小
}

// Layout 用于UI界面绘制元件
func (val *ValueBase) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: val.Size}
}
func (val *ValueBase) SetValue(value ValueMap) {
	for key := range val.ValueMap {
		val.ValueMap[key] = value[key]
	}
}
func (val *ValueBase) SetKeyValue(key string, value any) {
	val.ValueMap[key] = value
}
func (val *ValueBase) GetValue() (value ValueMap) { return val.ValueMap }

// Value 元件数据
type Value interface {
	Layout(gtx layout.Context) layout.Dimensions // 组件绘制实现
	CirLoad(value []string)                      // 网表文件写入值
	CirExport() []string                         // 网表文件导出值
	SetValue(value ValueMap)                     // 设置元件数据
	SetKeyValue(key string, value any)           // 设置元件数据
	GetValue() (value ValueMap)                  // 获取元件数据
	GetVoltageSourceCnt() int                    // 电压源数量
	GetInternalNodeCount() int                   // 内部引脚数量
	Reset()                                      // 元件值初始化
}

// ElementConfig 组件配置
type ElementConfig interface {
	Init(value *ElementBase) ElementFace // 初始化
	InitValue() Value                    // 元件值
	GetPostCount() int                   // 获取引脚数量
}

// ElementFace 组件底层接口
type ElementFace interface {
	Type() ElementType                                      // 元件类型
	Reset()                                                 // 数据重置
	GetValue() Value                                        // 获取元件自身数据
	Debug(stamp Stamp) string                               // 调试输出
	GetPinNodeList() (node PinList)                         // 得到引脚的节点ID了列表
	SetInternalNode(internalNodeIndex PinID, nodeID NodeID) // 设置内部引脚ID,扩展使用
	GetInternalNode(internalNodeIndex PinID) NodeID         // 得到内部引脚ID,扩展使用
	StartIteration(stamp Stamp)                             // 步长迭代开始
	Stamp(stamp Stamp)                                      // 加盖线性贡献
	DoStep(stamp Stamp)                                     // 执行仿真
	CalculateCurrent(stamp Stamp)                           // 电流计算
	StepFinished(stamp Stamp)                               // 步长迭代结束
}

// ElementWireBase 连接底层
type ElementWireBase struct {
	ID       ElementID // 元件ID
	WireList WireList  // 引脚连接信息
}

// GetPinWireList 得到引脚的线路连接列表
func (base *ElementWireBase) GetPinWireList() (wireID WireList) {
	return base.WireList
}

// ElementBase 元件基础配置
type ElementBase struct {
	Value                       // 基础记录值
	Nodes         PinList       // 节点ID列表
	VoltSource    VoltageList   // 电压源索引
	InternalNodes PinList       // 内部节点ID列表
	Current       *mat.VecDense // 节点电流数组，存储各引脚的电流值
}

// Init 初始化
func (base *ElementBase) Init(n int) {
	base.Nodes = make(PinList, n)
	base.Current = mat.NewVecDense(n, nil)
	base.VoltSource = make(VoltageList, base.Value.GetVoltageSourceCnt())
	base.InternalNodes = make(PinList, base.Value.GetInternalNodeCount())
	for id := range base.Nodes {
		base.Nodes[id] = ElementHeghNodeID
		base.Current.SetVec(id, 0)
	}
}

// GetPinNodeList 得到引脚的节点ID了列表
func (base *ElementBase) GetPinNodeList() (node PinList) {
	return base.Nodes
}

// Debug 调试输出
func (base *ElementBase) Debug(stamp Stamp) string { return "" }

// GetValue 获取元件自身数据
func (base *ElementBase) GetValue() Value { return base.Value }

// SetInternalNode 设置内部引脚ID,扩展使用
func (base *ElementBase) SetInternalNode(internalNodeIndex int, nodeID NodeID) {
	// 确保InternalNodes切片足够大
	for len(base.InternalNodes) <= internalNodeIndex {
		base.InternalNodes = append(base.InternalNodes, -1)
	}
	base.InternalNodes[internalNodeIndex] = nodeID
}

// GetInternalNode 得到内部引脚ID,扩展使用
func (base *ElementBase) GetInternalNode(internalNodeIndex int) NodeID {
	if internalNodeIndex < 0 || internalNodeIndex >= len(base.InternalNodes) {
		return 0 // 无效节点(接地)
	}
	return base.InternalNodes[internalNodeIndex]
}
