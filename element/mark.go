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

// 元件特性位标记
type Flag uint8

// 元件特性位标记，用于 Config.Flags
const (
	FlagNone       Flag = 0
	FlagReactive   Flag = 1 << iota // 储能元件（电容/电感），影响步长自适应
	FlagNonlinear                   // 非线性元件（三极管/二极管），需 Newton-Raphson 迭代
	FlagCacheStamp                  // 允许引脚电压缓存优化（DoStep 仅依赖引脚电压的元件）
)
