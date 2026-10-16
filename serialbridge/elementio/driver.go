// driver.go 把真实瞬态引擎按"一步"驱动起来：整段 TransientSimulation 循环跑在后台 goroutine 里，
// 适配器用 TimeMNA.StepOnce 触发单步、用"每步成功后的回调"作为步完成信号。
//
// 为什么用回调而不是轮询 TimeMNA.GoodSteps：goodStepCount 是引擎 goroutine 无锁改写的字段，
// 外部轮询属于数据竞争；每步成功后的 call(voltages) 回调（element/time/simulation.go:412-413）
// 经 channel 同步，既给出完成时刻又不碰引擎私有状态。
//
// 起步状态：SetContinuousMode 后立刻 Pause，再启动 goroutine，因此引擎第一次进入
// PushEvents 就停在 Paused 上等待，不会有"建立适配器就自由跑"的窗口。
//
// 有界性：Close/Stop 把状态置 Stopped，PushEvents 返回 false，TransientSimulation 返回并关闭
// done；等待方除 done 之外还有超时出口，绝不无界等待。
package elementio

import (
	"fmt"
	"time"

	"circuit/element"
	etime "circuit/element/time"
)

// engineDriver 是引擎 goroutine 的句柄：单步触发 + 步完成信号 + 退出观测。
type engineDriver struct {
	tm       *etime.TimeMNA
	stepDone chan struct{} // 每步成功后的完成信号（容量 1，非阻塞发送）
	done     chan struct{} // 引擎 goroutine 已退出
	exitErr  error         // 退出原因（nil = 正常停止）
}

// newEngineDriver 装配时间控制器与引擎 goroutine：
// 连续模式 + 暂停起步，并把 notifier 接到 ResumeCond 上，让 StepOnce 能唤醒等待中的引擎。
func newEngineDriver(con *element.Context, targetTime, minStep, maxStep float64) (*engineDriver, error) {
	tm, err := etime.NewTimeMNA(targetTime)
	if err != nil {
		return nil, fmt.Errorf("elementio: 创建时间控制器失败: %w", err)
	}
	// 时序类元件必须显式给步长上界（C1）；只接受成对且合法的取值，否则沿用引擎默认。
	if minStep > 0 && maxStep > minStep {
		if err := tm.SetStepLimits(minStep, maxStep); err != nil {
			return nil, fmt.Errorf("elementio: 设置步长限制失败: %w", err)
		}
	}
	con.Time = tm
	con.InitResumeCond()
	tm.SetNotifier(func() {
		if cond := con.ResumeCond(); cond != nil {
			cond.Broadcast()
		}
	})
	tm.SetContinuousMode()
	tm.Pause()
	d := &engineDriver{
		tm:       tm,
		stepDone: make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	go d.drive(con)
	return d, nil
}

// drive 跑完整段瞬态仿真；exitErr 在 done 关闭前写入，等待方接受 done 后再读它（经 channel 同步）。
func (d *engineDriver) drive(con *element.Context) {
	d.exitErr = etime.TransientSimulation(con, func([]float64) {
		select {
		case d.stepDone <- struct{}{}:
		default: // 已有未取走的完成信号：保留一个即可
		}
	})
	close(d.done)
}

// stepOnce 触发一个时间步（状态 Paused→Stepping，引擎在步首把它改回 Paused 并跑完这一步）。
func (d *engineDriver) stepOnce() { d.tm.StepOnce() }

// drainStepDone 丢掉上一次等待遗留的完成信号，保证 awaitStep 只认本次触发的那一步。
func (d *engineDriver) drainStepDone() {
	select {
	case <-d.stepDone:
	default:
	}
}

// awaitStep 等待本次触发的那一步完成：步完成信号 / 引擎退出 / 超时三条出口。
// 引擎退出时返回退出原因（正常停止即 ErrEngineStopped），超时返回 ErrStepTimeout。
func (d *engineDriver) awaitStep(timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-d.stepDone:
		return nil
	case <-d.done:
		if d.exitErr != nil {
			return fmt.Errorf("%w: %v", ErrEngineStopped, d.exitErr)
		}
		return ErrEngineStopped
	case <-timer.C:
		return fmt.Errorf("%w（%s 内未完成一步）", ErrStepTimeout, timeout)
	}
}

// stop 请求引擎停止（PushEvents 在步首返回 false，仿真循环优雅退出）。
func (d *engineDriver) stop() { d.tm.Stop() }

// failure 报告引擎是否已不可用（已退出且带错误）；未退出时返回 nil。
func (d *engineDriver) failure() error {
	select {
	case <-d.done:
		if d.exitErr != nil {
			return fmt.Errorf("%w: %v", ErrEngineStopped, d.exitErr)
		}
		return ErrEngineStopped
	default:
		return nil
	}
}

// awaitExit 等引擎 goroutine 退出，超时返回 ErrCloseTimeout（不阻塞调用方）。
func (d *engineDriver) awaitExit(timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-d.done:
		return nil
	case <-timer.C:
		return fmt.Errorf("%w（%s 内未退出）", ErrCloseTimeout, timeout)
	}
}
