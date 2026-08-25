package logic

import (
	"fmt"

	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
)

// 逻辑门类型常量
const (
	GateInverter = iota // 非门
	GateAnd             // 与门
	GateNand            // 与非门
	GateOr              // 或门
	GateNor             // 或非门
	GateXor             // 异或门
	GateXnor            // 同或门
)

// GateType 逻辑门元件类型标识（NodeType=12），网表 "U<name> <in1,in2,out> [type,V_high]"。
var GateType element.NodeType = element.AddElement(12, &Gate{
	&element.Config{
		Name: "U",
		Pin:  element.SetPin(element.PinBoolean, "in1", "in2", "out"),
		ValueInit: []any{
			int(GateInverter), // 0: 逻辑门类型
			float64(5.0),      // 1: 高电平电压 (V)
		},
		ValueName: []string{"type", "V_high"},
		Voltage:   []string{"v"},
	},
})

// Gate 是一个通用的逻辑门元件
type Gate struct{ *element.Config }

// Base 根据网表引脚数动态生成引脚配置（支持 3 输入以上多输入门）。
// 引脚格式：in1,in2,...,inN,out（最后一个引脚为输出）。
func (g *Gate) Base(elem ast.ElementNode) *element.Config {
	nPins := len(elem.Pins)
	if nPins < 2 {
		nPins = 2 // 最少 2 引脚（1 输入 + 输出）
	}
	pinNames := make([]string, nPins)
	for i := 0; i < nPins-1; i++ {
		pinNames[i] = fmt.Sprintf("in%d", i+1)
	}
	pinNames[nPins-1] = "out"

	cfg := g.Config
	return &element.Config{
		Name:       cfg.Name,
		Pin:        element.SetPin(element.PinBoolean, pinNames...),
		ValueInit:  cfg.ValueInit,
		ValueName:  cfg.ValueName,
		Current:    cfg.Current,
		OrigValue:  cfg.OrigValue,
		EventSlots: cfg.EventSlots,
		Voltage:    cfg.Voltage,
	}
}

// Stamp 放置输出电压源到地(GND)，使用上一收敛电压值初始化以避免时间步间的虚假瞬态脉冲。
// 首次时间步(t=0)时使用电压源 ID 奇偶性初始化以打破交叉耦合锁存器对称性。
func (g *Gate) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	pinNum := value.Config().PinNum()
	if pinNum < 1 {
		return
	}

	outputNodeIndex := pinNum - 1
	outputNode := value.GetNodes(outputNodeIndex)

	initV := m.GetNodeVoltage(outputNode)
	if t.Time() == 0.0 && initV == 0.0 {
		if int(value.GetVoltSource(0))&1 != 0 {
			// 打破交叉耦合锁存器对称性：奇电压源给非零初值（3.0V）。
			// 该初值仅影响 t=0 的 Stamp 瞬间，DoStep 立即按输入计算真实输出；
			// 55 CPU 寄存器 D 输入链的误写已由 Gate.Base 动态引脚修复
			// （Stamp/DoStep 用实例引脚数而非模板固定 3 引脚），与初值大小无关。
			initV = 3.0
		}
	}

	m.StampVoltageSource(outputNode, -1, value.GetVoltSource(0), initV)
}

// DoStep 根据逻辑门类型和输入计算输出。
// 使用 0.75 阻尼因子快速收敛交叉耦合锁存器，配合奇偶初始化打破对称性。
func (g *Gate) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	pinNum := value.Config().PinNum()
	if pinNum < 1 {
		return
	}

	gateType := value.GetInt(0)
	highVoltage := value.GetFloat64(1)
	if highVoltage <= 0 {
		highVoltage = 5.0
	}
	inputCount := pinNum - 1
	outputNode := value.GetNodes(inputCount)

	var logicResult bool

	isHigh := func(i int) bool {
		return m.GetNodeVoltage(value.GetNodes(i)) > highVoltage*0.5
	}

	switch gateType {
	case GateInverter:
		logicResult = !isHigh(0)
	case GateAnd:
		logicResult = true
		for i := 0; i < inputCount; i++ {
			if !isHigh(i) {
				logicResult = false
				break
			}
		}
	case GateNand:
		logicResult = true
		for i := 0; i < inputCount; i++ {
			if !isHigh(i) {
				logicResult = false
				break
			}
		}
		logicResult = !logicResult
	case GateOr:
		logicResult = false
		for i := 0; i < inputCount; i++ {
			if isHigh(i) {
				logicResult = true
				break
			}
		}
	case GateNor:
		logicResult = false
		for i := 0; i < inputCount; i++ {
			if isHigh(i) {
				logicResult = true
				break
			}
		}
		logicResult = !logicResult
	case GateXor:
		count := 0
		for i := 0; i < inputCount; i++ {
			if isHigh(i) {
				count++
			}
		}
		logicResult = (count%2 != 0)
	case GateXnor:
		count := 0
		for i := 0; i < inputCount; i++ {
			if isHigh(i) {
				count++
			}
		}
		logicResult = (count%2 == 0)
	default:
		return
	}

	desiredV := 0.0
	if logicResult {
		desiredV = highVoltage
	}

	currentV := m.GetNodeVoltage(outputNode)
	dampedV := currentV + 0.75*(desiredV-currentV)

	m.UpdateVoltageSource(value.GetVoltSource(0), dampedV)

	diff := currentV - desiredV
	if diff < 0 {
		diff = -diff
	}
	if diff > highVoltage*0.1 {
		t.NoConverged()
	}
}
