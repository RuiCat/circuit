package mcp

import (
	etime "circuit/element/time"
	"fmt"
	"time"
)

// ---------- D 组 · 仿真执行（异步 Job） ----------

// jobResult 任务信息载荷。
type jobResult struct {
	SessionID   string         `json:"sessionId"`
	JobID       string         `json:"jobId"`
	Kind        string         `json:"kind"`
	Continuous  bool           `json:"continuous,omitempty"` // 连续模式任务
	SimStatus   string         `json:"simStatus,omitempty"`  // 引擎运行状态（连续模式: running/paused/stepping/stopped）
	State       JobState       `json:"state"`
	Steps       int64          `json:"steps"`
	CurrentTime float64        `json:"currentTime"`
	FinalTime   float64        `json:"finalTime"`
	Error       string         `json:"error,omitempty"`
	StartedAt   time.Time      `json:"startedAt"`
	FinishedAt  time.Time      `json:"finishedAt,omitempty"`
	Probes      []ProbeSummary `json:"probes"`
	StepCount   int            `json:"stepCount,omitempty"` // step 实际执行步数
	TimedOut    bool           `json:"timedOut,omitempty"`  // step/advance 同步等待超时
}

func jobToResult(sess *Session, j *Job) jobResult {
	j.mu.Lock()
	defer j.mu.Unlock()
	r := jobResult{
		SessionID:   sess.ID,
		JobID:       j.ID,
		Kind:        j.Kind,
		Continuous:  j.Continuous,
		State:       j.State,
		Steps:       j.Steps,
		CurrentTime: j.CurrentTime,
		FinalTime:   j.FinalTime,
		Error:       j.Error,
		StartedAt:   j.StartedAt,
		FinishedAt:  j.FinishedAt,
		Probes:      append([]ProbeSummary(nil), j.Probes...),
	}
	if sess.Con != nil {
		if tm, ok := sess.Con.Time.(*etime.TimeMNA); ok {
			r.SimStatus = simStatusName(tm.Status())
		}
	}
	return r
}

// parseSimCfgArgs 从工具参数解析仿真配置（未提供的字段沿用会话默认）。
func parseSimCfgArgs(sess *Session, args map[string]any) (*SimConfig, error) {
	cfg := sess.SimCfg
	if v, ok := argFloat(args, "initialStep"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("initialStep 必须 > 0")
		}
		cfg.InitialStep = v
	}
	if v, ok := argFloat(args, "minStep"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("minStep 必须 > 0")
		}
		cfg.MinStep = v
	}
	if v, ok := argFloat(args, "maxStep"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("maxStep 必须 > 0")
		}
		cfg.MaxStep = v
	}
	if v, ok := argFloat(args, "absTol"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("absTol 必须 > 0")
		}
		cfg.AbsTol = v
	}
	if v, ok := argFloat(args, "relTol"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("relTol 必须 > 0")
		}
		cfg.RelTol = v
	}
	if v, ok := argInt(args, "maxIter"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("maxIter 必须 > 0")
		}
		cfg.MaxIter = v
	}
	if v, ok := argInt(args, "maxSteps"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("maxSteps 必须 > 0")
		}
		cfg.MaxSteps = v
	}
	if v, ok := argInt(args, "parallel"); ok {
		if v < 0 {
			return nil, fmt.Errorf("parallel 不能为负")
		}
		cfg.Parallel = v
	}
	if cfg.MinStep > cfg.MaxStep {
		return nil, fmt.Errorf("minStep(%g) 不能大于 maxStep(%g)", cfg.MinStep, cfg.MaxStep)
	}
	return &cfg, nil
}

// handleRunTransient circuit_run_transient：启动瞬态仿真（后台执行）。
// 参数: sessionId(必填), targetTime(必填, 秒), 可选步长/容差参数,
//
//	nodes[](探针原始节点ID), elements[](探针元件实例名), wait(可选, 同步等待)
func (h *Handler) handleRunTransient(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	target, ok := argFloat(args, "targetTime")
	if !ok || target <= 0 {
		return nil, fmt.Errorf("targetTime 必须 > 0（秒）")
	}
	cfg, err := parseSimCfgArgs(sess, args)
	if err != nil {
		return nil, err
	}
	job, err := sess.RunTransient(target, argIntSlice(args, "nodes"), argStrSlice(args, "elements"), cfg)
	if err != nil {
		return nil, err
	}
	// 可选同步等待（短电路适用）
	if wait, ok := argBool(args, "wait"); ok && wait {
		timeout := time.Duration(0)
		if t, ok := argFloat(args, "waitTimeout"); ok && t > 0 {
			timeout = time.Duration(t * float64(time.Second))
		}
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		job.Wait(timeout)
	}
	return jobToResult(sess, job), nil
}

// handleRunDC circuit_run_dc：DC 工作点分析。
// 参数: sessionId(必填), nodes[](可选), elements[](可选)
func (h *Handler) handleRunDC(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	cfg, err := parseSimCfgArgs(sess, args)
	if err != nil {
		return nil, err
	}
	job, err := sess.RunDC(argIntSlice(args, "nodes"), argStrSlice(args, "elements"), cfg)
	if err != nil {
		return nil, err
	}
	if wait, ok := argBool(args, "wait"); ok && wait {
		job.Wait(30 * time.Second)
	}
	return jobToResult(sess, job), nil
}

// handleJobStatus circuit_job_status：查询任务状态。
// 参数: sessionId(必填), jobId(可选, 默认最近任务)
func (h *Handler) handleJobStatus(args map[string]any) (any, error) {
	sess, err := h.store.Get(argStr(args, "sessionId"))
	if err != nil {
		return nil, err
	}
	job := h.lookupJob(sess, args)
	if job == nil {
		return nil, fmt.Errorf("会话 %s 没有匹配的任务（jobId 或最近任务）", sess.ID)
	}
	return jobToResult(sess, job), nil
}

// handleWaitJob circuit_wait_job：阻塞等待任务完成。
// 参数: sessionId(必填), jobId(可选), timeoutSeconds(可选, 默认30, 上限120)
func (h *Handler) handleWaitJob(args map[string]any) (any, error) {
	sess, err := h.store.Get(argStr(args, "sessionId"))
	if err != nil {
		return nil, err
	}
	job := h.lookupJob(sess, args)
	if job == nil {
		return nil, fmt.Errorf("会话 %s 没有匹配的任务（jobId 或最近任务）", sess.ID)
	}
	timeout := 30 * time.Second
	if t, ok := argFloat(args, "timeoutSeconds"); ok && t > 0 {
		timeout = time.Duration(t * float64(time.Second))
		if timeout > 120*time.Second {
			timeout = 120 * time.Second
		}
	}
	done := job.Wait(timeout)
	res := jobToResult(sess, job)
	if !done {
		res.Error = fmt.Sprintf("等待超时（%s），任务仍在运行", timeout)
	}
	return res, nil
}

// handleCancelJob circuit_cancel_job：取消任务。
// 参数: sessionId(必填), jobId(可选, 默认最近任务)
func (h *Handler) handleCancelJob(args map[string]any) (any, error) {
	sess, err := h.store.Get(argStr(args, "sessionId"))
	if err != nil {
		return nil, err
	}
	job := h.lookupJob(sess, args)
	if job == nil {
		return nil, fmt.Errorf("会话 %s 没有匹配的任务（jobId 或最近任务）", sess.ID)
	}
	job.Cancel()
	// 等待优雅退出（上限 5 秒）
	job.Wait(5 * time.Second)
	return jobToResult(sess, job), nil
}

// lookupJob 按 jobId 查找任务；未指定时返回最近任务。
func (h *Handler) lookupJob(sess *Session, args map[string]any) *Job {
	if id := argStr(args, "jobId"); id != "" {
		return sess.GetJob(id)
	}
	return sess.CurrentJob()
}
