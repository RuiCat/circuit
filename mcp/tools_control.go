package mcp

import (
	etime "circuit/element/time"
	"circuit/mna"
	"fmt"
	"time"
)

// ---------- D2 组 · 连续仿真控制（暂停 / 恢复 / 单步 / 推进） ----------
//
// 交互模型：circuit_run_continuous 启动永不自动结束的仿真（连续模式），
// 之后用 pause/resume/step/advance 逐步驱动。暂停时引擎阻塞在每步开头的
// 事件同步点（PushEvents），不触碰元件状态——此时可安全调用
// circuit_set_element_param 修改参数，恢复仿真会自动按新参数重新加盖线性元件。

// simStatusName 引擎运行状态名称。
func simStatusName(s mna.SimStatus) string {
	switch s {
	case mna.StatusPaused:
		return "paused"
	case mna.StatusStepping:
		return "stepping"
	case mna.StatusStopped:
		return "stopped"
	default:
		return "running"
	}
}

// controlTarget 定位会话的连续模式任务及其时间控制器。
// 非连续模式任务 / 已终结任务拒绝控制。
func (h *Handler) controlTarget(sess *Session, args map[string]any) (*Job, *etime.TimeMNA, error) {
	job := h.lookupJob(sess, args)
	if job == nil {
		return nil, nil, fmt.Errorf("会话 %s 没有匹配的任务（jobId 或最近任务）", sess.ID)
	}
	if !job.Continuous {
		return nil, nil, fmt.Errorf("任务 %s 不是连续模式仿真（由 circuit_run_continuous 启动），暂停/恢复/单步/推进仅对其可用", job.ID)
	}
	switch job.StateStr() {
	case JobDone, JobError, JobCancelled:
		return nil, nil, fmt.Errorf("任务 %s 已结束（%s），无法控制", job.ID, job.StateStr())
	}
	if sess.Con == nil {
		return nil, nil, fmt.Errorf("会话 %s 尚未加载网表", sess.ID)
	}
	tm, ok := sess.Con.Time.(*etime.TimeMNA)
	if !ok {
		return nil, nil, fmt.Errorf("会话 %s 的时间控制器类型异常", sess.ID)
	}
	return job, tm, nil
}

// waitForStatus 轮询等待时间控制器进入指定状态，超时返回 false。
// 仅用于控制工具（单步/推进）同步等待，步长自适应下"一步"耗时不确定。
func waitForStatus(tm *etime.TimeMNA, want mna.SimStatus, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for tm.Status() != want {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
	return true
}

// handleRunContinuous circuit_run_continuous：启动连续模式仿真（后台，永不自动结束）。
// 参数: sessionId(必填), 可选步长/容差参数（同 run_transient）,
//
//	nodes[](探针原始节点ID), elements[](探针元件实例名)
//
// 之后用 circuit_pause / circuit_resume / circuit_step / circuit_advance 驱动，
// 用 circuit_cancel_job 结束。
func (h *Handler) handleRunContinuous(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	cfg, err := parseSimCfgArgs(sess, args)
	if err != nil {
		return nil, err
	}
	job, err := sess.RunContinuous(argIntSlice(args, "nodes"), argStrSlice(args, "elements"), cfg)
	if err != nil {
		return nil, err
	}
	return jobToResult(sess, job), nil
}

// handlePause circuit_pause：暂停连续仿真（幂等）。
// 参数: sessionId(必填), jobId(可选, 默认最近任务)
// 暂停后引擎停在每步开头的同步点，可安全调用 circuit_set_element_param 修改参数。
func (h *Handler) handlePause(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	job, tm, err := h.controlTarget(sess, args)
	if err != nil {
		return nil, err
	}
	tm.Pause()
	return jobToResult(sess, job), nil
}

// handleResume circuit_resume：恢复已暂停的连续仿真。
// 参数: sessionId(必填), jobId(可选, 默认最近任务)
func (h *Handler) handleResume(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	job, tm, err := h.controlTarget(sess, args)
	if err != nil {
		return nil, err
	}
	tm.Resume()
	return jobToResult(sess, job), nil
}

// handleStep circuit_step：单步推进连续仿真。
// 参数: sessionId(必填), count(可选, 步数, 默认 1), jobId(可选)
// 每"步"是引擎的一个自适应时间步（步长由引擎决定，可用 set_sim_params 限制 maxStep）。
// 同步等待每步完成（自动回到 paused），超时返回当前进度。
func (h *Handler) handleStep(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	job, tm, err := h.controlTarget(sess, args)
	if err != nil {
		return nil, err
	}
	count := 1
	if c, ok := argInt(args, "count"); ok && c > 0 {
		count = c
	}
	executed := 0
	timedOut := false
	for i := 0; i < count; i++ {
		tm.StepOnce()
		if !waitForStatus(tm, mna.StatusPaused, 10*time.Second) {
			timedOut = true
			break
		}
		executed++
	}
	r := jobToResult(sess, job)
	r.StepCount = executed
	r.TimedOut = timedOut
	return r, nil
}

// handleAdvance circuit_advance：推进指定时间后自动暂停。
// 参数: sessionId(必填), duration(必填, 秒, >0), jobId(可选),
//
//	wait(可选, 是否同步等待推进完成, 默认 true), waitTimeout(可选, 秒, 默认 30)
func (h *Handler) handleAdvance(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	job, tm, err := h.controlTarget(sess, args)
	if err != nil {
		return nil, err
	}
	duration, ok := argFloat(args, "duration")
	if !ok || duration <= 0 {
		return nil, fmt.Errorf("duration 必须 > 0（秒）")
	}
	wait := true
	if w, ok := argBool(args, "wait"); ok {
		wait = w
	}
	timeout := 30 * time.Second
	if t, ok := argFloat(args, "waitTimeout"); ok && t > 0 {
		timeout = time.Duration(t * float64(time.Second))
	}
	if err := tm.AdvanceFor(duration); err != nil {
		return nil, err
	}
	timedOut := false
	if wait {
		// 到达目标时间后 IsSimulationFinished 自动置 Paused
		if !waitForStatus(tm, mna.StatusPaused, timeout) {
			timedOut = true
		}
	}
	r := jobToResult(sess, job)
	r.TimedOut = timedOut
	return r, nil
}
