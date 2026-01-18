package base

import (
	"circuit/element"
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

// GateType 定义了逻辑门
var GateType element.NodeType = element.AddElement(12, &Gate{ // 使用通用的逻辑门实现
	&element.Config{
		Name: "X",
		Pin:  element.SetPin(element.PinBoolean, "in1", "in2", "out"),
		ValueInit: []any{
			int(GateInverter), // 0: 逻辑门类型
			float64(5.0),      // 1: 高电平电压 (V)
		},
		Voltage: []string{"v"}, // 用于输出的内部电压源
	},
})

// Gate 是一个通用的逻辑门元件
type Gate struct{ *element.Config }

// Stamp 放置输出电压源
func (Gate) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	// 输入数量为 PinNum - 1
	outputNodeIndex := value.Type().PinNum() - 1
	outputNode := value.GetNodes(outputNodeIndex)
	mna.StampVoltageSource(0, outputNode, value.GetVoltSource(0), 0) // 初始电压为 0
}

// DoStep 根据逻辑门类型和输入计算输出
func (Gate) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	gateType := value.GetInt(0)
	highVoltage := value.GetFloat64(1)
	inputCount := value.Type().PinNum() - 1

	var logicResult bool

	// 获取输入状态的辅助函数
	isHigh := func(i int) bool {
		return mna.GetNodeVoltage(value.GetNodes(i)) > highVoltage*0.5
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
		// 针对多个输入的简单奇偶校验
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
	}

	targetV := 0.0
	if logicResult {
		targetV = highVoltage
	}

	mna.UpdateVoltageSource(value.GetVoltSource(0), targetV)
}
