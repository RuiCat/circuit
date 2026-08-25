package mcp

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

// callTool 通过 JSON 往返模拟 MCP 协议路径调用处理器（数字统一为 float64）。
func callTool(t *testing.T, h *Handler, name string, args map[string]any) map[string]any {
	t.Helper()
	// 需要按工具名分发
	var fn func(map[string]any) (any, error)
	switch name {
	case "circuit_new_session":
		fn = h.handleNewSession
	case "circuit_load_netlist":
		fn = h.handleLoadNetlist
	case "circuit_validate_netlist":
		fn = h.handleValidateNetlist
	case "circuit_list_sessions":
		fn = h.handleListSessions
	case "circuit_delete_session":
		fn = h.handleDeleteSession
	case "circuit_list_component_types":
		fn = h.handleListComponentTypes
	case "circuit_component_help":
		fn = h.handleComponentHelp
	case "circuit_list_elements":
		fn = h.handleListElements
	case "circuit_list_nodes":
		fn = h.handleListNodes
	case "circuit_get_element":
		fn = h.handleGetElement
	case "circuit_get_node_values":
		fn = h.handleGetNodeValues
	case "circuit_set_element_param":
		fn = h.handleSetElementParam
	case "circuit_set_event":
		fn = h.handleSetEvent
	case "circuit_set_sim_params":
		fn = h.handleSetSimParams
	case "circuit_set_engine_config":
		fn = h.handleSetEngineConfig
	case "circuit_set_trigger":
		fn = h.handleSetTrigger
	case "circuit_run_transient":
		fn = h.handleRunTransient
	case "circuit_run_continuous":
		fn = h.handleRunContinuous
	case "circuit_pause":
		fn = h.handlePause
	case "circuit_resume":
		fn = h.handleResume
	case "circuit_step":
		fn = h.handleStep
	case "circuit_advance":
		fn = h.handleAdvance
	case "circuit_run_dc":
		fn = h.handleRunDC
	case "circuit_job_status":
		fn = h.handleJobStatus
	case "circuit_wait_job":
		fn = h.handleWaitJob
	case "circuit_cancel_job":
		fn = h.handleCancelJob
	case "circuit_get_results":
		fn = h.handleGetResults
	case "circuit_export_plot":
		fn = h.handleExportPlot
	default:
		t.Fatalf("未知工具 %s", name)
	}
	res, err := fn(args)
	if err != nil {
		return map[string]any{"__error__": err.Error()}
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	return out
}

// callToolErr 断言工具调用报错并返回错误文本。
func callToolErr(t *testing.T, h *Handler, name string, args map[string]any) string {
	t.Helper()
	res := callTool(t, h, name, args)
	if _, ok := res["__error__"]; !ok {
		t.Fatalf("工具 %s 应报错，实际返回 %v", name, res)
	}
	return res["__error__"].(string)
}

// newSession 创建会话并返回 ID。
func newSession(t *testing.T, h *Handler, netlist string) string {
	t.Helper()
	res := callTool(t, h, "circuit_new_session", map[string]any{"netlist": netlist})
	if err, ok := res["__error__"]; ok {
		t.Fatalf("创建会话失败: %v", err)
	}
	return res["sessionId"].(string)
}

// runAndWait 运行瞬态仿真并等待完成。
func runAndWait(t *testing.T, h *Handler, sid string, args map[string]any) map[string]any {
	t.Helper()
	base := map[string]any{"sessionId": sid}
	for k, v := range args {
		base[k] = v
	}
	res := callTool(t, h, "circuit_run_transient", base)
	if err, ok := res["__error__"]; ok {
		t.Fatalf("run_transient 失败: %v", err)
	}
	jobID := res["jobId"].(string)
	status := callTool(t, h, "circuit_wait_job", map[string]any{
		"sessionId": sid, "jobId": jobID, "timeoutSeconds": 60,
	})
	if err, ok := status["__error__"]; ok {
		t.Fatalf("wait_job 失败: %v", err)
	}
	if st := status["state"].(string); st != "done" {
		t.Fatalf("任务状态 %s，错误: %v", st, status["error"])
	}
	return status
}

func TestToolValidateNetlist(t *testing.T) {
	h, _ := newTestHandler()
	ok := callTool(t, h, "circuit_validate_netlist", map[string]any{"netlist": rcNetlist})
	if v, _ := ok["valid"].(bool); !v {
		t.Fatalf("合法网表应通过: %v", ok)
	}
	bad := callTool(t, h, "circuit_validate_netlist", map[string]any{"netlist": "R1 [1,0] [1000"})
	if v, _ := bad["valid"].(bool); v {
		t.Fatalf("非法网表应失败: %v", bad)
	}
	if _, ok := bad["error"]; !ok {
		t.Fatal("应返回错误信息")
	}
}

func TestToolComponentCatalog(t *testing.T) {
	h, _ := newTestHandler()
	res := callTool(t, h, "circuit_list_component_types", nil)
	types, ok := res["types"].([]any)
	if !ok || len(types) < 10 {
		t.Fatalf("元件目录应 >= 10 种，实际 %d", len(types))
	}
	names := map[string]bool{}
	for _, ti := range types {
		m := ti.(map[string]any)
		names[strings.ToLower(m["name"].(string))] = true
	}
	for _, want := range []string{"r", "c", "v", "q", "d", "b"} {
		if !names[want] {
			t.Fatalf("目录缺少元件类型 %q", want)
		}
	}
	// 域过滤
	electrical := callTool(t, h, "circuit_list_component_types", map[string]any{"domain": "electrical"})
	for _, ti := range electrical["types"].([]any) {
		if m := ti.(map[string]any); m["domain"] != "electrical" {
			t.Fatalf("域过滤失效: %v", m)
		}
	}
	// 类型帮助
	help := callTool(t, h, "circuit_component_help", map[string]any{"type": "r"})
	if help["name"] != "r" {
		t.Fatalf("help 返回错误类型: %v", help)
	}
	params := help["params"].([]any)
	if len(params) < 1 {
		t.Fatal("电阻应有参数")
	}
	if p0 := params[0].(map[string]any); p0["name"] != "R" || p0["type"] != "number" {
		t.Fatalf("电阻参数定义错误: %v", p0)
	}
	errMsg := callToolErr(t, h, "circuit_component_help", map[string]any{"type": "zzz"})
	if !strings.Contains(errMsg, "不存在") {
		t.Fatalf("错误信息不符: %s", errMsg)
	}
}

func TestToolInspectSession(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, rcNetlist)

	// 元件清单
	res := callTool(t, h, "circuit_list_elements", map[string]any{"sessionId": sid})
	if n := res["count"].(float64); n != 3 {
		t.Fatalf("元件数应为 3，实际 %v", n)
	}
	// 元件详情
	elem := callTool(t, h, "circuit_get_element", map[string]any{"sessionId": sid, "instance": "R1"})
	if elem["type"] != "r" {
		t.Fatalf("R1 类型应为 r: %v", elem)
	}
	params := elem["params"].([]any)
	foundR := false
	for _, p := range params {
		m := p.(map[string]any)
		if m["name"] == "R" {
			foundR = true
			if m["value"].(float64) != 1000 {
				t.Fatalf("R1 阻值应为 1000: %v", m)
			}
		}
	}
	if !foundR {
		t.Fatal("R1 缺少参数 R")
	}
	// 节点清单（GND=-1 不入列；节点 1、2）
	nodes := callTool(t, h, "circuit_list_nodes", map[string]any{"sessionId": sid})
	if n := nodes["count"].(float64); n != 2 {
		t.Fatalf("节点数应为 2（1 和 2），实际 %v", n)
	}
	// 节点电压（仿真前无解，读数为 0）
	vals := callTool(t, h, "circuit_get_node_values", map[string]any{
		"sessionId": sid, "nodes": []any{1.0},
	})
	values := vals["values"].([]any)
	if len(values) != 1 {
		t.Fatalf("应返回 1 个节点值: %v", vals)
	}
	if v := values[0].(map[string]any)["value"].(float64); v != 0 {
		t.Fatalf("仿真前节点电压应为 0，实际 %v", v)
	}
	// 不存在的元件/节点报错
	if msg := callToolErr(t, h, "circuit_get_element", map[string]any{"sessionId": sid, "instance": "R99"}); !strings.Contains(msg, "不在网表中") {
		t.Fatalf("错误信息不符: %s", msg)
	}
	if msg := callToolErr(t, h, "circuit_get_node_values", map[string]any{"sessionId": sid, "nodes": []any{99.0}}); !strings.Contains(msg, "不在网表中") {
		t.Fatalf("错误信息不符: %s", msg)
	}
}

func TestToolSetElementParam(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, dividerNetlist)

	// 修改 R2 阻值 2000 → 4000
	res := callTool(t, h, "circuit_set_element_param", map[string]any{
		"sessionId": sid, "instance": "R2", "param": "R", "value": 4000.0,
	})
	if res["oldValue"].(float64) != 2000 || res["newValue"].(float64) != 4000 {
		t.Fatalf("参数修改结果错误: %v", res)
	}
	// 类型不匹配报错
	if msg := callToolErr(t, h, "circuit_set_element_param", map[string]any{
		"sessionId": sid, "instance": "R2", "param": "R", "value": "abc",
	}); !strings.Contains(msg, "类型") {
		t.Fatalf("类型不匹配应报错: %s", msg)
	}
	// 未知参数报错
	if msg := callToolErr(t, h, "circuit_set_element_param", map[string]any{
		"sessionId": sid, "instance": "R2", "param": "NOPE", "value": 1.0,
	}); !strings.Contains(msg, "无参数") {
		t.Fatalf("未知参数应报错: %s", msg)
	}
	// 索引越界报错
	if msg := callToolErr(t, h, "circuit_set_element_param", map[string]any{
		"sessionId": sid, "instance": "R2", "param": "9", "value": 1.0,
	}); !strings.Contains(msg, "越界") {
		t.Fatalf("越界应报错: %s", msg)
	}
}

func TestToolDCVoltageDivider(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, dividerNetlist)
	res := callTool(t, h, "circuit_run_dc", map[string]any{
		"sessionId": sid, "wait": true,
	})
	if err, ok := res["__error__"]; ok {
		t.Fatalf("run_dc 失败: %v", err)
	}
	// 分压器: 5V * 2k/(1k+2k) = 3.3333V
	results := callTool(t, h, "circuit_get_results", map[string]any{"sessionId": sid})
	finalV := results["finalV"].(map[string]any)
	v2 := finalV["node_2"].(float64)
	if math.Abs(v2-5.0/3.0*2.0) > 1e-4 {
		t.Fatalf("节点 2 应为 3.333V，实际 %v", v2)
	}
	v1 := finalV["node_1"].(float64)
	if math.Abs(v1-5) > 1e-4 {
		t.Fatalf("节点 1 应为 5V，实际 %v", v1)
	}
}

func TestToolTransientRC(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, rcNetlist)
	// 仿真 3ms，探针节点 1,2 与 C1 电流
	runAndWait(t, h, sid, map[string]any{
		"targetTime": 0.003,
		"nodes":      []any{1.0, 2.0},
		"elements":   []any{"C1"},
		"maxSteps":   100000.0,
	})
	res := callTool(t, h, "circuit_get_results", map[string]any{"sessionId": sid})
	series := res["series"].(map[string]any)
	times := res["time"].([]any)
	node2 := series["node_2"].([]any)
	if len(times) < 10 {
		t.Fatalf("采样点过少: %d", len(times))
	}
	// 在 t=1ms 处: Vc = 5*(1-e^-1) ≈ 3.1606
	// 找到最接近 1ms 的采样点
	best := 0
	bestDt := math.Inf(1)
	for i, tv := range times {
		dt := math.Abs(tv.(float64) - 0.001)
		if dt < bestDt {
			bestDt = dt
			best = i
		}
	}
	got := node2[best].(float64)
	want := 5 * (1 - math.Exp(-1))
	if math.Abs(got-want) > 0.05 {
		t.Fatalf("t=1ms 处 Vc 应为 %.4f，实际 %.4f（采样点 %d）", want, got, best)
	}
	// 终值 ≈ 5*(1-e^-3) = 4.751V（3 个时间常数）
	finalV := res["finalV"].(map[string]any)
	wantFinal := 5 * (1 - math.Exp(-3))
	if math.Abs(finalV["node_2"].(float64)-wantFinal) > 0.02 {
		t.Fatalf("t=3ms 处 Vc 应为 %.4f，实际 %v", wantFinal, finalV["node_2"])
	}
	// 电流探针存在
	if _, ok := series["I(C1)"]; !ok {
		t.Fatalf("缺少 I(C1) 探针: %v", res["probes"])
	}
	// CSV 输出
	csvRes := callTool(t, h, "circuit_get_results", map[string]any{"sessionId": sid, "format": "csv"})
	csv := csvRes["csv"].(string)
	if !strings.HasPrefix(csv, "time") || !strings.Contains(csv, "node_2") {
		t.Fatalf("CSV 内容错误: %s", csv[:min(200, len(csv))])
	}
}

func TestToolDecimation(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, rcNetlist)
	runAndWait(t, h, sid, map[string]any{"targetTime": 0.001, "maxSteps": 100000.0})
	res := callTool(t, h, "circuit_get_results", map[string]any{"sessionId": sid, "maxPoints": 50.0})
	times := res["time"].([]any)
	if len(times) > 50 {
		t.Fatalf("抽稀失效: %d 点", len(times))
	}
	// 探针过滤
	res2 := callTool(t, h, "circuit_get_results", map[string]any{
		"sessionId": sid, "probes": []any{"node_1"},
	})
	series := res2["series"].(map[string]any)
	if len(series) != 1 {
		t.Fatalf("探针过滤失效: %v", res2["probes"])
	}
}

func TestToolSetEventSwitch(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, eventNetlist)

	// 事件未触发时开关断开，R1 无电流
	runAndWait(t, h, sid, map[string]any{"targetTime": 0.001, "maxSteps": 10000.0})
	res := callTool(t, h, "circuit_get_results", map[string]any{"sessionId": sid})
	finalI := res["finalI"].(map[string]any)
	if i, ok := finalI["I(R1)"].(float64); ok && math.Abs(i) > 1e-3 {
		t.Fatalf("开关断开时 I(R1) 应≈0，实际 %v", i)
	}
	// 启动长任务，任务运行中注入事件 SB1=1（引擎 MarkReset 会清空事件，
	// 因此事件必须在仿真运行期间设置；PushEvents 每步消费一次并锁存到 NodeValue）。
	// 限制 maxStep 防止纯 DC 电路自适应步长快速跑完。
	callTool(t, h, "circuit_run_transient", map[string]any{
		"sessionId": sid, "targetTime": 0.05, "maxStep": 1e-6, "maxSteps": 10000000.0,
	})
	// 等待任务推进到第 5 步（确保 MarkReset 已完成且任务仍在运行）
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := callTool(t, h, "circuit_job_status", map[string]any{"sessionId": sid})
		if steps, ok := st["steps"].(float64); ok && steps >= 5 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("任务未能在 5 秒内推进")
		}
		time.Sleep(2 * time.Millisecond)
	}
	ev := callTool(t, h, "circuit_set_event", map[string]any{
		"sessionId": sid, "event": "SB1", "value": 1.0,
	})
	if err, ok := ev["__error__"]; ok {
		t.Fatalf("set_event 失败: %v", err)
	}
	status := callTool(t, h, "circuit_wait_job", map[string]any{
		"sessionId": sid, "timeoutSeconds": 60,
	})
	if st := status["state"].(string); st != "done" && st != "cancelled" {
		t.Fatalf("任务状态 %s: %v", st, status["error"])
	}
	res2 := callTool(t, h, "circuit_get_results", map[string]any{"sessionId": sid})
	finalI2 := res2["finalI"].(map[string]any)
	i, ok := finalI2["I(R1)"].(float64)
	if !ok {
		t.Fatalf("缺少 I(R1) 终值: %v", res2["finalI"])
	}
	if math.Abs(i-5e-3) > 1e-3 {
		t.Fatalf("开关闭合后 I(R1) 应≈5mA，实际 %v", i)
	}
}

func TestToolRunTransientConflict(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, rcNetlist)
	// 长仿真（不等待）
	callTool(t, h, "circuit_run_transient", map[string]any{
		"sessionId": sid, "targetTime": 10.0, "maxSteps": 100000000.0,
	})
	// 运行中再次启动应报错
	msg := callToolErr(t, h, "circuit_run_transient", map[string]any{
		"sessionId": sid, "targetTime": 0.001,
	})
	if !strings.Contains(msg, "运行中") {
		t.Fatalf("运行中应拒绝新任务: %s", msg)
	}
	// 取消任务
	callTool(t, h, "circuit_cancel_job", map[string]any{"sessionId": sid})
}

func TestToolExportPlot(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, rcNetlist)
	runAndWait(t, h, sid, map[string]any{"targetTime": 0.001})
	res := callTool(t, h, "circuit_export_plot", map[string]any{
		"sessionId": sid, "title": "RC测试",
	})
	html := res["html"].(string)
	for _, marker := range []string{"createElement('canvas')", "电路仿真结果", "RC测试", "node_2"} {
		if !strings.Contains(html, marker) {
			t.Fatalf("HTML 缺少标记 %q", marker)
		}
	}
}

func TestToolSimParamsAndTrigger(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, rcNetlist)
	res := callTool(t, h, "circuit_set_sim_params", map[string]any{
		"sessionId": sid, "maxStep": 1e-4, "maxIter": 100.0,
	})
	cfg := res["simCfg"].(map[string]any)
	if cfg["maxStep"].(float64) != 1e-4 || cfg["maxIter"].(float64) != 100 {
		t.Fatalf("仿真参数未生效: %v", cfg)
	}
	// 非法参数
	if msg := callToolErr(t, h, "circuit_set_sim_params", map[string]any{
		"sessionId": sid, "initialStep": -1.0,
	}); !strings.Contains(msg, "> 0") {
		t.Fatalf("非法参数应报错: %s", msg)
	}
	// 触发点
	tr := callTool(t, h, "circuit_set_trigger", map[string]any{
		"sessionId": sid, "triggers": []any{0.0005, 0.001},
	})
	if tr["count"].(float64) != 2 {
		t.Fatalf("触发点设置失败: %v", tr)
	}
}

// TestToolSetEngineConfig 验证 circuit_set_engine_config：
// 设置 GMCONT/LUSPARSE 等引擎配置（EngineCfg），下次仿真生效。
func TestToolSetEngineConfig(t *testing.T) {
	h, _ := newTestHandler()
	sid := newSession(t, h, rcNetlist)

	res := callTool(t, h, "circuit_set_engine_config", map[string]any{
		"sessionId": sid, "gminCont": true, "sparseLU": true,
		"gminFinal": 1e-5, "globalGmin": 1e-3,
	})
	cfg := res["engineConfig"].(map[string]any)
	if cfg["gminCont"] != true || cfg["sparseLU"] != true {
		t.Fatalf("引擎配置未生效: %v", cfg)
	}
	if cfg["gminFinal"].(float64) != 1e-5 || cfg["globalGmin"].(float64) != 1e-3 {
		t.Fatalf("数值配置未生效: %v", cfg)
	}
	if up, ok := res["updated"].([]any); !ok || len(up) != 4 {
		t.Fatalf("updated 应为 4 项: %v", res["updated"])
	}

	// 部分更新：只改一个布尔，其余保持
	res2 := callTool(t, h, "circuit_set_engine_config", map[string]any{
		"sessionId": sid, "gminDbg": true,
	})
	cfg2 := res2["engineConfig"].(map[string]any)
	if cfg2["gminDbg"] != true || cfg2["gminCont"] != true {
		t.Fatalf("部分更新应保留旧值: %v", cfg2)
	}
	if up, ok := res2["updated"].([]any); !ok || len(up) != 1 {
		t.Fatalf("updated 应为 1 项: %v", res2["updated"])
	}

	// 非法参数
	if msg := callToolErr(t, h, "circuit_set_engine_config", map[string]any{
		"sessionId": sid, "globalGmin": -1.0,
	}); !strings.Contains(msg, "不能为负") {
		t.Fatalf("非法参数应报错: %s", msg)
	}
}

func TestToolListAndDeleteSessions(t *testing.T) {
	h, _ := newTestHandler()
	sid1 := newSession(t, h, rcNetlist)
	newSession(t, h, dividerNetlist)
	res := callTool(t, h, "circuit_list_sessions", nil)
	if n := res["sessions"].([]any); len(n) != 2 {
		t.Fatalf("应有 2 个会话，实际 %d", len(n))
	}
	callTool(t, h, "circuit_delete_session", map[string]any{"sessionId": sid1})
	res2 := callTool(t, h, "circuit_list_sessions", nil)
	if n := res2["sessions"].([]any); len(n) != 1 {
		t.Fatalf("删除后应有 1 个会话，实际 %d", len(n))
	}
	if msg := callToolErr(t, h, "circuit_delete_session", map[string]any{"sessionId": sid1}); !strings.Contains(msg, "不存在") {
		t.Fatalf("重复删除应报错: %s", msg)
	}
}

func TestConcurrentSessions(t *testing.T) {
	h, _ := newTestHandler()
	sidA := newSession(t, h, dividerNetlist) // 期望 3.333V
	sidB := newSession(t, h, rcNetlist)      // 期望 5V 稳态
	// 两个会话并行启动仿真（不在 goroutine 中断言）
	ra := callTool(t, h, "circuit_run_transient", map[string]any{
		"sessionId": sidA, "targetTime": 0.001,
	})
	rb := callTool(t, h, "circuit_run_transient", map[string]any{
		"sessionId": sidB, "targetTime": 0.003,
	})
	if err, ok := ra["__error__"]; ok {
		t.Fatalf("会话 A 启动失败: %v", err)
	}
	if err, ok := rb["__error__"]; ok {
		t.Fatalf("会话 B 启动失败: %v", err)
	}
	// 串行等待（两个任务已并发执行）
	for _, sid := range []string{sidA, sidB} {
		status := callTool(t, h, "circuit_wait_job", map[string]any{
			"sessionId": sid, "timeoutSeconds": 60,
		})
		if st := status["state"].(string); st != "done" {
			t.Fatalf("会话 %s 任务状态 %s: %v", sid, st, status["error"])
		}
	}
	resA := callTool(t, h, "circuit_get_results", map[string]any{"sessionId": sidA})
	finalA := resA["finalV"].(map[string]any)
	if math.Abs(finalA["node_2"].(float64)-5.0/3.0*2.0) > 1e-3 {
		t.Fatalf("会话 A 分压结果错误: %v", finalA["node_2"])
	}
	resB := callTool(t, h, "circuit_get_results", map[string]any{"sessionId": sidB})
	finalB := resB["finalV"].(map[string]any)
	wantFinal := 5 * (1 - math.Exp(-3))
	if math.Abs(finalB["node_2"].(float64)-wantFinal) > 0.02 {
		t.Fatalf("会话 B 稳态结果错误: %v（期望 %.4f）", finalB["node_2"], wantFinal)
	}
}

func TestHierarchicalNodePath(t *testing.T) {
	netlist := `# 子电路分压器
.subckt div in out
  R1 [in,out] [1000]
  R2 [out,-1] [1000]
.ends div
V1 [1,-1] [0,0,0,0,5]
X1 [1,2] div
`
	h, _ := newTestHandler()
	sid := newSession(t, h, netlist)
	res := callTool(t, h, "circuit_get_node_values", map[string]any{
		"sessionId": sid, "paths": []any{"X1.out"},
	})
	values := res["values"].([]any)
	if len(values) != 1 {
		t.Fatalf("层级路径查询失败: %v", res)
	}
	v := values[0].(map[string]any)
	if v["path"] != "X1.out" {
		t.Fatalf("路径字段错误: %v", v)
	}
}
