package element

import (
	"circuit/mna"
	"log"
)

// Mark 用于区分事件的标记
type Mark uint8

// 接口回调类型
const (
	MarkReset            Mark = iota // 元件重置
	MarkStartIteration               // 步长迭代开始
	MarkStamp                        // 加盖线性贡献
	MarkDoStep                       // 执行仿真
	MarkCalculateCurrent             // 电流计算
	MarkStepFinished                 // 步长迭代结束
)

// CallMark 统一调用
func CallMark(mark Mark, mna mna.MNA, time mna.Time, value []NodeFace) {
	switch mark {
	case MarkReset:
		for _, v := range value {
			ElementLitt[v.Base().NodeType].Reset(v)
		}
	case MarkStartIteration:
		for _, v := range value {
			ElementLitt[v.Base().NodeType].StartIteration(mna, time, v)
		}
	case MarkStamp:
		for _, v := range value {
			ElementLitt[v.Base().NodeType].Stamp(mna, time, v)
		}
	case MarkDoStep:
		for _, v := range value {
			ElementLitt[v.Base().NodeType].DoStep(mna, time, v)
		}
	case MarkCalculateCurrent:
		for _, v := range value {
			ElementLitt[v.Base().NodeType].CalculateCurrent(mna, time, v)
		}
	case MarkStepFinished:
		for _, v := range value {
			ElementLitt[v.Base().NodeType].StepFinished(mna, time, v)
		}
	default:
		log.Fatalf("未知 CallMark 操作: %d", mark)
	}
}
