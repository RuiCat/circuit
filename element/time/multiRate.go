// multiRate.go：多速率调度骨架——按元件声明的步长倍率把元件分快慢两组，
// 慢组只在宏边界推进内部状态，其余全局步只重驱动输出。
//
// 设计（P3/T6）：
//   - 分组：fast = 未实现 MultiRateSteppable（或声明倍率 <=1）的元件；
//     slow = 实现该接口且倍率 >1。分组在仿真启动时构建一次（NodeType → 注册表 face），
//     按 Nodelist 顺序分发，顺序与原 CallMark(MarkDoStep) 一致。
//   - 跨组对齐：只有一条全局时间网格。宏边界 = 全局接受步序号 % stride == 0；步序号按
//     CurrentTime() 变化惰性递增，被拒绝的步重试同一时刻不重复计数 ⇒ 触发点截断与减半步长
//     都不会把宏边界切到两个全局步之间，慢组推进时刻必然是某个全局接受步的时刻。
//   - 回滚/收敛：慢组在非宏步不调用 DoStep（不推进内部状态），改调 HoldStep 只重驱动输出。
//     这是必须的：A/Z 在每次牛顿迭代开始都会回滚到线性加盖状态，跳过 DoStep 会让输出源
//     RHS 退回 Stamp 时的初值，而不是"保持上一拍输出"。宏边界步被拒绝时，引擎的 OrigValue
//     回滚会撤销该步的状态推进；重试时步序号不变仍判推进，由元件自己的幂等戳避免二次推进。
//   - 默认路径：未启用或没有慢元件时 plan=nil，doStep 原样走 CallMark/ParallelCallMark，
//     不改任何数值、收敛或时间推进语义。
//
// 已知边界：本骨架不减少全局时间步数——真正的"大步长"需要矩阵分块（BTF）后对块单独积分，
// 属 roadmap 阶段 6，不在本轮；本轮省的是慢组的 DoStep 调用数。并行 DoStep 与多速率
// 互斥（显式报错，不静默降级）。
package time

import (
	"fmt"

	"circuit/element"
	"circuit/mna"
)

// MultiRateSteppable 是元件侧的多速率能力声明（可选接口）。实现它的元件声明自己的
// 步长倍率 N，并承诺在被跳过的全局步里由 HoldStep 重驱动全部输出。
//
// 契约：
//   - MultiRateStride 返回 N>=1；N<=1 表示该实例不参与多速率（等价 fast 组），
//     因此"同一元件按参数选择倍率"可以直接读实例槽位实现；
//   - DoStep 在宏边界（全局接受步序号能被 N 整除）被调用，负责推进内部状态；
//   - HoldStep 在其余全局步的每次牛顿迭代被调用，只重驱动输出（电压源写回），
//     **不得推进内部状态**——否则等于没有降速。
type MultiRateSteppable interface {
	MultiRateStride(value element.NodeFace) int
	HoldStep(m mna.Mna, t mna.Time, value element.NodeFace)
}

// multiRateEntry 是计划里的一个元件分派项。
type multiRateEntry struct {
	node      element.NodeFace
	face      element.ElementFaceList
	steppable MultiRateSteppable // nil = fast 组。
	stride    int                // >1 才可能进入 slow 组。
}

// multiRatePlan 是一次仿真内的多速率分派计划。只在主仿真循环（单 goroutine）使用：
// beginStep/run 不并发调用，测试读取计划状态也在仿真结束后。
type multiRatePlan struct {
	entries  []multiRateEntry
	hold     []bool  // 本步每个 entry 是否走 HoldStep（按步缓存，迭代/重试不重算）。
	index    int     // 0 基全局接受步序号；-1 = 尚未开始。
	lastTime float64 // 上一次计入的仿真时刻。
	started  bool
	slow     int // 慢元件数（>0 才保留计划）。
}

// newMultiRatePlan 构建多速率计划：enabled=false 或没有慢元件时返回 (nil, nil)，
// 调用方据此走原分派路径（默认行为零影响）。
func newMultiRatePlan(con *element.Context, enabled bool) (*multiRatePlan, error) {
	if !enabled {
		return nil, nil
	}
	if con.ParallelOpts != nil {
		return nil, fmt.Errorf("多速率调度与并行 DoStep 互斥（ParallelOpts 非空）；请关闭并行或停用多速率")
	}
	plan := &multiRatePlan{index: -1}
	for _, node := range con.Nodelist {
		face, ok := element.ElementList[node.Type()]
		if !ok {
			continue // 与 CallMark 的既有行为一致：未注册类型静默跳过。
		}
		entry := multiRateEntry{node: node, face: face}
		if steppable, ok := face.(MultiRateSteppable); ok {
			entry.steppable = steppable
			entry.stride = steppable.MultiRateStride(node)
			if entry.stride < 1 {
				entry.stride = 1
			}
			if entry.stride > 1 {
				plan.slow++
			}
		}
		plan.entries = append(plan.entries, entry)
	}
	if plan.slow == 0 {
		return nil, nil
	}
	plan.hold = make([]bool, len(plan.entries))
	return plan, nil
}

// run 执行一步 DoStep 分派，顺序与 CallMark(MarkDoStep) 的 Nodelist 顺序一致。
// 慢元件在非宏步走 HoldStep（只重驱动输出），其余元件每步、每迭代都走 DoStep。
func (p *multiRatePlan) run(con *element.Context) error {
	p.beginStep(con.CurrentTime())
	for i := range p.entries {
		entry := &p.entries[i]
		if entry.stride > 1 && p.hold[i] {
			entry.steppable.HoldStep(con, con.Time, entry.node)
			continue
		}
		entry.face.DoStep(con, con.Time, entry.node)
	}
	return nil
}

// beginStep 在全局接受步推进时更新快慢组判定；同一时刻的牛顿迭代/重试不重复计数。
// 用 CurrentTime 的严格相等判定"还是同一步"：主循环只在接受解之后推进时间，
// 被拒绝的步在同一时刻重试（见 simulation.go 的 luRetry / 残差拒绝路径）。
func (p *multiRatePlan) beginStep(now float64) {
	if p.started && now == p.lastTime {
		return
	}
	p.started = true
	p.lastTime = now
	p.index++
	for i := range p.entries {
		entry := &p.entries[i]
		p.hold[i] = entry.stride > 1 && p.index%entry.stride != 0
	}
}
