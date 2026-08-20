# MCP 服务器使用文档

`circuit` 项目提供 MCP (Model Context Protocol) 服务器，使 LLM 客户端（Claude Desktop、
Cursor、dsh 等）可以直接进行电路仿真：加载网表、检视电路、修改参数、异步运行仿真、
获取结果与导出波形图。

## 启动

```bash
# stdio 模式（默认，供 Claude Desktop / dsh 等本地客户端）
go run ./cmd mcpserver

# streamable HTTP 模式（远程接入）
go run ./cmd mcpserver -transport http -addr :18080

# SSE 模式
go run ./cmd mcpserver -transport sse -addr :18080

# 会话空闲 30 分钟自动清理
go run ./cmd mcpserver -session-ttl 30m
```

### Claude Desktop 配置示例

```json
{
  "mcpServers": {
    "circuit": {
      "command": "/path/to/circuit",
      "args": ["mcpserver"]
    }
  }
}
```

## 工具列表（25 个）

### A 组 · 会话与网表管理

| 工具 | 功能 | 关键参数 |
|---|---|---|
| `circuit_new_session` | 创建仿真会话（可带初始网表） | `netlist?`, `name?` |
| `circuit_load_netlist` | 加载/替换会话网表（失败回滚） | `sessionId`, `netlist` |
| `circuit_validate_netlist` | 只校验不建会话 | `netlist` |
| `circuit_list_sessions` | 列出会话及元信息 | — |
| `circuit_delete_session` | 删除会话（先取消运行中任务） | `sessionId` |

### B 组 · 元件目录与电路检视

| 工具 | 功能 | 关键参数 |
|---|---|---|
| `circuit_list_component_types` | 全部已注册元件类型目录 | `domain?` |
| `circuit_component_help` | 单个元件类型完整说明 | `type` |
| `circuit_list_elements` | 会话内元件清单（参数/电流） | `sessionId`, `type?`, `instance?` |
| `circuit_list_nodes` | 节点清单（域/单位/连接元件/电压） | `sessionId` |
| `circuit_get_element` | 单个元件完整状态 | `sessionId`, `instance` |
| `circuit_get_node_values` | 读节点电压/压力（支持层级路径） | `sessionId`, `nodes[]?`, `paths[]?` |

### C 组 · 电路修改

| 工具 | 功能 | 关键参数 |
|---|---|---|
| `circuit_set_element_param` | 修改元件参数（类型校验） | `sessionId`, `instance`, `param`, `value` |
| `circuit_set_event` | 设置事件值（运行中可用） | `sessionId`, `event`, `value` |
| `circuit_set_sim_params` | 设置默认仿真参数 | `sessionId`, 各数值字段 |
| `circuit_set_trigger` | 设置时间触发点 | `sessionId`, `triggers[]` |

### D 组 · 仿真执行（异步 Job）

| 工具 | 功能 | 关键参数 |
|---|---|---|
| `circuit_run_transient` | 启动瞬态仿真（后台） | `sessionId`, `targetTime`, 探针/容差等 |
| `circuit_run_dc` | DC 工作点分析 | `sessionId`, `nodes[]?`, `elements[]?` |
| `circuit_job_status` | 查询任务进度 | `sessionId`, `jobId?` |
| `circuit_wait_job` | 阻塞等待完成（上限 120s） | `sessionId`, `jobId?`, `timeoutSeconds?` |
| `circuit_cancel_job` | 取消任务（引擎优雅停止） | `sessionId`, `jobId?` |

### E 组 · 结果获取与导出

| 工具 | 功能 | 关键参数 |
|---|---|---|
| `circuit_get_results` | 结果时间序列（抽稀）/CSV/终值快照 | `sessionId`, `jobId?`, `format?`, `maxPoints?`, `probes[]?` |
| `circuit_export_plot` | 自包含 HTML 波形页 | `sessionId`, `jobId?`, `title?` |

### F 组 · 稳态分析（AC 相量 / 频率扫描 / 潮流计算）

| 工具 | 功能 | 关键参数 |
|---|---|---|
| `circuit_run_ac` | 线性 AC 相量分析（复数 MNA 一次求解） | `sessionId`, `frequency?`, `nodes[]?` |
| `circuit_sweep_ac` | AC 频率扫描（对数/线性） | `sessionId`, `fMin`, `fMax`, `points?`, `logScale?`, `nodes[]?` |
| `circuit_run_powerflow` | 潮流计算（牛顿-拉夫逊，标幺值） | `buses[]`, `branches[]`, `tol?`, `maxIter?` 等 |

稳态分析为**秒级同步瞬算**（不占 jobs 异步框架），详细使用见 [powerflow.md](powerflow.md)。

## 典型工作流

### 1. RC 电路瞬态仿真

```
circuit_new_session
  netlist: V1 [1,-1] [0,0,0,0,5]
           R1 [1,2] [1000]
           C1 [2,-1] [1e-6]

circuit_run_transient  sessionId=s1  targetTime=0.003  nodes=[2]
circuit_wait_job       sessionId=s1
circuit_get_results    sessionId=s1        # time/series/probes/finalV/finalI
circuit_export_plot    sessionId=s1        # HTML 波形页
```

### 2. 修改参数后重跑

```
circuit_set_element_param  sessionId=s1  instance=R1  param=R  value=2000
circuit_run_transient      sessionId=s1  targetTime=0.003  nodes=[2]
```

### 3. 事件驱动开关（运行中注入）

事件类元件（EventSwitch `B`、MomentarySwitch `MB`）由事件驱动。注意：
**引擎在每次仿真启动时清空事件表，事件必须在仿真运行期间设置**（与 TUI 行为一致）：

```
circuit_run_transient  sessionId=s1  targetTime=0.05  maxStep=1e-6
circuit_set_event      sessionId=s1  event=SB1  value=1    # 运行中注入
circuit_wait_job       sessionId=s1
```

### 4. 子电路与层级节点

```
.subckt div in out
  R1 [in,out] [1000]
  R2 [out,-1] [1000]
.ends div
V1 [1,-1] [0,0,0,0,5]
X1 [1,2] div          # 注意：子电路名不带方括号
```

查询子电路内部节点：`circuit_get_node_values` 的 `paths` 参数传 `["X1.out"]`。

### 5. AC 相量分析（RC 低通）

```
circuit_new_session
  netlist: V1 [1,-1] [1,0,0,0,1]    # waveform=1(AC): 1V∠0°
           R1 [1,2] [1000]
           C1 [2,-1] [100e-9]

circuit_run_ac     sessionId=s1  frequency=1591.55   # f0=1/(2πRC)
# → nodeVoltages[2] ≈ {mag:0.707, phaseDeg:-45} (-3dB / -45°)

circuit_sweep_ac   sessionId=s1  fMin=100  fMax=10000  points=200  logScale=true
# → frequencies[] + 各节点 mag/phase 曲线
```

### 6. 潮流计算（2 母线）

```
circuit_run_powerflow
  buses:    [{id:1, type:"slack", v:1.0},
             {id:2, type:"pq", p:0.5, q:0.2}]
  branches: [{from:1, to:2, r:0, x:0.1}]
# → converged:true, busVoltages[2] ≈ {mag:1.0184, phaseDeg:2.814}
#   genPower[1] ≈ {p:-0.5, q:-0.172}, totalLoss ≈ {p:0, q:0.028}
```

## 注意事项

- **地节点是 `-1`**（不是 0）：`V1 [1,-1] ...`。节点 0 是普通节点，电路必须接地否则矩阵奇异。
- 仿真为**异步任务模型**：`run_transient` 立即返回 `jobId`，用 `job_status`/`wait_job` 跟进；
  同一会话同时只能有一个运行中任务。
- 结果默认**抽稀到 10000 点**（`maxPoints` 可调），探针 `nodes`/`elements` 可过滤输出列。
- 结果列名：节点电压 `node_<原始ID>`；电流 `I(<实例名>)` 或 `I(<实例名>.<槽名>)`。
- 元件参数按**参数名**（如电阻 `R`、电容 `C`）或索引修改，值类型须与参数一致。
- 会话空闲默认 1 小时自动清理（`-session-ttl` 可调），或手动 `circuit_delete_session`。
- **AC/潮流限制**：AC 分析仅支持 R/C/L/V/I（小信号另支持 D/U），不支持子电路；
  小信号工作点要求纯 DC 电路（无 C/L）；潮流母线为标幺值、`theta` 按度接收。
