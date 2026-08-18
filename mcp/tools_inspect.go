package mcp

import (
	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
	"fmt"
	"sort"
	"strings"
)

// ---------- B 组 · 元件目录与电路检视 ----------

// ComponentTypeInfo 元件类型目录条目。
type ComponentTypeInfo struct {
	Name           string      `json:"name"`                     // 网表类型名（如 "r"）
	NodeType       uint        `json:"nodeType"`                 // 注册 ID
	Domain         string      `json:"domain"`                   // 域
	Pins           []PinInfo   `json:"pins"`                     // 引脚（节点为空）
	Params         []ParamInfo `json:"params"`                   // 参数定义
	VoltageSources []string    `json:"voltageSources,omitempty"` // 电压源名
	InternalPins   []string    `json:"internalPins,omitempty"`
	Flags          []string    `json:"flags,omitempty"`
	CurrentSlots   []string    `json:"currentSlots,omitempty"` // 电流槽参数名
}

// ParamInfo 参数定义。
type ParamInfo struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default any    `json:"default,omitempty"`
}

// buildComponentTypes 从全局注册表构建元件类型目录。
func buildComponentTypes() []ComponentTypeInfo {
	names := make([]string, 0, len(element.ElementListName))
	for name := range element.ElementListName {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ComponentTypeInfo, 0, len(names))
	seen := map[uint]bool{}
	for _, name := range names {
		nt := element.ElementListName[name]
		if seen[uint(nt)] {
			continue
		}
		seen[uint(nt)] = true
		face, ok := element.ElementList[nt]
		if !ok {
			continue
		}
		cfg := face.Base(ast.ElementNode{})
		if cfg == nil {
			continue
		}
		info := ComponentTypeInfo{
			Name:           cfg.GetName(),
			NodeType:       uint(nt),
			Domain:         domainOf(cfg),
			VoltageSources: append([]string(nil), cfg.Voltage...),
			InternalPins:   append([]string(nil), cfg.Internal...),
			Flags:          flagsOf(cfg.Flags),
		}
		for _, p := range cfg.Pin {
			info.Pins = append(info.Pins, PinInfo{Name: p.Name, Type: pinTypeName(p.Type)})
		}
		for i, pn := range cfg.ValueName {
			var def any
			if i < len(cfg.ValueInit) {
				def = cfg.ValueInit[i]
			}
			info.Params = append(info.Params, ParamInfo{
				Index:   i,
				Name:    pn,
				Type:    paramType(def),
				Default: def,
			})
		}
		for _, idx := range cfg.Current {
			if idx >= 0 && idx < len(cfg.ValueName) {
				info.CurrentSlots = append(info.CurrentSlots, cfg.ValueName[idx])
			}
		}
		out = append(out, info)
	}
	return out
}

// handleListComponentTypes circuit_list_component_types：全部已注册元件类型目录。
// 参数: domain(可选, 过滤 "electrical"/"pneumatic"/"hydraulic"/"logic")
func (h *Handler) handleListComponentTypes(args map[string]any) (any, error) {
	domain := argStr(args, "domain")
	all := buildComponentTypes()
	if domain == "" {
		return map[string]any{"types": all, "count": len(all)}, nil
	}
	filtered := make([]ComponentTypeInfo, 0, len(all))
	for _, t := range all {
		if t.Domain == domain {
			filtered = append(filtered, t)
		}
	}
	return map[string]any{"types": filtered, "count": len(filtered)}, nil
}

// handleComponentHelp circuit_component_help：单个元件类型详细说明。
// 参数: type(必填, 类型名，如 "r"/"v"/"q")
func (h *Handler) handleComponentHelp(args map[string]any) (any, error) {
	typeName := strings.ToLower(argStr(args, "type"))
	if typeName == "" {
		return nil, errMissing("type")
	}
	nt, ok := element.ElementListName[typeName]
	if !ok {
		return nil, fmt.Errorf("元件类型 %q 不存在（可用 circuit_list_component_types 查看全部类型）", typeName)
	}
	face, ok := element.ElementList[nt]
	if !ok {
		return nil, fmt.Errorf("元件类型 %q 无实现", typeName)
	}
	cfg := face.Base(ast.ElementNode{})
	if cfg == nil {
		return nil, fmt.Errorf("元件类型 %q 配置为空", typeName)
	}
	info := ComponentTypeInfo{
		Name:           cfg.GetName(),
		NodeType:       uint(nt),
		Domain:         domainOf(cfg),
		VoltageSources: append([]string(nil), cfg.Voltage...),
		InternalPins:   append([]string(nil), cfg.Internal...),
		Flags:          flagsOf(cfg.Flags),
	}
	for _, p := range cfg.Pin {
		info.Pins = append(info.Pins, PinInfo{Name: p.Name, Type: pinTypeName(p.Type)})
	}
	for i, pn := range cfg.ValueName {
		var def any
		if i < len(cfg.ValueInit) {
			def = cfg.ValueInit[i]
		}
		info.Params = append(info.Params, ParamInfo{Index: i, Name: pn, Type: paramType(def), Default: def})
	}
	return info, nil
}

// handleListElements circuit_list_elements：会话内元件清单。
// 参数: sessionId(必填), type(可选过滤), instance(可选过滤)
// 注意：仿真运行中引擎会修改元件内部状态，此工具仅在无运行中任务时可用。
func (h *Handler) handleListElements(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	if j := sess.RunningJob(); j != nil {
		return nil, fmt.Errorf("会话 %s 有运行中的任务 %s，请等待完成或取消后再检视元件（运行中可查询节点电压快照）", sess.ID, j.ID)
	}
	filterType := strings.ToLower(argStr(args, "type"))
	filterInst := argStr(args, "instance")
	con := sess.Con
	out := make([]ElementInfo, 0, len(con.Nodelist))
	for _, elem := range con.Nodelist {
		inst := instanceName(elem)
		if filterInst != "" && inst != filterInst {
			continue
		}
		if filterType != "" && elem.Config().GetName() != filterType {
			continue
		}
		out = append(out, buildElementInfo(elem, con))
	}
	return map[string]any{"elements": out, "count": len(out)}, nil
}

// handleListNodes circuit_list_nodes：会话内节点清单（含当前电压）。
// 参数: sessionId(必填)
// 仿真运行中电压取自任务逐步快照。
func (h *Handler) handleListNodes(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	var snapV map[string]float64
	if j := sess.RunningJob(); j != nil {
		snapV, _ = j.Snapshot()
	}
	nodes := buildNodeInfos(sess.Con)
	if snapV != nil {
		for i := range nodes {
			if v, ok := snapV[fmt.Sprintf("node_%d", nodes[i].RawID)]; ok {
				nodes[i].Voltage = v
			}
		}
	}
	return map[string]any{"nodes": nodes, "count": len(nodes)}, nil
}

// handleGetElement circuit_get_element：单个元件详情。
// 参数: sessionId(必填), instance(必填)
// 注意：仿真运行中引擎会修改元件内部状态，此工具仅在无运行中任务时可用。
func (h *Handler) handleGetElement(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	if j := sess.RunningJob(); j != nil {
		return nil, fmt.Errorf("会话 %s 有运行中的任务 %s，请等待完成或取消后再检视元件（运行中可查询节点电压快照）", sess.ID, j.ID)
	}
	inst := argStr(args, "instance")
	if inst == "" {
		return nil, errMissing("instance")
	}
	for _, elem := range sess.Con.Nodelist {
		if instanceName(elem) == inst {
			return buildElementInfo(elem, sess.Con), nil
		}
	}
	return nil, fmt.Errorf("元件 %s 不在网表中（可用 circuit_list_elements 查看）", inst)
}

// handleGetNodeValues circuit_get_node_values：读取节点电压/压力（可同时查多个，支持层级路径）。
// 参数: sessionId(必填), nodes[](可选, 原始节点ID), paths[](可选, 层级路径如 "X1.out")
// 仿真运行中返回任务逐步快照值。
func (h *Handler) handleGetNodeValues(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	con := sess.Con
	rawIDs := argIntSlice(args, "nodes")
	paths := argStrSlice(args, "paths")

	// 运行中的任务：电压取任务快照（快照按原始节点 ID 键控）
	var snapV map[string]float64
	if j := sess.RunningJob(); j != nil {
		snapV, _ = j.Snapshot()
	}
	compactToRaw := make(map[int]int)
	for rawID, compactIdx := range con.CompactNodeID {
		compactToRaw[compactIdx] = int(rawID)
	}

	// 节点 → 单位
	nodeUnit := make(map[int]string)
	nodePinType := make(map[int]element.PinType)
	for _, elem := range con.Nodelist {
		cfg := elem.Config()
		for i := range cfg.Pin {
			compactIdx := int(elem.GetNodes(i))
			if compactIdx < 0 {
				continue
			}
			if _, ok := nodePinType[compactIdx]; !ok {
				nodePinType[compactIdx] = cfg.Pin[i].Type
			}
		}
	}
	for compactIdx, pt := range nodePinType {
		nodeUnit[compactIdx] = nodeUnitOf(pt)
	}

	type nodeValue struct {
		RawID int     `json:"rawId,omitempty"`
		Path  string  `json:"path,omitempty"`
		Name  string  `json:"name"`
		Value float64 `json:"value"`
		Unit  string  `json:"unit"`
	}
	var values []nodeValue
	readCompact := func(compactIdx int, name string, rawID int, path string) {
		unit := "V"
		if u, ok := nodeUnit[compactIdx]; ok {
			unit = u
		}
		val := 0.0
		if snapV != nil {
			// 运行中：取任务快照
			if rawID >= 0 {
				val = snapV[fmt.Sprintf("node_%d", rawID)]
			} else if r, ok := compactToRaw[compactIdx]; ok {
				val = snapV[fmt.Sprintf("node_%d", r)]
			}
		} else {
			val = con.GetNodeVoltage(mna.NodeID(compactIdx))
		}
		values = append(values, nodeValue{
			RawID: rawID,
			Path:  path,
			Name:  name,
			Value: val,
			Unit:  unit,
		})
	}
	// 节点
	if len(rawIDs) == 0 && len(paths) == 0 {
		// 默认全部节点
		rawAll := make([]int, 0, len(con.CompactNodeID))
		for rawID := range con.CompactNodeID {
			rawAll = append(rawAll, int(rawID))
		}
		sort.Ints(rawAll)
		rawIDs = rawAll
	}
	for _, rawID := range rawIDs {
		compactIdx, ok := con.CompactNodeID[mna.NodeID(rawID)]
		if !ok {
			return nil, fmt.Errorf("节点 %d 不在网表中", rawID)
		}
		readCompact(compactIdx, fmt.Sprintf("node_%d", rawID), rawID, "")
	}
	// 层级路径
	for _, path := range paths {
		compactIdx, ok := con.HierarchicalNodeID[path]
		if !ok {
			return nil, fmt.Errorf("层级路径 %q 不存在（可用 circuit_list_nodes 查看 hierarchicalPaths）", path)
		}
		readCompact(int(compactIdx), path, -1, path)
	}
	return map[string]any{"values": values}, nil
}
