# MCP 服务器实现规划（电路仿真）

> 状态：✅ 已实施完成（P0–P6 全部落地，2026-08-19）· 使用文档见 [docs/mcp.md](mcp.md)

## 1. 目标

为 `circuit` 项目新增一个 MCP（Model Context Protocol）服务器，使任何 MCP 客户端
（Claude Desktop、Cursor、dsh 等）可以：

- 用网表文本描述电路并校验；
- 检视元件目录、电路元件与节点、读取电压/电流/压力；
- 修改元件参数、设置事件（驱动开关类元件）；
- 异步运行瞬态/DC 仿真，查询进度，获取结果（JSON/CSV/自包含 HTML 波形页）。

## 2. 现状分析（仿真引擎能力盘点）

| 能力 | 现状入口 | 备注 |
|---|---|---|
| 网表解析 | `load.LoadString/LoadContext` | SPICE 风格，支持子电路、总线展开、`.value` 变量；已带 100MB 大小上限 |
| 元件注册 | `element/register`（一个空导入注册全部 15 类元件包） | 强弱电/气路/油路/逻辑/传感等 |
| 元件检视 | `element.Config`（引脚、参数名/初值、电压源、内部节点、特性位） | `ValueName` 提供参数名 |
| 瞬态仿真 | `time.NewTimeMNA` + `time.TransientSimulation(con, call)` | 回调每步一次；自适应步长 |
| 连续/暂停/单步 | `TransientSimulationContinuous`、`TimeMNA.Stop()/Status()` | 引擎原生支持 Stopped/Running/Stepping/Paused 状态机，`PushEvents` 每步检查状态 |
| 事件驱动 | `con.SetEvent(name, value)`（goroutine 安全） | 驱动 EventSwitch/MomentarySwitch 等 |
| 结果输出 | `cmd/main.go` 列构建 + `cmd/web.go` 的 `writeHTML`（package main，需重构复用） | HTML 自包含波形页 |
| 电压/电流读取 | `con.GetRawNodeVoltage`、`con.GetVoltageSourceCurrent`、`elem.GetFloat64` | |

结论：引擎能力完备，MCP 层主要是**包装 + 会话/任务管理**，无需改动仿真内核。

## 3. 总体架构

```
MCP 客户端 (Claude Desktop / dsh / cursor ...)
        │  stdio (默认) / SSE / streamable HTTP (可选)
        ▼
cmd/main.go  ── 入口：`circuit mcpserver` 子命令（合并后单一入口）
        │
mcp 包（独立库，不依赖 cmd）
  ├─ server.go    MCP Server 装配：注册全部工具、资源、提示词；传输启动
  ├─ session.go   会话存储：map[sessionID]*Session + RWMutex + 空闲 TTL 清理
  ├─ engine.go    网表加载 → Context 构建 → 参数应用 → 仿真编排
  ├─ jobs.go      异步任务引擎：状态机 + 取消 + 结果缓存 + 抽稀采样
  ├─ results.go   结果编码（JSON/CSV）、探针定义、HTML 波形页导出
  └─ tools_*.go   工具处理器（按组拆分文件）
```

关键设计决策：

1. **异步任务（Job）模型**：仿真可能耗时数十秒，MCP 工具调用不应阻塞。
   `run_transient` 立即返回 `job_id`，后台 goroutine 执行；
   配套 `job_status / wait_job / cancel_job / get_results`。
   取消通过引擎原生 `TimeMNA.Stop()`（状态原子存储，主循环优雅退出），
   无需 kill goroutine。
2. **每会话串行仿真**：一个会话同一时刻最多一个运行中 job（互斥），保证确定性；
   多个会话可并行（SessionStore 并发安全）。
3. **探针（Probe）模型**：结果只采样用户声明的节点（原始节点 ID）
   与元件（实例名 → 电流/流量），支持抽稀（max_points 上限，默认 10k 点）
   控制 token 消耗与回包大小。
4. **修改时机约束**：`set_element_param` 等修改类工具仅允许在没有运行中 job 时
   调用（返回明确错误），避免仿真中篡改矩阵状态；`set_event` 例外——引擎文档
   声明其 goroutine 安全，可在运行中调用（与 TUI 行为一致）。
5. **纯函数处理器**：每个工具处理器为 `(args, server) → (result, error)` 纯函数，
   便于单元测试与并发安全审查。

## 4. MCP SDK 选型

| 方案 | 说明 | 结论 |
|---|---|---|
| [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)（推荐） | Go 生态最成熟（v0.39+，活跃维护），支持 stdio / SSE / streamable HTTP，工具、资源、提示词、日志、采样齐备，文档完善 | ✅ 采用 |
| modelcontextprotocol/go-sdk（官方） | 官方 SDK，较新，API 仍在演进，示例较少 | 备选 |

> go.mod 当前 `go 1.24.6`，满足 mcp-go 要求。需新增依赖 `github.com/mark3labs/mcp-go`。

## 5. 会话与任务状态机

```
会话 Session { id, name, netlist, *element.Context, simCfg, jobs, createdAt, lastActiveAt }
  ├─ 创建: new_session（可带初始网表） / load_netlist（替换，重建 Context）
  ├─ 存活: 空闲 TTL（默认 1h，flag 可调），delete_session 立即释放
  └─ 仿真: 每会话同时最多 1 个 job

Job { id, kind(transient|dc|continuous|step), state, steps, currentTime,
      finalTime, error, resultRef }
  state: pending → running → done | error | cancelled
  cancel: 调用 con.Time.Stop()，主循环优雅退出 → cancelled/done(部分结果)
```

## 6. 完整工具列表

### A 组 · 会话与网表管理

| # | 工具名 | 功能 | 关键参数 | 返回 |
|---|---|---|---|---|
| 1 | `circuit_new_session` | 创建仿真会话（可带初始网表） | `netlist?`, `name?` | `session_id`、元件/节点数 |
| 2 | `circuit_load_netlist` | 加载/替换会话网表（重建 Context，失败回滚旧网表） | `session_id`, `netlist` | 元件/节点/子电路统计 |
| 3 | `circuit_validate_netlist` | 只校验不建会话（语法+语义+跨域混接检查） | `netlist` | 错误列表/警告/统计 |
| 4 | `circuit_list_sessions` | 列出会话及元信息 | — | 会话列表（ID/名称/元件/节点/最后活动） |
| 5 | `circuit_delete_session` | 删除会话，释放资源（运行中 job 先取消） | `session_id` | 删除结果 |

### B 组 · 元件目录与电路检视

| # | 工具名 | 功能 | 关键参数 | 返回 |
|---|---|---|---|---|
| 6 | `circuit_list_component_types` | 全部已注册元件类型目录（名称/NodeType/引脚/参数/特性位） | `domain?`(electrical/pneumatic/hydraulic/...) | 类型列表 |
| 7 | `circuit_component_help` | 单个元件类型的完整说明（引脚名/类型、参数名+默认值、电压源、内部节点、网表语法示例） | `type` | 结构化说明 |
| 8 | `circuit_list_elements` | 会话内元件清单（实例名/类型/引脚→原始节点/参数值/当前电流） | `session_id`, `type?`, `instance?` | 元件数组 |
| 9 | `circuit_list_nodes` | 会话内节点清单（原始ID/紧凑索引/域类型/连接的元件列表） | `session_id` | 节点数组 |
| 10 | `circuit_get_element` | 单个元件详情：全部参数（类型化）、内部节点、电压源、层级父/子 | `session_id`, `instance` | 完整状态 |
| 11 | `circuit_get_node_values` | 读取节点电压/压力（可同时查多个，支持层级路径 `X1.out`） | `session_id`, `nodes[]`, `paths[]?` | 节点→值(带单位) |

### C 组 · 电路修改

| # | 工具名 | 功能 | 关键参数 | 返回 |
|---|---|---|---|---|
| 12 | `circuit_set_element_param` | 按实例名+参数名/索引改参数，类型校验（float/int/bool/string） | `session_id`, `instance`, `param`, `value` | 旧值→新值 |
| 13 | `circuit_set_event` | 设置事件值（运行中可用，驱动事件开关等） | `session_id`, `event`, `value` | 当前事件表 |
| 14 | `circuit_set_sim_params` | 设置仿真默认参数（容差/步长限制/迭代上限/最大步数/初始步长） | `session_id`, 各数值字段 | 生效后的参数表 |
| 15 | `circuit_set_trigger` | 管理时间触发点（时间点列表，步长自动截断对齐） | `session_id`, `triggers[]` | 触发点列表 |

### D 组 · 仿真执行（异步 Job）

| # | 工具名 | 功能 | 关键参数 | 返回 |
|---|---|---|---|---|
| 16 | `circuit_run_transient` | 启动瞬态仿真（后台执行） | `session_id`, `target_time`, 步长/容差?, `nodes[]?`, `elements[]?`（探针）, `max_points?`, `wait?` | `job_id`+状态 |
| 17 | `circuit_run_dc` | DC 工作点分析（纯 DC 电路取 t=0 解；返回全部节点电压+元件电流） | `session_id` | 工作点表 |
| 18 | `circuit_job_status` | 查询任务进度 | `session_id`, `job_id` | 状态/步数/当前时间/错误 |
| 19 | `circuit_wait_job` | 阻塞等待完成（超时上限，如 120s） | `session_id`, `job_id`, `timeout_seconds?` | 同 status+摘要 |
| 20 | `circuit_cancel_job` | 取消运行中任务（`TimeMNA.Stop()` 优雅退出） | `session_id`, `job_id` | 取消结果 |

### E 组 · 结果获取与导出

| # | 工具名 | 功能 | 关键参数 | 返回 |
|---|---|---|---|---|
| 21 | `circuit_get_results` | 获取已完成任务的结果：时间序列（抽稀）、末值、统计 | `session_id`, `job_id?`(默认最近), `format`(json/csv), `max_points?`, `probes?` | 结果数据 |
| 22 | `circuit_export_plot` | 导出自包含 HTML 波形页（复用 `cmd/web.go` 模板，重构为共享包） | `session_id`, `job_id?`, `title?` | HTML 文本 |

### F 组 · Phase 2 高级能力（后续迭代）

| # | 工具名 | 功能 | 说明 |
|---|---|---|---|
| 23 | `circuit_add_element` / `circuit_remove_element` | 会话内动态增删元件 | 加载器已支持动态元件，需处理节点重编号与 stamp 失效 |
| 24 | `circuit_step` / `circuit_continuous_start` / `circuit_continuous_stop` | 单步/连续仿真控制 | 引擎原生 Stepping/Paused 状态机已具备 |
| 25 | MCP Resources | `circuit://sessions/{id}`、`circuit://components/{name}` | 网表文本、元件说明可读资源 |
| 26 | MCP Prompts | 「设计 RC 低通」「分压器分析」「运放跟随器」等模板 | 引导式分析模板 |

## 7. 目录结构（新增/修改）

```
mcp/                      （新增包）
  server.go
  session.go
  engine.go
  jobs.go
  results.go
  tools_session.go        A 组
  tools_inspect.go        B 组
  tools_modify.go         C 组
  tools_simulate.go       D 组
  tools_export.go         E 组
  session_test.go / jobs_test.go / tools_*_test.go
  integration_test.go     mcp-go client 端到端 stdio 测试
（已合并：mcpserver 作为 cmd/main.go 的子命令）
webout/webout.go          （新增共享包：从 cmd/web.go 提取 writeHTML/webMeta，cmd 与 mcp 共用）
cmd/main.go               （修改：改用 webout 包，删除重复代码）
docs/mcp.md               （新增：MCP 使用文档）
README.md                 （修改：增加 MCP 章节）
```

## 8. 实施步骤

| 阶段 | 内容 | 验收 |
|---|---|---|
| P0 骨架 | 引入 mcp-go；`circuit mcpserver` 可启动 stdio 服务 | 客户端能连接并列出工具 |
| P1 会话 | A 组 5 工具 + session store + TTL | 会话创建/替换/校验/删除单测通过 |
| P2 检视 | B 组 6 工具（元件目录来自 `element.ElementListName` 全局表） | 能查目录/元件/节点/电压 |
| P3 修改 | C 组 4 工具（参数类型校验、事件、仿真参数、触发点） | 改参数后仿真结果变化可验证 |
| P4 仿真 | Job 引擎 + D 组 5 工具 + E 组 `get_results` | RC 电路全流程：加载→仿真→取数，数值与解析解比对 |
| P5 导出 | 重构 `cmd/web.go` → `webout` 包；`circuit_export_plot`；cmd 改用共享包 | HTML 波形页可用 |
| P6 收尾 | 集成测试（并发两会话）、`docs/mcp.md`、README 更新 | 全测试通过 |
| P7 可选 | F 组：动态元件、单步/连续、Resources/Prompts、SSE/HTTP 传输 | 按需 |

## 9. 测试策略

- **单元测试**：工具处理器为纯函数，直接调用断言（含非法参数、类型校验、跨域网表报错）。
- **数值校验**：RC 充放电（1kΩ/1µF，τ=1ms）解析解比对；分压器 DC 工作点比对。
- **集成测试**：用 mcp-go client 启动 stdio 服务器走完整流程
  （new_session → load_netlist → run_transient → wait_job → get_results → export_plot）。
- **并发测试**：两个会话并行仿真互不干扰；运行中 cancel 优雅退出。

## 10. 风险与对策

| 风险 | 对策 |
|---|---|
| 仿真耗时超客户端耐心 | 异步 Job + wait 超时上限 + cancel；`max_steps` 硬上限 |
| 回包过大（大量时间点） | 抽稀采样 max_points 默认 10k；probes 过滤；CSV 按需 |
| 修改元件参数导致不收敛 | 修改仅在无运行 job 时允许；参数范围校验（阻值>0 等） |
| mcp-go API 演进 | 锁定版本；封装层隔离，仅 server.go 触碰 SDK |
| 并发数据竞争 | 会话级互斥 + 纯函数处理器 + 竞态检测测试（`-race`） |

## 11. 待确认决策

1. SDK：采用 mark3labs/mcp-go（推荐）还是官方 go-sdk？ → **已确认：mark3labs/mcp-go v0.58.0**
2. 首期范围：P0–P6（A–E 组 22 个工具）全做，还是先做核心子集？ → **已确认：P0–P6 全部 22 个工具**
3. 传输方式：仅 stdio，还是同时支持 SSE/streamable HTTP？ → **已确认：stdio + HTTP + SSE 三传输**

## 12. 实施记录

- 新增 `mcp/` 包（server/session/engine/jobs/results/elementref + 5 个工具文件 + 4 个测试文件）
- MCP 服务器作为 `circuit mcpserver` 子命令并入 cmd/main.go（stdio/http/sse 三传输，`-session-ttl` 会话清理）
- 重构 `cmd/web.go` → `webout/` 共享包（HTML 波形页模板，cmd 与 MCP 导出共用）
- 新增 `docs/mcp.md` 使用文档；README 增加 MCP 章节
- 测试：单元测试（会话/工具/数值校验）+ stdio 端到端集成测试（22 工具协议级验证）+ `-race` 并发测试
- 并发设计：会话级互斥 + 异步 Job（引擎 `TimeMNA.Stop()` 优雅取消）+ 运行中电压快照（仿真协程内录制，避免与无锁引擎读竞争）；运行中禁止元件检视/参数修改
- 注意事项（测试中确认）：GND 为 -1；X 子电路实例语法为 `X1 [1,2] div`（子电路名不带方括号，docs/netlist-format.md 示例有误）；事件需在仿真运行中注入（MarkReset 会清空事件表）
