package element

// Mark 用于区分事件的标记。
type Mark uint8

// 接口回调类型。
const (
	MarkReset            Mark = iota // 元件重置。
	MarkStartIteration               // 步长迭代开始。
	MarkStamp                        // 加盖线性贡献。
	MarkDoStep                       // 执行仿真。
	MarkCalculateCurrent             // 电流计算。
	MarkStepFinished                 // 步长迭代结束。
	MarkUpdateElements               // 更新元件状态。
	MarkRollbackElements             // 回滚元件状态。
)
