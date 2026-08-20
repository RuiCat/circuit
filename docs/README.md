# Circuit 文档

Go 实现的泛型电路/系统仿真器，支持电子、液压、气路元件的统一仿真，
以及逻辑电路与潮流计算的联动稳态分析。

## 文档目录

| 文档 | 说明 |
|------|------|
| [architecture.md](architecture.md) | 系统架构与数理基础 — MNA 方程、Newton-Raphson 迭代、Adams 预测-校正法、LU 分解、稳态分析三层、Flags 系统、接口定义、数据流 |
| [netlist-format.md](netlist-format.md) | 网表格式参考 — 语法、元件定义、变量、注释、稳态分析网表子集 |
| [component-reference.md](component-reference.md) | 元件参考手册 — 所有支持的元件类型、引脚、参数、Flags 特性标记 |
| [simulation-guide.md](simulation-guide.md) | 仿真使用指南 — 时域仿真 + 稳态分析 API、Time 接口配置、ConfigFace 接口（含半加器示例） |
| [mcp.md](mcp.md) | MCP 服务器使用文档 — 25 个工具、典型工作流（瞬态/事件/AC/潮流） |
| [powerflow.md](powerflow.md) | 稳态分析指南 — AC 相量 / 小信号混合 / 潮流计算 |
| [powerflow-plan.md](powerflow-plan.md) | 潮流计算落地规划（✅ 已全部实现） |
| [mcp-implementation-plan.md](mcp-implementation-plan.md) | MCP 服务器实现规划（✅ 已全部实现） |

## 快速开始

```bash
# 运行 CLI 仿真主程序（circuit 子命令）
go run ./cmd -t 0.01 cmd/circuits/02_rc_filter.net

# 启动 MCP 服务器（stdio 模式）
go run ./cmd mcpserver

# 稳态分析：AC 相量 / 小信号 / 潮流
go test -v ./analysis/... ./analysis/powerflow/

# CLI 直接运行潮流网表（B/BR 语法, 29 号算例）
go run ./cmd --format table cmd/circuits/29_powerflow_3bus.net

# 运行所有单元测试
go test ./...
```

> 注意：本机 go 缓存目录（`~/.cache/go-build` 等）只读，构建/测试需先设置
> `GOCACHE`/`GOMODCACHE`/`GOPATH` 指向工作区 `.cache/` 目录（已 gitignore）。

## 项目状态

- [x] 泛型矩阵/向量与 LU 分解（支持 float32/64, complex64/128）+ 均衡化
- [x] MNA（改进节点分析）求解器 + 14 种盖章操作 + 复数实例化（NewMnaComplex）
- [x] 基础元件（R, C, L, D, Q, V, I, OpAmp, Gate, Switch, Motor, Transformer, VCVS, DFlipFlop, Sync…）按物理域分 14 目录
- [x] 元件 Flags 位标记系统（Reactive / Nonlinear / CacheStamp）
- [x] ConfigFace / ElementFace / Time / NodeFace 四大核心接口
- [x] 网表解析与加载（含子电路展开 + 节点重编号 + 物理域隔离）
- [x] 瞬态仿真（Newton-Raphson + 3 阶 Adams-Bashforth-Moulton 预测校正 + 自适应步长 + LTE 估计）
- [x] 并行仿真（多线程 DoStep + 盖章缓存优化）
- [x] 逻辑电路仿真（RTL 半加器/全加器/寄存器/CPU 等）
- [x] 事件同步系统（EventSlots + PushEvents/PullEvents，B 开关/RLY 线圈/VR 电阻）
- [x] 稳态分析：AC 相量分析 + 频率扫描（analysis 包）
- [x] 小信号混合分析：DC 工作点 + 二极管/逻辑门线性化
- [x] 潮流计算：Slack/PV/PQ 母线 + 牛顿-拉夫逊（powerflow 包）
- [x] CLI 直接运行 B/BR 潮流网表（csv/tsv/table/html 四格式输出）
- [x] MCP 服务器（25 工具：会话/检视/修改/仿真/导出/稳态分析）
- [ ] UI 图形界面接入仿真器（GUI 已移除，交互界面为 TUI + MCP）

## 许可

MIT License, Copyright (c) 2025 RuiCat
