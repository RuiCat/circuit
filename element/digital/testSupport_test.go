// testSupport_test.go：digital 包测试用的最小替身（不引入 circuit/element/time）。
package digital

import (
	"circuit/element"
	"circuit/mna"
)

// timeStubBase 承载 mna.Time 其余（本包不调用的）方法。单独抽一层是必须的：
// 直接嵌入 mna.Time 会让字段名 Time 与 stubTime 自己声明的 Time() 方法重名而编译失败。
type timeStubBase struct {
	mna.Time
}

// stubTime 是只实现 CurrentTime/CurrentStep/Time 的 mna.Time 替身：其余方法由嵌入的 nil 接口提供
// （本包只调用这三个方法，测试不会触达未实现路径）。
type stubTime struct {
	timeStubBase
	now  float64 // 当前仿真时间。
	step float64 // 当前步长。
}

// Time 返回替身设置的仿真时间（与 mna.Time 接口的方法签名一致）。
func (s *stubTime) Time() float64 { return s.now }

// CurrentTime 返回替身设置的仿真时间。
func (s *stubTime) CurrentTime() float64 { return s.now }

// CurrentStep 返回替身设置的步长。
func (s *stubTime) CurrentStep() float64 { return s.step }

// newSlotNode 建一个带 n 个 float64 槽位的元件节点，模拟 Config.ValueInit 全 float64 的元件实例。
func newSlotNode(n int) *element.Node {
	node := &element.Node{NodeValue: make([]any, n)}
	for i := range node.NodeValue {
		node.NodeValue[i] = float64(0)
	}
	return node
}
