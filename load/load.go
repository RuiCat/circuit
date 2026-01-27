package load

import (
	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// LoadString 加载仿真网表。
func LoadString(s string) (con *element.Context, err error) {
	return LoadContext(strings.NewReader(s))
}

// LoadContext 加载仿真网表。
func LoadContext(r io.Reader) (con *element.Context, err error) {
	// 解析网表
	parseTree, err := ast.NewParseTree(r)
	if err != nil {
		return nil, err
	}

	// 创建元件列表
	var elements []element.NodeFace
	maxNodeID := mna.NodeID(-1)

	// 第一遍扫描：创建元件并收集引脚节点
	for _, elemNode := range parseTree.ElementNodes {
		element, err := createElementFromAST(elemNode)
		if err != nil {
			return nil, err
		}

		// 检查引脚数量是否足够
		config := element.Config()
		pinNum := config.PinNum()
		if len(elemNode.Pins) < pinNum {
			return nil, fmt.Errorf("第 %d 行: 元件 '%s' 引脚数量不足。需要 %d，得到 %d", elemNode.Line, elemNode.Type, pinNum, len(elemNode.Pins))
		}

		// 设置引脚节点
		for i := 0; i < pinNum; i++ {
			// 解析节点ID
			nodeID, err := strconv.Atoi(elemNode.Pins[i].Value)
			if err != nil {
				return nil, fmt.Errorf("第 %d 行: 引脚 %d 的节点ID无效 '%s'", elemNode.Line, i, elemNode.Pins[i].Value)
			}
			// 节点ID可以是负数（如-1表示地节点）
			element.SetNodePin(i, mna.NodeID(nodeID))
			if mna.NodeID(nodeID) > maxNodeID {
				maxNodeID = mna.NodeID(nodeID)
			}
		}

		// 设置元件参数值
		if err := setElementValues(element, elemNode.Values, parseTree); err != nil {
			return nil, fmt.Errorf("第 %d 行: %v", elemNode.Line, err)
		}

		elements = append(elements, element)
	}

	// 分配内部节点和电压源
	currentVoltageID := mna.VoltageID(0)
	currentInternalNodeID := maxNodeID + 1

	for _, element := range elements {
		config := element.Config()

		// 分配内部节点
		for i := 0; i < config.InternalNum(); i++ {
			element.SetNodesInternal(i, currentInternalNodeID)
			currentInternalNodeID++
		}

		// 分配电压源
		for i := 0; i < config.VoltageNum(); i++ {
			element.SetVoltSource(i, currentVoltageID)
			currentVoltageID++
		}
	}

	// 计算总节点数和电压源数量
	nodesNum := int(currentInternalNodeID)
	voltageSourcesNum := int(currentVoltageID)

	// 创建上下文
	con = &element.Context{}
	con.Nodelist = elements

	// 创建可更新的矩阵和向量
	mnaUpdate := mna.NewMnaUpdate(nodesNum, voltageSourcesNum)
	// 类型断言为具体类型
	if mnaUpdateType, ok := mnaUpdate.(*mna.MnaUpdateType[float64]); ok {
		con.MnaUpdateType = mnaUpdateType
	} else {
		return nil, fmt.Errorf("无法创建MNA更新类型")
	}

	return con, nil
}

// createElementFromAST 根据AST元素节点创建元件实例
func createElementFromAST(elemNode *ast.ElementNode) (element.NodeFace, error) {
	// 根据类型名称查找元件类型
	nodeType, ok := element.ElementListName[strings.ToUpper(elemNode.Type)]
	if !ok {
		return nil, fmt.Errorf("未知的元件类型 '%s'", elemNode.Type)
	}
	eleFace, ok := element.ElementList[nodeType]
	if !ok {
		return nil, fmt.Errorf("未知的元件类型 '%s'", elemNode.Type)
	}

	// 创建元件配置
	config := eleFace.Base(*elemNode)
	if config == nil {
		return nil, fmt.Errorf("无法创建元件 '%s' 的配置", elemNode.Type)
	}

	// 创建节点
	node := &element.Node{
		ConfigPtr:    config,
		NodeType:     nodeType,
		NodeValue:    make([]any, config.ValueNum()),
		OrigValue:    make(map[int]any),
		Nodes:        make([]mna.NodeID, config.PinNum()),
		VoltSource:   make([]mna.VoltageID, config.VoltageNum()),
		NodeInternal: make([]mna.NodeID, config.InternalNum()),
	}

	// 初始化参数
	copy(node.NodeValue, config.ValueInit)

	// 备份元件数据
	for _, n := range config.OrigValue {
		node.OrigValue[n] = config.ValueInit[n]
	}

	return node, nil
}

// setElementValues 设置元件参数值
func setElementValues(element element.NodeFace, values []ast.Value, parseTree *ast.ParseTree) error {
	config := element.Config()
	valueNum := config.ValueNum()
	valueInit := config.ValueInit
	// 遍历提供的值，但不超过参数数量
	for i := 0; i < len(values) && i < valueNum; i++ {
		val := values[i]
		if val.IsVar {
			if v, ok := parseTree.ValueNodes[val.Value]; ok {
				val.Value = v
			}
		}
		// 使用 StringToAny 根据参数初始值的类型来解析值
		parsedValue := ast.StringToAny(val, valueInit[i])
		// 根据类型设置值
		switch v := parsedValue.(type) {
		case string:
			element.SetString(i, v)
		case bool:
			element.SetBool(i, v)
		case int:
			element.SetInt(i, v)
		case int8:
			element.SetInt(i, int(v))
		case int16:
			element.SetInt(i, int(v))
		case int32:
			element.SetInt(i, int(v))
		case int64:
			element.SetInt(i, int(v))
		case uint:
			element.SetInt(i, int(v))
		case uint16:
			element.SetInt(i, int(v))
		case uint32:
			element.SetInt(i, int(v))
		case uint64:
			element.SetInt(i, int(v))
		case float32:
			element.SetFloat64(i, float64(v))
		case float64:
			element.SetFloat64(i, v)
		case complex64, complex128:
			// 复数处理：暂时设置为0
			element.SetFloat64(i, 0)
		default:
			// 其他类型，尝试解析为字符串
			element.SetString(i, fmt.Sprint(v))
		}
	}
	// 注意：如果 values 长度小于 valueNum，剩余的参数将保持默认值
	// 元件初始化
	config.Reset(element)
	return nil
}
