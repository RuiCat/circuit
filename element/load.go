package element

import (
	"bufio"
	"circuit/mna"
	"circuit/utils"
	"fmt"
	"os"
	"strings"
)

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
