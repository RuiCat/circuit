package mcp

import (
	"math"
	"strings"
	"testing"
	"time"
)

// waitSimStatus 轮询任务引擎状态，超时失败。
func waitSimStatus(t *testing.T, h *Handler, sid string, want string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		st := callTool(t, h, "circuit_job_status", map[string]any{"sessionId": sid})
		if s, _ := st["simStatus"].(string); s == want {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待 simStatus=%s 超时，最近状态: %v", want, st)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestToolContinuousControl 连续模式完整工作流：
// run_continuous → pause → 暂停中改参数 → step ×3 → resume → advance → cancel。
func TestToolContinuousControl(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, rcNetlist)

	res := callTool(t, h, "circuit_run_continuous", map[string]any{
		"sessionId": sid, "maxStep": 5e-5, "maxSteps": 1e7,
		"nodes": []any{1.0, 2.0}, "elements": []any{"C1"},
	})
	if err, ok := res["__error__"]; ok {
		t.Fatalf("run_continuous 失败: %v", err)
	}
	if c, _ := res["continuous"].(bool); !c {
		t.Fatalf("任务应为连续模式: %v", res)
	}
	jobID := res["jobId"].(string)

	// 暂停
	p := callTool(t, h, "circuit_pause", map[string]any{"sessionId": sid})
	if err, ok := p["__error__"]; ok {
		t.Fatalf("pause 失败: %v", err)
	}
	if s, _ := p["simStatus"].(string); s != "paused" {
		t.Fatalf("pause 后 simStatus 应为 paused: %v", p)
	}

	// 暂停中修改元件参数（此前运行中会被拒绝）
	sp := callTool(t, h, "circuit_set_element_param", map[string]any{
		"sessionId": sid, "instance": "R1", "param": "R", "value": 2000.0,
	})
	if err, ok := sp["__error__"]; ok {
		t.Fatalf("暂停中 set_element_param 应成功: %v", err)
	}
	if v, _ := sp["newValue"].(float64); math.Abs(v-2000) > 1e-9 {
		t.Fatalf("改参返回值错误: %v", sp)
	}

	// 单步 ×3：每步同步完成回到 paused
	st := callTool(t, h, "circuit_step", map[string]any{"sessionId": sid, "count": 3.0})
	if err, ok := st["__error__"]; ok {
		t.Fatalf("step 失败: %v", err)
	}
	if n, _ := st["stepCount"].(float64); n != 3 {
		t.Fatalf("stepCount 应为 3: %v", st)
	}
	if s, _ := st["simStatus"].(string); s != "paused" {
		t.Fatalf("step 后应回到 paused: %v", st)
	}
	t1, _ := st["currentTime"].(float64)
	if t1 <= 0 {
		t.Fatalf("step 后 currentTime 应 > 0: %v", st)
	}

	// 恢复
	r := callTool(t, h, "circuit_resume", map[string]any{"sessionId": sid})
	if err, ok := r["__error__"]; ok {
		t.Fatalf("resume 失败: %v", err)
	}
	if s, _ := r["simStatus"].(string); s != "running" {
		t.Fatalf("resume 后 simStatus 应为 running: %v", r)
	}

	// 运行中修改参数应被拒绝
	msg := callToolErr(t, h, "circuit_set_element_param", map[string]any{
		"sessionId": sid, "instance": "R1", "param": "R", "value": 500.0,
	})
	if !strings.Contains(msg, "暂停") {
		t.Fatalf("运行中改参错误信息不符: %s", msg)
	}

	// 推进 1ms 后自动暂停
	adv := callTool(t, h, "circuit_advance", map[string]any{
		"sessionId": sid, "duration": 0.001,
	})
	if err, ok := adv["__error__"]; ok {
		t.Fatalf("advance 失败: %v", err)
	}
	if s, _ := adv["simStatus"].(string); s != "paused" {
		t.Fatalf("advance 完成应暂停: %v", adv)
	}
	t2, _ := adv["currentTime"].(float64)
	if t2 < t1 || math.Abs(t2-0.001-t1) > 1e-3 {
		t.Fatalf("advance 后时间应前进 ≈1ms（t1=%g t2=%g）", t1, t2)
	}

	// 暂停后读取节点电压
	nv := callTool(t, h, "circuit_get_node_values", map[string]any{"sessionId": sid, "nodes": []any{2.0}})
	if err, ok := nv["__error__"]; ok {
		t.Fatalf("暂停中 get_node_values 失败: %v", err)
	}

	// 结束
	c := callTool(t, h, "circuit_cancel_job", map[string]any{"sessionId": sid, "jobId": jobID})
	if err, ok := c["__error__"]; ok {
		t.Fatalf("cancel 失败: %v", err)
	}
	if s, _ := c["state"].(string); s != "cancelled" {
		t.Fatalf("cancel 后 state 应为 cancelled: %v", c)
	}
}

// TestToolContinuousRelinearize 暂停中改参数后恢复仿真，验证线性元件按新参数重新加盖：
// 分压器 R1=1k/R2=2k 稳态 V2=3.333V；暂停改 R1=2k 后续稳态应变为 2.5V
// （若未重盖，矩阵仍用旧 R1，V2 不变）。
func TestToolContinuousRelinearize(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, dividerNetlist)

	callTool(t, h, "circuit_run_continuous", map[string]any{
		"sessionId": sid, "maxStep": 1e-4, "maxSteps": 1e7,
	})
	// 推进到稳态（纯 DC 瞬时收敛）
	adv := callTool(t, h, "circuit_advance", map[string]any{
		"sessionId": sid, "duration": 0.0005,
	})
	if err, ok := adv["__error__"]; ok {
		t.Fatalf("advance 失败: %v", err)
	}
	nv1 := callTool(t, h, "circuit_get_node_values", map[string]any{"sessionId": sid, "nodes": []any{2.0}})
	v1 := nodeValueOf(t, nv1, "node_2")
	if math.Abs(v1-5.0*2000/3000) > 1e-3 {
		t.Fatalf("初始稳态 V2 应为 %.4f，实际 %v", 5.0*2000/3000, v1)
	}

	// 暂停中改 R1: 1k → 2k
	sp := callTool(t, h, "circuit_set_element_param", map[string]any{
		"sessionId": sid, "instance": "R1", "param": "R", "value": 2000.0,
	})
	if err, ok := sp["__error__"]; ok {
		t.Fatalf("暂停中改参失败: %v", err)
	}

	// 再推进，验证稳态按新 R1 收敛
	adv2 := callTool(t, h, "circuit_advance", map[string]any{
		"sessionId": sid, "duration": 0.0005,
	})
	if err, ok := adv2["__error__"]; ok {
		t.Fatalf("advance2 失败: %v", err)
	}
	nv2 := callTool(t, h, "circuit_get_node_values", map[string]any{"sessionId": sid, "nodes": []any{2.0}})
	v2 := nodeValueOf(t, nv2, "node_2")
	want2 := 5.0 * 2000 / (2000 + 2000) // 2.5V
	if math.Abs(v2-want2) > 1e-3 {
		t.Fatalf("改参后稳态 V2 应为 %.4f，实际 %v（线性元件可能未重新加盖）", want2, v2)
	}

	callTool(t, h, "circuit_cancel_job", map[string]any{"sessionId": sid})
}

// nodeValueOf 从 get_node_values 结果中取指定列名的值。
func nodeValueOf(t *testing.T, res map[string]any, name string) float64 {
	t.Helper()
	vals, ok := res["values"].([]any)
	if !ok {
		t.Fatalf("缺少 values 数组: %v", res)
	}
	for _, vi := range vals {
		m := vi.(map[string]any)
		if m["name"] == name {
			return m["value"].(float64)
		}
	}
	t.Fatalf("values 中找不到 %q: %v", name, res)
	return 0
}

// TestToolControlNonContinuousRejected 控制工具对普通（非连续）任务应拒绝。
func TestToolControlNonContinuousRejected(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, rcNetlist)
	callTool(t, h, "circuit_run_transient", map[string]any{
		"sessionId": sid, "targetTime": 10.0, "maxSteps": 1e8,
	})
	msg := callToolErr(t, h, "circuit_pause", map[string]any{"sessionId": sid})
	if !strings.Contains(msg, "连续模式") {
		t.Fatalf("对非连续任务 pause 应报连续模式错误: %s", msg)
	}
	callTool(t, h, "circuit_cancel_job", map[string]any{"sessionId": sid})
}

// TestToolContinuousJobStatus 连续任务 job_status 应携带 continuous/simStatus 字段。
func TestToolContinuousJobStatus(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, rcNetlist)
	callTool(t, h, "circuit_run_continuous", map[string]any{
		"sessionId": sid, "maxStep": 1e-4, "maxSteps": 1e7,
	})
	waitSimStatus(t, h, sid, "running", 5*time.Second)
	st := callTool(t, h, "circuit_job_status", map[string]any{"sessionId": sid})
	if c, _ := st["continuous"].(bool); !c {
		t.Fatalf("job_status 应标记 continuous: %v", st)
	}
	callTool(t, h, "circuit_cancel_job", map[string]any{"sessionId": sid})
}
