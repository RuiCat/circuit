package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// 集成测试：构建 circuit 二进制（mcpserver 子命令），通过 mcp-go 客户端走完整 stdio 协议流程。

var (
	binOnce     sync.Once
	binPath     string
	binBuildErr error
)

// buildServerBinary 构建 circuit 二进制（含 mcpserver 子命令，仅一次）。
func buildServerBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "mcpserver-bin")
		if err != nil {
			binBuildErr = err
			return
		}
		binPath = filepath.Join(dir, "circuit")
		cmd := exec.Command("go", "build", "-o", binPath, "./cmd")
		cmd.Dir = ".." // 仓库根目录（本文件位于 mcp/ 子目录）
		out, err := cmd.CombinedOutput()
		if err != nil {
			binBuildErr = fmt.Errorf("构建 circuit 失败: %v\n%s", err, out)
		}
	})
	if binBuildErr != nil {
		t.Fatalf("服务器二进制构建失败: %v", binBuildErr)
	}
	return binPath
}

// toolCall 通过协议调用工具，返回解析后的 JSON。
func toolCall(t *testing.T, c *client.Client, name string, args map[string]any) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := c.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: name, Arguments: args},
	})
	if err != nil {
		t.Fatalf("CallTool(%s) 失败: %v", name, err)
	}
	if res.IsError {
		txt := extractText(t, res)
		t.Fatalf("工具 %s 返回错误: %s", name, txt)
	}
	return parseResult(t, res)
}

func extractText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return "<无文本内容>"
}

func parseResult(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(extractText(t, res)), &out); err != nil {
		t.Fatalf("结果解析失败: %v", err)
	}
	return out
}

func TestStdioEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("短模式跳过集成测试")
	}
	bin := buildServerBinary(t)
	c, err := client.NewStdioMCPClient(bin, nil, "mcpserver")
	if err != nil {
		t.Fatalf("启动 stdio 客户端失败: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	initRes, err := c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "integration-test",
				Version: "1.0.0",
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}
	if initRes.ServerInfo.Name != "circuit-simulator" {
		t.Fatalf("服务器名称不符: %v", initRes.ServerInfo.Name)
	}

	// 工具列表：应包含全部 33 个工具
	toolsRes, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools 失败: %v", err)
	}
	if len(toolsRes.Tools) != 33 {
		t.Fatalf("工具数量应为 33，实际 %d", len(toolsRes.Tools))
	}
	names := map[string]bool{}
	for _, tool := range toolsRes.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{
		"circuit_new_session", "circuit_load_netlist", "circuit_validate_netlist",
		"circuit_list_sessions", "circuit_delete_session",
		"circuit_list_component_types", "circuit_component_help",
		"circuit_list_elements", "circuit_list_nodes",
		"circuit_get_element", "circuit_get_node_values",
		"circuit_debug_matrix", "circuit_debug_element", "circuit_debug_node_voltages",
		"circuit_set_element_param", "circuit_set_event",
		"circuit_set_sim_params", "circuit_set_trigger",
		"circuit_run_transient", "circuit_run_continuous",
		"circuit_pause", "circuit_resume", "circuit_step", "circuit_advance",
		"circuit_run_dc",
		"circuit_job_status", "circuit_wait_job", "circuit_cancel_job",
		"circuit_get_results", "circuit_export_plot",
		"circuit_run_ac", "circuit_sweep_ac", "circuit_run_powerflow",
	} {
		if !names[want] {
			t.Fatalf("缺少工具 %s", want)
		}
	}

	// 完整流程：创建会话 → 校验 → 仿真 → 取结果 → 导出 → 删除
	sess := toolCall(t, c, "circuit_new_session", map[string]any{"netlist": rcNetlist})
	sid := sess["sessionId"].(string)

	val := toolCall(t, c, "circuit_validate_netlist", map[string]any{"netlist": dividerNetlist})
	if ok, _ := val["valid"].(bool); !ok {
		t.Fatalf("校验应通过: %v", val)
	}

	run := toolCall(t, c, "circuit_run_transient", map[string]any{
		"sessionId": sid, "targetTime": 0.003, "nodes": []any{2.0}, "wait": true,
	})
	if st := run["state"].(string); st != "done" {
		t.Fatalf("任务未完成: %v", run)
	}
	jobID := run["jobId"].(string)

	results := toolCall(t, c, "circuit_get_results", map[string]any{
		"sessionId": sid, "jobId": jobID,
	})
	series := results["series"].(map[string]any)
	node2, ok := series["node_2"].([]any)
	if !ok || len(node2) == 0 {
		t.Fatalf("缺少 node_2 序列: %v", results["probes"])
	}
	// t=3ms 处 Vc ≈ 4.751V
	last := node2[len(node2)-1].(float64)
	if last < 4.6 || last > 4.9 {
		t.Fatalf("node_2 终值异常: %v", last)
	}

	plot := toolCall(t, c, "circuit_export_plot", map[string]any{"sessionId": sid})
	if html, ok := plot["html"].(string); !ok || !strings.Contains(html, "createElement('canvas')") {
		t.Fatalf("导出 HTML 异常")
	}

	del := toolCall(t, c, "circuit_delete_session", map[string]any{"sessionId": sid})
	if v, _ := del["deleted"].(bool); !v {
		t.Fatalf("删除失败: %v", del)
	}
}

func TestStdioToolErrorPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("短模式跳过集成测试")
	}
	bin := buildServerBinary(t)
	c, err := client.NewStdioMCPClient(bin, nil, "mcpserver")
	if err != nil {
		t.Fatalf("启动 stdio 客户端失败: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "integration-test", Version: "1.0.0"},
		},
	}); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}
	// 调用不存在的会话应返回 isError=true
	res, err := c.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "circuit_list_elements", Arguments: map[string]any{"sessionId": "s999"}},
	})
	if err != nil {
		t.Fatalf("CallTool 失败: %v", err)
	}
	if !res.IsError {
		t.Fatalf("应返回 isError=true: %v", extractText(t, res))
	}
	txt := extractText(t, res)
	if !strings.Contains(txt, "不存在") {
		t.Fatalf("错误信息不符: %s", txt)
	}
}

// TestToolRunAC AC 相量分析工具:RC 低通 -3dB 点验证
func TestToolRunAC(t *testing.T) {
	if testing.Short() {
		t.Skip("短模式跳过集成测试")
	}
	bin := buildServerBinary(t)
	c, err := client.NewStdioMCPClient(bin, nil, "mcpserver")
	if err != nil {
		t.Fatalf("启动 stdio 客户端失败: %v", err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ClientInfo:      mcp.Implementation{Name: "integration-test", Version: "1.0.0"},
	}}); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}
	acNetlist := "v1 [1,-1] [1,0,0,0,1]\nr1 [1,2] [1000]\nc1 [2,-1] [100e-9]"
	sess := toolCall(t, c, "circuit_new_session", map[string]any{"netlist": acNetlist})
	sid := sess["sessionId"].(string)

	// f0 = 1/(2π·1000·100n) ≈ 1591.55Hz,|H|=0.707,-45°
	f0 := 1 / (2 * math.Pi * 1000 * 100e-9)
	res := toolCall(t, c, "circuit_run_ac", map[string]any{
		"sessionId": sid, "frequency": f0, "nodes": []any{2.0},
	})
	nvs := res["nodeVoltages"].([]any)
	if len(nvs) != 1 {
		t.Fatalf("nodeVoltages 应只含节点 2: %v", nvs)
	}
	nv := nvs[0].(map[string]any)
	mag := nv["mag"].(float64)
	phase := nv["phaseDeg"].(float64)
	if math.Abs(mag-1/math.Sqrt2) > 1e-3 {
		t.Fatalf("节点2幅值: 期望 0.7071, 实际 %g", mag)
	}
	if math.Abs(phase+45) > 0.1 {
		t.Fatalf("节点2相位: 期望 -45°, 实际 %g", phase)
	}

	// 频率扫描
	sweep := toolCall(t, c, "circuit_sweep_ac", map[string]any{
		"sessionId": sid, "fMin": f0 / 10, "fMax": f0 * 10, "points": 50, "logScale": true,
	})
	freqs := sweep["frequencies"].([]any)
	if len(freqs) != 50 {
		t.Fatalf("扫描点数应 50, 实际 %d", len(freqs))
	}
	curves := sweep["curves"].([]any)
	if len(curves) != 2 {
		t.Fatalf("未过滤时应返回 2 条曲线(节点1,2), 实际 %d", len(curves))
	}
	// 过滤节点后只返回 node 2
	sweep2 := toolCall(t, c, "circuit_sweep_ac", map[string]any{
		"sessionId": sid, "fMin": f0 / 10, "fMax": f0 * 10, "points": 50, "logScale": true, "nodes": []any{2.0},
	})
	curves2 := sweep2["curves"].([]any)
	if len(curves2) != 1 {
		t.Fatalf("过滤后曲线数应 1, 实际 %d", len(curves2))
	}
	if n, _ := curves2[0].(map[string]any)["node"].(float64); n != 2 {
		t.Fatalf("曲线节点应为 2, 实际 %v", n)
	}
}

// TestToolRunPowerFlow 潮流计算工具:2 母线解析解验证
func TestToolRunPowerFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("短模式跳过集成测试")
	}
	bin := buildServerBinary(t)
	c, err := client.NewStdioMCPClient(bin, nil, "mcpserver")
	if err != nil {
		t.Fatalf("启动 stdio 客户端失败: %v", err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ClientInfo:      mcp.Implementation{Name: "integration-test", Version: "1.0.0"},
	}}); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}
	res := toolCall(t, c, "circuit_run_powerflow", map[string]any{
		"buses": []any{
			map[string]any{"id": 1.0, "type": "slack", "v": 1.0},
			map[string]any{"id": 2.0, "type": "pq", "p": 0.5, "q": 0.2},
		},
		"branches": []any{
			map[string]any{"from": 1.0, "to": 2.0, "r": 0.0, "x": 0.1},
		},
	})
	if conv, _ := res["converged"].(bool); !conv {
		t.Fatalf("潮流未收敛: %v", res["mismatchLog"])
	}
	bvs := res["busVoltages"].([]any)
	if len(bvs) != 2 {
		t.Fatalf("母线电压应 2 条: %v", bvs)
	}
	var v2 map[string]any
	for _, bv := range bvs {
		if bm := bv.(map[string]any); int(bm["bus"].(float64)) == 2 {
			v2 = bm
		}
	}
	if v2 == nil {
		t.Fatalf("缺少母线 2: %v", bvs)
	}
	if math.Abs(v2["mag"].(float64)-1.018432) > 1e-4 {
		t.Fatalf("|V2|: 期望 1.018432, 实际 %g", v2["mag"].(float64))
	}
	if math.Abs(v2["phaseDeg"].(float64)-2.814) > 0.1 {
		t.Fatalf("θ2: 期望 2.814°, 实际 %g", v2["phaseDeg"].(float64))
	}
}
