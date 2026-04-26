package load

import (
	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// LoadString 加载仿真网表。
func LoadString(s string) (con *element.Context, err error) {
	return LoadContext(strings.NewReader(s))
}

// LoadContext 加载仿真网表。
func LoadContext(r io.Reader) (con *element.Context, err error) {
	parseTree, err := ast.NewParseTree(r)
	if err != nil {
		return nil, err
	}

	// 构建子电路定义查找表（扁平化，大小写不敏感）
	subcktMap := buildSubcircuitMap(parseTree.SubCircuitDefs)

	// === 第一阶段：展开子电路实例为平铺元件列表 ===
	allElementNodes := make([]*ast.ElementNode, 0, len(parseTree.ElementNodes))
	nodeNameToID := make(map[string]mna.NodeID)

	// 先收集顶层非X元件
	for _, elem := range parseTree.ElementNodes {
		if !strings.EqualFold(elem.Type, "x") {
			allElementNodes = append(allElementNodes, elem)
		}
	}

	// 从所有顶层引脚（含 X 实例引脚）确定内部节点起始编号
	maxExternalNodeID := mna.NodeID(0)
	for _, elem := range parseTree.ElementNodes {
		for _, pin := range elem.Pins {
			if id, err := strconv.Atoi(pin.Value); err == nil && mna.NodeID(id) > maxExternalNodeID {
				maxExternalNodeID = mna.NodeID(id)
			}
		}
	}

	// 展开 X 实例
	nextNodeID := maxExternalNodeID + 1
	for _, elem := range parseTree.ElementNodes {
		if !strings.EqualFold(elem.Type, "x") {
			continue
		}
		if len(elem.Values) == 0 {
			return nil, fmt.Errorf("第 %d 行: X 实例缺少子电路名称", elem.Line)
		}
		subcktName := elem.Values[0].Value
		subckt, ok := subcktMap[strings.ToLower(subcktName)]
		if !ok {
			return nil, fmt.Errorf("第 %d 行: 子电路 '%s' 未定义", elem.Line, subcktName)
		}
		instanceName := strings.ToUpper(elem.Type) + elem.ID

		expanded, err := expandSubCircuitInstance(
			subckt, elem.Pins, instanceName, subcktMap,
			&nextNodeID, nodeNameToID, parseTree,
		)
		if err != nil {
			return nil, err
		}
		allElementNodes = append(allElementNodes, expanded...)
	}

	// === 第二阶段：从所有平铺节点创建元件实例 ===
	var elements []element.NodeFace
	maxNodeID := mna.NodeID(0)
	usedNodes := make(map[mna.NodeID]struct{})

	for _, elemNode := range allElementNodes {
		element, err := createElementFromAST(elemNode)
		if err != nil {
			return nil, err
		}

		config := element.Config()
		pinNum := config.PinNum()
		if len(elemNode.Pins) < pinNum {
			return nil, fmt.Errorf("第 %d 行: 元件 '%s' 引脚数量不足。需要 %d，得到 %d", elemNode.Line, elemNode.Type, pinNum, len(elemNode.Pins))
		}

		for i := 0; i < pinNum; i++ {
			nodeID, err := strconv.Atoi(elemNode.Pins[i].Value)
			if err != nil {
				return nil, fmt.Errorf("第 %d 行: 引脚 %d 的节点ID无效 '%s'", elemNode.Line, i, elemNode.Pins[i].Value)
			}
			element.SetNodePin(i, mna.NodeID(nodeID))
			if mna.NodeID(nodeID) > maxNodeID {
				maxNodeID = mna.NodeID(nodeID)
			}
			if nodeID >= 0 {
				usedNodes[mna.NodeID(nodeID)] = struct{}{}
			}
		}

		if err := setElementValues(element, elemNode.Values, parseTree); err != nil {
			return nil, fmt.Errorf("第 %d 行: %v", elemNode.Line, err)
		}

		elements = append(elements, element)
	}

	// === 第三阶段：节点压缩 ===
	rawIDs := make([]mna.NodeID, 0, len(usedNodes))
	for id := range usedNodes {
		rawIDs = append(rawIDs, id)
	}
	sort.Slice(rawIDs, func(i, j int) bool { return rawIDs[i] < rawIDs[j] })
	compactNodeID := make(map[mna.NodeID]int, len(rawIDs))
	for i, id := range rawIDs {
		compactNodeID[id] = i
	}

	for _, elem := range elements {
		cfg := elem.Config()
		for i := 0; i < cfg.PinNum(); i++ {
			rawID := elem.GetNodes(i)
			if compactID, ok := compactNodeID[rawID]; ok {
				elem.SetNodePin(i, mna.NodeID(compactID))
			}
		}
	}

	if len(rawIDs) > 0 {
		maxNodeID = mna.NodeID(len(rawIDs) - 1)
	} else {
		maxNodeID = 0
	}

	// === 第四阶段：分配内部节点和电压源编号 ===
	currentVoltageID := mna.VoltageID(0)
	currentInternalNodeID := maxNodeID + 1

	for i := range elements {
		config := elements[i].Config()
		for j := 0; j < config.InternalNum(); j++ {
			elements[i].SetNodesInternal(j, currentInternalNodeID)
			currentInternalNodeID++
		}
		for j := 0; j < config.VoltageNum(); j++ {
			elements[i].SetVoltSource(j, currentVoltageID)
			currentVoltageID++
		}
	}

	nodesNum := int(currentInternalNodeID)
	voltageSourcesNum := int(currentVoltageID)

	// === 第五阶段：创建上下文 ===
	con = &element.Context{}
	con.Nodelist = elements
	con.CompactNodeID = compactNodeID
	con.HierarchicalNodeID = make(map[string]mna.NodeID, len(nodeNameToID))
	for hierName, rawID := range nodeNameToID {
		if compactID, ok := compactNodeID[rawID]; ok {
			con.HierarchicalNodeID[hierName] = mna.NodeID(compactID)
		}
	}

	mnaUpdate := mna.NewMnaUpdate(nodesNum, voltageSourcesNum)
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

// buildSubcircuitMap 递归收集所有 SubCircuitDefs 到 flat lookup map（大小写不敏感）
func buildSubcircuitMap(defs []*ast.SubCircuitDef) map[string]*ast.SubCircuitDef {
	result := make(map[string]*ast.SubCircuitDef)
	var add func([]*ast.SubCircuitDef)
	add = func(list []*ast.SubCircuitDef) {
		for _, def := range list {
			result[strings.ToLower(def.Name)] = def
			add(def.Defs)
		}
	}
	add(defs)
	return result
}

// cloneElementNode 深拷贝 ElementNode
func cloneElementNode(elem *ast.ElementNode) *ast.ElementNode {
	pins := make([]ast.Value, len(elem.Pins))
	copy(pins, elem.Pins)
	values := make([]ast.Value, len(elem.Values))
	copy(values, elem.Values)
	return &ast.ElementNode{
		Type:   elem.Type,
		ID:     elem.ID,
		Pins:   pins,
		Values: values,
		Line:   elem.Line,
	}
}

// isNumber 检查字符串是否表示数字
func isNumber(s string) bool {
	if len(s) == 0 {
		return false
	}
	if s[0] == '-' || s[0] == '+' {
		if len(s) > 1 {
			return s[1] >= '0' && s[1] <= '9'
		}
		return false
	}
	return s[0] >= '0' && s[0] <= '9'
}

// expandSubCircuitInstance 递归展开 X 子电路实例为平铺元件列表
func expandSubCircuitInstance(
	subckt *ast.SubCircuitDef,
	instancePins []ast.Value,
	instanceName string,
	allSubckts map[string]*ast.SubCircuitDef,
	nextNodeID *mna.NodeID,
	nodeNameToID map[string]mna.NodeID,
	parseTree *ast.ParseTree,
) ([]*ast.ElementNode, error) {
	// 构建端口映射（大小写不敏感）：端口名 → 外部引脚值
	portMap := make(map[string]string)
	for i, port := range subckt.Ports {
		if i < len(instancePins) {
			portMap[strings.ToLower(port.Value)] = instancePins[i].Value
		}
	}

	// 记录端口→外部节点的映射，用于层级路径查找
	for i, port := range subckt.Ports {
		if i < len(instancePins) {
			hierName := instanceName + "." + port.Value
			if id, err := strconv.Atoi(instancePins[i].Value); err == nil {
				nodeNameToID[hierName] = mna.NodeID(id)
			}
		}
	}

	var flatElements []*ast.ElementNode

	for _, elem := range subckt.Elements {
		newElem := cloneElementNode(elem)
		newElem.ID = instanceName + "." + elem.ID

		for i, pin := range elem.Pins {
			newElem.Pins[i].Value = resolveSubcircuitPin(
				pin.Value, portMap, instanceName, nextNodeID, nodeNameToID,
			)
		}

		if strings.EqualFold(elem.Type, "x") && len(elem.Values) > 0 {
			nestedName := elem.Values[0].Value
			nestedSubckt, ok := allSubckts[strings.ToLower(nestedName)]
			if !ok {
				return nil, fmt.Errorf("第 %d 行: 子电路 '%s' 未定义 (在被 '%s' 引用的子电路 '%s' 中)",
					elem.Line, nestedName, instanceName, subckt.Name)
			}
			nestedInstanceName := instanceName + "." + strings.ToUpper(elem.Type) + elem.ID

			nestedElements, err := expandSubCircuitInstance(
				nestedSubckt, newElem.Pins, nestedInstanceName,
				allSubckts, nextNodeID, nodeNameToID, parseTree,
			)
			if err != nil {
				return nil, err
			}
			flatElements = append(flatElements, nestedElements...)
		} else {
			flatElements = append(flatElements, newElem)
		}
	}

	return flatElements, nil
}

// resolveSubcircuitPin 解析子电路中的引脚值：端口名替换 / GND保留 / 内部节点分配唯一ID
func resolveSubcircuitPin(
	pinValue string,
	portMap map[string]string,
	instanceName string,
	nextNodeID *mna.NodeID,
	nodeNameToID map[string]mna.NodeID,
) string {
	if mapped, ok := portMap[strings.ToLower(pinValue)]; ok {
		return mapped
	}
	if pinValue == "0" || pinValue == "-1" {
		return pinValue
	}
	hierName := instanceName + "." + pinValue
	if existingID, ok := nodeNameToID[hierName]; ok {
		return strconv.Itoa(int(existingID))
	}
	newID := *nextNodeID
	*nextNodeID++
	nodeNameToID[hierName] = newID
	return strconv.Itoa(int(newID))
}
