package element

import (
	"bufio"
	"circuit/mna"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LoadNetlist 从文件加载网表。
// 参数:
//
//	filePath: 网表文件的路径。
//
// 返回:
//
//	[]NodeFace: 解析出的元件列表。
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
//	error: 如果解析出错，则返回错误。
func LoadNetlistFromString(netlist string) (_ []NodeFace, nodesNum int, voltageSourcesNum int, _ error) {
	return parseNetlist(bufio.NewScanner(strings.NewReader(netlist)))
}

// parseNetlist 是加载网表的核心逻辑。
// 该函数分三步处理网表数据：
// 1. 第一遍扫描：创建元件，解析参数值，并收集引脚连接信息。
// 2. 第二遍扫描：设置元件的引脚连接，并找出最大的节点ID。
// 3. 第三遍扫描：为需要内部节点或电压源的元件分配ID。
// 参数:
//
//	scanner: 用于读取网表数据的 bufio.Scanner。
//
// 返回:
//
//	[]NodeFace: 解析出的元件列表。
//	error: 如果解析过程中出现任何错误，则返回错误。
func parseNetlist(scanner *bufio.Scanner) (_ []NodeFace, nodesNum int, voltageSourcesNum int, _ error) {
	// tempElement 用于临时存储解析过程中的元件信息
	type tempElement struct {
		element  NodeFace // 元件实例
		pinStrs  []string // 引脚连接的字符串表示
		line     int      // 元件在源文件中的行号
		typeName string   // 元件的名称
	}

	var tempElements []tempElement
	var elements []NodeFace
	lineNumber := 0

	// 构建一个从元件类型名称到 NodeType 的映射，以加快查找速度
	typeMap := make(map[string]NodeType)
	for nt, ele := range ElementLitt {
		config := ele.GetConfig()
		if config != nil && config.Name != "" {
			uname := strings.ToUpper(config.Name)
			typeMap[uname] = nt
		}
	}

	// 第一遍扫描: 创建元件，解析参数值，并收集引脚连接的字符串
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// 忽略注释和空行
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

		// 提取元件类型名称（从名称字符串中分离出非数字前缀）
		nameStr := strings.ToUpper(parts[0])
		typeName := ""
		for i, char := range nameStr {
			if char >= '0' && char <= '9' {
				typeName = nameStr[:i]
				break
			}
		}
		if typeName == "" {
			typeName = nameStr
		}

		// 根据类型名称查找 NodeType
		nodeType, ok := typeMap[typeName]
		if !ok {
			return nil, 0, 0, fmt.Errorf("第 %d 行: 未知的元件类型 '%s'", lineNumber, parts[0])
		}

		// 检查元件配置和引脚数量
		config := nodeType.Config()
		if config == nil {
			return nil, 0, 0, fmt.Errorf("第 %d 行: 找不到元件类型的配置 '%s'", lineNumber, typeName)
		}
		pinNum := config.PinNum()
		if len(parts)-1 < pinNum {
			return nil, 0, 0, fmt.Errorf("第 %d 行: 元件 '%s' 引脚数量不足. 需要 %d, 得到 %d", lineNumber, parts[0], pinNum, len(parts)-1)
		}

		// 创建并加载元件
		element := NewElementValue(nodeType)
		if element == nil {
			return nil, 0, 0, fmt.Errorf("第 %d 行: 无法创建元件 '%s'", lineNumber, typeName)
		}

		pinStrs := parts[1 : pinNum+1]
		valueStrs := parts[pinNum+1:]

		eleFace, _ := ElementLitt[nodeType]
		eleFace.CirLoad(element, valueStrs)

		tempElements = append(tempElements, tempElement{element, pinStrs, lineNumber, parts[0]})
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, 0, fmt.Errorf("读取网表时出错: %w", err)
	}

	// 第二遍扫描: 设置引脚连接并找出最大的节点ID
	maxNodeID := mna.NodeID(-1)
	for _, te := range tempElements {
		for i, pinStr := range te.pinStrs {
			nodeIDVal, err := strconv.Atoi(pinStr)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("第 %d 行: 无效的节点ID '%s'", te.line, pinStr)
			}
			nodeID := mna.NodeID(nodeIDVal)
			if nodeID > maxNodeID {
				maxNodeID = nodeID
			}
			te.element.SetNodePin(i, nodeID)
		}
	}

	// 第三遍扫描: 分配并设置内部节点和电压源
	currentVoltageID := mna.VoltageID(0)
	// 内部节点从外部节点ID之后开始编号
	currentInternalNodeID := maxNodeID + 1

	for _, te := range tempElements {
		config := te.element.Type().Config()
		// 分配内部节点
		for i := 0; i < config.InternalNum(); i++ {
			te.element.SetNodesInternal(i, currentInternalNodeID)
			currentInternalNodeID++
		}
		// 分配电压源
		for i := 0; i < config.VoltageNum(); i++ {
			te.element.SetVoltSource(i, currentVoltageID)
			currentVoltageID++
		}
		elements = append(elements, te.element)
	}

	return elements, int(currentInternalNodeID), int(currentVoltageID), nil
}
