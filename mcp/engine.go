package mcp

import (
	"circuit/element"
	"circuit/mna"
	"fmt"
	"sort"
	"strings"
)

// ---------- 单位与类型辅助 ----------

// unitOf 返回元件电流/流量列的单位：气动=kg/s，液压=m³/s，其余=A。
func unitOf(cfg *element.Config) string {
	for _, p := range cfg.Pin {
		switch p.Type {
		case element.PinPneumatic:
			return "kg/s"
		case element.PinHydraulic:
			return "m³/s"
		}
	}
	return "A"
}

// nodeUnitOf 返回节点"电压"列单位：气动/液压=Pa，其余=V。
func nodeUnitOf(t element.PinType) string {
	switch t {
	case element.PinPneumatic, element.PinHydraulic:
		return "Pa"
	}
	return "V"
}

// pinTypeName 引脚类型名称。
func pinTypeName(t element.PinType) string {
	switch t {
	case element.PinLowVoltage:
		return "low_voltage"
	case element.PinHighVoltage:
		return "high_voltage"
	case element.PinBoolean:
		return "boolean"
	case element.PinPneumatic:
		return "pneumatic"
	case element.PinHydraulic:
		return "hydraulic"
	}
	return "unknown"
}

// domainOf 由引脚类型推断元件域：pneumatic > hydraulic > electrical。
func domainOf(cfg *element.Config) string {
	domain := "electrical"
	for _, p := range cfg.Pin {
		switch p.Type {
		case element.PinPneumatic:
			return "pneumatic"
		case element.PinHydraulic:
			domain = "hydraulic"
		case element.PinBoolean:
			if domain == "electrical" {
				domain = "logic"
			}
		}
	}
	return domain
}

// nodeDomain 节点域：由连接元件的引脚类型推断（电气/气动/液压/逻辑）。
func nodeDomain(pinType element.PinType) string {
	switch pinType {
	case element.PinPneumatic:
		return "pneumatic"
	case element.PinHydraulic:
		return "hydraulic"
	case element.PinBoolean:
		return "logic"
	}
	return "electrical"
}

// instanceName 返回元件实例名（回退到类型名）。
func instanceName(elem element.NodeFace) string {
	if elem == nil {
		return ""
	}
	inst := elem.Base().InstanceName
	if inst == "" {
		inst = elem.Config().GetName()
	}
	return inst
}

// ---------- 元件信息模型 ----------

// ElementInfo 元件信息。
type ElementInfo struct {
	Instance    string           `json:"instance"`              // 实例名（如 "R1"、"X1.R1"）
	Type        string           `json:"type"`                  // 元件类型名（如 "r"）
	NodeType    uint             `json:"nodeType"`              // 注册类型 ID
	Domain      string           `json:"domain"`                // 域
	Pins        []PinInfo        `json:"pins"`                  // 引脚 → 原始节点
	Params      []ParamValue     `json:"params"`                // 参数值
	Currents    []CurrentInfo    `json:"currents,omitempty"`    // 当前电流值
	VoltSources []VoltSourceInfo `json:"voltSources,omitempty"` // 电压源
	Internal    []string         `json:"internalPins,omitempty"`
	Flags       []string         `json:"flags,omitempty"`
	Parent      string           `json:"parent,omitempty"`   // 父实例名（层级）
	Children    []string         `json:"children,omitempty"` // 子实例名（层级）
}

// PinInfo 引脚信息。
type PinInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Node    int    `json:"node"`                   // 原始节点 ID（-1=地）
	Compact int    `json:"compactIndex,omitempty"` // 紧凑索引
}

// ParamValue 参数值。
type ParamValue struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// CurrentInfo 电流信息。
type CurrentInfo struct {
	Name  string  `json:"name"` // 如 "I(R1)"、"I(R1.I)"
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// VoltSourceInfo 电压源信息。
type VoltSourceInfo struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

// paramType 返回参数值的 JSON 类型名。
func paramType(v any) string {
	switch v.(type) {
	case float64:
		return "number"
	case int:
		return "integer"
	case bool:
		return "boolean"
	case string:
		return "string"
	case nil:
		return "null"
	}
	return fmt.Sprintf("%T", v)
}

// flagsOf 返回特性位名称列表。
func flagsOf(f element.Flag) []string {
	var out []string
	if f&element.FlagReactive != 0 {
		out = append(out, "reactive")
	}
	if f&element.FlagNonlinear != 0 {
		out = append(out, "nonlinear")
	}
	if f&element.FlagCacheStamp != 0 {
		out = append(out, "cache_stamp")
	}
	return out
}

// buildElementInfo 构建单个元件信息（参数为当前值）。
func buildElementInfo(elem element.NodeFace, con *element.Context) ElementInfo {
	base := elem.Base()
	cfg := elem.Config()
	info := ElementInfo{
		Instance: instanceName(elem),
		Type:     cfg.GetName(),
		NodeType: uint(base.NodeType),
		Domain:   domainOf(cfg),
		Flags:    flagsOf(cfg.Flags),
	}
	// 引脚
	for i, p := range cfg.Pin {
		nodeID := elem.GetNodes(i)
		pi := PinInfo{
			Name: p.Name,
			Type: pinTypeName(p.Type),
			Node: int(nodeID),
		}
		if idx, ok := con.CompactNodeID[nodeID]; ok {
			pi.Compact = idx
		}
		info.Pins = append(info.Pins, pi)
	}
	// 参数
	for i, name := range cfg.ValueName {
		v := base.NodeValue[i]
		info.Params = append(info.Params, ParamValue{
			Index: i,
			Name:  name,
			Type:  paramType(v),
			Value: v,
		})
	}
	// 电压源
	for i, name := range cfg.Voltage {
		info.VoltSources = append(info.VoltSources, VoltSourceInfo{
			Name: name,
			ID:   int(elem.GetVoltSource(i)),
		})
	}
	info.Internal = append(info.Internal, cfg.Internal...)
	// 电流
	for _, ci := range elementCurrents(con, elem) {
		info.Currents = append(info.Currents, ci)
	}
	// 层级
	if p := elem.GetParent(); p != nil {
		info.Parent = instanceName(p)
	}
	for _, c := range elem.GetChildren() {
		info.Children = append(info.Children, instanceName(c))
	}
	return info
}

// elementCurrents 计算元件所有电流槽的当前值（电压源支路电流 + Config.Current 声明）。
func elementCurrents(con *element.Context, elem element.NodeFace) []CurrentInfo {
	cfg := elem.Config()
	inst := instanceName(elem)
	unit := unitOf(cfg)
	var out []CurrentInfo
	// 1) 电压源支路电流（由 mna 求解器直接给出）
	for vs := 0; vs < cfg.VoltageNum(); vs++ {
		vsID := elem.GetVoltSource(vs)
		name := fmt.Sprintf("I(%s)", inst)
		if cfg.VoltageNum() > 1 {
			name = fmt.Sprintf("I(%s[%d])", inst, vs)
		}
		out = append(out, CurrentInfo{
			Name:  name,
			Value: con.GetVoltageSourceCurrent(vsID),
			Unit:  unit,
		})
	}
	// 2) 元件声明的自身电流索引（跳过 int/bool/string 槽位）
	for _, idx := range cfg.Current {
		if idx < 0 || idx >= cfg.ValueNum() {
			continue
		}
		switch cfg.ValueInit[idx].(type) {
		case float64, nil:
		default:
			continue
		}
		name := fmt.Sprintf("I(%s)", inst)
		if len(cfg.Current) > 1 {
			suffix := cfg.ValueName[idx]
			if suffix == "" {
				suffix = fmt.Sprintf("i%d", idx)
			}
			name = fmt.Sprintf("I(%s.%s)", inst, suffix)
		}
		out = append(out, CurrentInfo{
			Name:  name,
			Value: elem.GetFloat64(idx),
			Unit:  unit,
		})
	}
	return out
}

// ---------- 节点信息 ----------

// NodeInfo 节点信息。
type NodeInfo struct {
	RawID        int      `json:"rawId"`                       // 原始节点 ID
	CompactIdx   int      `json:"compactIndex"`                // 紧凑索引
	Domain       string   `json:"domain"`                      // 域
	Unit         string   `json:"unit"`                        // 电压/压力单位
	Elements     []string `json:"elements"`                    // 连接的元件实例名
	Hierarchical []string `json:"hierarchicalPaths,omitempty"` // 层级路径（如 "X1.out"）
	Voltage      float64  `json:"voltage"`                     // 当前电压/压力值
}

// buildNodeInfos 构建节点列表（含当前电压值）。
func buildNodeInfos(con *element.Context) []NodeInfo {
	// 节点域：按首个连接引脚类型推断
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
	// 节点 → 连接元件
	nodeElems := make(map[int][]string)
	for _, elem := range con.Nodelist {
		cfg := elem.Config()
		inst := instanceName(elem)
		for i := range cfg.Pin {
			compactIdx := int(elem.GetNodes(i))
			if compactIdx < 0 {
				continue
			}
			nodeElems[compactIdx] = append(nodeElems[compactIdx], inst)
		}
	}
	// 层级路径 → 紧凑索引
	pathByCompact := make(map[int][]string)
	for path, compactIdx := range con.HierarchicalNodeID {
		pathByCompact[int(compactIdx)] = append(pathByCompact[int(compactIdx)], path)
	}

	rawIDs := make([]int, 0, len(con.CompactNodeID))
	for rawID := range con.CompactNodeID {
		rawIDs = append(rawIDs, int(rawID))
	}
	sort.Ints(rawIDs)

	out := make([]NodeInfo, 0, len(rawIDs))
	for _, rawID := range rawIDs {
		compactIdx := con.CompactNodeID[mna.NodeID(rawID)]
		info := NodeInfo{
			RawID:      rawID,
			CompactIdx: compactIdx,
			Domain:     "electrical",
			Unit:       "V",
			Voltage:    con.GetRawNodeVoltage(mna.NodeID(rawID)),
		}
		if pt, ok := nodePinType[compactIdx]; ok {
			info.Domain = nodeDomain(pt)
			info.Unit = nodeUnitOf(pt)
		}
		if insts, ok := nodeElems[compactIdx]; ok {
			info.Elements = insts
		}
		if paths, ok := pathByCompact[compactIdx]; ok {
			sort.Strings(paths)
			info.Hierarchical = paths
		}
		out = append(out, info)
	}
	return out
}

// ---------- 网表统计 ----------

// NetlistStats 网表加载统计。
type NetlistStats struct {
	Elements    int      `json:"elements"`
	Nodes       int      `json:"nodes"`
	VoltSources int      `json:"voltSources"`
	Subcircuits int      `json:"subcircuits"`
	HasReactive bool     `json:"hasReactive"`
	Events      []string `json:"events,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

// collectStats 从上下文收集统计信息。
func collectStats(con *element.Context) NetlistStats {
	st := NetlistStats{
		Elements:    len(con.Nodelist),
		Nodes:       len(con.CompactNodeID),
		VoltSources: con.GetVoltageSourcesNum(),
		HasReactive: con.HasReactiveElements(),
	}
	seen := map[string]bool{}
	for _, elem := range con.Nodelist {
		cfg := elem.Config()
		if cfg == nil {
			continue
		}
		for nameIdx, valueIdx := range cfg.EventSlots {
			if valueIdx < 0 {
				continue // 生产者不算
			}
			if nameIdx >= 0 && nameIdx < len(elem.Base().NodeValue) {
				if s, ok := elem.Base().NodeValue[nameIdx].(string); ok && s != "" {
					seen[s] = true
				}
			}
		}
	}
	for e := range seen {
		st.Events = append(st.Events, e)
	}
	sort.Strings(st.Events)
	return st
}

// netlistError 解析网表错误信息（提取首行错误）。
func netlistError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if i := strings.IndexByte(msg, '\n'); i > 0 {
		msg = msg[:i]
	}
	return fmt.Errorf("%s", msg)
}
