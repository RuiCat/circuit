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
		if err := setElementValues(element, elemNode.Values); err != nil {
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
func setElementValues(element element.NodeFace, values []ast.Value) error {
	config := element.Config()
	valueNum := config.ValueNum()
	valueInit := config.ValueInit

	// 遍历提供的值，但不超过参数数量
	for i := 0; i < len(values) && i < valueNum; i++ {
		val := values[i]
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

/*
// LoadContext 加载仿真网表。
func LoadContext(scanner *bufio.Scanner) (con *Context, err error) {
	var nodesNum, voltageSourcesNum int
	con = &Context{}
	con.Nodelist, nodesNum, voltageSourcesNum, err = parseNetlist(scanner)
	if err != nil {
		return nil, err
	}
	// 总方程数量
	n := nodesNum + voltageSourcesNum
	// 创建可更新的矩阵和向量
	con.MnaUpdateType = &mna.MnaUpdateType[float64]{
		MnaType: &mna.MnaType[float64]{
			NodesNum:          nodesNum,
			VoltageSourcesNum: voltageSourcesNum,
		},
		A:     maths.NewUpdateMatrixPtr(maths.NewDenseMatrix[float64](n, n)),
		Z:     maths.NewUpdateVectorPtr(maths.NewDenseVector[float64](n)),
		X:     maths.NewDenseVector[float64](n),
		LastX: maths.NewDenseVector[float64](n),
	}
	// 设置引用
	con.MnaUpdateType.MnaType.A = con.MnaUpdateType.A
	con.MnaUpdateType.MnaType.Z = con.MnaUpdateType.Z
	con.MnaUpdateType.MnaType.X = con.MnaUpdateType.X
	return con, nil
}


// NewElement 根据元件类型创建新的元件实例。
// 参数:
//
//	netlist: 网表列表。
//
// 返回:
//
//	NodeFace: 新创建的元件节点接口，如果类型未注册则返回nil。
//	error: 初始化错误信息。
func NewElement(netlist utils.NetList) (NodeFace, error) {
	// 根据类型名称查找元件类型。
	nameStr, _ := netlist.SeparationPrick(0)
	nodeType, ok := ElementListName[strings.ToUpper(nameStr)]
	if !ok {
		return nil, fmt.Errorf("未知的元件类型 '%s'", nameStr)
	}
	eleFace, ok := ElementList[nodeType]
	if !ok {
		return nil, fmt.Errorf("未知的元件类型 '%s'", nameStr)
	}
	// 创建元件。
	config, netlist := eleFace.Base(netlist[1:])
	node := &Node{
		ConfigPtr:    config,
		NodeType:     nodeType,
		NodeValue:    make([]any, config.ValueNum()),
		OrigValue:    make(map[int]any),
		Nodes:        make([]mna.NodeID, config.PinNum()),
		VoltSource:   make([]mna.VoltageID, config.VoltageNum()),
		NodeInternal: make([]mna.NodeID, config.InternalNum()),
	}
	// 初始化参数。
	copy(node.NodeValue, config.ValueInit)
	// 设置元件参数。
	pinNum := len(node.Nodes)
	if len(netlist) < pinNum {
		return nil, fmt.Errorf("元件 '%s' 引脚数量不足。需要 %d，得到 %d", netlist[0], pinNum, len(netlist)-1)
	}
	pinStrs := netlist[:pinNum]
	valueStrs := netlist[pinNum:]
	// 设置引脚。
	for i := range pinNum {
		node.SetNodePin(i, mna.NodeID(pinStrs.ParseInt(i, -1)))
	}
	// 加载参数。
	config.CirLoad(node, valueStrs)
	// 元件初始化。
	config.Reset(node)
	// 备份元件数据。
	for _, n := range config.OrigValue {
		node.OrigValue[n] = config.ValueInit[n]
	}
	return node, nil
}

// LoadNetlist 从文件加载网表。
// 参数:
//
//	filePath: 网表文件的路径。
//
// 返回:
//
//	[]NodeFace: 解析出的元件列表。
//	nodesNum: 节点数量。
//	voltageSourcesNum: 电压源数量。
//	error: 如果文件无法打开或解析出错，则返回错误。
func LoadNetlist(filePath string) (_ []NodeFace, nodesNum int, voltageSourcesNum int, _ error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("无法打开文件 %s: %w", filePath, err)
	}
	defer file.Close()
	return parseNetlist(bufio.NewScanner(file))
}

// LoadNetlistFromString 从字符串加载网表。
// 参数:
//
//	netlist: 包含网表数据的字符串。
//
// 返回:
//
//	[]NodeFace: 解析出的元件列表。
//	nodesNum: 节点数量。
//	voltageSourcesNum: 电压源数量。
//	error: 如果解析出错，则返回错误。
func LoadNetlistFromString(netlist string) (_ []NodeFace, nodesNum int, voltageSourcesNum int, _ error) {
	return parseNetlist(bufio.NewScanner(strings.NewReader(netlist)))
}

// parseNetlist 是加载网表的核心逻辑。
// 参数:
//
//	scanner: 用于读取网表数据的 bufio.Scanner。
//
// 返回:
//
//	[]NodeFace: 解析出的元件列表。
//	nodesNum: 节点数量。
//	voltageSourcesNum: 电压源数量。
//	error: 如果解析过程中出现任何错误，则返回错误。
func parseNetlist(scanner *bufio.Scanner) (_ []NodeFace, nodesNum int, voltageSourcesNum int, _ error) {
	var elements []NodeFace
	lineNumber := 0
	maxNodeID := mna.NodeID(-1)

	// 第一遍扫描：创建元件并收集引脚节点。
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// 忽略注释和空行。
		if commentIndex := strings.Index(line, "#"); commentIndex != -1 {
			line = line[:commentIndex]
		}
		if commentIndex := strings.Index(line, "*"); commentIndex != -1 {
			line = line[:commentIndex]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 1 {
			continue
		}

		// 创建 NetList。
		netlist := utils.NetList(parts)

		// 使用 NewElement 创建元件。
		element, err := NewElement(netlist)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("第 %d 行: %v", lineNumber, err)
		}

		// 检查引脚数量是否足够。
		config := element.Config()
		pinNum := config.PinNum()
		if len(parts)-1 < pinNum {
			return nil, 0, 0, fmt.Errorf("第 %d 行: 元件 '%s' 引脚数量不足。需要 %d，得到 %d", lineNumber, parts[0], pinNum, len(parts)-1)
		}

		// 更新最大节点ID。
		for i := 0; i < pinNum; i++ {
			nodeID := element.GetNodes(i)
			if nodeID > maxNodeID {
				maxNodeID = nodeID
			}
		}

		elements = append(elements, element)
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, 0, fmt.Errorf("读取网表时出错: %w", err)
	}

	// 分配内部节点和电压源。
	currentVoltageID := mna.VoltageID(0)
	currentInternalNodeID := maxNodeID + 1

	for _, element := range elements {
		config := element.Config()

		// 分配内部节点。
		for i := 0; i < config.InternalNum(); i++ {
			element.SetNodesInternal(i, currentInternalNodeID)
			currentInternalNodeID++
		}

		// 分配电压源。
		for i := 0; i < config.VoltageNum(); i++ {
			element.SetVoltSource(i, currentVoltageID)
			currentVoltageID++
		}
	}

	return elements, int(currentInternalNodeID), int(currentVoltageID), nil
}
*/
