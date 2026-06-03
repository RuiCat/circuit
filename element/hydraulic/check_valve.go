// 油路单向阀元件实现。
package hydraulic

import (
	"circuit/element"
	"circuit/mna"
)

// HCVType 油路单向阀类型标识(NodeType=32)，网表"HCV<name> <n1,n2> [R_on,R_off,V_open]"。
var HCVType element.NodeType = element.AddElement(32, &HCV{
	&element.Config{
		Name: "hcv",
		Pin:  element.SetPin(element.PinHydraulic, "hcv1", "hcv2"),
		ValueInit: []any{
			float64(1e5),  // 0: 正向导通液阻 R_on
			float64(1e15), // 1: 反向关断液阻 R_off
			float64(0),    // 2: 开启阈值压力 V_open
			int(0),        // 3: 状态 (0=反向关断, 1=正向导通)
			int(0),        // 4: 上次盖印状态 last_state
		},
		ValueName: []string{"R_on", "R_off", "V_open", "state", "last_state"},
		Current:   []int{3},
		OrigValue: []int{4},
	},
})

// HCVCheckValve 油路单向阀。等价于理想二极管，DoStep中检测压力差切换阻抗。
type HCV struct{ *element.Config }

// DoStep 检测压力差极性并动态调整阻抗，标记未收敛
// 当状态发生变化时（正向↔反向），标记未收敛并触发重新 Stamp
func (s *HCV) DoStep(mna mna.Mna, time mna.Time, value element.NodeFace) {
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	v_diff := v1 - v2
	V_open := value.GetFloat64(2)

	var newState int
	if v_diff > V_open {
		newState = 1
	} else {
		newState = 0
	}

	lastState := value.GetInt(4)
	if newState != lastState {
		value.SetInt(3, newState)
		time.NoConverged()
		s.Stamp(mna, time, value)
		value.SetInt(4, newState)
	}
}

// Stamp 根据单向阀状态将导通/关断液阻加盖到MNA导纳矩阵
// 正向导通时使用 R_on，反向关断时使用 R_off
func (HCV) Stamp(mna mna.Mna, time mna.Time, value element.NodeFace) {
	state := value.GetInt(3)
	var resistance float64
	if state == 1 {
		resistance = value.GetFloat64(0)
	} else {
		resistance = value.GetFloat64(1)
	}
	mna.StampImpedance(value.GetNodes(0), value.GetNodes(1), resistance)
}

// CalculateCurrent 根据单向阀当前状态和压力差计算流量
func (HCV) CalculateCurrent(mna mna.Mna, time mna.Time, value element.NodeFace) {
	state := value.GetInt(3)
	var resistance float64
	if state == 1 {
		resistance = value.GetFloat64(0)
	} else {
		resistance = value.GetFloat64(1)
	}
	v1 := mna.GetNodeVoltage(value.GetNodes(0))
	v2 := mna.GetNodeVoltage(value.GetNodes(1))
	if resistance > 0 {
		current := (v1 - v2) / resistance
		mna.StampCurrentSource(value.GetNodes(0), value.GetNodes(1), -current)
	}
}
