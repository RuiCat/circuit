package base

import (
	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
	"fmt"
	"strings"
)

// 同步元件类型常量
const (
	SyncTypeTruthTable = iota // 真值表模式
	SyncTypeStateMachine       // 状态机模式（预留）
	SyncTypeScript             // 脚本模式（预留）
)

// SyncType 同步元件类型标识。
var SyncType element.NodeType

// Sync 同步元件，实现事件驱动的同步逻辑。
// 模拟同步芯片行为：引脚根据当前时间和其它引脚状态更新。
// 输入引脚读取节点电压，经内部逻辑/状态机评估后，输出电压通过电压源驱动到输出引脚。
type Sync struct{ *element.Config }

// Base 同步元件的配置信息。
// Values[0]: 同步类型 (0=真值表)
// Values[1]: 输入引脚数 N
// Values[2]: 输出引脚数 M
// Values[3]: 高电平电压 V_high
// Values[4+]: 模式相关参数
//   - 真值表模式: 2^N * M 个 bool 值，按行优先排列
func (s *Sync) Base(elem ast.ElementNode) *element.Config {
	nInputs := 2
	nOutputs := 1
	vHigh := float64(5.0)
	syncType := SyncTypeTruthTable

	if len(elem.Values) > 0 {
		syncType = parseSyncInt(elem.Values[0])
	}
	if len(elem.Values) > 1 {
		nInputs = parseSyncInt(elem.Values[1])
		if nInputs < 1 {
			nInputs = 1
		}
	}
	if len(elem.Values) > 2 {
		nOutputs = parseSyncInt(elem.Values[2])
		if nOutputs < 1 {
			nOutputs = 1
		}
	}
	if len(elem.Values) > 3 {
		vHigh = parseSyncFloat(elem.Values[3])
	}

	totalPins := nInputs + nOutputs

	pins := make([]string, totalPins)
	for i := 0; i < nInputs; i++ {
		pins[i] = fmt.Sprintf("in%d", i)
	}
	for i := 0; i < nOutputs; i++ {
		pins[nInputs+i] = fmt.Sprintf("out%d", i)
	}

	voltageNames := make([]string, nOutputs)
	for i := range voltageNames {
		voltageNames[i] = fmt.Sprintf("vo%d", i)
	}

	// 真值表数据：2^N * M 个 int (0/1)
	truthTableSize := 1
	for i := 0; i < nInputs; i++ {
		truthTableSize *= 2
	}
	truthTableSize *= nOutputs

	valueInit := []any{
		syncType, // 0: 同步类型
		nInputs,  // 1: 输入引脚数
		nOutputs, // 2: 输出引脚数
		vHigh,    // 3: 高电平电压
	}
	valueNames := []string{"type", "n_inputs", "n_outputs", "V_high"}

	// 真值表索引
	truthIdx := len(valueInit)
	for i := 0; i < truthTableSize; i++ {
		valueInit = append(valueInit, false)
		valueNames = append(valueNames, fmt.Sprintf("tt%d", i))
	}

	// 从 Values 填充真值表数据
	if syncType == SyncTypeTruthTable {
		for i := truthIdx; i < len(valueInit) && (i-truthIdx) < len(elem.Values); i++ {
			vi := i - truthIdx + 4 // +4 for type, nInputs, nOutputs, Vhigh
			if vi < len(elem.Values) {
				valueInit[i] = parseSyncBool(elem.Values[vi])
			}
		}
	}

	origValue := []int{truthIdx}

	return &element.Config{
		Name:      "S",
		Pin:       element.SetPin(element.PinBoolean, pins...),
		ValueInit: valueInit,
		ValueName: valueNames,
		Voltage:   voltageNames,
		OrigValue: origValue,
		Flags:     element.FlagNonlinear,
	}
}

// Stamp 为每个输出引脚放置独立电压源，驱动输出电压。
func (s *Sync) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	nInputs := value.GetInt(1)
	nOutputs := value.GetInt(2)

	for i := 0; i < nOutputs; i++ {
		outNode := value.GetNodes(nInputs + i)
		m.StampVoltageSource(outNode, -1, value.GetVoltSource(i), 0)
	}
}

// DoStep 读取输入电压，根据同步类型评估逻辑，更新输出引脚电压。
func (s *Sync) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	syncType := value.GetInt(0)
	nInputs := value.GetInt(1)
	nOutputs := value.GetInt(2)
	vHigh := value.GetFloat64(3)

	switch syncType {
	case SyncTypeTruthTable:
		s.doTStepTruthTable(m, t, value, nInputs, nOutputs, vHigh)
	}
}

// doTStepTruthTable 真值表模式：读取输入组合，查表输出。
func (s *Sync) doTStepTruthTable(m mna.Mna, t mna.Time, value element.NodeFace,
	nInputs, nOutputs int, vHigh float64) {

	// 构建输入组合索引
	inputIdx := 0
	for i := 0; i < nInputs; i++ {
		inNode := value.GetNodes(i)
		inputIdx <<= 1
		if m.GetNodeVoltage(inNode) > vHigh*0.5 {
			inputIdx |= 1
		}
	}

	// 真值表基准索引 (在 ValueInit 中的位置)
	truthBase := 4 // type, nInputs, nOutputs, Vhigh

	// 查找真值表并驱动输出
	for i := 0; i < nOutputs; i++ {
		ttIdx := truthBase + inputIdx*nOutputs + i
		desired := 0.0
		if value.GetBool(ttIdx) {
			desired = vHigh
		}

		outNode := value.GetNodes(nInputs + i)
		currentV := m.GetNodeVoltage(outNode)
		dampedV := currentV + 0.5*(desired-currentV)

		m.UpdateVoltageSource(value.GetVoltSource(i), dampedV)

		diff := currentV - desired
		if diff < 0 {
			diff = -diff
		}
		if diff > vHigh*0.1 {
			t.NoConverged()
		}
	}
}

// Reset 重置同步元件的内部状态。
func (s *Sync) Reset(value element.NodeFace) {
	// 当前无需特殊重置
}

// parseSyncInt 从 ast.Value 解析 int。
func parseSyncInt(v ast.Value) int {
	switch strings.ToLower(v.Value) {
	case "0":
		return 0
	case "1":
		return 1
	case "2":
		return 2
	case "3":
		return 3
	case "4":
		return 4
	case "5":
		return 5
	case "6":
		return 6
	case "7":
		return 7
	case "8":
		return 8
	default:
		return 0
	}
}

// parseSyncFloat 从 ast.Value 解析 float64。
func parseSyncFloat(v ast.Value) float64 {
	switch strings.ToLower(v.Value) {
	case "5":
		return 5.0
	case "3.3":
		return 3.3
	case "1.8":
		return 1.8
	default:
		return 5.0
	}
}

// parseSyncBool 从 ast.Value 解析 bool。
func parseSyncBool(v ast.Value) bool {
	switch strings.ToLower(v.Value) {
	case "1", "true":
		return true
	default:
		return false
	}
}

// init 注册同步元件到全局元件列表。
func init() {
	if _, ok := element.ElementListName["S"]; ok {
		SyncType = element.ElementListName["S"]
		return
	}
	SyncType = element.AddElement(element.NodeType(18), &Sync{
		&element.Config{
			Name: "S",
			Pin:  element.SetPin(element.PinBoolean, "in0", "in1", "out0"),
			ValueInit: []any{
				SyncTypeTruthTable, // 0: 同步类型
				2,                  // 1: 输入数 N
				1,                  // 2: 输出数 M
				float64(5.0),       // 3: 高电平电压
				false,              // 4: tt0 (in=00 -> out=0)
				false,              // 5: tt1 (in=01 -> out=0)
				false,              // 6: tt2 (in=10 -> out=0)
				true,               // 7: tt3 (in=11 -> out=1) - AND gate
			},
			ValueName: []string{"type", "n_inputs", "n_outputs", "V_high",
				"tt0", "tt1", "tt2", "tt3"},
			Voltage:   []string{"vo0"},
			OrigValue: []int{4},
			Flags:     element.FlagNonlinear,
		},
	})
	element.ElementListName["S"] = SyncType
}
