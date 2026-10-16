// robustness_test.go：K1 的「不得 panic」——非法/退化入参走文档化路径而不是崩溃。
package digital

import (
	"math"
	"testing"

	"circuit/element"
	"circuit/mna"
)

// TestNoPanicOnDegenerateInputs 覆盖 nil 接收者、nil 接口与 typed-nil 底层节点。
func TestNoPanicOnDegenerateInputs(t *testing.T) {
	var q *DelayQueue
	if err := q.Plan(nil, 0, 1, 1); err == nil {
		t.Fatal("nil 队列必须报错而不是 panic")
	}
	q.Apply(nil, nil, nil, 0, 0) // 全 nil：不得 panic

	DriveNow(nil, 0, 1)  // nil MNA：不得 panic
	MarkStamp(nil, 0, 1) // nil 节点：不得 panic

	if got := SinceLastStamp(nil, 0, 1); !math.IsInf(got, 1) {
		t.Fatalf("nil 节点应视为尚未推进(+Inf)，实际 %v", got)
	}
	if got := SinceLastStamp(newSlotNode(2), -1, 1); !math.IsInf(got, 1) {
		t.Fatalf("负槽位下标应视为尚未推进(+Inf)，实际 %v", got)
	}

	// typed-nil 的 *element.Node 作为 NodeFace：Base() 返回 nil，必须走降级路径。
	var typedNil *element.Node
	var face element.NodeFace = typedNil
	if got := SinceLastStamp(face, 0, 1); !math.IsInf(got, 1) {
		t.Fatalf("typed-nil 节点应视为尚未推进(+Inf)，实际 %v", got)
	}
	MarkStamp(face, 0, 1)
	if err := (&DelayQueue{}).Plan(face, 0, 1, 5); err == nil {
		t.Fatal("typed-nil 节点必须报错而不是 panic")
	}
	(&DelayQueue{}).Apply(nil, &stubTime{now: 1}, face, 0, 0) // 不得 panic
}

// TestDriveNowWritesVoltageSource 是输出通道的独立用例：只经电压源写入，不写解向量。
func TestDriveNowWritesVoltageSource(t *testing.T) {
	m := mna.NewMnaUpdate(2, 2)
	DriveNow(m, 1, 5.0)
	if got := m.GetZ().Get(testVoltRow + 1); got != 5.0 {
		t.Fatalf("DriveNow 应写入电压源 1 的 Z 行，实际 %v", got)
	}
	if got := m.GetX().Get(testVoltRow + 1); got != 0.0 {
		t.Fatalf("DriveNow 不得直接写解向量 X，实际 %v", got)
	}
	// 越界电压源由 mna 自身忽略，不得 panic。
	DriveNow(m, mna.VoltageID(99), 1.0)
	DriveNow(m, mna.VoltageID(-1), 1.0)
}
