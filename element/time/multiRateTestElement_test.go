// multiRateTestElement_test.go：多速率用例用的合成元件（仅测试期注册）。
//
// 该元件实现 MultiRateSteppable：DoStep 按接受仿真时刻幂等推进一次并记录推进时刻，
// HoldStep 只重驱动输出（计数 hold 次数）。步长倍率从实例参数槽 0 读取，因此同一类型
// 可以按网表参数被分到快组（倍率 1）或慢组（倍率 >1），用于验证"按参数选择倍率"。
package time

import (
	"math"

	"circuit/element"
	"circuit/mna"
)

// 合成元件的槽位下标。
const (
	mrSlotStride   = iota // 0：步长倍率（网表位置参数，默认 1）。
	mrSlotAdvances        // 1：DoStep 推进次数（按接受时刻幂等）。
	mrSlotHolds           // 2：HoldStep 调用次数。
	mrSlotLastStep        // 3：幂等戳（NaN = 尚未推进）。
	mrSlotTimes           // 4：推进时刻列表（[]float64，写时复制）。
	mrSlotOut             // 5：输出电平（恒 0，避免状态反馈影响时间网格）。
)

// 合成元件的 NodeType（测试专用，避开已用号段）与输出引脚下标。
const (
	mrTestNodeType = element.NodeType(60)
	mrTestPinOut   = 1
)

// mrTestElementType 是合成元件注册后的类型号（测试期注册，名称 MRTEST）。
var mrTestElementType = element.AddElement(mrTestNodeType, &mrTestElement{Config: &element.Config{
	Name: "MRTEST",
	Pin:  element.SetPin(element.PinBoolean, "in", "out"),
	ValueInit: []any{
		float64(1), float64(0), float64(0), math.NaN(), []float64(nil), float64(0),
	},
	ValueName: []string{"stride", "advances", "holds", "lastStep", "advanceTimes", "out"},
	OrigValue: []int{mrSlotAdvances, mrSlotHolds, mrSlotLastStep, mrSlotTimes, mrSlotOut},
	Voltage:   []string{"out"},
}})

// mrTestElement 是支持多速率的合成元件。
type mrTestElement struct{ *element.Config }

// Stamp 为 out 引脚建立电压源（与既有数字元件同口径：初值取节点当前电压）。
func (e *mrTestElement) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	if m == nil || value == nil {
		return
	}
	node := value.GetNodes(mrTestPinOut)
	m.StampVoltageSource(node, mna.Gnd, value.GetVoltSource(0), m.GetNodeVoltage(node))
}

// MultiRateStride 返回实例参数槽 0 的步长倍率（<=1 表示不参与多速率）。
func (e *mrTestElement) MultiRateStride(value element.NodeFace) int {
	if value == nil {
		return 1
	}
	return int(value.GetFloat64(mrSlotStride))
}

// DoStep 在宏边界推进一次内部状态；同一仿真时刻重复调用（牛顿迭代/收敛重试）幂等。
func (e *mrTestElement) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	if t == nil || value == nil {
		return
	}
	now := t.Time()
	if last := value.GetFloat64(mrSlotLastStep); !math.IsNaN(last) && last == now {
		e.drive(m, value)
		return
	}
	value.SetFloat64(mrSlotAdvances, value.GetFloat64(mrSlotAdvances)+1)
	value.SetFloat64(mrSlotLastStep, now)
	mrAppendTime(value, now)
	e.drive(m, value)
}

// HoldStep 只重驱动输出，不推进内部状态。
func (e *mrTestElement) HoldStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	if value == nil {
		return
	}
	value.SetFloat64(mrSlotHolds, value.GetFloat64(mrSlotHolds)+1)
	e.drive(m, value)
}

// drive 把输出电平写回电压源。
func (e *mrTestElement) drive(m mna.Mna, value element.NodeFace) {
	if m == nil {
		return
	}
	m.UpdateVoltageSource(value.GetVoltSource(0), value.GetFloat64(mrSlotOut))
}

// mrTimesOf 读取推进时刻槽。
func mrTimesOf(value element.NodeFace) []float64 {
	node := value.Base()
	if node == nil || mrSlotTimes >= len(node.NodeValue) {
		return nil
	}
	times, _ := node.NodeValue[mrSlotTimes].([]float64)
	return times
}

// mrAppendTime 追加一个推进时刻（写时复制：引擎的 Update/Rollback 只做槽位浅拷贝，
// 原地扩容会把未接受的推进带过回滚）。
func mrAppendTime(value element.NodeFace, now float64) {
	node := value.Base()
	if node == nil || mrSlotTimes >= len(node.NodeValue) {
		return
	}
	if raw := node.NodeValue[mrSlotTimes]; raw != nil {
		if _, ok := raw.([]float64); !ok {
			return
		}
	}
	old := mrTimesOf(value)
	next := make([]float64, len(old)+1)
	copy(next, old)
	next[len(old)] = now
	node.NodeValue[mrSlotTimes] = next
}
