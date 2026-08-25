package mcp

import (
	"context"

	_ "circuit/element/register" // 注册全部元件类型（本包自包含，任何入口均可工作）

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Version 服务器版本。
const Version = "1.0.0"

// NewMCPServer 创建电路仿真 MCP 服务器，注册全部工具。
func NewMCPServer(store *SessionStore) *server.MCPServer {
	h := NewHandler(store)
	srv := server.NewMCPServer(
		"circuit-simulator",
		Version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)
	register := func(tool mcp.Tool, fn func(map[string]any) (any, error)) {
		srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args, _ := req.Params.Arguments.(map[string]any)
			if args == nil {
				args = map[string]any{}
			}
			res, err := fn(args)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultJSON(res)
		})
	}

	// ---------- A 组 · 会话与网表管理 ----------
	register(
		mcp.NewTool("circuit_new_session",
			mcp.WithDescription("创建电路仿真会话。可附带初始网表文本（SPICE 风格），返回会话 ID 与电路统计。"),
			mcp.WithString("netlist", mcp.Description("网表文本（可选）。例如: R1 [1,0] [1000]")),
			mcp.WithString("name", mcp.Description("会话名称（可选）")),
		),
		h.handleNewSession,
	)
	register(
		mcp.NewTool("circuit_load_netlist",
			mcp.WithDescription("加载或替换会话的网表。失败时保留旧网表。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("netlist", mcp.Description("网表文本"), mcp.Required()),
		),
		h.handleLoadNetlist,
	)
	register(
		mcp.NewTool("circuit_validate_netlist",
			mcp.WithDescription("校验网表语法与语义，不创建会话。返回错误列表或电路统计。"),
			mcp.WithString("netlist", mcp.Description("网表文本"), mcp.Required()),
		),
		h.handleValidateNetlist,
	)
	register(
		mcp.NewTool("circuit_list_sessions",
			mcp.WithDescription("列出全部仿真会话及其元信息（元件/节点数、最近任务状态）。"),
		),
		h.handleListSessions,
	)
	register(
		mcp.NewTool("circuit_delete_session",
			mcp.WithDescription("删除会话并释放资源；若有运行中任务会先取消。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
		),
		h.handleDeleteSession,
	)

	// ---------- B 组 · 元件目录与电路检视 ----------
	register(
		mcp.NewTool("circuit_list_component_types",
			mcp.WithDescription("列出引擎支持的全部元件类型目录：名称、引脚、参数定义、特性位。"),
			mcp.WithString("domain", mcp.Description("按域过滤: electrical / pneumatic / hydraulic / logic（可选）")),
		),
		h.handleListComponentTypes,
	)
	register(
		mcp.NewTool("circuit_component_help",
			mcp.WithDescription("查看单个元件类型的完整说明（引脚、参数名与默认值、电压源、内部节点）。"),
			mcp.WithString("type", mcp.Description("元件类型名，如 r / c / v / q / d / op / x"), mcp.Required()),
		),
		h.handleComponentHelp,
	)
	register(
		mcp.NewTool("circuit_list_elements",
			mcp.WithDescription("列出会话内全部元件：实例名、类型、引脚连接、参数值、当前电流。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("type", mcp.Description("按类型过滤（可选）")),
			mcp.WithString("instance", mcp.Description("按实例名过滤（可选）")),
		),
		h.handleListElements,
	)
	register(
		mcp.NewTool("circuit_list_nodes",
			mcp.WithDescription("列出会话内全部节点：原始 ID、紧凑索引、域、单位、连接元件、当前电压。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
		),
		h.handleListNodes,
	)
	register(
		mcp.NewTool("circuit_get_element",
			mcp.WithDescription("查看单个元件的完整状态：全部参数、引脚、电压源、内部节点、层级父子。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("instance", mcp.Description("元件实例名，如 R1 / X1.R1"), mcp.Required()),
		),
		h.handleGetElement,
	)
	register(
		mcp.NewTool("circuit_get_node_values",
			mcp.WithDescription("读取节点当前电压/压力值。支持原始节点 ID 与层级路径（如 X1.out）。不指定时返回全部节点。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithArray("nodes", mcp.Description("原始节点 ID 列表（可选）"), mcp.Items(map[string]any{"type": "integer"})),
			mcp.WithArray("paths", mcp.Description("层级路径列表，如 [\"X1.out\"]（可选）"), mcp.Items(map[string]any{"type": "string"})),
		),
		h.handleGetNodeValues,
	)
	register(
		mcp.NewTool("circuit_debug_matrix",
			mcp.WithDescription("查看 MNA 矩阵行（A 系数 / Z 右侧 / X 解）：节点行与电压源约束行。用于排查运行中数值异常。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithArray("rows", mcp.Description("行号数组（可选；0..NodeNum-1=节点行，≥NodeNum=电压源约束行；默认全部节点行）"), mcp.Items(map[string]any{"type": "integer"})),
			mcp.WithBoolean("all", mcp.Description("true=包含全部电压源约束行（可选）")),
		),
		h.handleDebugMatrix,
	)
	register(
		mcp.NewTool("circuit_debug_element",
			mcp.WithDescription("查看元件全部内部状态：NodeValue 原值（含 DFF 状态/事件值/回滚备份）、引脚、电压源目标值。与 circuit_get_element 相比不裁剪参数。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("instance", mcp.Description("元件实例名，如 X1.X3.X1.DFF1"), mcp.Required()),
		),
		h.handleDebugElement,
	)
	register(
		mcp.NewTool("circuit_debug_node_voltages",
			mcp.WithDescription("按紧凑索引读全部节点电压（与 get_node_values 的原始 ID 互补），附原始 ID 与层级路径标签。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithArray("compact", mcp.Description("紧凑索引列表（可选）"), mcp.Items(map[string]any{"type": "integer"})),
			mcp.WithBoolean("all", mcp.Description("true=全部节点（可选）")),
		),
		h.handleDebugNodeVoltages,
	)

	// ---------- C 组 · 电路修改 ----------
	register(
		mcp.NewTool("circuit_set_element_param",
			mcp.WithDescription("修改元件参数（按参数名或索引）。运行中任务不可调用。返回旧值→新值。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("instance", mcp.Description("元件实例名，如 R1"), mcp.Required()),
			mcp.WithString("param", mcp.Description("参数名（如 R）或索引（如 0）"), mcp.Required()),
			mcp.WithAny("value", mcp.Description("新值，类型须与参数一致（number/integer/boolean/string）"), mcp.Required()),
		),
		h.handleSetElementParam,
	)
	register(
		mcp.NewTool("circuit_set_event",
			mcp.WithDescription("设置事件值，驱动事件类元件（EventSwitch/MomentarySwitch 等）。运行中任务也可调用。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("event", mcp.Description("事件名（如 SB1）"), mcp.Required()),
			mcp.WithAny("value", mcp.Description("事件值：number/boolean/string"), mcp.Required()),
		),
		h.handleSetEvent,
	)
	register(
		mcp.NewTool("circuit_set_sim_params",
			mcp.WithDescription("设置会话的默认仿真参数（容差/步长限制/迭代上限/最大步数/并行数）。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithNumber("initialStep", mcp.Description("初始步长（秒）")),
			mcp.WithNumber("minStep", mcp.Description("最小步长（秒）")),
			mcp.WithNumber("maxStep", mcp.Description("最大步长（秒）")),
			mcp.WithNumber("absTol", mcp.Description("绝对容差")),
			mcp.WithNumber("relTol", mcp.Description("相对容差")),
			mcp.WithInteger("maxIter", mcp.Description("最大非线性迭代次数")),
			mcp.WithInteger("maxSteps", mcp.Description("最大时间步数")),
			mcp.WithInteger("parallel", mcp.Description("并行 worker 数（0=串行）")),
		),
		h.handleSetSimParams,
	)
	register(
		mcp.NewTool("circuit_set_trigger",
			mcp.WithDescription("设置时间触发点列表（秒）。步长会自动截断对齐触发点。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithArray("triggers", mcp.Description("触发时间数组，如 [0.001, 0.002]"), mcp.Required(),
				mcp.Items(map[string]any{"type": "number"})),
		),
		h.handleSetTrigger,
	)

	// ---------- D 组 · 仿真执行 ----------
	register(
		mcp.NewTool("circuit_run_transient",
			mcp.WithDescription("启动瞬态仿真（后台异步执行，立即返回 jobId）。探针节点/元件可过滤输出列。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithNumber("targetTime", mcp.Description("仿真目标时间（秒），必须 > 0"), mcp.Required()),
			mcp.WithNumber("initialStep", mcp.Description("初始步长（秒，可选）")),
			mcp.WithNumber("minStep", mcp.Description("最小步长（秒，可选）")),
			mcp.WithNumber("maxStep", mcp.Description("最大步长（秒，可选）")),
			mcp.WithNumber("absTol", mcp.Description("绝对容差（可选）")),
			mcp.WithNumber("relTol", mcp.Description("相对容差（可选）")),
			mcp.WithInteger("maxIter", mcp.Description("最大非线性迭代次数（可选）")),
			mcp.WithInteger("maxSteps", mcp.Description("最大时间步数（可选）")),
			mcp.WithInteger("parallel", mcp.Description("并行 worker 数（可选）")),
			mcp.WithArray("nodes", mcp.Description("输出探针：原始节点 ID 列表，如 [1,2]"), mcp.Items(map[string]any{"type": "integer"})),
			mcp.WithArray("elements", mcp.Description("输出探针：元件实例名列表（输出其电流），如 [\"V1\",\"R1\"]"), mcp.Items(map[string]any{"type": "string"})),
			mcp.WithBoolean("wait", mcp.Description("是否同步等待完成（可选，短电路适用）")),
			mcp.WithNumber("waitTimeout", mcp.Description("同步等待超时（秒，默认 30）")),
		),
		h.handleRunTransient,
	)
	register(
		mcp.NewTool("circuit_run_dc",
			mcp.WithDescription("DC 工作点分析：纯 DC 电路即时收敛到稳态解；含储能元件时返回 t=0 初始解。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithArray("nodes", mcp.Description("输出探针：原始节点 ID 列表"), mcp.Items(map[string]any{"type": "integer"})),
			mcp.WithArray("elements", mcp.Description("输出探针：元件实例名列表"), mcp.Items(map[string]any{"type": "string"})),
			mcp.WithBoolean("wait", mcp.Description("是否同步等待完成（可选）")),
		),
		h.handleRunDC,
	)
	register(
		mcp.NewTool("circuit_run_continuous",
			mcp.WithDescription("启动连续模式仿真（后台，永不自动结束）：配合 circuit_pause / circuit_resume / circuit_step / circuit_advance 逐步驱动，实现「暂停-改元件-步进」交互。用 circuit_cancel_job 结束。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithNumber("initialStep", mcp.Description("初始步长（秒，可选）")),
			mcp.WithNumber("minStep", mcp.Description("最小步长（秒，可选）")),
			mcp.WithNumber("maxStep", mcp.Description("最大步长（秒，可选）")),
			mcp.WithNumber("absTol", mcp.Description("绝对容差（可选）")),
			mcp.WithNumber("relTol", mcp.Description("相对容差（可选）")),
			mcp.WithInteger("maxIter", mcp.Description("最大非线性迭代次数（可选）")),
			mcp.WithInteger("maxSteps", mcp.Description("最大时间步数（可选）")),
			mcp.WithInteger("parallel", mcp.Description("并行 worker 数（可选）")),
			mcp.WithArray("nodes", mcp.Description("输出探针：原始节点 ID 列表，如 [1,2]"), mcp.Items(map[string]any{"type": "integer"})),
			mcp.WithArray("elements", mcp.Description("输出探针：元件实例名列表（输出其电流），如 [\"V1\",\"R1\"]"), mcp.Items(map[string]any{"type": "string"})),
		),
		h.handleRunContinuous,
	)
	register(
		mcp.NewTool("circuit_pause",
			mcp.WithDescription("暂停连续模式仿真（幂等）。暂停后引擎停在每步开头的同步点，可安全调用 circuit_set_element_param 修改元件参数。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("jobId", mcp.Description("任务 ID（可选，默认最近任务）")),
		),
		h.handlePause,
	)
	register(
		mcp.NewTool("circuit_resume",
			mcp.WithDescription("恢复已暂停的连续模式仿真。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("jobId", mcp.Description("任务 ID（可选，默认最近任务）")),
		),
		h.handleResume,
	)
	register(
		mcp.NewTool("circuit_step",
			mcp.WithDescription("单步推进连续模式仿真（每步为引擎的一个自适应时间步，同步等待完成后回到 paused）。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithInteger("count", mcp.Description("步数（可选，默认 1）")),
			mcp.WithString("jobId", mcp.Description("任务 ID（可选，默认最近任务）")),
		),
		h.handleStep,
	)
	register(
		mcp.NewTool("circuit_advance",
			mcp.WithDescription("推进指定时间后自动暂停（连续模式）。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithNumber("duration", mcp.Description("推进时长（秒），必须 > 0"), mcp.Required()),
			mcp.WithBoolean("wait", mcp.Description("是否同步等待推进完成（可选，默认 true）")),
			mcp.WithNumber("waitTimeout", mcp.Description("同步等待超时（秒，可选，默认 30）")),
			mcp.WithString("jobId", mcp.Description("任务 ID（可选，默认最近任务）")),
		),
		h.handleAdvance,
	)
	register(
		mcp.NewTool("circuit_job_status",
			mcp.WithDescription("查询仿真任务状态与进度（步数/当前时间/错误）。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("jobId", mcp.Description("任务 ID（可选，默认最近任务）")),
		),
		h.handleJobStatus,
	)
	register(
		mcp.NewTool("circuit_wait_job",
			mcp.WithDescription("阻塞等待任务完成（超时上限 120 秒）。返回最终状态。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("jobId", mcp.Description("任务 ID（可选，默认最近任务）")),
			mcp.WithNumber("timeoutSeconds", mcp.Description("超时秒数（可选，默认 30，上限 120）")),
		),
		h.handleWaitJob,
	)
	register(
		mcp.NewTool("circuit_cancel_job",
			mcp.WithDescription("取消运行中的任务（引擎优雅停止，保留已完成的采样）。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("jobId", mcp.Description("任务 ID（可选，默认最近任务）")),
		),
		h.handleCancelJob,
	)

	// ---------- E 组 · 结果获取与导出 ----------
	register(
		mcp.NewTool("circuit_get_results",
			mcp.WithDescription("获取任务结果：时间序列（自动抽稀）、探针统计、终态节点电压与元件电流快照。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("jobId", mcp.Description("任务 ID（可选，默认最近任务）")),
			mcp.WithString("format", mcp.Description("输出格式: json / csv（可选，默认 json）")),
			mcp.WithInteger("maxPoints", mcp.Description("抽稀点数上限（可选，默认 10000，0=不抽稀）")),
			mcp.WithArray("probes", mcp.Description("列名过滤（可选），如 [\"node_1\",\"I(R1)\"]"), mcp.Items(map[string]any{"type": "string"})),
		),
		h.handleGetResults,
	)
	register(
		mcp.NewTool("circuit_export_plot",
			mcp.WithDescription("导出自包含 HTML 波形页（内联 Canvas 交互图表，可直接保存为 .html 打开）。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithString("jobId", mcp.Description("任务 ID（可选，默认最近任务）")),
			mcp.WithString("title", mcp.Description("页面标题（可选）")),
		),
		h.handleExportPlot,
	)

	// ---------- F 组 · 稳态分析（AC 相量 / 频率扫描 / 潮流计算） ----------
	register(
		mcp.NewTool("circuit_run_ac",
			mcp.WithDescription("AC 相量分析：对会话网表做线性频域稳态求解（复数 MNA 一次求解），返回节点电压幅值/相位、支路电流与复功率。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithNumber("frequency", mcp.Description("分析频率 Hz（可选，默认取网表 V 源 frequency）")),
			mcp.WithArray("nodes", mcp.Description("输出过滤：原始节点 ID 列表（可选，默认全部）"), mcp.Items(map[string]any{"type": "integer"})),
		),
		h.handleRunAC,
	)
	register(
		mcp.NewTool("circuit_sweep_ac",
			mcp.WithDescription("AC 频率扫描：对数或线性采样 fMin~fMax，返回各节点幅频/相频曲线数据（供 export_plot 复用）。"),
			mcp.WithString("sessionId", mcp.Description("会话 ID"), mcp.Required()),
			mcp.WithNumber("fMin", mcp.Description("起始频率 Hz（>0）"), mcp.Required()),
			mcp.WithNumber("fMax", mcp.Description("终止频率 Hz（>fMin）"), mcp.Required()),
			mcp.WithInteger("points", mcp.Description("采样点数（可选，默认 100，上限 10000）")),
			mcp.WithBoolean("logScale", mcp.Description("对数采样（可选，默认 false=线性）")),
			mcp.WithArray("nodes", mcp.Description("输出过滤：原始节点 ID 列表（可选，默认全部）"), mcp.Items(map[string]any{"type": "integer"})),
		),
		h.handleSweepAC,
	)
	register(
		mcp.NewTool("circuit_run_powerflow",
			mcp.WithDescription("潮流计算：Slack/PV/PQ 母线模型 + 极坐标牛顿-拉夫逊迭代，返回母线电压、发电机注入功率、线路潮流与网损（标幺值）。"),
			mcp.WithArray("buses", mcp.Description("母线数组：[{id, type: slack|pv|pq, v?, theta?, p?, q?, qmin?, qmax?}]（theta 单位为度）"), mcp.Required(),
				mcp.Items(map[string]any{"type": "object"})),
			mcp.WithArray("branches", mcp.Description("支路数组：[{from, to, r, x, b?, tap?}]（标幺）"), mcp.Required(),
				mcp.Items(map[string]any{"type": "object"})),
			mcp.WithNumber("baseMva", mcp.Description("标幺基值（可选，默认 100）")),
			mcp.WithNumber("frequency", mcp.Description("频率 Hz（可选，仅记录）")),
			mcp.WithNumber("tol", mcp.Description("收敛阈值（可选，默认 1e-6）")),
			mcp.WithInteger("maxIter", mcp.Description("最大迭代次数（可选，默认 50）")),
			mcp.WithNumber("damping", mcp.Description("阻尼因子（可选，默认 1.0）")),
			mcp.WithBoolean("flatStart", mcp.Description("平启动（可选，默认 true）")),
		),
		h.handleRunPowerFlow,
	)

	return srv
}
