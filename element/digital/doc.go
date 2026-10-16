// Package digital 是数字/时序类元件共用的骨架包：只提供电平原语、输出驱动、
// 步长守卫、延时队列与步进幂等工具，不含触发点、不含墙钟、不注册任何元件类型。
//
// 契约与不变量（K1）：
//   - 只依赖 circuit/mna 与 circuit/element，不 import circuit/element/time；
//   - 不注册触发点（不触碰 mna.Time 的触发点设置通道）：需要精确定时的元件只能靠
//     调用方显式 max-step + StepGuard；
//   - 不注册元件类型（本包是库包，NodeType 号段由消费方声明）；
//   - 跨步状态一律经 element.NodeFace 的 NodeValue 槽位读写，包内不持有全局可变状态。
//
// 元件侧典型用法：
//   - 输出：DriveNow 把电压写进本元件持有的电压源（唯一允许的输出方式）；
//   - 输入判决：Level.IsHigh；输入负载：InputLoad.ConductanceAt；
//   - 延时翻转：DelayQueue.Plan 排程 + 每步 DelayQueue.Apply 写回到期项；
//   - 时序自检：StepGuard 在当前步长超过元件标称时序的 1/10 时返回错误；
//   - 步进幂等：SinceLastStamp/MarkStamp 保证同一仿真时刻对同一元件只推进一次。
package digital
