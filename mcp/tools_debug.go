package mcp

import (
	"circuit/element"
	"circuit/mna"
	"fmt"
	"math"
	"sort"
	"strings"
)

// ---------- 调试组 · 运行数据检视（矩阵 / 元件内部状态） ----------
//
// 这两个工具服务于"运行过程中出问题"的排查：连续模式暂停后，
// 直接查看 MNA 矩阵 A、右侧向量 Z、解向量 X 的原始数值，
// 以及元件全部 NodeValue（含内部状态、电压源目标值），
// 不再依赖猜测或层级路径映射。

// MatrixRow 矩阵单行非零元。
type MatrixRow struct {
	Row    int             `json:"row"`              // 行号（紧凑索引；节点行 < NodeNum，电压源约束行 ≥ NodeNum）
	Type   string          `json:"type"`             // "node" / "voltageSource"
	Right  float64         `json:"right"`            // Z 右侧向量值（节点行=注入电流；电压源行=源电压）
	Solve  float64         `json:"solve"`            // X 解向量值（节点行=节点电压；电压源行=电压源电流）
	Elems  []MatrixEntry   `json:"elems"`            // 非零元（列 → 值）
	Labels map[string]any  `json:"labels,omitempty"` // 行号 → 语义标签（层级路径/元件+引脚）
}

// MatrixEntry 矩阵单元素。
type MatrixEntry struct {
	Col    int     `json:"col"`
	Value  float64 `json:"value"`
	Label  string  `json:"label,omitempty"` // 列语义标签（紧凑 → 层级路径/元件）
}

// MatrixDump 矩阵检视结果。
type MatrixDump struct {
	NodeNum        int         `json:"nodeNum"`        // 节点数
	VoltSourceNum  int         `json:"voltSourceNum"`  // 电压源数
	TotalDim       int         `json:"totalDim"`       // 总维度
	Rows           []MatrixRow `json:"rows"`           // 请求的行
	CompactToRaw   map[int]int `json:"compactToRaw"`   // 紧凑 → 原始节点 ID
	RawToCompact   map[int]int `json:"rawToCompact"`   // 原始节点 ID → 紧凑
	HierLabels     map[int]string `json:"hierLabels,omitempty"` // 紧凑索引 → 层级路径
	VSIndexToElem  map[int]string   `json:"vsIndexToElem,omitempty"` // 电压源索引 → 元件实例名
}

// buildCompactLabels 构建紧凑索引 → 语义标签 的反查表。
func buildCompactLabels(con *element.Context) (map[int]string, map[int]string, map[int]int, map[int]int) {
	hierByCompact := make(map[int]string)
	for path, compact := range con.HierarchicalNodeID {
		if _, dup := hierByCompact[int(compact)]; !dup {
			hierByCompact[int(compact)] = path
		}
	}
	vsByIndex := make(map[int]string)
	// 电压源索引按元件遍历顺序分配（VoltSource 数组值即全局索引）
	for _, elem := range con.Nodelist {
		for _, v := range elem.Base().VoltSource {
			if _, dup := vsByIndex[int(v)]; !dup {
				vsByIndex[int(v)] = instanceName(elem)
			}
		}
	}
	compactToRaw := make(map[int]int)
	rawToCompact := make(map[int]int)
	for rawID, compactIdx := range con.CompactNodeID {
		compactToRaw[compactIdx] = int(rawID)
		rawToCompact[int(rawID)] = compactIdx
	}
	return hierByCompact, vsByIndex, compactToRaw, rawToCompact
}

// handleDebugMatrix circuit_debug_matrix：查看 MNA 矩阵行（A/Z/X）。
// 参数: sessionId(必填), rows[](可选, 行号数组；默认全部节点行), all(可选, true=含全部电压源行)
// 行号含义：0..NodeNum-1 为节点行，≥NodeNum 为电压源约束行。
func (h *Handler) handleDebugMatrix(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	if j := sess.RunningJob(); j != nil {
		return nil, fmt.Errorf("会话 %s 有运行中的任务，请先暂停（circuit_pause）再查看矩阵", sess.ID)
	}
	con := sess.Con
	if con == nil {
		return nil, fmt.Errorf("会话 %s 尚未加载网表", sess.ID)
	}
	A := con.GetA()
	Z := con.GetZ()
	X := con.GetX()
	nodeNum := con.GetNodeNum()
	vsNum := con.GetVoltageSourcesNum()
	totalDim := nodeNum + vsNum

	hierByCompact, vsByIndex, compactToRaw, rawToCompact := buildCompactLabels(con)

	wantRows := argIntSlice(args, "rows")
	all, _ := argBool(args, "all")
	if len(wantRows) == 0 {
		for i := 0; i < nodeNum; i++ {
			wantRows = append(wantRows, i)
		}
		if all {
			for i := nodeNum; i < totalDim; i++ {
				wantRows = append(wantRows, i)
			}
		}
	}

	rowLabel := func(r int) (string, bool) {
		if r < nodeNum {
			if p, ok := hierByCompact[r]; ok {
				return p, true
			}
			return fmt.Sprintf("node[%d]", r), false
		}
		vi := r - nodeNum
		if e, ok := vsByIndex[vi]; ok {
			return fmt.Sprintf("vs[%d]=%s", vi, e), true
		}
		return fmt.Sprintf("vs[%d]", vi), false
	}
	colLabel := func(c int) string {
		if c < nodeNum {
			if p, ok := hierByCompact[c]; ok {
				return p
			}
			return fmt.Sprintf("node[%d]", c)
		}
		vi := c - nodeNum
		if e, ok := vsByIndex[vi]; ok {
			return fmt.Sprintf("vs[%d]=%s", vi, e)
		}
		return fmt.Sprintf("vs[%d]", vi)
	}

	out := MatrixDump{
		NodeNum:       nodeNum,
		VoltSourceNum: vsNum,
		TotalDim:      totalDim,
		CompactToRaw:  compactToRaw,
		RawToCompact:  rawToCompact,
		HierLabels:    make(map[int]string),
		VSIndexToElem: make(map[int]string),
	}
	// 填充标签（只放有语义的）
	for compact, path := range hierByCompact {
		out.HierLabels[compact] = path
	}
	for vi, elem := range vsByIndex {
		out.VSIndexToElem[vi] = elem
	}

	for _, r := range wantRows {
		if r < 0 || r >= totalDim {
			continue
		}
		cols, vals := A.GetRow(r)
		row := MatrixRow{
			Row:   r,
			Right: Z.Get(r),
			Solve: X.Get(r),
		}
		if r < nodeNum {
			row.Type = "node"
		} else {
			row.Type = "voltageSource"
		}
		if lab, ok := rowLabel(r); ok {
			row.Labels = map[string]any{"semantic": lab}
		}
		for i, c := range cols {
			row.Elems = append(row.Elems, MatrixEntry{
				Col:   c,
				Value: vals.Get(i),
				Label: colLabel(c),
			})
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

// ElementState 元件内部状态（NodeValue 全量 + 电压源目标值）。
type ElementState struct {
	Instance string         `json:"instance"`
	Type     string         `json:"type"`
	NodeType uint           `json:"nodeType"`
	Params   []ParamValue   `json:"params"`   // 全部 NodeValue（含内部状态）
	Pins     []PinInfo      `json:"pins"`     // 引脚 → 原始/紧凑节点
	VoltSrc  []VoltSourceInfo `json:"voltSources,omitempty"`
	VoltVals []float64      `json:"voltTargets,omitempty"` // 电压源目标电压（若可读）
	OrigVal  map[int]any    `json:"origValue,omitempty"`   // 回滚备份值
}

// handleDebugElement circuit_debug_element：查看元件全部内部状态（NodeValue/OrigValue/电压源）。
// 参数: sessionId(必填), instance(必填)。
// 与 circuit_get_element 的区别：不裁剪参数，直接暴露 NodeValue 原值，
// 用于排查 DFF 状态、事件值、回滚备份等运行数据。
func (h *Handler) handleDebugElement(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	inst := argStr(args, "instance")
	if inst == "" {
		return nil, errMissing("instance")
	}
	con := sess.Con
	if con == nil {
		return nil, fmt.Errorf("会话 %s 尚未加载网表", sess.ID)
	}
	for _, elem := range con.Nodelist {
		if instanceName(elem) != inst {
			continue
		}
		node := elem.Base()
		cfg := node.Config()
		out := ElementState{
			Instance: inst,
			Type:     cfg.GetName(),
			NodeType: uint(node.NodeType),
			Pins:     buildPinInfos(elem, con),
			VoltSrc:  buildVoltSourceInfos(elem),
		}
		for i := 0; i < len(node.NodeValue); i++ {
			name := ""
			if cfg != nil && i < len(cfg.ValueName) {
				name = cfg.ValueName[i]
			}
			out.Params = append(out.Params, ParamValue{
				Index: i,
				Name:  name,
				Type:  paramType(node.NodeValue[i]),
				Value: node.NodeValue[i],
			})
		}
		if len(node.OrigValue) > 0 {
			out.OrigVal = make(map[int]any)
			for k, v := range node.OrigValue {
				out.OrigVal[k] = v
			}
		}
		// 电压源目标值：通过 MNA 读取约束行 RHS（若仿真已执行）
		if len(node.VoltSource) > 0 && sess.Con.Time != nil {
			nodeNum := con.GetNodeNum()
			for _, v := range node.VoltSource {
				row := nodeNum + int(v)
				if row < 0 {
					continue
				}
				out.VoltVals = append(out.VoltVals, con.GetZ().Get(row))
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("元件 %s 不在网表中", inst)
}

// buildPinInfos 引脚 → 原始/紧凑节点。
func buildPinInfos(elem element.NodeFace, con *element.Context) []PinInfo {
	node := elem.Base()
	cfg := node.Config()
	out := make([]PinInfo, 0, len(node.Nodes))
	for i, compact := range node.Nodes {
		name := ""
		if cfg != nil && i < len(cfg.Pin) {
			name = cfg.Pin[i].Name
		}
		raw := -1
		for rid, c := range con.CompactNodeID {
			if c == int(compact) {
				raw = int(rid)
				break
			}
		}
		out = append(out, PinInfo{Name: name, Node: raw, Compact: int(compact)})
	}
	return out
}

// buildVoltSourceInfos 电压源 ID 列表。
func buildVoltSourceInfos(elem element.NodeFace) []VoltSourceInfo {
	out := make([]VoltSourceInfo, 0, len(elem.Base().VoltSource))
	for _, v := range elem.Base().VoltSource {
		out = append(out, VoltSourceInfo{Name: fmt.Sprintf("vs%d", v), ID: int(v)})
	}
	return out
}

// handleDebugNodeVoltages circuit_debug_node_voltages：按紧凑索引读电压（与 get_node_values 的原始 ID 互补）。
// 参数: sessionId(必填), compact[](可选, 紧凑索引列表), all(可选, true=全部节点)。
func (h *Handler) handleDebugNodeVoltages(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	con := sess.Con
	if con == nil {
		return nil, fmt.Errorf("会话 %s 尚未加载网表", sess.ID)
	}
	_, _, compactToRaw, _ := buildCompactLabels(con)

	idxs := argIntSlice(args, "compact")
	all, _ := argBool(args, "all")
	if len(idxs) == 0 && all {
		for i := 0; i < con.GetNodeNum(); i++ {
			idxs = append(idxs, i)
		}
	}
	type nv struct {
		Compact int     `json:"compact"`
		Raw     int     `json:"raw"`
		Voltage float64 `json:"voltage"`
		Label   string  `json:"label,omitempty"`
	}
	out := struct {
		Items []nv `json:"items"`
	}{}
	for _, idx := range idxs {
		if idx < 0 || idx >= con.GetNodeNum() {
			continue
		}
		item := nv{
			Compact: idx,
			Raw:     compactToRaw[idx],
			Voltage: con.GetNodeVoltage(mna.NodeID(idx)),
		}
		if lab, ok := reverseLookup(con, idx); ok {
			item.Label = lab
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// reverseLookup 紧凑索引 → 层级路径（首个命中）。
func reverseLookup(con *element.Context, compact int) (string, bool) {
	for path, c := range con.HierarchicalNodeID {
		if int(c) == compact {
			return path, true
		}
	}
	return "", false
}

// debugFmt 格式化浮点（紧凑、去尾零）。
func debugFmt(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Sprintf("%v", f)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", f), "0"), ".")
}

var _ = element.NodeType(0)
var _ = sort.Ints
