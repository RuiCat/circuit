package load

import (
	"circuit/element"
	"circuit/element/hierarchical"
	"circuit/load/ast"
	"circuit/mna"
	"fmt"
	"io"
	"log"
	"os"
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
	// 读取全部网表文本
	textBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("读取网表失败: %w", err)
	}

	// 防止超大网表
	const maxNetlistSize = 100 * 1024 * 1024 // 100MB
	if len(textBytes) > maxNetlistSize {
		return nil, fmt.Errorf("网表文件过大: %d 字节（最大 %d 字节）", len(textBytes), maxNetlistSize)
	}

	// 展开总线表示法（如 data[7:0] → data7 data6 ... data0）
	expandedText, err := ExpandBusNotation(string(textBytes))
	if err != nil {
		return nil, fmt.Errorf("总线展开失败: %w", err)
	}

	// 防止展开后文本过大（ExpandBusNotation 内部已做增量检查，此处为双保险）
	if len(expandedText) > maxExpandedSize {
		return nil, fmt.Errorf("展开后网表过大: %d 字节（最大 %d 字节）", len(expandedText), maxExpandedSize)
	}

	parseTree, err := ast.NewParseTree(strings.NewReader(expandedText))
	if err != nil {
		return nil, err
	}

	// 构建子电路定义查找表（扁平化，大小写不敏感）
	subcktMap := buildSubcircuitMap(parseTree.SubCircuitDefs)

	// === 第一阶段：展开子电路实例为平铺元件列表，同时构建层级封装元件的 AST 节点 ===
	allElementNodes := make([]*ast.ElementNode, 0, len(parseTree.ElementNodes))
	wrapperNodes := make([]*ast.ElementNode, 0) // 层级封装元件 AST 节点
	nodeNameToID := make(map[string]mna.NodeID)

	// 先收集顶层非 X 元件
	for _, elem := range parseTree.ElementNodes {
		if !strings.EqualFold(elem.Type, "x") {
			allElementNodes = append(allElementNodes, elem)
		}
	}

	// 从所有顶层引脚（含 X 实例引脚）确定内部节点起始编号
	maxExternalNodeID := mna.NodeID(0)
	for _, elem := range parseTree.ElementNodes {
		for _, pin := range elem.Pins {
			if id, err := strconv.Atoi(pin.Value); err == nil {
				if id < -1 || id > maxRawNodeID {
					return nil, fmt.Errorf("第 %d 行: 节点号 %d 超出合法范围（-1 ~ %d）", elem.Line, id, maxRawNodeID)
				}
				if mna.NodeID(id) > maxExternalNodeID {
					maxExternalNodeID = mna.NodeID(id)
				}
			}
		}
	}

	// 展开 X 实例，同时创建层级封装元件
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
			&nextNodeID, nodeNameToID, parseTree, make(map[string]bool), 0,
		)
		if err != nil {
			return nil, err
		}

		// 创建层级封装元件的 AST 节点
		wrapperNode := &ast.ElementNode{
			Type:     "X",
			ID:       elem.ID,
			Pins:     elem.Pins,
			Values:   elem.Values,
			Line:     elem.Line,
			Children: expanded,
		}
		wrapperNodes = append(wrapperNodes, wrapperNode)
		allElementNodes = append(allElementNodes, expanded...)
		allElementNodes = append(allElementNodes, wrapperNode)
	}

	// 为顶层元件填充实例名（子电路内元件已在展开时填充为 "X1.R1" 形式）
	for _, elemNode := range allElementNodes {
		if elemNode.InstanceName == "" {
			elemNode.InstanceName = strings.ToUpper(elemNode.Type) + elemNode.ID
		}
	}

	// 解析顶层元件中的命名引脚（非数字节点名如 s0n, t0n, nop0 等）
	for _, elemNode := range allElementNodes {
		for i := range elemNode.Pins {
			pinVal := elemNode.Pins[i].Value
			if pinVal == "0" || pinVal == "-1" {
				continue
			}
			if _, err := strconv.Atoi(pinVal); err == nil {
				continue
			}
			if existingID, ok := nodeNameToID[pinVal]; ok {
				elemNode.Pins[i].Value = strconv.Itoa(int(existingID))
			} else {
				newID := nextNodeID
				nextNodeID++
				nodeNameToID[pinVal] = newID
				elemNode.Pins[i].Value = strconv.Itoa(int(newID))
			}
		}
	}

	// === 第二阶段：从所有平铺节点创建元件实例 ===
	var elements []element.NodeFace
	maxNodeID := mna.NodeID(0)
	usedNodes := make(map[mna.NodeID]struct{})
	astToInstance := make(map[*ast.ElementNode]element.NodeFace)

	// 节点物理域表：记录每个节点已被哪个域（电气/气路/液压）的引脚占用。
	// 气/油/电是三套独立系统，同一节点只允许连接同一域的引脚；
	// 全局地节点 -1 是共享参考点（电气地=0V、气路地=大气压、油路地=油箱），各域共用，豁免检查。
	nodeSystem := make(map[mna.NodeID]int)

	for _, elemNode := range allElementNodes {
		var instance element.NodeFace
		var err error

		if strings.EqualFold(elemNode.Type, "X") {
			instance, err = createWrapperInstance(elemNode)
		} else {
			instance, err = createElementFromAST(elemNode)
		}
		if err != nil {
			return nil, err
		}

		config := instance.Config()
		pinNum := config.PinNum()
		if len(elemNode.Pins) < pinNum {
			return nil, fmt.Errorf("第 %d 行: 元件 '%s' 引脚数量不足。需要 %d，得到 %d", elemNode.Line, elemNode.Type, pinNum, len(elemNode.Pins))
		}

		// 层级封装 X 的引脚类型是 PinBoolean 占位（子电路端口物理域未知），
		// 其展开的子元件已单独参与本校验，故跳过 X 实例本身，避免误判。
		skipDomainCheck := strings.EqualFold(elemNode.Type, "X")

		for i := 0; i < pinNum; i++ {
			nodeID, err := strconv.Atoi(elemNode.Pins[i].Value)
			if err != nil {
				return nil, fmt.Errorf("第 %d 行: 引脚 %d 的节点ID无效 '%s'", elemNode.Line, i, elemNode.Pins[i].Value)
			}
			if nodeID < -1 || nodeID > maxRawNodeID {
				return nil, fmt.Errorf("第 %d 行: 引脚 %d 的节点号 %d 超出合法范围（-1 ~ %d）", elemNode.Line, i, nodeID, maxRawNodeID)
			}

			// 物理域校验：气/油/电为独立系统，同一节点禁止混接不同域引脚。
			if nodeID >= 0 && !skipDomainCheck {
				sys := pinDomain(config.Pin[i].Type)
				if prev, ok := nodeSystem[mna.NodeID(nodeID)]; ok && prev != sys {
					return nil, fmt.Errorf(
						"第 %d 行: 元件 '%s' 的%s引脚(%s)连接到节点 %d，但该节点已连接%s引脚（气/油/电为独立系统，禁止跨域混接）",
						elemNode.Line, elemNode.InstanceName, domainName(sys), config.Pin[i].Name, nodeID, domainName(prev))
				}
				nodeSystem[mna.NodeID(nodeID)] = sys
			}

			instance.SetNodePin(i, mna.NodeID(nodeID))
			if mna.NodeID(nodeID) > maxNodeID {
				maxNodeID = mna.NodeID(nodeID)
			}
			if nodeID >= 0 {
				usedNodes[mna.NodeID(nodeID)] = struct{}{}
			}
		}

		if err := setElementValues(instance, elemNode.Values, parseTree); err != nil {
			return nil, fmt.Errorf("第 %d 行: %v", elemNode.Line, err)
		}

		elements = append(elements, instance)
		astToInstance[elemNode] = instance
	}

	// 建立层级封装元件的父子关系
	for _, wrapperNode := range wrapperNodes {
		wrapperInst := astToInstance[wrapperNode]
		if wrapperInst == nil {
			continue
		}
		var children []element.NodeFace
		for _, childAST := range wrapperNode.Children {
			if childInst := astToInstance[childAST]; childInst != nil {
				childInst.SetParent(wrapperInst)
				children = append(children, childInst)
			}
		}
		wrapperInst.SetChildren(children)
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

	// 校验矩阵总维度，防止 O(n²) 稠密分配耗尽内存。
	if totalDim := nodesNum + voltageSourcesNum; totalDim > maxNodeCount {
		return nil, fmt.Errorf("电路规模过大: %d 个节点/电压源（上限 %d）", totalDim, maxNodeCount)
	}

	// === 第五阶段：创建上下文 ===
	con = &element.Context{}
	con.Nodelist = elements
	con.CompactNodeID = compactNodeID
	for _, elem := range elements {
		if elem.Config().Flags&element.FlagReactive != 0 {
			con.HasReactive = true
			break
		}
	}
	con.HierarchicalNodeID = make(map[string]mna.NodeID, len(nodeNameToID))
	for hierName, rawID := range nodeNameToID {
		if compactID, ok := compactNodeID[rawID]; ok {
			con.HierarchicalNodeID[hierName] = mna.NodeID(compactID)
		}
	}

	// 解析引擎仿真配置（环境变量 → EngineConfig），集中注入 Context。
	// 引擎内部（element/time/simulation.go）与 MNA 存储选择都读 con.EngineCfg，
	// 不再直接 os.Getenv。
	con.EngineCfg = loadEngineConfig()

	mnaUpdate := mna.NewMnaUpdate(nodesNum, voltageSourcesNum)
	// 稀疏模式（con.EngineCfg.SparseLU，load 从 CIRCUIT_LUSPARSE 解析）：
	// A 改用 CSR 稀疏存储，与瞬态引擎的稀疏 LU 分解器配套使用。
	if con.EngineCfg.SparseLU {
		mnaUpdate = mna.NewMnaUpdateSparse(nodesNum, voltageSourcesNum)
	}
	if mnaUpdateType, ok := mnaUpdate.(*mna.MnaUpdateType[float64]); ok {
		con.MnaUpdateType = mnaUpdateType
	} else {
		return nil, fmt.Errorf("无法创建MNA更新类型")
	}

	return con, nil
}

// loadEngineConfig 从环境变量解析引擎仿真配置。
// 保持与既有 CIRCUIT_* 环境变量完全兼容；数值解析失败时静默回退默认值。
func loadEngineConfig() element.EngineConfig {
	cfg := element.EngineConfig{}
	cfg.SparseLU = os.Getenv("CIRCUIT_LUSPARSE") != ""
	cfg.GminCont = os.Getenv("CIRCUIT_GMCONT") != ""
	cfg.GminDbg = os.Getenv("CIRCUIT_GMIN_DBG") != ""
	cfg.NoEq = os.Getenv("CIRCUIT_NOEQ") != ""
	if v := os.Getenv("CIRCUIT_NATGMIN"); v != "" {
		fmt.Sscanf(v, "%g", &cfg.NaturalGmin)
	}
	if v := os.Getenv("CIRCUIT_GMINFINAL"); v != "" {
		fmt.Sscanf(v, "%g", &cfg.GminFinal)
	}
	if v := os.Getenv("CIRCUIT_GLOBALGMIN"); v != "" && v != "0" {
		// 支持显式数值（如 CIRCUIT_GLOBALGMIN=1e-7）或布尔启用（默认 1e-9 S=1GΩ）。
		// 全局 gmin 是"到地电导"：值必须远小于 RTL 上拉电导（1kΩ=1e-3 S），
		// 否则会把逻辑节点拉到中间电平、门失去增益、锁存器焊死中间态。
		// 不叠加 continuationGmin（Gmin stepping 的 1kΩ 级阻尼）——延续阻尼
		// 只由 PN 结元件自身读取，不进入矩阵对角。
		var f float64
		if _, err := fmt.Sscanf(v, "%g", &f); err == nil && f > 0 && f < 1 {
			cfg.GlobalGmin = f
		} else {
			cfg.GlobalGmin = 1e-9
		}
	}
	return cfg
}

// createElementFromAST 根据AST元素节点创建元件实例
func createElementFromAST(elemNode *ast.ElementNode) (element.NodeFace, error) {
	// 根据类型名称查找元件类型（大小写不敏感，统一小写）
	nodeType, ok := element.ElementListName[strings.ToLower(elemNode.Type)]
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
		InstanceName: elemNode.InstanceName,
		NodeValue:    make([]any, config.ValueNum()),
		OrigValue:    make(map[int]any),
		Nodes:        make([]mna.NodeID, config.PinNum()),
		VoltSource:   make([]mna.VoltageID, config.VoltageNum()),
		NodeInternal: make([]mna.NodeID, config.InternalNum()),
	}
	if node.InstanceName == "" {
		// 兜底：无实例名时用 类型+ID 拼装
		node.InstanceName = strings.ToUpper(elemNode.Type) + elemNode.ID
	}

	// 初始化参数
	copy(node.NodeValue, config.ValueInit)

	// 备份元件数据
	for _, n := range config.OrigValue {
		node.OrigValue[n] = config.ValueInit[n]
	}

	return node, nil
}

// createWrapperInstance 为层级封装元件 X 创建实例。
// 层级封装元件的 Config 根据其引脚动态生成。
func createWrapperInstance(elemNode *ast.ElementNode) (element.NodeFace, error) {
	// 创建引脚名称列表
	pinNames := make([]string, len(elemNode.Pins))
	for i, p := range elemNode.Pins {
		pinNames[i] = p.Value
	}

	subcktName := ""
	if len(elemNode.Values) > 0 {
		subcktName = elemNode.Values[0].Value
	}

	config := &element.Config{
		Name: "X",
		Pin:  element.SetPin(element.PinBoolean, pinNames...),
		ValueInit: []any{
			subcktName,
		},
		ValueName: []string{"subckt"},
	}

	// 查找 Wrapper 的 NodeType
	nodeType := hierarchical.DefaultWrapperNodeType
	if nt, ok := element.ElementListName["x"]; ok {
		nodeType = nt
	}

	node := &element.Node{
		ConfigPtr:    config,
		NodeType:     nodeType,
		InstanceName: elemNode.InstanceName,
		NodeValue:    make([]any, config.ValueNum()),
		OrigValue:    make(map[int]any),
		Nodes:        make([]mna.NodeID, config.PinNum()),
		VoltSource:   make([]mna.VoltageID, config.VoltageNum()),
		NodeInternal: make([]mna.NodeID, config.InternalNum()),
	}

	copy(node.NodeValue, config.ValueInit)

	for _, n := range config.OrigValue {
		node.OrigValue[n] = config.ValueInit[n]
	}

	return node, nil
}

// setElementValues 设置元件参数值。
// 使用 ast.StringToAny 根据参数初始值类型解析网表值，
// 然后将 uint8/uint16/uint32/uint64 等无符号类型统一映射为 int 存入元件；
// 明确拒绝 complex64/complex128 类型的参数值（电路元件参数不支持复数）。
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
		// 检测参数值能否解析为目标类型，不能则输出警告（避免 [true] 等被静默回退默认值）
		warnIfUnparseable(val, valueInit[i])
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
		case uint8:
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
			return fmt.Errorf("不支持复数类型的元件参数值")
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

// warnIfUnparseable 检测网表参数值能否解析为目标类型，不能则输出警告到 stderr。
// 用于暴露 [true] 这类被 StringToAny 静默回退默认值的参数书写错误。
func warnIfUnparseable(val ast.Value, target any) {
	if val.IsVar || val.Value == "" {
		return
	}
	var err error
	switch target.(type) {
	case bool:
		_, err = strconv.ParseBool(val.Value)
	case int, int8, int16, int32, int64:
		_, err = strconv.ParseInt(val.Value, 10, 64)
	case uint, uint8, uint16, uint32, uint64:
		_, err = strconv.ParseUint(val.Value, 10, 64)
	case float32, float64:
		_, err = strconv.ParseFloat(val.Value, 64)
	default:
		return // 字符串等类型总能解析，不检测
	}
	if err != nil {
		log.Printf("警告: 网表参数值 '%s' 无法解析为 %T，已忽略并使用默认值", val.Value, target)
	}
}

// maxSubCircuitDepth 是子电路嵌套展开的最大允许深度，
// 用于防止无限递归或过深的嵌套导致栈溢出。
const maxSubCircuitDepth = 100

// maxNodeCount MNA 稠密矩阵总维度（节点数+电压源数）上限。
// 该实现使用稠密矩阵，分配规模为 O(n²)，需限制 n 防止恶意网表耗尽内存。
const maxNodeCount = 5000

// maxRawNodeID 网表原始节点号的上限（防止 nextNodeID 整数回绕为负）。
// 原始节点号经压缩后映射为 0..N-1，此上限仅用于防回绕，设置得足够宽裕。
const maxRawNodeID = 1 << 31

// buildSubcircuitMap 递归收集所有 SubCircuitDefs 到 flat lookup map（大小写不敏感）
func buildSubcircuitMap(defs []*ast.SubCircuitDef) map[string]*ast.SubCircuitDef {
	result := make(map[string]*ast.SubCircuitDef)
	var add func([]*ast.SubCircuitDef, int)
	add = func(list []*ast.SubCircuitDef, depth int) {
		if depth > maxSubCircuitDepth {
			return
		}
		for _, def := range list {
			result[strings.ToLower(def.Name)] = def
			add(def.Defs, depth+1)
		}
	}
	add(defs, 0)
	return result
}

// cloneElementNode 深拷贝 ElementNode
func cloneElementNode(elem *ast.ElementNode) *ast.ElementNode {
	pins := make([]ast.Value, len(elem.Pins))
	copy(pins, elem.Pins)
	values := make([]ast.Value, len(elem.Values))
	copy(values, elem.Values)
	return &ast.ElementNode{
		Type:     elem.Type,
		ID:       elem.ID,
		Pins:     pins,
		Values:   values,
		Line:     elem.Line,
		Children: elem.Children,
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

// expandSubCircuitInstance 递归展开 X 子电路实例为平铺元件列表。
// 通过 depth 参数与 maxSubCircuitDepth 限制最大嵌套深度；
// 通过 visited 集合检测循环引用，防止无限递归。
func expandSubCircuitInstance(
	subckt *ast.SubCircuitDef,
	instancePins []ast.Value,
	instanceName string,
	allSubckts map[string]*ast.SubCircuitDef,
	nextNodeID *mna.NodeID,
	nodeNameToID map[string]mna.NodeID,
	parseTree *ast.ParseTree,
	visited map[string]bool,
	depth int,
) ([]*ast.ElementNode, error) {
	if depth > maxSubCircuitDepth {
		return nil, fmt.Errorf("子电路 '%s' 嵌套深度超过限制 %d", subckt.Name, maxSubCircuitDepth)
	}
	// 检测循环引用
	subcktKey := strings.ToLower(subckt.Name)
	if visited[subcktKey] {
		return nil, fmt.Errorf("第 %d 行: 子电路 '%s' 存在循环引用", subckt.Line, subckt.Name)
	}
	visited[subcktKey] = true
	defer delete(visited, subcktKey)
	// 构建端口映射（大小写不敏感）：端口名 → 外部引脚值
	portMap := make(map[string]string)
	for i, port := range subckt.Ports {
		if i < len(instancePins) {
			portMap[strings.ToLower(port.Value)] = instancePins[i].Value
		}
	}
	if len(instancePins) < len(subckt.Ports) {
		return nil, fmt.Errorf("子电路 '%s' 实例引脚数(%d)少于端口数(%d)", subckt.Name, len(instancePins), len(subckt.Ports))
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
		newElem.InstanceName = instanceName + "." + strings.ToUpper(elem.Type) + elem.ID

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
				allSubckts, nextNodeID, nodeNameToID, parseTree, visited, depth+1,
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

// pinDomain 返回引脚所属的物理域编号：0=电气（弱电/强电/布尔），1=气路，2=液压。
// 气/油/电是三套独立系统，同一节点只允许连接同一域的引脚。
func pinDomain(t element.PinType) int {
	switch t {
	case element.PinPneumatic:
		return 1
	case element.PinHydraulic:
		return 2
	}
	return 0
}

// domainName 返回物理域的中文名称，用于错误信息。
func domainName(d int) string {
	switch d {
	case 1:
		return "气路"
	case 2:
		return "液压"
	}
	return "电气"
}

// resolveSubcircuitPin 解析子电路中的引脚值：端口名替换 / 数字保留 / 内部节点分配唯一ID
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
	if _, err := strconv.Atoi(pinValue); err == nil {
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
