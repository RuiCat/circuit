package interactive

import (
	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
	"fmt"
	"math"
	"strconv"
)

// CoilType 继电器线圈元件类型 (NodeType=15)
// 网表语法: RLY<name> <c1, c2> [eventName1, eventName2, ..., R_coil, L_coil, I_pullin, I_hold]
// 例: RLY1 [0, 1] [RLY1, 100, 0.01, 0.01, 0.005]   — 1个通道
// 例: RLY1 [0, 1] [RLY1_A, RLY1_B, 100, 0.01, 0.01, 0.005] — 2个通道
// 事件通道数：值列表中前导的连续字符串数量
// 线圈等效 RL 串联模型 + 梯形积分 + 磁滞
var CoilType element.NodeType

// Coil 继电器线圈
type Coil struct{ *element.Config }

func init() {
	CoilType = element.AddElement(15, &Coil{
		&element.Config{
			Name: "RLY",
			Pin:  element.SetPin(element.PinLowVoltage, "c1", "c2"),
			ValueInit: []any{
				"RLY1",         // 0: eventName (模板默认)
				float64(100),   // 1: R_coil — 线圈电阻 (Ω)
				float64(0.01),  // 2: L_coil — 线圈电感 (H)
				float64(0.01),  // 3: I_pullin — 吸合电流阈值 (A)
				float64(0.005), // 4: I_hold — 保持电流阈值 (A, < I_pullin)
				float64(0),     // 5: G_eq — 梯形积分等效电导
				float64(0),     // 6: I_hist — 梯形积分历史电流源
				float64(0),     // 7: V_diff — 线圈两端电压差
				float64(0),     // 8: I_coil — 线圈当前电流
				int(0),         // 9: state — 0=释放, 1=吸合
			},
			ValueName: []string{"event", "R_coil", "L_coil", "I_pullin", "I_hold",
				"G_eq", "I_hist", "V_diff", "I_coil", "state"},
			Current:   []int{8}, // 模板: I_coil 在 paramBase+7（单通道时=8）
			OrigValue:  []int{5, 6, 7, 8},
			EventSlots: map[int]int{0: -9}, // 模板: nameIdx(0) → -stateIdx(9)
		},
	})
}

// Base 根据网表动态构建事件通道和参数
// 值列表中前导的连续字符串视为事件名，数量决定通道数
func (c *Coil) Base(elem ast.ElementNode) *element.Config {
	// 统计前导字符串数量作为事件通道数
	nEvents := 0
	for _, v := range elem.Values {
		// 尝试解析为数字，能解析说明不是事件名
		if _, err := strconv.ParseFloat(v.Value, 64); err == nil {
			break
		}
		nEvents++
	}
	if nEvents < 1 {
		nEvents = 1
	}

	// 参数起始位置 = nEvents（事件名之后）
	paramBase := nEvents

	// 9个固定参数 + N个事件名
	valueInit := make([]any, paramBase+9)
	valueNames := make([]string, paramBase+9)

	for i := 0; i < nEvents; i++ {
		eventName := fmt.Sprintf("RLY%d", i+1) // 默认名
		if i < len(elem.Values) {
			eventName = elem.Values[i].Value
		}
		valueInit[i] = eventName
		valueNames[i] = fmt.Sprintf("event%d", i)
	}

	defaultParams := []struct {
		val  any
		name string
	}{
		{float64(100), "R_coil"},
		{float64(0.01), "L_coil"},
		{float64(0.01), "I_pullin"},
		{float64(0.005), "I_hold"},
		{float64(0), "G_eq"},
		{float64(0), "I_hist"},
		{float64(0), "V_diff"},
		{float64(0), "I_coil"},
		{float64(0), "state"},
	}

	stateIdx := paramBase + 8 // state 的位置
	for i, p := range defaultParams {
		idx := paramBase + i
		valueInit[idx] = p.val
		valueNames[idx] = p.name
		// 从网表覆盖前4个参数(R_coil, L_coil, I_pullin, I_hold)
		if i < 4 {
			valIdx := paramBase + i // 在 elem.Values 中的索引
			if valIdx < len(elem.Values) {
				if f, err := strconv.ParseFloat(elem.Values[valIdx].Value, 64); err == nil {
					valueInit[idx] = f
				}
			}
		}
	}

	// 构建 EventSlots: 每个事件名绑定到 state
	eventSlots := make(map[int]int)
	for i := 0; i < nEvents; i++ {
		eventSlots[i] = -stateIdx // 生产者: nameIdx(i) → -stateIdx
	}

	return &element.Config{
		Name:       "RLY",
		Pin:        element.SetPin(element.PinLowVoltage, "c1", "c2"),
		ValueInit:  valueInit,
		ValueName:  valueNames,
		Current:    []int{paramBase + 7}, // I_coil（随事件通道数动态偏移）
		OrigValue:  []int{paramBase + 4, paramBase + 5, paramBase + 6, paramBase + 7}, // G_eq, I_hist, V_diff, I_coil
		EventSlots: eventSlots,
	}
}

// StartIteration 更新梯形积分的历史电流源
func (c *Coil) StartIteration(mna mna.Mna, time mna.Time, value element.NodeFace) {
	nEvents := c.countEvents(value)
	base := nEvents
	L := value.GetFloat64(base + 1) // L_coil
	dt := time.CurrentStep()
	if dt <= 0 {
		return
	}

	// 注意：此变量虽命名为 G_eq，但计算的是电阻值（欧姆），在 Stamp 中用作串联电阻
	G_eq := math.Max(2*L/dt, 1e-12) // 梯形积分的等效电导
	V_diff := mna.GetNodeVoltage(value.GetNodes(0)) - mna.GetNodeVoltage(value.GetNodes(1))
	I_coil := value.GetFloat64(base + 7) // I_coil (上一时间步)
	R := value.GetFloat64(base)          // R_coil
	V_L := V_diff - I_coil*R             // 电感两端电压
	I_hist := G_eq*V_L - I_coil

	value.SetFloat64(base+4, G_eq)   // G_eq
	value.SetFloat64(base+6, I_hist) // I_hist
	value.SetFloat64(base+5, V_diff) // V_diff
}

// Stamp 加盖 RL 串联等效阻抗到 MNA 矩阵
func (c *Coil) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	nEvents := c.countEvents(value)
	base := nEvents
	R := value.GetFloat64(base)        // R_coil
	G_eq := value.GetFloat64(base + 4) // G_eq

	// RL 串联总阻抗: R_coil + 1/G_eq (G_eq=2L/dt, 所以 1/G_eq=dt/(2L))
	if G_eq > 0 {
		mna.StampImpedance(value.GetNodes(0), value.GetNodes(1), R+1/G_eq)
	} else if R > 0 {
		mna.StampImpedance(value.GetNodes(0), value.GetNodes(1), R)
	}
}

// DoStep 加盖历史电流源
func (c *Coil) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	nEvents := c.countEvents(value)
	base := nEvents
	R := value.GetFloat64(base)          // R_coil
	G_eq := value.GetFloat64(base + 4)   // G_eq
	I_hist := value.GetFloat64(base + 6) // I_hist
	denom := 1 + G_eq*R
	if denom > 0 {
		I_source := I_hist / denom
		mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), I_source)
	}
}

// CalculateCurrent 计算线圈电流
func (c *Coil) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	nEvents := c.countEvents(value)
	base := nEvents
	G_eq := value.GetFloat64(base + 4) // G_eq
	R := value.GetFloat64(base)        // R_coil
	V_diff := mna.GetNodeVoltage(value.GetNodes(0)) - mna.GetNodeVoltage(value.GetNodes(1))
	I_hist := value.GetFloat64(base + 6) // I_hist
	denom := 1 + G_eq*R
	var I_coil float64
	if denom > 0 {
		I_coil = (G_eq*V_diff - I_hist) / denom
	}

	value.SetFloat64(base+7, I_coil) // I_coil
	value.SetFloat64(base+5, V_diff) // V_diff
}

// StepFinished 根据磁滞更新吸合状态
func (c *Coil) StepFinished(mna mna.Mna, time mna.Time, value element.NodeFace) {
	nEvents := c.countEvents(value)
	base := nEvents
	I_coil := math.Abs(value.GetFloat64(base + 7)) // I_coil
	I_pullin := value.GetFloat64(base + 2)         // I_pullin
	I_hold := value.GetFloat64(base + 3)           // I_hold
	state := int(value.GetFloat64(base + 8))       // state

	if state == 0 && I_coil >= I_pullin {
		state = 1 // 吸合
	} else if state == 1 && I_coil < I_hold {
		state = 0 // 释放
	}
	value.SetFloat64(base+8, float64(state))
}

// countEvents 统计 eventName 类型的 ValueInit 项数量
// TODO: nEvents 在生命期内不变（完全由 Config.ValueInit 决定），目前 StartIteration/Stamp/DoStep/CalculateCurrent/StepFinished
// 五个方法每次调用皆遍历 ValueInit（O(ValueNum)）。优化方向：在 NewConfig/BuildNodes 阶段将 nEvents 预计算，存入
// ValueInit 固定槽位（如 index 20），或提取为 Coil 结构体字段，后续方法直接读取即可。
func (c *Coil) countEvents(value element.NodeFace) int {
	cfg := value.Base().ConfigPtr
	if cfg == nil {
		return 1
	}
	n := 0
	for _, v := range cfg.ValueInit {
		if _, ok := v.(string); ok {
			n++
		} else {
			break
		}
	}
	if n < 1 {
		n = 1
	}
	return n
}
