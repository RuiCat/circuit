// SR锁存器元件实现，组合逻辑电平触发的置位/复位锁存。
package logic

import (
	"circuit/element"
	"circuit/mna"
)

// SRType SR锁存器类型标识(NodeType=40)，网表"SR<name> <s,r,q,nq> [V_high]"。
var SRLatchType element.NodeType = element.AddElement(40, &SRLatch{
	&element.Config{
		Name: "SR",
		Pin:  element.SetPin(element.PinBoolean, "s", "r", "q", "nq"),
		ValueInit: []any{
			float64(5.0), // 0: 高电平电压 V_high
			false,        // 1: 内部 Q 状态
		},
		ValueName: []string{"V_high", "q_state"},
		Voltage:   []string{"vq", "vnq"},
		Flags:     element.FlagNonlinear,
		OrigValue: []int{1},
	},
})

// SRLatch SR锁存器。电平触发组合逻辑: S=1→Q=1, R=1→Q=0, S=R=1→禁止(Q=NQ=0)。
// 直接电压源驱动Q/NQ输出。
type SRLatch struct{ *element.Config }

// Stamp 根据当前Q状态设置Q和NQ输出电压源初始值。
func (f *SRLatch) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {
	highVoltage := value.GetFloat64(0)
	qState := value.GetBool(1)

	qVoltage := 0.0
	nqVoltage := highVoltage
	if qState {
		qVoltage = highVoltage
		nqVoltage = 0.0
	}

	m.StampVoltageSource(value.GetNodes(2), -1, value.GetVoltSource(0), qVoltage)
	m.StampVoltageSource(value.GetNodes(3), -1, value.GetVoltSource(1), nqVoltage)
}

// DoStep SR锁存器每步仿真。读取S/R输入端电平(>50%V_high)，
// 根据组合逻辑真值表更新Q并驱动输出电压源。
func (f *SRLatch) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {
	highVoltage := value.GetFloat64(0)
	qState := value.GetBool(1)

	sNode := value.GetNodes(0)
	rNode := value.GetNodes(1)
	s := m.GetNodeVoltage(sNode) > highVoltage*0.5
	r := m.GetNodeVoltage(rNode) > highVoltage*0.5

	prevQ := qState

	if s && r {
		qState = false
	} else if s {
		qState = true
	} else if r {
		qState = false
	}

	if qState != prevQ {
		value.SetBool(1, qState)
		t.NoConverged()
	}

	desiredQ := 0.0
	if qState {
		desiredQ = highVoltage
	}
	desiredNQ := 0.0
	if !qState {
		desiredNQ = highVoltage
	}

	m.UpdateVoltageSource(value.GetVoltSource(0), desiredQ)
	m.UpdateVoltageSource(value.GetVoltSource(1), desiredNQ)
}
